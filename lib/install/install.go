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
	"strconv"
	"sync"
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
func New(ctx context.Context, config Config) (*Installer, error) {
	err := upsertSystemAccount(ctx, config.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localCtx, cancel := context.WithCancel(ctx)
	grpcServer := grpc.NewServer()
	installer := &Installer{
		Config:    config,
		parentCtx: ctx,
		ctx:       localCtx,
		cancel:    cancel,
		rpc:       grpcServer,
		// TODO(dmitri): arbitrary channel buffer size
		eventsC: make(chan Event, 100),
	}
	installpb.RegisterAgentServer(grpcServer, installer)
	return installer, nil
}

// Serve starts the server using the specified engine
func (i *Installer) Serve(engine Engine, listener net.Listener) error {
	i.engine = engine
	return trace.Wrap(i.rpc.Serve(listener))
}

// Stop releases resources allocated by the installer
func (i *Installer) Stop(ctx context.Context) error {
	i.cancel()
	i.Config.Process.Shutdown(ctx)
	i.rpc.GracefulStop()
	var errors []error
	for _, c := range i.closers {
		if err := c.Close(ctx); err != nil {
			errors = append(errors, err)
		}
	}
	i.serveWG.Wait()
	return trace.NewAggregate(errors...)
}

// Interface defines the interface of the installer as presented
// to engine
type Interface interface {
	PlanBuilderGetter
	// NotifyOperationAvailable is invoked by the engine to notify the server
	// that the operation has been created
	NotifyOperationAvailable(ops.SiteOperationKey) error
	// NewAgent returns a new unstarted installer agent.
	// Call agent.Serve() on the resulting instance to start agent's service loop
	NewAgent(url string) (rpcserver.Server, error)
	// Finalize executes additional steps common to all workflows after the
	// installation has completed
	Finalize(operation ops.SiteOperation) error
	// CompleteFinalInstallStep marks the final install step as completed unless
	// the application has a custom install step. In case of the custom step,
	// the user completes the final installer step
	CompleteFinalInstallStep(delay time.Duration) error
	// Wait blocks the installer until the wizard process has been explicitly shut down
	// or specified context has expired
	Wait() error
	// PrintStep publishes a progress entry described with (format, args)
	PrintStep(format string, args ...interface{}) error
}

// NotifyOperationAvailable is invoked by the engine to notify the server
// that the operation has been created
func (i *Installer) NotifyOperationAvailable(operationKey ops.SiteOperationKey) error {
	i.addCloser(CloserFunc(func(ctx context.Context) error {
		i.WithField("operation", operationKey.OperationID).Info("Stopping agent service.")
		return trace.Wrap(i.Process.AgentService().StopAgents(ctx, operationKey))
	}))
	if err := i.upsertAdminAgent(operationKey.SiteDomain); err != nil {
		return trace.Wrap(err)
	}
	i.serveWG.Add(1)
	go func() {
		i.progressLoop(operationKey)
		i.serveWG.Done()
	}()
	return nil
}

// NewAgent creates a new installer agent
// FIXME: accept (serverAddr,token) tuple instead of agentURL
func (i *Installer) NewAgent(agentURL string) (rpcserver.Server, error) {
	serverAddr, token, err := SplitAgentURL(agentURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverCreds, clientCreds, err := rpc.Credentials(defaults.RPCAgentSecretsDir)
	if err != nil {
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
		Token:        token,
	}
	return NewAgent(i.ctx, AgentConfig{
		FieldLogger:   i.FieldLogger,
		AdvertiseAddr: i.AdvertiseAddr,
		ServerAddr:    serverAddr,
		Credentials: rpcserver.Credentials{
			Server: serverCreds,
			Client: clientCreds,
		},
		RuntimeConfig: runtimeConfig,
	})
}

// Finalize executes additional steps after the installation has completed
func (i *Installer) Finalize(operation ops.SiteOperation) error {
	var errors []error
	if err := i.uploadInstallLog(operation.Key()); err != nil {
		errors = append(errors, err)
	}
	if err := i.emitAuditEvents(i.ctx, operation); err != nil {
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
func (i *Installer) PrintStep(format string, args ...interface{}) error {
	event := Event{Progress: &ops.ProgressEntry{Message: fmt.Sprintf(format, args...)}}
	return trace.Wrap(i.send(event))
}

// Execute executes the installation using the specified engine
// Implements installpb.AgentServer
func (i *Installer) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	i.executeOnce.Do(func() {
		i.serveWG.Add(1)
		go func() {
			if err := i.execute(); err != nil {
				i.WithError(err).Info("Failed to execute.")
				if err := i.sendError(err); err != nil {
					// TODO: only exit if unable to send the error.
					// Otherwise, the client will shut down the server at
					// the most appropriate time
				}
			}
			i.serveWG.Done()
			i.Stop(i.parentCtx)
		}()
	})
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
			if resp.Complete {
				return nil
			}
		case <-stream.Context().Done():
			return trace.Wrap(stream.Context().Err())
		case <-i.parentCtx.Done():
			return trace.Wrap(i.parentCtx.Err())
		case <-i.ctx.Done():
			// Clean exit
			return nil
		}
	}
	return nil
}

// Shutdown shuts down the installer.
// Implements installpb.AgentServer
func (i *Installer) Shutdown(ctx context.Context, req *installpb.ShutdownRequest) (*installpb.ShutdownResponse, error) {
	// The caller should be blocked at least as long as the wizard process is closing.
	// TODO(dmitri): find out how this returns to the caller and whether it would make sense
	// to split the shut down into several steps with wizard shutdown to be invoked as part of Shutdown
	// and the rest - from a goroutine so the caller is not receiving an error when the server stops
	// serving
	i.Stop(ctx)
	return &installpb.ShutdownResponse{}, nil
}

// Wait blocks until either the context has been cancelled or the wizard process
// exits with an error
func (i *Installer) Wait() error {
	return trace.Wrap(i.Process.Wait())
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
		Local:        true,
	}
}

