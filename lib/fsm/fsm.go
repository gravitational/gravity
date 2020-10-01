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

package fsm

import (
	"context"
	"fmt"
	"path"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Engine defines interface for specific FSM implementations
type Engine interface {
	// GetExecutor returns a new executor based on the provided parameters
	GetExecutor(ExecutorParams, Remote) (PhaseExecutor, error)
	// ChangePhaseState updates the phase state based on the provided parameters
	ChangePhaseState(context.Context, StateChange) error
	// GetPlan returns the up-to-date operation plan
	GetPlan() (*storage.OperationPlan, error)
	// RunCommand executes the phase specified by params on the specified
	// server using the provided runner
	RunCommand(context.Context, rpc.RemoteRunner, storage.Server, Params) error
	// Complete transitions the operation to a completed state.
	// Completed state is either successful or failed depending on the state of
	// the operation plan.
	// The optional error can be used to specify the reason for failure and
	// defines the final operation failure
	Complete(error) error
}

// ExecutorParams combines parameters needed for creating a new executor
type ExecutorParams struct {
	// Plan is the operation plan
	Plan storage.OperationPlan
	// Phase is the plan phase
	Phase storage.OperationPhase
	// Progress is the progress reporter
	Progress utils.Progress
}

// Key returns an operation key from these params
func (p ExecutorParams) Key() ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   p.Plan.AccountID,
		SiteDomain:  p.Plan.ClusterName,
		OperationID: p.Plan.OperationID,
	}
}

// Params combines parameters for phase execution/rollback
type Params struct {
	// PhaseID is the id of the phase to execute/rollback
	PhaseID string
	// Force is whether to force execution/rollback
	Force bool
	// Resume determines whether a failed/in-progress phase is rerun.
	//
	// It is different from Force which forces a phase in any state
	// to be rerun - this is unexpected when the operation is resumed
	// and only the unfinished/failed steps are re-executed
	Resume bool
	// Rollback indicates that the specified phase should be rolled back.
	Rollback bool
	// Progress is optional progress reporter
	Progress utils.Progress
	// DryRun allows to only print phases without executing/rolling back.
	DryRun bool
}

// CheckAndSetDefaults makes sure all required parameters are set
func (p *Params) CheckAndSetDefaults() error {
	if p.PhaseID == "" {
		return trace.BadParameter("missing PhaseID")
	}
	if p.Progress == nil {
		p.Progress = utils.NewNopProgress()
	}
	return nil
}

// FSM is the generic FSM implementation that provides methods for phases
// execution and rollback, state transitioning and command execution
type FSM struct {
	// Config is the FSM config
	Config
	// FieldLogger is used for logging
	logrus.FieldLogger
	// preExecFn is called before phase execution if set
	preExecFn PhaseHookFn
	// postExecFn is called after phase execution if set
	postExecFn PhaseHookFn
}

// PhaseHookFn defines the phase hook function
type PhaseHookFn func(context.Context, Params) error

// Config represents config
type Config struct {
	// Engine is the specific FSM engine
	Engine
	// Runner is used to run remote commands
	Runner rpc.RemoteRunner
	// Insecure allows to turn off cert validation in dev mode
	Insecure bool
	// Logger allows to override default logger
	Logger logrus.FieldLogger
}

// CheckAndSetDefaults makes sure the config is valid and sets some defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Engine == nil {
		return trace.BadParameter("missing Engine")
	}
	if c.Logger == nil {
		c.Logger = logrus.WithField(trace.Component, "fsm")
	}
	return nil
}

// New returns a new FSM instance
func New(config Config) (*FSM, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &FSM{
		Config:      config,
		FieldLogger: config.Logger,
	}, nil
}

// ExecutePlan iterates over all phases of the plan and executes them in order
func (f *FSM) ExecutePlan(ctx context.Context, progress utils.Progress) error {
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}

	// Make sure the plan is being executed/resumed on the correct node.
	err = CheckPlanCoordinator(plan)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, phase := range plan.Phases {
		f.WithField("phase", phase.ID).Debug("Executing phase.")
		err := f.ExecutePhase(ctx, Params{
			PhaseID:  phase.ID,
			Progress: progress,
			Resume:   true,
		})
		if err != nil {
			return trace.Wrap(err, "failed to execute phase %q", phase.ID)
		}
	}
	return nil
}

