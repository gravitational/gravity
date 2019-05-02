/*
Copyright 2018-2019 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install/server"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/signals"

	"github.com/gravitational/trace"
)

// New returns a new instance of the unstarted installer server
func New(ctx context.Context, config Config) (*Installer, error) {
	err := upsertSystemAccount(ctx, config.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var agent *rpcserver.PeerServer
	if config.LocalAgent {
		agent, err = config.newAgent(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		go agent.Serve()
	}
	server := server.New(ctx, defaults.InstallerToken)
	localCtx, cancel := context.WithCancel(ctx)
	installer := &Installer{
		Config: config,
		server: server,
		ctx:    localCtx,
		cancel: cancel,
		agent:  agent,
	}
	return installer, nil
}

// Serve starts the server using the specified engine
func (i *Installer) Serve(engine Engine, listener net.Listener) error {
	i.engine = engine
	return trace.Wrap(i.server.Serve(i, listener))
}

// Stop stops the server and releases resources allocated by the installer.
//
// Implements signals.Stopper
func (i *Installer) Stop(ctx context.Context) error {
	i.Info("Stop.")
	if err := i.stop(ctx); err != nil {
		i.WithError(err).Warn("Failed to stop.")
	}
	i.server.Stop(ctx)
	return nil
}

// Abort stops the server, releases resources allocated by the installer and cleans up state.
//
// Implements signals.Aborter
func (i *Installer) Abort(ctx context.Context) error {
	i.Info("Abort.")
	if err := i.abort(ctx); err != nil {
		i.WithError(err).Warn("Failed to abort.")
	}
	i.server.Interrupt(ctx)
	return nil
}

// Interface defines the interface of the installer as presented
// to engine
type Interface interface {
	PlanBuilderGetter
	// NotifyOperationAvailable is invoked by the engine to notify the server
	// that the operation has been created
	NotifyOperationAvailable(ops.SiteOperationKey) error
	// Finalize executes additional steps common to all workflows after the
	// installation has completed
	Finalize(operation ops.SiteOperation) error
	// CompleteFinalInstallStep marks the final install step as completed unless
	// the application has a custom install step. In case of the custom step,
	// the user completes the final installer step
	CompleteFinalInstallStep(key ops.SiteOperationKey, delay time.Duration) error
	// Wait blocks the installer until the wizard process has been explicitly shut down
	// or specified context has expired
	Wait() error
	// PrintStep publishes a progress entry described with (format, args)
	PrintStep(format string, args ...interface{}) error
}

// NotifyOperationAvailable is invoked by the engine to notify the server
// that the operation has been created
func (i *Installer) NotifyOperationAvailable(key ops.SiteOperationKey) error {
	i.operationKey = key
	i.addAborter(signals.AborterFunc(func(ctx context.Context, interrupted bool) error {
		if interrupted {
			i.WithField("operation", key.OperationID).Info("Aborting agent service.")
			return trace.Wrap(i.Process.AgentService().AbortAgents(ctx, key))
		}
		return nil
	}))
	if err := i.upsertAdminAgent(key.SiteDomain); err != nil {
		return trace.Wrap(err)
	}
	i.server.RunProgressLoop(i.Operator, key)
	return nil
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
func (i *Installer) CompleteFinalInstallStep(key ops.SiteOperationKey, delay time.Duration) error {
	req := ops.CompleteFinalInstallStepRequest{
		AccountID:           defaults.SystemAccountID,
		SiteDomain:          key.SiteDomain,
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
	event := server.Event{Progress: &ops.ProgressEntry{Message: fmt.Sprintf(format, args...)}}
	return trace.Wrap(i.server.Send(event))
}

// Wait blocks until either the context has been cancelled or the wizard process
// exits with an error
func (i *Installer) Wait() error {
	return trace.Wrap(i.Process.Wait())
}

// Shutdown stops the active operation.
// Implements server.Executor
func (i *Installer) Shutdown(ctx context.Context) error {
	i.Info("Shutdown.")
	return trace.Wrap(i.stop(ctx))
}

// Execute executes the install operation using the specified engine
// Implements server.Executor
func (i *Installer) Execute() error {
	err := i.engine.Validate(i.ctx, i.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	err = i.engine.Execute(i.ctx, i, i.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	// Only explicitly stop agents after the operation has been completed
	i.addStopper(signals.StopperFunc(func(ctx context.Context) error {
		i.WithField("operation", i.operationKey.OperationID).Info("Stopping agent service.")
		return trace.Wrap(i.Process.AgentService().StopAgents(ctx, i.operationKey))
	}))
	i.printPostInstallBanner()
	return nil
}

// AbortOperation aborts the installation and cleans up the operation state.
// Implements server.Executor
func (i *Installer) AbortOperation(ctx context.Context) error {
	i.Info("Abort.")
	if err := i.abort(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
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

// stop stops the operation in progress
func (i *Installer) stop(ctx context.Context) error {
	i.cancel()
	var errors []error
	for _, c := range i.stoppers {
		if err := c.Stop(ctx); err != nil {
			errors = append(errors, err)
		}
	}
	i.Config.Process.Shutdown(ctx)
	i.server.WaitForOperation()
	return trace.NewAggregate(errors...)
}

// abort aborts the active operation
func (i *Installer) abort(ctx context.Context) error {
	i.cancel()
	var errors []error
	for _, c := range i.aborters {
		if err := c.Abort(ctx); err != nil {
			errors = append(errors, err)
		}
	}
	i.Config.Process.Shutdown(ctx)
	i.server.WaitForOperation()
	if err := i.AbortHandler(ctx); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// TODO(dmitri): this information should also be displayed when working with the operation
// manually
func (i *Installer) printPostInstallBanner() {
	var buf bytes.Buffer
	i.printEndpoints(&buf)
	if m, ok := modules.Get().(modules.Messager); ok {
		fmt.Fprintf(&buf, "\n%v", m.PostInstallMessage())
	}
	event := server.Event{
		Progress: &ops.ProgressEntry{
			Message:    buf.String(),
			Completion: constants.Completed,
		},
		// Send the completion event
		Complete: true,
	}
	i.server.Send(event)
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
	operator, err := i.LocalClusterClient()
	if err != nil {
		return trace.Wrap(err)
	}
	err = operator.StreamOperationLogs(operationKey, file)
	if err != nil {
		return trace.Wrap(err, "failed to upload install log")
	}
	i.WithField("file", i.UserLogFile).Debug("Uploaded install log to the cluster.")
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

func (i *Installer) addStopper(stopper signals.Stopper) {
	i.stoppers = append(i.stoppers, stopper)
}

func (i *Installer) addAborter(aborter signals.Aborter) {
	i.aborters = append(i.aborters, aborter)
}

// Installer manages the installation process
type Installer struct {
	// Config specifies the configuration for the install operation
	Config
	stoppers []signals.Stopper
	aborters []signals.Aborter
	// ctx controls the lifespan of internal processes
	ctx    context.Context
	cancel context.CancelFunc
	server *server.Server
	engine Engine
	// operationKey references the install operation once it has been
	// created by the engine
	operationKey ops.SiteOperationKey
	// agent is an optional RPC agent if the installer
	// has been configured to use local host as one of the cluster nodes
	agent *rpcserver.PeerServer
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

// // timeSinceBeginning returns formatted operation duration
// func (i *Installer) timeSinceBeginning(key ops.SiteOperationKey) string {
// 	operation, err := i.Operator.GetSiteOperation(key)
// 	if err != nil {
// 		i.Errorf("Failed to retrieve operation: %v.", trace.DebugReport(err))
// 		return "<unknown>"
// 	}
// 	return time.Since(operation.Created).String()
// }
