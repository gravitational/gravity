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
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	libphase "github.com/gravitational/gravity/lib/environ/internal/phases"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// New returns a new state machine for updating cluster environment variables
func New(ctx context.Context, config Config) (*libfsm.FSM, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reconciler := update.NewDefaultReconciler(config.Backend, config.LocalBackend,
		config.Operation.SiteDomain, config.Operation.ID, config.FieldLogger)
	plan, err := reconciler.ReconcilePlan(ctx, config.Plan)
	if err != nil {
		// This is not critical and will be retried during the operation
		config.WithError(err).Warn("Failed to reconcile operation plan.")
		plan = &config.Plan
	}
	engine := &engine{
		Config:     config,
		spec:       configToExecutor(config),
		operator:   config.Operator,
		reconciler: reconciler,
		plan:       *plan,
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

// Check validates this configuration and sets defaults where necessary
func (r *Config) checkAndSetDefaults() (err error) {
	if r.Operation == nil {
		return trace.BadParameter("operation is required")
	}
	if r.Operator == nil {
		return trace.BadParameter("operator service is required")
	}
	if r.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if r.LocalBackend == nil {
		return trace.BadParameter("local backend is required")
	}
	if r.ClusterPackages == nil {
		return trace.BadParameter("cluster package service is required")
	}
	if r.Runner == nil {
		return trace.BadParameter("remote command runner is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = &libfsm.Logger{
			FieldLogger: log.WithField(trace.Component, "environ"),
			Key:         r.Operation.Key(),
			Operator:    r.Operator,
		}
	}
	return nil
}

// Config describes configuration for updating cluster runtime environment variables
type Config struct {
	// FieldLogger is the logger
	log.FieldLogger
	// Operation references the active operation
	Operation *ops.SiteOperation
	// Operator is the cluster operator service
	Operator ops.Operator
	// Apps is the cluster application service
	Apps app.Applications
	// Backend specifies the cluster backend
	Backend storage.Backend
	// LocalBackend specifies the authorative backend that stores up-to-date
	// operation state. It will be synced with the Backend at phase boundaries
	// if it's available
	LocalBackend storage.Backend
	// ClusterPackages specifies the cluster package service
	ClusterPackages pack.PackageService
	// Client specifies the optional kubernetes client
	Client *kubernetes.Clientset
	// Plan specifies the actual operation plan
	Plan storage.OperationPlan
	// Runner specifies the remote command runner
	Runner libfsm.RemoteRunner
	// Silent controls whether the process outputs messages to stdout
	localenv.Silent
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
	err = r.operator.CreateProgressEntry(key, entry)
	if err != nil {
		r.WithFields(log.Fields{
			log.ErrorKey: err,
			"entry":      entry,
		}).Warn("Failed to create progress entry.")
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
		err = ops.CompleteOperation(r.Operation.Key(), r.operator)
	} else {
		var msg string
		if fsmErr != nil {
			msg = trace.Unwrap(fsmErr).Error()
		}
		err = ops.FailOperation(r.Operation.Key(), r.operator, msg)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	r.Debug("Marked operation complete.")
	return nil
}

// ChangePhaseState creates a new changelog entry
func (r *engine) ChangePhaseState(ctx context.Context, change libfsm.StateChange) error {
	err := r.operator.CreateOperationPlanChange(r.Operation.Key(),
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

	plan, err := r.reconciler.ReconcilePlan(ctx, r.plan)
	if err != nil {
		return trace.Wrap(err)
	}
	r.plan = *plan

	r.Debugf("Applied %v.", change)
	return nil
}

// GetExecutor returns the appropriate phase executor based on the
// provided parameters
func (r *engine) GetExecutor(params libfsm.ExecutorParams, remote libfsm.Remote) (libfsm.PhaseExecutor, error) {
	return r.spec(params, remote)
}

// RunCommand executes the phase specified by params on the specified server
// using the provided runner
func (r *engine) RunCommand(ctx context.Context, runner libfsm.RemoteRunner, server storage.Server, params libfsm.Params) error {
	args := []string{"plan", "execute",
		"--phase", params.PhaseID,
		"--operation-id", r.Operation.ID,
	}
	if params.Force {
		args = append(args, "--force")
	}
	return runner.Run(ctx, server, args...)
}

// GetPlan returns the most up-to-date operation plan
func (r *engine) GetPlan() (*storage.OperationPlan, error) {
	return &r.plan, nil
}

// engine is the updater engine
type engine struct {
	Config
	// spec specifies the function that resolves to an executor
	spec       libfsm.FSMSpecFunc
	reconciler update.Reconciler
	operator
	plan storage.OperationPlan
	localenv.Silent
}

// configToExecutor returns a function that maps configuration and a set of parameters
// to a phase executor
func configToExecutor(config Config) libfsm.FSMSpecFunc {
	return func(params libfsm.ExecutorParams, remote libfsm.Remote) (libfsm.PhaseExecutor, error) {
		logger := &libfsm.Logger{
			FieldLogger: log.WithFields(log.Fields{
				constants.FieldPhase: params.Phase.ID,
			}),
			Key:      params.Key(),
			Operator: config.Operator,
		}
		if params.Phase.Data != nil {
			logger.Server = params.Phase.Data.Server
		}
		switch params.Phase.Executor {
		case libphase.UpdateConfig:
			return libphase.NewUpdateConfig(params,
				config.Operator, *config.Operation, config.Apps, config.ClusterPackages,
				logger)
		case libphase.RestartContainer:
			return libphase.NewRestart(params, config.Operator, config.Apps, config.Operation.ID,
				logger)
		case libphase.Elections:
			return libphase.NewElections(params, config.Operator, logger)
		case libphase.Drain:
			return libphase.NewDrain(params, config.Client, logger)
		case libphase.Taint:
			return libphase.NewTaint(params, config.Client, logger)
		case libphase.Untaint:
			return libphase.NewUntaint(params, config.Client, logger)
		case libphase.Uncordon:
			return libphase.NewUncordon(params, config.Client, logger)
		case libphase.Endpoints:
			return libphase.NewEndpoints(params, config.Client, logger)

		default:
			return nil, trace.BadParameter("unknown executor %v for phase %q",
				params.Phase.Executor, params.Phase.ID)
		}
	}
}

// operator describes the subset of ops.Operator required for the fsm engine
type operator interface {
	CreateProgressEntry(ops.SiteOperationKey, ops.ProgressEntry) error
	CreateOperationPlanChange(ops.SiteOperationKey, storage.PlanChange) error
	GetOperationPlan(ops.SiteOperationKey) (*storage.OperationPlan, error)
	SetOperationState(ops.SiteOperationKey, ops.SetOperationStateRequest) error
}
