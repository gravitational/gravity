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
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	libphase "github.com/gravitational/gravity/lib/environ/internal/phases"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// New returns a new state machine for updating cluster environment variables
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

// Check validates this configuration and sets defaults where necessary
func (r *Config) checkAndSetDefaults() (err error) {
	if r.Operation == nil {
		return trace.BadParameter("operation is required")
	}
	if r.Operator == nil {
		return trace.BadParameter("operator service is required")
	}
	if r.Spec == nil {
		r.Spec = configToExecutor(*r)
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

// Config describes configuration for updating cluster environment variables
type Config struct {
	// Operation references the active garbage collection operation
	Operation *ops.SiteOperation
	// Operator is the cluster operator service
	Operator ops.Operator
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
	args := []string{"system", "envars", "--phase", params.PhaseID}
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

// engine is the updater engine
type engine struct {
	Config
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
		switch {
		case strings.HasPrefix(params.Phase.ID, libphase.Masters),
			strings.HasPrefix(params.Phase.ID, libphase.Nodes):
			return libphase.NewSync(params, config.Emitter, logger)

		default:
			return nil, trace.BadParameter("unknown phase %q", params.Phase.ID)
		}
	}
}
