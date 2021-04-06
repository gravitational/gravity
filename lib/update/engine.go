/*
Copyright 2019 Gravitational, Inc.

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

// This file implements a generic FSM update engine
package update

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// NewMachine returns a new state machine for an update operation.
func NewMachine(ctx context.Context, config Config, engine *Engine) (*fsm.FSM, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	machine, err := fsm.New(fsm.Config{
		Engine: engine,
		Runner: config.Runner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	machine.SetPreExec(engine.UpdateProgress)
	return machine, nil
}

// NewEngine returns a new update engine using the given dispatcher to dispatch phases
func NewEngine(ctx context.Context, config Config, dispatcher Dispatcher) (*Engine, error) {
	plan, err := config.LocalBackend.GetOperationPlan(config.Operation.SiteDomain, config.Operation.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reconciler := NewDefaultReconciler(
		config.Backend, config.LocalBackend,
		config.Operation.SiteDomain,
		config.Operation.ID,
		config.FieldLogger)
	p, err := reconciler.ReconcilePlan(ctx, *plan)
	if err != nil {
		// This is not critical and will be retried during the operation
		config.WithError(err).Warn("Failed to reconcile operation plan.")
		p = plan
	}
	return &Engine{
		Config:     config,
		operator:   config.Operator,
		reconciler: reconciler,
		plan:       *p,
		dispatcher: dispatcher,
	}, nil
}

// UpdateProgress creates an appropriate progress entry in the operator
func (r *Engine) UpdateProgress(ctx context.Context, params fsm.Params) error {
	plan, err := r.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}

	phase, err := fsm.FindPhase(plan, params.PhaseID)
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
func (r *Engine) Complete(ctx context.Context, fsmErr error) error {
	plan, err := r.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	if fsmErr == nil {
		fsmErr = trace.Errorf("completed manually")
	}
	return fsm.CompleteOrFailOperation(ctx, plan, r.Operator, fsmErr.Error())
}

// ChangePhaseState creates a new changelog entry
func (r *Engine) ChangePhaseState(ctx context.Context, change fsm.StateChange) error {
	r.WithField("change", change).Debug("Apply.")
	_, err := r.LocalBackend.CreateOperationPlanChange(storage.PlanChange{
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
	return nil
}

// RunCommand executes the phase specified by params on the specified server
// using the provided runner
func (r *Engine) RunCommand(ctx context.Context, runner rpc.RemoteRunner, server storage.Server, params fsm.Params) error {
	command := "execute"
	if params.Rollback {
		command = "rollback"
	}
	args := []string{"plan", command,
		"--phase", params.PhaseID,
		"--operation-id", r.Operation.ID,
	}
	if params.Force {
		args = append(args, "--force")
	}
	return runner.Run(ctx, server, args...)
}

// GetPlan returns the most up-to-date operation plan
func (r *Engine) GetPlan() (*storage.OperationPlan, error) {
	return &r.plan, nil
}

// GetExecutor returns a new executor based on the provided parameters
func (r *Engine) GetExecutor(params fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
	return r.dispatcher.Dispatch(params, remote)
}

// Engine is the updater engine
type Engine struct {
	// Config specifies engine configuration
	Config
	// Silent controls whether the log output is verbose
	localenv.Silent
	reconciler Reconciler
	operator
	plan       storage.OperationPlan
	dispatcher Dispatcher
}

// Dispatcher routes the set of execution parameters to a specific operation phase
type Dispatcher interface {
	// Dispatch returns an executor for the given parameters and the specified remote
	Dispatch(fsm.ExecutorParams, fsm.Remote) (fsm.PhaseExecutor, error)
}

// operator describes the subset of ops.Operator required for the FSM engine
type operator interface {
	CreateProgressEntry(ops.SiteOperationKey, ops.ProgressEntry) error
	SetOperationState(context.Context, ops.SiteOperationKey, ops.SetOperationStateRequest) error
	ActivateSite(ops.ActivateSiteRequest) error
}