func (i *Installer) execute() (err error) {
	err = i.engine.Validate(i.ctx, i.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	err = i.engine.Execute(i.ctx, i, i.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	i.printPostInstallBanner()
	return nil
}

func (i *Installer) progressLoop(operationKey ops.SiteOperationKey) {
	i.WithField("operation", operationKey.OperationID).Info("Start progress feedback loop.")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var lastProgress *ops.ProgressEntry
	for {
		select {
		case <-ticker.C:
			if progress := i.updateProgress(operationKey, lastProgress); progress != nil {
				lastProgress = progress
				if progress.IsCompleted() {
					return
				}
			}
		case <-i.parentCtx.Done():
			return
		case <-i.ctx.Done():
			return
		}
	}
}

func (i *Installer) updateProgress(operationKey ops.SiteOperationKey, lastProgress *ops.ProgressEntry) *ops.ProgressEntry {
	progress, err := i.Operator.GetSiteOperationProgress(operationKey)
	if err != nil {
		i.WithError(err).Warn("Failed to query operation progress.")
		return nil
	}
	if lastProgress != nil && lastProgress.IsEqual(*progress) {
		return nil
	}
	i.send(Event{Progress: progress})
	return progress
}

// FIXME(dmitri): this information should also be displayed when working with the operation
// manually
func (i *Installer) printPostInstallBanner() {
	var buf bytes.Buffer
	i.printEndpoints(&buf)
	if m, ok := modules.Get().(modules.Messager); ok {
		fmt.Fprintf(&buf, "\n%v", m.PostInstallMessage())
	}
	i.Printer.Print(buf.String())
	event := Event{Progress: &ops.ProgressEntry{
		Message:    buf.String(),
		Completion: constants.Completed,
	}}
	i.send(event)
}

func (i *Installer) sendError(err error) error {
	return trace.Wrap(i.send(Event{Error: err}))
}

// send streams the specified progress event to the client.
// The method will not block - event will be dropped if it cannot be published
// (subject to internal channel buffer capacity)
func (i *Installer) send(event Event) error {
	select {
	case i.eventsC <- event:
		// Pushed the progress event
		return nil
	case <-i.parentCtx.Done():
		return nil
	case <-i.ctx.Done():
		return nil
	default:
		log.WithField("event", event).Warn("Failed to publish event.")
		return trace.BadParameter("failed to publish event")
	}
}

func (i *Installer) printEndpoints(w io.Writer) {
	status, err := i.getClusterStatus()
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
func (i *Installer) getClusterStatus() (*status.Status, error) {
	clusterOperator, err := localenv.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := clusterOperator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	status, err := status.FromCluster(i.ctx, clusterOperator, *cluster, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return status, nil
}

// upsertAdminAgent creates an admin agent for the cluster being installed
func (i *Installer) upsertAdminAgent(clusterName string) error {
	agent, err := i.Process.UsersService().CreateClusterAdminAgent(clusterName,
		storage.NewUser(storage.ClusterAdminAgent(clusterName), storage.UserSpecV2{
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

func (i *Installer) addCloser(closer Closer) {
	i.closers = append(i.closers, closer)
}

// Installer manages the installation process
type Installer struct {
	// Config specifies the configuration for the install operation
	Config
	closers []Closer
	// parentCtx specifies the external context.
	// If cancelled, all operations abort with the corresponding error
	parentCtx context.Context
	// ctx defines the local server context used to cancel internal operation
	ctx     context.Context
	cancel  context.CancelFunc
	eventsC chan Event
	// rpc is the fabric to communicate to the server client prcess
	rpc         *grpc.Server
	engine      Engine
	executeOnce sync.Once
	serveWG     sync.WaitGroup
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