// RollbackPlan rolls back all phases of the plan that have been attempted so
// far in the reverse order.
func (f *FSM) RollbackPlan(ctx context.Context, progress utils.Progress, dryRun bool) error {
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	allPhases := plan.GetLeafPhases()
	for i := len(allPhases) - 1; i >= 0; i -= 1 {
		phase := allPhases[i]
		log := f.WithFields(logrus.Fields{"phase": phase.ID, "state": phase.GetState()})
		if phase.IsUnstarted() || phase.IsRolledBack() {
			log.Info("Skip rollback.")
			continue
		}
		log.Info("Rollback.")
		err := f.RollbackPhase(ctx, Params{
			PhaseID:  phase.ID,
			Progress: progress,
			DryRun:   dryRun,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ExecutePhase executes the specified phase of the plan
func (f *FSM) ExecutePhase(ctx context.Context, p Params) error {
	err := p.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	phase, err := FindPhase(plan, p.PhaseID)
	if err != nil {
		return trace.Wrap(err)
	}
	if phase.IsCompleted() && !p.Force {
		return nil
	}
	if phase.IsInProgress() && !(p.Force || p.Resume || phase.HasSubphases()) {
		return trace.BadParameter(
			"phase %q is in progress, use --force flag to force execution", phase.ID)
	}
	err = f.prerequisitesComplete(phase.ID)
	if err != nil && !p.Force {
		return trace.Wrap(err)
	}
	if f.preExecFn != nil {
		if err := f.preExecFn(ctx, p); err != nil {
			return trace.Wrap(err)
		}
	}
	err = f.executePhase(ctx, p, *phase)
	if err != nil {
		return trace.Wrap(err)
	}
	if f.postExecFn != nil {
		if err := f.postExecFn(ctx, p); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// RollbackPhase rolls back the specified phase of the plan
func (f *FSM) RollbackPhase(ctx context.Context, p Params) error {
	err := p.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	err = CanRollback(plan, p.PhaseID)
	if err != nil {
		if !p.Force {
			return trace.Wrap(err)
		}
		f.WithError(err).Warn("Forcing rollback.")
	}
	phase, err := FindPhase(plan, p.PhaseID)
	if err != nil {
		return trace.Wrap(err)
	}
	if !phase.HasSubphases() {
		// Check whether this phase should be run on a local or remote server.
		var execServer *storage.Server
		if phase.Data != nil {
			if phase.Data.ExecServer != nil {
				execServer = phase.Data.ExecServer
			} else {
				execServer = phase.Data.Server
			}
		}

		execWhere := CanRunLocally
		if execServer != nil {
			execWhere, err = canExecuteOnServer(ctx, *execServer, f.Runner, f.FieldLogger)
			if err != nil {
				return trace.Wrap(err)
			}
		}

		switch execWhere {
		case CanRunLocally:
			message := fmt.Sprintf("Rolling back %q locally", phase.ID)
			if p.DryRun {
				p.Progress.NextStep("[DRY-RUN] %v", message)
				return nil
			}
			p.Progress.NextStep(message)
			return f.rollbackPhaseLocally(ctx, p, *phase)

		case CanRunRemotely:
			message := fmt.Sprintf("Rolling back %q on node %v", phase.ID, execServer.Hostname)
			if p.DryRun {
				p.Progress.NextStep("[DRY-RUN] %v", message)
				return nil
			}
			p.Progress.NextStep(message)
			return f.rollbackPhaseRemotely(ctx, p, *phase, *execServer)

		default:
			return trace.BadParameter(
				`Node %[1]v does not appear to have an upgrade agent running so phase %[2]q rollback cannot be performed remotely from this node.
You can redeploy upgrade agents on all cluster nodes using "./gravity agent deploy", or execute "./gravity plan rollback --phase=%[2]v" directly from %[1]v."`,
				execServer.Hostname, p.PhaseID)
		}

		return nil
	}
	for i := len(phase.Phases) - 1; i >= 0; i-- {
		p.PhaseID = phase.Phases[i].ID
		err = f.RollbackPhase(ctx, p)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (f *FSM) rollbackPhaseRemotely(ctx context.Context, p Params, phase storage.OperationPhase, server storage.Server) error {
	return f.RunCommand(ctx, f.Runner, server, Params{
		PhaseID:  p.PhaseID,
		Force:    p.Force,
		Resume:   p.Resume,
		Rollback: true,
		Progress: p.Progress,
	})
}

// SetPreExec sets the hook that's called before phase execution
func (f *FSM) SetPreExec(fn PhaseHookFn) {
	f.preExecFn = fn
}

// SetPostExec sets the hook that's called after phase execution
func (f *FSM) SetPostExec(fn PhaseHookFn) {
	f.postExecFn = fn
}

// Close releases all FSM resources
func (f *FSM) Close() error {
	return trace.Wrap(f.Runner.Close())
}

func (f *FSM) executePhase(ctx context.Context, p Params, phase storage.OperationPhase) error {
	if phase.Executor == "" && len(phase.Phases) != 0 {
		if p.Force {
			return trace.BadParameter("cannot use force with composite phase %q, please only use --force on a single phase", phase.ID)
		}
		// Always execute a composite phase locally
		return trace.Wrap(f.executePhaseLocally(ctx, p, phase))
	}

	// Choose server to execute phase on
	var execServer *storage.Server
	if phase.Data != nil {
		if phase.Data.ExecServer != nil {
			execServer = phase.Data.ExecServer
		} else {
			execServer = phase.Data.Server
		}
	}

	var err error
	execWhere := CanRunLocally
	if execServer != nil {
		execWhere, err = canExecuteOnServer(ctx, *execServer, f.Runner, f.FieldLogger)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	switch execWhere {
	case ShouldRunRemotely:
		err = trace.NotFound("no agent is running on node %v, please execute phase %q locally on that node",
			serverName(*execServer), phase.ID)

	case CanRunLocally:
		err = trace.Wrap(f.executePhaseLocally(ctx, p, phase))

	case CanRunRemotely:
		err = trace.Wrap(f.executePhaseRemotely(ctx, p, phase, *execServer))
		if err == nil {
			// if the remote upgrade phase is successful, we need to mark it in our local database
			// because etcd might not be available to synchronize the changes back to us
			err = f.ChangePhaseState(ctx, StateChange{
				Phase: phase.ID,
				State: storage.OperationPhaseStateCompleted,
			})
		}

	default:
		err = trace.BadParameter("unsupported execution location: %v", execWhere)
	}

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// executePhaseRemotely executes the specified operation phase on the specified server
func (f *FSM) executePhaseRemotely(ctx context.Context, p Params, phase storage.OperationPhase, server storage.Server) error {
	if phase.HasSubphases() {
		return trace.BadParameter(
			"phase %v has subphases and should not be executed remotely", phase.ID)
	}

	p.Progress.NextStep("Executing %q on remote node %v", phase.ID,
		server.Hostname)

	return f.RunCommand(ctx, f.Runner, server, p)
}

// executePhaseLocally executes the specified operation phase on this server
func (f *FSM) executePhaseLocally(ctx context.Context, p Params, phase storage.OperationPhase) error {
	if !phase.HasSubphases() {
		p.Progress.NextStep("Executing %q locally", phase.ID)
		return trace.Wrap(f.executeOnePhase(ctx, p, phase))
	}
	if phase.Parallel {
		return trace.Wrap(f.executeSubphasesConcurrently(ctx, p, phase))
	}
	return trace.Wrap(f.executeSubphasesSequentially(ctx, p, phase))
}

func (f *FSM) executeSubphasesSequentially(ctx context.Context, p Params, phase storage.OperationPhase) error {
	for _, subphase := range phase.Phases {
		p.PhaseID = subphase.ID
		err := f.ExecutePhase(ctx, p)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (f *FSM) executeSubphasesConcurrently(ctx context.Context, p Params, phase storage.OperationPhase) error {
	errorsCh := make(chan error, len(phase.Phases))
	for _, subphase := range phase.Phases {
		go func(p Params, subphase storage.OperationPhase) {
			p.PhaseID = subphase.ID
			err := f.ExecutePhase(ctx, p)
			if err != nil {
				logrus.WithFields(logrus.Fields{
					logrus.ErrorKey: err,
					"phase":         p.PhaseID,
				}).Warn("Failed to execute phase.")
			}
			errorsCh <- trace.Wrap(err, "failed to execute phase %q", p.PhaseID)
		}(p, subphase)
	}
	return utils.CollectErrors(ctx, errorsCh)
}

func (f *FSM) executeOnePhase(ctx context.Context, p Params, phase storage.OperationPhase) error {
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	executor, err := f.GetExecutor(ExecutorParams{
		Plan:     *plan,
		Phase:    phase,
		Progress: p.Progress,
	}, f)
	if err != nil {
		return trace.Wrap(err)
	}
	logger := executor.WithField("phase", phase.ID)
	err = executor.PreCheck(ctx)
	if err != nil {
		logger.WithError(err).Error("Phase precheck failed.")
		return trace.Wrap(err)
	}

	err = f.ChangePhaseState(ctx,
		StateChange{
			Phase: phase.ID,
			State: storage.OperationPhaseStateInProgress,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	logger.Info("Executing phase.")

	err = executor.Execute(ctx)
	if err != nil {
		logger.WithError(err).Error("Phase execution failed.")
		if err := f.ChangePhaseState(ctx,
			StateChange{
				Phase: phase.ID,
				State: storage.OperationPhaseStateFailed,
				Error: trace.Wrap(err),
			}); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(err)
	}

	err = executor.PostCheck(ctx)
	if err != nil {
		logger.WithError(err).Error("Phase postcheck failed.")
		return trace.Wrap(err)
	}

	err = f.ChangePhaseState(ctx,
		StateChange{
			Phase: phase.ID,
			State: storage.OperationPhaseStateCompleted,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (f *FSM) rollbackPhaseLocally(ctx context.Context, p Params, phase storage.OperationPhase) error {
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	executor, err := f.GetExecutor(ExecutorParams{
		Plan:     *plan,
		Phase:    phase,
		Progress: p.Progress,
	}, f)
	if err != nil {
		return trace.Wrap(err)
	}

	err = f.ChangePhaseState(ctx,
		StateChange{
			Phase: phase.ID,
			State: storage.OperationPhaseStateInProgress,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	err = executor.Rollback(ctx)
	if err != nil {
		executor.Errorf("Phase %v rollback failed: %v.", phase.ID, err)
		if err := f.ChangePhaseState(ctx,
			StateChange{
				Phase: phase.ID,
				State: storage.OperationPhaseStateFailed,
				Error: trace.Wrap(err),
			}); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(err)
	}

	err = f.ChangePhaseState(ctx,
		StateChange{
			Phase: phase.ID,
			State: storage.OperationPhaseStateRolledBack,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// prerequisitesComplete checks if specified phase can be executed in the
// provided plan
func (f *FSM) prerequisitesComplete(phaseID string) error {
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	allPhases := FlattenPlan(plan)
	for phaseID != path.Dir(phaseID) {
		phase, err := FindPhase(plan, phaseID)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, required := range phase.Requires {
			for _, p := range allPhases {
				if p.ID == required && !p.IsCompleted() {
					return trace.BadParameter(
						"required phase %q is not completed", p.ID)
				}
			}
		}
		phaseID = path.Dir(phaseID)
	}
	return nil
}

// StateChange represents phase state transition
type StateChange struct {
	// Phase is the id of the phase that changes state
	Phase string
	// State is the new phase state
	State string
	// Error is the error that happened during phase execution
	Error trace.Error
}

// String returns a textual representation of this state change
func (c StateChange) String() string {
	if c.Error != nil {
		return fmt.Sprintf("StateChange(Phase=%v, State=%v, Error=%v)",
			c.Phase, c.State, c.Error)
	}
	return fmt.Sprintf("StateChange(Phase=%v, State=%v)",
		c.Phase, c.State)
}

// RootPhase is the name of the top-level phase
const RootPhase = "/"
