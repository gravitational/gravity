/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
)

// New returns a new instance of the unstarted installer server
func New(ctx context.Context, cancel context.CancelFunc, config Config) (*Installer, error) {
	err := upsertSystemAccount(ctx, config.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	listener, err := net.Listen("unix", filepath.Join(config.StateDir, "installer.sock"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	grpcServer := grpc.NewServer()
	installer := &Installer{
		Config: config,
		cancel: cancel,
		rpc:    grpcServer,
		// TODO(dmitri): arbitrary channel buffer size
		eventsC:  make(chan Event, 100),
		listener: listener,
	}
	installpb.RegisterAgentServer(grpcServer, installer)
	return installer, nil
}

// Start starts the server using the specified engine
func (i *Installer) Start(engine Engine) {
	i.engine = engine
	go i.rpc.Serve(i.listener)
}

// Stop releases resources allocated by the installer
func (i *Installer) Stop(ctx context.Context) error {
	if err := i.listener.Close(); err != nil {
		i.WithError(err).Warn("Failed to close listener.")
	}
	var errors []error
	for _, c := range i.closers {
		if err := c.Close(); err != nil {
			errors = append(errors, err)
		}
	}
	i.Config.Process.Shutdown(ctx)
	return trace.NewAggregate(errors...)
}

// Interface defines the interface of the installer as presented
// to an engine
type Interface interface {
	PlanBuilderGetter
	// NotifyOperationAvailable is invoked by the engine to notify the server
	// that the operation has been created
	NotifyOperationAvailable(context.Context, ops.SiteOperationKey)
	// NewAgent returns a new unstarted installer agent.
	// Call agent.Serve() on the resulting instance to start agent's service loop
	NewAgent(url string) (rpcserver.Server, error)
	// Finalize executes additional steps common to all workflows after the
	// installation has completed
	Finalize(ctx context.Context, operation ops.SiteOperation) error
	// CompleteFinalInstallStep marks the final install step as completed unless
	// the application has a custom install step. In case of the custom step,
	// the user completes the final installer step
	CompleteFinalInstallStep(delay time.Duration) error
	// Wait blocks the installer until the wizard process has been explicitly shut down
	// or specified context has expired
	Wait(context.Context) error
	// PrintStep publishes a progress entry described with (format, args)
	PrintStep(ctx context.Context, format string, args ...interface{}) error
}

// NotifyOperationAvailable is invoked by the engine to notify the server
// that the operation has been created
func (i *Installer) NotifyOperationAvailable(ctx context.Context, operationKey ops.SiteOperationKey) {
	i.addCloser(CloserFunc(func() error {
		return trace.Wrap(i.Process.AgentService().StopAgents(ctx, operationKey))
	}))
	go i.runProgressLoop(ctx, operationKey)
}

// NewAgent creates a new installer agent
func (i *Installer) NewAgent(agentURL string) (rpcserver.Server, error) {
	listener, err := net.Listen("tcp", defaults.GravityRPCAgentAddr(i.AdvertiseAddr))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverCreds, clientCreds, err := rpc.Credentials(defaults.RPCAgentSecretsDir)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	var mounts []*pb.Mount
	for name, source := range i.Mounts {
		mounts = append(mounts, &pb.Mount{Name: name, Source: source})
	}
	runtimeConfig := pb.RuntimeConfig{
		SystemDevice: i.SystemDevice,
		DockerDevice: i.DockerDevice,
		Role:         i.Role,
		Mounts:       mounts,
	}
	if err = FetchCloudMetadata(i.CloudProvider, &runtimeConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	config := rpcserver.PeerConfig{
		Config: rpcserver.Config{
			Listener: listener,
			Credentials: rpcserver.Credentials{
				Server: serverCreds,
				Client: clientCreds,
			},
			RuntimeConfig: runtimeConfig,
		},
	}
	agent, err := NewAgentFromURL(agentURL, config, i.FieldLogger)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

// Finalize executes additional steps after the installation has completed
func (i *Installer) Finalize(ctx context.Context, operation ops.SiteOperation) error {
	var errors []error
	if err := i.uploadInstallLog(operation.Key()); err != nil {
		errors = append(errors, err)
	}
	if err := i.emitAuditEvents(ctx, operation); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// CompleteFinalInstallStep marks the final install step as completed unless
// the application has a custom install step - in which case it does nothing
// because it will be completed by user later
func (i *Installer) CompleteFinalInstallStep(delay time.Duration) error {
	req := ops.CompleteFinalInstallStepRequest{
		AccountID:           defaults.SystemAccountID,
		SiteDomain:          i.SiteDomain,
		WizardConnectionTTL: delay,
	}
	i.WithField("req", req).Debug("Completing final install step.")
	if err := i.Operator.CompleteFinalInstallStep(req); err != nil {
		return trace.Wrap(err, "failed to complete final install step")
	}
	return nil
}

// PrintStep publishes a progress entry described with (format, args) tuple to the client
func (i *Installer) PrintStep(ctx context.Context, format string, args ...interface{}) error {
	message := fmt.Sprintf("%v\t%v\n", time.Now().UTC().Format(constants.HumanDateFormatSeconds),
		fmt.Sprintf(format, args...))
	event := Event{Progress: &ops.ProgressEntry{Message: message}}
	return i.send(ctx, event)
}

func (i *Installer) Validate(ctx context.Context, req *installpb.ValidateRequest) (*installpb.ValidateResponse, error) {
	// TODO
	return &installpb.ValidateResponse{}, nil
}

// Execute executes the installation using the specified engine
// Implements installpb.AgentServer
func (i *Installer) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	if err := i.engine.Validate(stream.Context(), i.Config); err != nil {
		return trace.Wrap(err)
	}
	go i.execute(stream.Context())
	for {
		select {
		case event := <-i.eventsC:
			resp := &installpb.ProgressResponse{}
			if event.Progress != nil {
				resp.Message = event.Progress.Message
			} else if event.Error != nil {
				resp.Errors = append(resp.Errors, &installpb.Error{Message: event.Error.Error()})
			}
			err := stream.Send(resp)
			if err != nil {
				return trace.Wrap(err)
			}
		case <-stream.Context().Done():
			return trace.Wrap(stream.Context().Err())
		}
	}
	return nil
}

// Shutdown shuts down the installer.
// Implements installpb.AgentServer
func (i *Installer) Shutdown(ctx context.Context, req *installpb.ShutdownRequest) (*installpb.ShutdownResponse, error) {
	i.cancel()
	return &installpb.ShutdownResponse{}, nil
}

// GetProgress streams installer progress to the client.
// Implements installpb.AgentServer
func (i *Installer) GetProgress(req *installpb.ProgressRequest, stream installpb.Agent_GetProgressServer) error {
	select {
	case event := <-i.eventsC:
		resp := &installpb.ProgressResponse{}
		if event.Progress != nil {
			resp.Message = event.Progress.Message
		} else if event.Error != nil {
			resp.Errors = append(resp.Errors, &installpb.Error{Message: event.Error.Error()})
		}
		err := stream.Send(resp)
		if err != nil {
			return trace.Wrap(err)
		}
	case <-stream.Context().Done():
		return trace.Wrap(stream.Context().Err())
	}
	return nil
}

// Wait blocks until either the context has been cancelled or the wizard process
// exits with an error
func (i *Installer) Wait(ctx context.Context) error {
	errC := make(chan error, 1)
	go func() {
		errC <- i.Process.Wait()
	}()
	select {
	case err := <-errC:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// NewStateMachine returns a new instance of the installer state machine.
// Implements engine.StateMachineFactory
func (i *Installer) NewStateMachine(operator ops.Operator, operationKey ops.SiteOperationKey) (fsm *fsm.FSM, err error) {
	config := FSMConfig{
		Operator:           operator,
		OperationKey:       operationKey,
		Packages:           i.Packages,
		Apps:               i.Apps,
		LocalPackages:      i.LocalPackages,
		LocalApps:          i.LocalApps,
		LocalBackend:       i.LocalBackend,
		LocalClusterClient: i.LocalClusterClient,
		Insecure:           i.Insecure,
		UserLogFile:        i.UserLogFile,
		ReportProgress:     true,
	}
	config.Spec = FSMSpec(config)
	return NewFSM(config)
}

// NewCluster returns a new request to create a cluster.
// Implements engine.ClusterFactory
func (i *Installer) NewCluster() ops.NewSiteRequest {
	return ops.NewSiteRequest{
		AppPackage:   i.AppPackage.String(),
		AccountID:    defaults.SystemAccountID,
		Email:        fmt.Sprintf("installer@%v", i.SiteDomain),
		Provider:     i.CloudProvider,
		DomainName:   i.SiteDomain,
		InstallToken: i.Token.Token,
		ServiceUser: storage.OSUser{
			Name: i.ServiceUser.Name,
			UID:  strconv.Itoa(i.ServiceUser.UID),
			GID:  strconv.Itoa(i.ServiceUser.GID),
		},
		CloudConfig: storage.CloudConfig{
			GCENodeTags: i.GCENodeTags,
		},
		DNSOverrides: i.DNSOverrides,
		DNSConfig:    i.DNSConfig,
		Docker:       i.Docker,
	}
}

func (i *Installer) execute(ctx context.Context) error {
	if err := i.bootstrap(); err != nil {
		return trace.Wrap(err)
	}
	err := i.engine.Execute(ctx, i, i.Config)
	if err != nil {
		i.sendError(ctx, err)
		return trace.Wrap(err)
	}
	i.printPostInstallBanner(ctx)
	return nil
}

func (i *Installer) bootstrap() error {
	if err := i.upsertAdminAgent(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (i *Installer) runProgressLoop(ctx context.Context, operationKey ops.SiteOperationKey) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var lastProgress *ops.ProgressEntry
	for {
		select {
		case <-ticker.C:
			if progress := i.updateProgress(ctx, operationKey, lastProgress); progress != nil {
				lastProgress = progress
			}
		case <-ctx.Done():
			return
		}
	}
}

func (i *Installer) updateProgress(ctx context.Context, operationKey ops.SiteOperationKey, lastProgress *ops.ProgressEntry) *ops.ProgressEntry {
	progress, err := i.Operator.GetSiteOperationProgress(operationKey)
	if err != nil {
		i.WithError(err).Warn("Failed to query operation progress.")
		return nil
	}
	if lastProgress != nil && lastProgress.IsEqual(*progress) {
		return nil
	}
	message := fmt.Sprintf("%v\t%v\n", time.Now().UTC().Format(constants.HumanDateFormatSeconds),
		progress.Message)
	event := Event{Progress: &ops.ProgressEntry{Message: message}}
	i.send(ctx, event)
	return progress
}

func (i *Installer) printPostInstallBanner(ctx context.Context) {
	var buf bytes.Buffer
	i.printEndpoints(ctx, &buf)
	if m, ok := modules.Get().(modules.Messager); ok {
		fmt.Fprintf(&buf, "\n%v\n", m.PostInstallMessage())
	}
	i.Printer.Print(buf.String())
	event := Event{Progress: &ops.ProgressEntry{
		Message:    buf.String(),
		Completion: constants.Completed,
	}}
	i.send(ctx, event)
}

func (i *Installer) sendError(ctx context.Context, err error) error {
	return trace.Wrap(i.send(ctx, Event{Error: err}))
}

// send streams the specified progress event to the client.
// The method will not block - event will be dropped if it cannot be published
// (subject to internal channel buffer capacity)
func (i *Installer) send(ctx context.Context, event Event) error {
	select {
	case i.eventsC <- event:
		// Pushed the progress event
		return nil
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	default:
		log.WithField("event", event).Warn("Failed to publish event.")
		return nil
	}
}

func (i *Installer) printEndpoints(ctx context.Context, w io.Writer) {
	status, err := i.getClusterStatus(ctx)
	if err != nil {
		i.WithError(err).Error("Failed to collect cluster status.")
		return
	}
	fmt.Fprintln(w)
	status.Cluster.Endpoints.Cluster.WriteTo(w)
	fmt.Fprintln(w)
	status.Cluster.Endpoints.Applications.WriteTo(w)
}

// getClusterStatus collects status of the installer cluster.
func (i *Installer) getClusterStatus(ctx context.Context) (*status.Status, error) {
	clusterOperator, err := localenv.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := clusterOperator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	status, err := status.FromCluster(ctx, clusterOperator, *cluster, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return status, nil
}

// upsertAdminAgent creates an admin agent for the cluster being installed
func (i *Installer) upsertAdminAgent() error {
	agent, err := i.Process.UsersService().CreateClusterAdminAgent(i.SiteDomain,
		storage.NewUser(storage.ClusterAdminAgent(i.SiteDomain), storage.UserSpecV2{
			AccountID: defaults.SystemAccountID,
		}))
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	i.WithField("agent", agent).Info("Created cluster agent.")
	return nil
}

// uploadInstallLog uploads user-facing operation log to the installed cluster
func (i *Installer) uploadInstallLog(operationKey ops.SiteOperationKey) error {
	file, err := os.Open(i.UserLogFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	err = i.Operator.StreamOperationLogs(operationKey, file)
	if err != nil {
		return trace.Wrap(err, "failed to upload install log")
	}
	i.Debug("Uploaded install log to the cluster.")
	return nil
}

// emitAuditEvents sends the install operation's start/finish
// events to the installed cluster's audit log.
func (i *Installer) emitAuditEvents(ctx context.Context, operation ops.SiteOperation) error {
	operator, err := localenv.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	fields := events.FieldsForOperation(operation)
	events.Emit(ctx, operator, events.OperationStarted, fields.WithField(
		events.FieldTime, operation.Created))
	events.Emit(ctx, operator, events.OperationCompleted, fields)
	return nil
}

func (i *Installer) addCloser(closer io.Closer) {
	i.closers = append(i.closers, closer)
}

// Installer manages the installation process
type Installer struct {
	// Config specifies the configuration for the install operation
	Config
	closers  []io.Closer
	cancel   context.CancelFunc
	eventsC  chan Event
	listener net.Listener
	rpc      *grpc.Server
	engine   Engine
}

// Engine implements the process of cluster installation
type Engine interface {
	// Validate allows the engine to prepare for the installation
	// and validate the environment before the operation is executed.
	Validate(context.Context, Config) error
	// Execute executes the steps to install a cluster.
	// If the method returns with an error, the installer will continue
	// running until it receives a shutdown signal.
	//
	// The method is expected to be re-entrant as the service might be re-started
	// multiple times before the operation is complete.
	//
	// installer is the reference to the installer.
	// config specifies the configuration for the operation
	Execute(ctx context.Context, installer Interface, config Config) error
}

// String formats this event for readability
func (r Event) String() string {
	var buf bytes.Buffer
	fmt.Print(&buf, "event(")
	if r.Progress != nil {
		fmt.Fprintf(&buf, "progress(completed=%v, message=%v),",
			r.Progress.Completion, r.Progress.Message)
	}
	if r.Error != nil {
		fmt.Fprintf(&buf, "error(%v)", r.Error.Error())
	}
	fmt.Print(&buf, ")")
	return buf.String()
}

// // timeSinceBeginning returns formatted operation duration
// func (i *Installer) timeSinceBeginning(key ops.SiteOperationKey) string {
// 	operation, err := i.Operator.GetSiteOperation(key)
// 	if err != nil {
// 		i.Errorf("Failed to retrieve operation: %v.", trace.DebugReport(err))
// 		return "<unknown>"
// 	}
// 	return time.Since(operation.Created).String()
// }

// Event describes the installer progress step
type Event struct {
	// Progress describes the operation progress
	Progress *ops.ProgressEntry
	// Error specifies the error if any
	Error error
}
