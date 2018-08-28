package fsm

import (
	"context"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/app"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	libpack "github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	libphase "github.com/gravitational/gravity/lib/vacuum/internal/phases"
	"github.com/gravitational/gravity/lib/vacuum/prune/pack"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// New returns a new state machine for garbage collection
func New(config Config) (*libfsm.FSM, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	engine := &engine{
		Config: config,
	}
	machine, err := libfsm.New(libfsm.Config{
		Engine: engine,
		Runner: config.Runner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	machine.SetPreExec(engine.UpdateProgress)
	return machine, nil
}

// Check validates install FSM config and sets some defaults
func (r *Config) checkAndSetDefaults() (err error) {
	if r.App == nil {
		return trace.BadParameter("application package is required")
	}
	if r.Operation == nil {
		return trace.BadParameter("operation is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("package service is required")
	}
	if r.Apps == nil {
		return trace.BadParameter("application service is required")
	}
	if r.Operator == nil {
		return trace.BadParameter("operator service is required")
	}
	if r.Spec == nil {
		r.Spec = configToExecutor(*r)
	}
	return nil
}

// Config describes configuration of the cluster garbage collector
type Config struct {
	// Operation references the active garbage collection operation
	Operation *ops.SiteOperation
	// Packages is the cluster package service
	Packages libpack.PackageService
	// App references the cluster application
	App *pack.Application
	// RemoteApps lists optional applications from remote clusters
	RemoteApps []pack.Application
	// Apps is the cluster application service
	Apps app.Applications
	// Operator is the cluster operator service
	Operator ops.Operator
	// LocalPackages is the machine-local pack service
	LocalPackages libpack.PackageService
	// RuntimePath is the path to the runtime container's rootfs
	RuntimePath string
	// FieldLogger is the logger
	log.FieldLogger
	// Spec specifies the function that resolves to an executor
	Spec libfsm.FSMSpecFunc
	// Runner specifies the remote command runner
	Runner libfsm.RemoteRunner
	// Silent controls whether the process outputs messages to stdout
	localenv.Silent
	// Emitter outputs progress messages to stdout
	utils.Emitter
}

// UpdateProgress creates an appropriate progress entry in the operator
func (r *engine) UpdateProgress(ctx context.Context, params libfsm.Params) error {
	plan, err := r.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}

	phase, err := libfsm.FindPhase(plan, params.PhaseID)
	if err != nil {
		return trace.Wrap(err)
	}

	key := r.Operation.Key()
	entry := ops.ProgressEntry{
		SiteDomain:  key.SiteDomain,
		OperationID: key.OperationID,
		Completion:  100 / utils.Max(len(plan.Phases), 1) * phase.Step,
		Step:        phase.Step,
		State:       ops.ProgressStateInProgress,
		Message:     phase.Description,
		Created:     time.Now().UTC(),
	}
	err = r.Operator.CreateProgressEntry(key, entry)
	if err != nil {
		r.Warnf("Failed to create progress entry %v: %v.", entry,
			trace.DebugReport(err))
	}
	return nil
}

// Complete marks the operation as either completed or failed based
// on the state of the operation plan
func (r *engine) Complete(fsmErr error) error {
	plan, err := r.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}

	if libfsm.IsCompleted(plan) {
		err = ops.CompleteOperation(r.Operation.Key(), r.Operator)
	} else {
		err = ops.FailOperation(r.Operation.Key(), r.Operator, trace.Unwrap(fsmErr).Error())
	}
	if err != nil {
		return trace.Wrap(err)
	}

	r.Debug("Marked operation complete.")
	return nil
}

// ChangePhaseState creates an new changelog entry
func (r *engine) ChangePhaseState(ctx context.Context, change libfsm.StateChange) error {
	err := r.Operator.CreateOperationPlanChange(r.Operation.Key(),
		storage.PlanChange{
			ID:          uuid.New(),
			ClusterName: r.Operation.SiteDomain,
			OperationID: r.Operation.ID,
			PhaseID:     change.Phase,
			NewState:    change.State,
			Error:       utils.ToRawTrace(change.Error),
			Created:     time.Now().UTC(),
		})
	if err != nil {
		return trace.Wrap(err)
	}

	r.Debugf("Applied %v.", change)
	return nil
}

// GetExecutor returns the appropriate phase executor based on the
// provided parameters
func (r *engine) GetExecutor(params libfsm.ExecutorParams, remote libfsm.Remote) (libfsm.PhaseExecutor, error) {
	return r.Spec(params, remote)
}

// RunCommand executes the phase specified by params on the specified server
// using the provided runner
func (r *engine) RunCommand(ctx context.Context, runner libfsm.RemoteRunner, server storage.Server, params libfsm.Params) error {
	args := []string{"gc", "--phase", params.PhaseID}
	if params.Force {
		args = append(args, "--force")
	}
	return runner.Run(ctx, server, args...)
}

// GetPlan returns the most up-to-date operation plan
func (r *engine) GetPlan() (*storage.OperationPlan, error) {
	plan, err := r.Operator.GetOperationPlan(r.Operation.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

// engine is the garbage collection engine
type engine struct {
	// Config is the collector's configuration
	Config
	localenv.Silent
}

// configToExecutor returns a function that maps configuration and a set of parameters
// to a phase executor
func configToExecutor(config Config) libfsm.FSMSpecFunc {
	return func(params libfsm.ExecutorParams, remote libfsm.Remote) (libfsm.PhaseExecutor, error) {
		switch {
		case strings.HasPrefix(params.Phase.ID, libphase.Journal):
			return libphase.NewJournal(params, config.RuntimePath, config.Emitter)

		case params.Phase.ID == libphase.ClusterPackages:
			return libphase.NewPackages(
				params,
				*config.App,
				config.RemoteApps,
				config.Packages,
				config.Emitter)

		case strings.HasPrefix(params.Phase.ID, libphase.Packages):
			return libphase.NewPackages(
				params,
				*config.App,
				config.RemoteApps,
				config.LocalPackages,
				config.Emitter)

		case strings.HasPrefix(params.Phase.ID, libphase.Registry):
			return libphase.NewRegistry(
				params,
				config.App.Locator,
				config.Apps,
				config.Packages,
				config.Emitter)

		default:
			return nil, trace.BadParameter("unknown phase %q", params.Phase.ID)
		}
	}
}
