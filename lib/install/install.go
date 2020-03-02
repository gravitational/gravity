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
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install/dispatcher"
	"github.com/gravitational/gravity/lib/install/dispatcher/buffered"
	"github.com/gravitational/gravity/lib/install/engine"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/install/server"
	"github.com/gravitational/gravity/lib/ops"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// New returns a new instance of the unstarted installer server.
// ctx is only used for the duration of this call and is not stored beyond that.
// Use Serve to start server operation
func New(ctx context.Context, config RuntimeConfig) (installer *Installer, err error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(utils.NewFailedPreconditionError(err))
	}
	var agent *rpcserver.PeerServer
	if config.Config.LocalAgent {
		agent, err = newAgent(ctx, config.Config)
		if err != nil {
			return nil, trace.Wrap(utils.NewFailedPreconditionError(err))
		}
	}
	server := server.New()
	dispatcher := buffered.New()
	localCtx, cancel := context.WithCancel(context.Background())
	installer = &Installer{
		FieldLogger: config.FieldLogger,
		ctx:         localCtx,
		cancel:      cancel,
		config:      config,
		server:      server,
		agent:       agent,
		errC:        make(chan error, 2),
		execC:       make(chan *installpb.ExecuteRequest),
		execDoneC:   make(chan ExecResult, 1),
		dispatcher:  dispatcher,
	}
	installer.startExecuteLoop()
	if err := installer.maybeStartAgent(); err != nil {
		return nil, trace.Wrap(utils.NewFailedPreconditionError(err))
	}
	return installer, nil
}

// Run runs the server operation
func (i *Installer) Run(listener net.Listener) error {
	go func() {
		i.errC <- i.server.Run(i, listener)
	}()
	err := <-i.errC
	i.stop()
	return installpb.WrapServiceError(err)
}

// Stop stops the server and releases resources allocated by the installer.
// Implements signals.Stopper
func (i *Installer) Stop(ctx context.Context) error {
	i.Info("Stop service.")
	return i.server.ManualStop(ctx, false)
}

// Execute executes the install operation using the configured engine.
// Implements server.Executor
func (i *Installer) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	if !i.submit(req) && req.HasSpecificPhase() {
		return status.Error(codes.AlreadyExists, "operation is already active")
	}
	for {
		select {
		case event := <-i.dispatcher.Chan():
			err := stream.Send(event)
			if err != nil {
				return trace.Wrap(err)
			}
		case result := <-i.execDoneC:
			if result.Error != nil {
				// Phase finished with an error.
				// See https://github.com/grpc/grpc-go/blob/v1.22.0/codes/codes.go#L78
				return status.Error(codes.Aborted, FormatAbortError(result.Error))
			}
			if result.CompletionEvent != nil {
				err := stream.Send(result.CompletionEvent.AsProgressResponse())
				if err != nil {
					return trace.Wrap(err)
				}
			}
			return nil
		}
	}
}

// SetPhase sets phase state without executing it.
func (i *Installer) SetPhase(req *installpb.SetStateRequest) error {
	i.WithField("req", req).Info("Set phase.")
	machine, err := i.config.FSMFactory.NewFSM(i.config.Operator, req.OperationKey())
	if err != nil {
		return trace.Wrap(err)
	}
	return machine.ChangePhaseState(i.ctx, fsm.StateChange{
		Phase: req.Phase.ID,
		State: req.State,
	})
}

// Complete manually completes the operation given with key.
// Implements server.Executor
func (i *Installer) Complete(key ops.SiteOperationKey) error {
	i.WithField("key", key).Info("Complete.")
	machine, err := i.config.FSMFactory.NewFSM(i.config.Operator, key)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(machine.Complete(trace.Errorf("completed manually")))
}

// GenerateDebugReport captures the state of the operation to the file given with path.
// Implements server.DebugReporter
func (i *Installer) GenerateDebugReport(path string) error {
	i.WithField("path", path).Info("Generate debug report.")
	op, err := ops.GetWizardOperation(i.config.Operator)
	if err != nil {
		return trace.Wrap(err)
	}
	err = i.generateDebugReport(op.ClusterKey(), path)
	if err != nil {
		return trace.Wrap(err)
	}
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
	// PrintStep publishes a progress entry described with (format, args)
	PrintStep(format string, args ...interface{})
}

// submit submits the specified request for execution.
// Returns true if the request has actually started an operation
func (i *Installer) submit(req *installpb.ExecuteRequest) bool {
	select {
	case i.execC <- req:
		return true
	default:
		// Another operation is already in flight
		return false
	}
}

func (i *Installer) startExecuteLoop() {
	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		for {
			select {
			case req := <-i.execC:
				status, err := i.execute(req)
				select {
				case <-i.execDoneC:
					// Empty the result channel
				default:
				}
				if err != nil {
					i.WithFields(log.Fields{
						log.ErrorKey: err,
						"req":        req,
					}).Warn("Failed to execute.")
					i.execDoneC <- ExecResult{Error: err}
				} else {
					var result ExecResult
					if status.IsCompleted() {
						result.CompletionEvent = i.newCompletionEvent(status)
					}
					i.execDoneC <- result
				}
				if status == dispatcher.StatusCompletedPending {
					if err := i.wait(); err != nil {
						i.WithError(err).Warn("Failed to wait for installer to complete.")
					}
				}
			case <-i.ctx.Done():
				return
			}
		}
	}()
}

