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
		return nil, trace.Wrap(err)
	}
	var agent *rpcserver.PeerServer
	if config.Config.LocalAgent {
		agent, err = newAgent(ctx, config.Config)
		if err != nil {
			return nil, trace.Wrap(err)
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
		// Reserve the buffer for a single result
		execDoneC:  make(chan execResult, 1),
		dispatcher: dispatcher,
	}
	installer.startExecuteLoop()
	if err := installer.maybeStartAgent(); err != nil {
		return nil, trace.Wrap(err)
	}
	return installer, nil
}

// Run runs the server operation
func (i *Installer) Run(listener net.Listener) error {
	go func() {
		i.errC <- i.server.Run(i, listener)
	}()
	err := <-i.errC
	if installpb.IsAbortError(err) {
		i.abort()
		return trace.Wrap(err)
	}
	i.stop()
	return trace.Wrap(err)
}

// Stop stops the server and releases resources allocated by the installer.
// Implements signals.Stopper
func (i *Installer) Stop(ctx context.Context) error {
	i.Info("Stop.")
	i.server.Interrupt(ctx)
	return nil
}

// Execute executes the install operation using the configured engine.
// Implements server.Executor
func (i *Installer) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	if !i.submit(req) && !req.IsResume() {
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
			if result.err != nil {
				// FIXME: send directly via stream.Send
				// Maybe send the error wrapped with grpc.codes.FailedPrecondition?
				return status.Error(codes.FailedPrecondition, err.Error())
				// return trace.Wrap(result.err)
			}
			err := stream.Send(result.completionEvent.AsProgressResponse())
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
	}
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
					i.WithError(err).Warn("Failed to execute.")
					i.execDoneC <- execResult{err: err}
				} else {
					i.execDoneC <- execResult{
						completionEvent: i.completionEvent(status),
					}
				}
				if status == dispatcher.StatusCompletedPending {
					i.wait()
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
	if req.Phase != nil {
		return i.executePhase(*req.Phase)
	}
	status, err := i.config.Engine.Execute(i.ctx, i, i.config.Config)
	if err != nil {
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	return status, nil
}

func (i *Installer) executePhase(phase installpb.ExecuteRequest_Phase) (dispatcher.Status, error) {
	opKey := installpb.KeyFromProto(phase.Key)
	machine, err := i.config.FSMFactory.NewFSM(i.config.Operator, opKey)
	if err != nil {
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	if phase.IsResume() {
		err := ExecuteOperation(i.ctx, machine, i.FieldLogger)
		if err != nil {
			return dispatcher.StatusUnknown, trace.Wrap(err)
		}
		return dispatcher.StatusCompleted, nil
	}
	params := fsm.Params{
		PhaseID:  phase.ID,
		Force:    phase.Force,
		Progress: dispatcher.NewProgressReporter(i.ctx, i.dispatcher, phaseTitle(phase)),
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
	i.wg.Wait()
	i.dispatcher.Close()
	if i.agent != nil {
		i.agent.Stop(ctx)
	}
	err := i.runStoppers(ctx)
	i.config.Process.Shutdown(ctx)
	i.server.Stop(ctx)
	return trace.Wrap(err)
}

// abortWithContext aborts the active operation and invokes the abort handler
func (i *Installer) abortWithContext(ctx context.Context) error {
	i.cancel()
	i.wg.Wait()
	i.dispatcher.Close()
	if i.agent != nil {
		i.agent.Stop(ctx)
	}
	err := i.stopAborters(ctx)
	i.config.Process.Shutdown(ctx)
	i.server.Stop(ctx)
	return trace.Wrap(err)
}

// Installer manages the installation process
type Installer struct {
	// FieldLogger specifies the installer's logger
	log.FieldLogger
	config RuntimeConfig
	// ctx controls the lifespan of internal processes
	ctx      context.Context
	cancel   context.CancelFunc
	server   *server.Server
	stoppers []signals.Stopper
	aborters []signals.Stopper
	// agent is an optional RPC agent if the installer
	// has been configured to use local host as one of the cluster nodes
	agent *rpcserver.PeerServer
	// errC receives termination signals from either explicit Stop or agent
	// closing with an error
	errC chan error

	execC      chan *installpb.ExecuteRequest
	execDoneC  chan execResult
	dispatcher dispatcher.EventDispatcher
	wg         sync.WaitGroup
}

func phaseTitle(phase installpb.ExecuteRequest_Phase) string {
	return fmt.Sprintf("Executing phase %v", phase.ID)
}

type execResult struct {
	completionEvent dispatcher.Event
	err             error
}
