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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install/engine"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/install/server"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns a new instance of the unstarted installer server.
// Use Serve to start server operation
func New(ctx context.Context, config RuntimeConfig) (installer *Installer, err error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	var agent *rpcserver.PeerServer
	if config.Config.LocalAgent {
		agent, err = newAgent(ctx, config.Config)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	server := server.New(ctx)
	localCtx, cancel := context.WithCancel(ctx)
	return &Installer{
		FieldLogger: config.FieldLogger,
		config:      config,
		server:      server,
		ctx:         localCtx,
		cancel:      cancel,
		agent:       agent,
	}, nil
}

// Run runs the server operation using the specified engine
func (i *Installer) Run(listener net.Listener) error {
	errC := make(chan error, 1)
	go func() {
		errC <- i.server.Serve(i, listener)
	}()
	select {
	case err := <-errC:
		i.WithError(err).Warn("Server.Serve finished.")
		i.stop()
		return trace.Wrap(err)
	case err := <-i.server.Done():
		i.WithError(err).Warn("Server finished.")
		if installpb.IsAbortedErr(err) {
			i.abort()
			return trace.Wrap(err)
		}
		i.stop()
		return trace.Wrap(err)
	}
}

// Stop stops the server and releases resources allocated by the installer.
// Implements signals.Stopper
func (i *Installer) Stop(ctx context.Context) error {
	i.Info("Stop.")
	i.server.Interrupt(ctx)
	return nil
}

// Interface defines the interface of the installer as presented
// to engine
type Interface interface {
	engine.ClusterFactory
	// ExecuteOperation executes the specified operation to completion
	ExecuteOperation(ops.SiteOperationKey) error
	// NotifyOperationAvailable is invoked by the engine to notify the server
	// that the operation has been created
	NotifyOperationAvailable(ops.SiteOperation) error
	// CompleteOperation executes additional steps common to all workflows after the
	// installation has completed
	CompleteOperation(operation ops.SiteOperation) error
	// CompleteFinalInstallStep marks the final install step as completed unless
	// the application has a custom install step. In case of the custom step,
	// the user completes the final installer step
	CompleteFinalInstallStep(key ops.SiteOperationKey, delay time.Duration) error
	// Wait blocks the installer until the wizard process has been explicitly shut down
	// or specified context has expired
	Wait() error
	// PrintStep publishes a progress entry described with (format, args)
	PrintStep(format string, args ...interface{})
}

// NotifyOperationAvailable is invoked by the engine to notify the server
// that the operation has been created.
// Implements Interface
func (i *Installer) NotifyOperationAvailable(op ops.SiteOperation) error {
	if i.agent != nil {
		i.startAgent(op)
	}
	i.addAborter(signals.StopperFunc(func(ctx context.Context) error {
		i.WithField("operation", op.ID).Info("Aborting agent service.")
		return trace.Wrap(i.config.Process.AgentService().AbortAgents(ctx, op.Key()))
	}))
	if err := i.upsertAdminAgent(op.SiteDomain); err != nil {
		return trace.Wrap(err)
	}
	go ProgressLooper{
		FieldLogger:  i.FieldLogger,
		Operator:     i.config.Operator,
		OperationKey: op.Key(),
		Dispatcher:   i.server,
	}.Run(i.ctx)

	return nil
}

// Returns a new cluster request
// Implements engine.ClusterFactory
func (i *Installer) NewCluster() ops.NewSiteRequest {
	return i.config.ClusterFactory.NewCluster()
}

// ExecuteOperation executes the specified operation to completion.
// Implements Interface
func (i *Installer) ExecuteOperation(operationKey ops.SiteOperationKey) error {
	err := initOperationPlan(i.config.Operator, i.config.Planner)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	machine, err := i.config.FSMFactory.NewFSM(i.config.Operator, operationKey)
	if err != nil {
		return trace.Wrap(err)
	}
	err = machine.ExecutePlan(i.ctx, utils.DiscardProgress)
	if err != nil {
		i.WithError(err).Warn("Failed to execute operation plan.")
	}
	if completeErr := machine.Complete(err); completeErr != nil {
		i.WithError(completeErr).Warn("Failed to complete operation.")
		if err == nil {
			err = completeErr
		}
	}
	return trace.Wrap(err)
}

// CompleteOperation executes additional steps after the installation has completed.
// Implements Interface
func (i *Installer) CompleteOperation(operation ops.SiteOperation) error {
	var errors []error
	if err := i.uploadInstallLog(operation.Key()); err != nil {
		errors = append(errors, err)
	}
	if err := i.emitAuditEvents(i.ctx, operation); err != nil {
		errors = append(errors, err)
	}
	// Explicitly stop agents only iff the operation has been completed successfully
	i.addStopper(signals.StopperFunc(func(ctx context.Context) error {
		i.WithField("operation", operation.ID).Info("Stopping agent service.")
		return trace.Wrap(i.config.Process.AgentService().StopAgents(ctx, operation.Key()))
	}))
	i.sendElapsedTime(operation.Created)
	i.sendPostInstallBanner()
	return trace.NewAggregate(errors...)
}

// CompleteFinalInstallStep marks the final install step as completed unless
// the application has a custom install step - in which case it does nothing
// because it will be completed by user later.
// Implements Interface
func (i *Installer) CompleteFinalInstallStep(key ops.SiteOperationKey, delay time.Duration) error {
	req := ops.CompleteFinalInstallStepRequest{
		AccountID:           defaults.SystemAccountID,
		SiteDomain:          key.SiteDomain,
		WizardConnectionTTL: delay,
	}
	i.WithField("req", req).Debug("Completing final install step.")
	if err := i.config.Operator.CompleteFinalInstallStep(req); err != nil {
		return trace.Wrap(err, "failed to complete final install step")
	}
	return nil
}

// PrintStep publishes a progress entry described with (format, args) tuple to the client.
// Implements Interface
func (i *Installer) PrintStep(format string, args ...interface{}) {
	event := server.Event{Progress: &ops.ProgressEntry{Message: fmt.Sprintf(format, args...)}}
	i.server.Send(event)
}

// Wait blocks until either the context has been cancelled or the wizard process
// exits with an error.
// Implements Interface
func (i *Installer) Wait() error {
	i.stopStoppers(i.ctx)
	return trace.Wrap(i.config.Process.Wait())
}

// Shutdown stops the active operation.
// Implements server.Executor
func (i *Installer) Shutdown(ctx context.Context) error {
	i.Info("Shutdown.")
	return trace.Wrap(i.stopWithContext(ctx))
}

// Execute executes the install operation using the specified engine
// Implements server.Executor
func (i *Installer) Execute(phase *installpb.ExecuteRequest_Phase) error {
	if phase != nil {
		return i.executePhase(*phase)
	}
	err := i.config.Engine.Execute(i.ctx, i, i.config.Config)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Complete manually completes the operation given with opKey.
// Implements server.Executor
func (i *Installer) Complete(opKey ops.SiteOperationKey) error {
	machine, err := i.config.FSMFactory.NewFSM(i.config.Operator, opKey)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(machine.Complete(trace.Errorf("completed manually")))
}

func (i *Installer) executePhase(phase installpb.ExecuteRequest_Phase) error {
	opKey := installpb.KeyFromProto(phase.Key)
	machine, err := i.config.FSMFactory.NewFSM(i.config.Operator, opKey)
	if err != nil {
		return trace.Wrap(err)
	}
	if phase.ID == fsm.RootPhase {
		return trace.Wrap(i.executeOperation(machine))
	}
	p := fsm.Params{
		PhaseID: phase.ID,
		Force:   phase.Force,
	}
	if phase.Rollback {
		return trace.Wrap(machine.RollbackPhase(i.ctx, p))
	}
	return trace.Wrap(machine.ExecutePhase(i.ctx, p))
}

func (i *Installer) executeOperation(machine *fsm.FSM) error {
	return trace.Wrap(ExecuteOperation(i.ctx, machine, i.FieldLogger))
}

func (i *Installer) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
	defer cancel()
	if err := i.stopWithContext(ctx); err != nil {
		i.WithError(err).Warn("Failed to stop.")
	}
}

func (i *Installer) abort() {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
	defer cancel()
	if err := i.abortWithContext(ctx); err != nil {
		i.WithError(err).Warn("Failed to abort.")
	}
}

// stop stops the operation in progress
func (i *Installer) stopWithContext(ctx context.Context) error {
	i.cancel()
	if i.agent != nil {
		i.agent.Stop(ctx)
	}
	err := i.stopStoppers(ctx)
	i.config.Process.Shutdown(ctx)
	i.server.Stop(ctx)
	return trace.Wrap(err)
}

// abortWithContext aborts the active operation and invokes the abort handler
func (i *Installer) abortWithContext(ctx context.Context) error {
	i.server.SendAbort()
	i.cancel()
	var errors []error
	for _, c := range i.aborters {
		if err := c.Stop(ctx); err != nil {
			errors = append(errors, err)
		}
	}
	i.config.Process.Shutdown(ctx)
	i.server.Stop(ctx)
	return trace.NewAggregate(errors...)
}

func (i *Installer) sendElapsedTime(timeStarted time.Time) {
	event := server.Event{
		Progress: &ops.ProgressEntry{
			Message: color.GreenString("Installation succeeded in %v", time.Since(timeStarted)),
		},
	}
	i.server.Send(event)
}

// TODO(dmitri): this information should also be displayed when working with the operation
// manually
func (i *Installer) sendPostInstallBanner() {
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
	// FIXME: logging
	// i.Info("Send completion event.")
	i.server.Send(event)
}

func (i *Installer) stopStoppers(ctx context.Context) error {
	var errors []error
	for _, c := range i.stoppers {
		if err := c.Stop(ctx); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
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
	agent, err := i.config.Process.UsersService().CreateClusterAdminAgent(clusterName,
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
	file, err := os.Open(i.config.UserLogFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	err = i.config.Operator.StreamOperationLogs(operationKey, file)
	if err != nil {
		return trace.Wrap(err, "failed to upload install log")
	}
	i.WithField("file", i.config.UserLogFile).Debug("Uploaded install log to the cluster.")
	return nil
}

// emitAuditEvents sends the install operation's start/finish
// events to the installed cluster's audit log.
func (i *Installer) emitAuditEvents(ctx context.Context, operation ops.SiteOperation) error {
	operator, err := localenv.ClusterOperator()
	if err != nil {
		i.WithError(err).Warn("Failed to create cluster operator.")
		return trace.Wrap(err)
	}
	fields := events.FieldsForOperation(operation)
	events.Emit(i.ctx, operator, events.OperationInstallStart, fields.WithField(
		events.FieldTime, operation.Created))
	events.Emit(i.ctx, operator, events.OperationInstallComplete, fields)
	return nil
}

func (i *Installer) addStopper(stopper signals.Stopper) {
	i.stoppers = append(i.stoppers, stopper)
}

func (i *Installer) addAborter(aborter signals.Stopper) {
	i.aborters = append(i.aborters, aborter)
}

func (i *Installer) startAgent(operation ops.SiteOperation) error {
	profile, ok := operation.InstallExpand.Agents[i.config.Role]
	if !ok {
		return trace.BadParameter("no agent profile for role %q", i.config.Role)
	}
	token, err := getTokenFromURL(profile.AgentURL)
	if err != nil {
		return trace.Wrap(err)
	}
	go i.agent.ServeWithToken(token)
	return nil
}

// Installer manages the installation process
type Installer struct {
	// FieldLogger specifies the installer's logger
	log.FieldLogger
	config   RuntimeConfig
	stoppers []signals.Stopper
	aborters []signals.Stopper
	// ctx controls the lifespan of internal processes
	ctx    context.Context
	cancel context.CancelFunc
	server *server.Server
	// agent is an optional RPC agent if the installer
	// has been configured to use local host as one of the cluster nodes
	agent *rpcserver.PeerServer
}

// Engine implements the process of cluster installation
type Engine interface {
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