func (i *Installer) maybeStartAgent() error {
	op, err := ops.GetWizardOperation(i.config.Operator)
	if err != nil {
		// Ignore the failure to query the operation as there might be multiple
		// reasons the operation is not available.
		i.WithError(err).Info("Failed to query install operation.")
		return nil
	}
	err = i.startAgent(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	i.registerExitHandlersForAgents(*op)
	return nil
}

func (i *Installer) execute(req *installpb.ExecuteRequest) (dispatcher.Status, error) {
	i.WithField("req", req).Info("Execute.")
	if req.HasSpecificPhase() {
		return i.executePhase(*req.Phase)
	}
	status, err := i.config.Engine.Execute(i.ctx, i, i.config.Config)
	if err != nil {
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	return status, nil
}

func (i *Installer) executePhase(phase installpb.Phase) (dispatcher.Status, error) {
	opKey := installpb.KeyFromProto(phase.Key)
	machine, err := i.config.FSMFactory.NewFSM(i.config.Operator, opKey)
	if err != nil {
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	progressReporter := dispatcher.NewProgressReporter(i.ctx, i.dispatcher, phaseTitle(phase))
	defer progressReporter.Stop()
	if phase.IsResume() {
		err := i.ExecuteOperation(opKey)
		if err != nil {
			return dispatcher.StatusUnknown, trace.Wrap(err)
		}
		return dispatcher.StatusCompleted, nil
	}
	params := fsm.Params{
		PhaseID:  phase.ID,
		Force:    phase.Force,
		Progress: progressReporter,
	}
	if phase.Rollback {
		err := machine.RollbackPhase(i.ctx, params)
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	err = machine.ExecutePhase(i.ctx, params)
	return dispatcher.StatusUnknown, trace.Wrap(err)
}

func (i *Installer) startAgent(operation ops.SiteOperation) error {
	if i.agent == nil {
		return nil
	}
	profile, ok := operation.InstallExpand.Agents[i.config.Role]
	if !ok {
		return trace.BadParameter("no agent profile for role %q", i.config.Role)
	}
	token, err := getTokenFromURL(profile.AgentURL)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		i.errC <- i.agent.ServeWithToken(token)
	}()
	return nil
}

// HandleStopped executes the list of registered stoppers before the service is shut down.
// Implements server.Completer
func (i *Installer) HandleStopped(ctx context.Context) error {
	i.Info("Stop signaled.")
	return trace.Wrap(i.stopWithContext(ctx, i.stoppers))
}

// HandleAborted completes the operation by running the list of registered abort handlers
// Implements server.Completer
func (i *Installer) HandleAborted(ctx context.Context) error {
	i.Info("Abort signaled.")
	return trace.Wrap(i.stopWithContext(ctx, i.aborters))
}

// HandleCompleted completes the operation by running the list of registered completion handlers
// Implements server.Completer
func (i *Installer) HandleCompleted(ctx context.Context) error {
	i.Info("Completion signaled.")
	return trace.Wrap(i.stopWithContext(ctx, i.completers))
}

// stop runs the specified list of stoppers and shuts down the server
func (i *Installer) stopWithContext(ctx context.Context, stoppers []signals.Stopper) error {
	// Shut down process first to unblock a possible wait
	i.config.Process.Shutdown(ctx)
	i.cancel()
	i.wg.Wait()
	i.dispatcher.Close()
	return i.runStoppers(ctx, stoppers)
}

// Installer manages the installation process
type Installer struct {
	// FieldLogger specifies the installer's logger
	log.FieldLogger
	config RuntimeConfig
	// ctx controls the lifespan of internal processes
	ctx    context.Context
	cancel context.CancelFunc
	server *server.Server
	// stoppers lists resources to close when the server is shutting down
	stoppers []signals.Stopper
	// aborters lists resources to close when the server has been interrupted
	aborters []signals.Stopper
	// completers lists resources to close when the server is shutting down
	// after a successfully completed operation
	completers []signals.Stopper
	// agent is an optional RPC agent if the installer
	// has been configured to use local host as one of the cluster nodes
	agent      *rpcserver.PeerServer
	dispatcher dispatcher.EventDispatcher

	// errC receives termination signals from either explicit Stop or agent
	// closing with an error
	errC chan error

	// execC relays execution requests to the executor loop from outside
	execC chan *installpb.ExecuteRequest
	// execDoneC is signaled by the executor loop to let the client-facing gRPC API
	// know when to stop expecting events and exit
	execDoneC chan ExecResult

	// wg is a wait group used to ensure completion of internal processes
	wg sync.WaitGroup
}

func (i *Installer) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
	defer cancel()
	i.server.Stop(ctx)
}

func phaseTitle(phase installpb.Phase) string {
	if phase.IsResume() {
		return "Resuming operation"
	}
	return fmt.Sprintf("Executing phase %v", phase.ID)
}

func phaseForOperation(op ops.SiteOperation) *installpb.Phase {
	return &installpb.Phase{
		ID:  fsm.RootPhase,
		Key: installpb.KeyToProto(op.Key()),
	}
}
