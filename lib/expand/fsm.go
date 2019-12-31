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

package expand

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
)

// FSMConfig is the expand FSM configuration
type FSMConfig struct {
	// OperationKey is the key of the join operation
	OperationKey ops.SiteOperationKey
	// Operator is operator of the cluster the node is joining to
	Operator ops.Operator
	// Apps is apps service of the cluster the node is joining to
	Apps app.Applications
	// Packages is package service of the cluster the node is joining to
	Packages pack.PackageService
	// LocalBackend is local backend of the joining node
	LocalBackend storage.Backend
	// LocalApps is local apps service of the joining node
	LocalApps app.Applications
	// LocalPackages is local package service of the joining node
	LocalPackages *localpack.PackageServer
	// JoinBackend is the local backend that stores join-specific data
	JoinBackend storage.Backend
	// Spec is the FSM spec
	Spec fsm.FSMSpecFunc
	// Credentials is the credentials for gRPC agents
	Credentials credentials.TransportCredentials
	// Runner is optional runner to use when running remote commands
	Runner rpc.AgentRepository
	// DebugMode turns on FSM debug mode
	DebugMode bool
	// Insecure turns on FSM insecure mode
	Insecure bool
}

// CheckAndSetDefaults validates expand FSM configuration and sets defaults
func (c *FSMConfig) CheckAndSetDefaults() error {
	err := c.OperationKey.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	if c.Operator == nil {
		return trace.BadParameter("missing Operator")
	}
	if c.Apps == nil {
		return trace.BadParameter("missing Apps")
	}
	if c.Packages == nil {
		return trace.BadParameter("missing Packages")
	}
	if c.LocalBackend == nil {
		return trace.BadParameter("missing LocalBackend")
	}
	if c.LocalApps == nil {
		return trace.BadParameter("missing LocalApps")
	}
	if c.LocalPackages == nil {
		return trace.BadParameter("missing LocalPackages")
	}
	if c.Credentials == nil {
		c.Credentials, err = install.ClientCredentials(c.Packages)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if c.Runner == nil {
		c.Runner = fsm.NewAgentRunner(c.Credentials)
	}
	if c.Spec == nil {
		c.Spec = FSMSpec(*c)
	}
	return nil
}

// NewFSM returns a new state machine for expand operation
func NewFSM(config FSMConfig) (*fsm.FSM, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := logrus.WithField(trace.Component, "fsm:join")
	engine := &fsmEngine{
		FSMConfig:   config,
		FieldLogger: logger,
	}
	fsm, err := fsm.New(fsm.Config{
		Engine: engine,
		Runner: config.Runner,
		Logger: logger,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fsm.SetPreExec(engine.UpdateProgress)
	return fsm, nil
}

// fsmEngine is the expand FSM engine
type fsmEngine struct {
	// FSMConfig is the expand FSM configuration
	FSMConfig
	// FieldLogger is used for logging
	logrus.FieldLogger
}

// GetExecutor returns a new executor based on the provided parameters
func (e *fsmEngine) GetExecutor(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    e.Operator,
	}
	executor, err := e.Spec(p, remote)
	if err != nil {
		logger.Warnf("Failed to initialize phase: %v.", err)
		return nil, trace.Wrap(err)
	}
	return executor, nil
}

// ChangePhaseState updates the phase state based on the provided parameters
func (e *fsmEngine) ChangePhaseState(ctx context.Context, change fsm.StateChange) error {
	logger := e.WithField("change", change)
	planChange := storage.PlanChange{
		ID:          uuid.New(),
		ClusterName: e.OperationKey.SiteDomain,
		OperationID: e.OperationKey.OperationID,
		PhaseID:     change.Phase,
		NewState:    change.State,
		Error:       utils.ToRawTrace(change.Error),
		Created:     time.Now().UTC(),
	}
	_, err := e.JoinBackend.CreateOperationPlanChange(planChange)
	if err != nil {
		return trace.Wrap(err)
	}
	err = e.Operator.CreateOperationPlanChange(e.OperationKey, planChange)
	if err != nil {
		logger.WithError(err).Warn("Failed to create changelog entry.")
	}
	logger.Debug("Applied.")
	return nil
}

// GetPlan returns the up-to-date operation plan
func (e *fsmEngine) GetPlan() (*storage.OperationPlan, error) {
	return fsm.GetOperationPlan(e.JoinBackend, e.OperationKey)
}

// RunCommand executes the phase specified by params on the specified
// server using the provided runner
func (e *fsmEngine) RunCommand(ctx context.Context, runner rpc.RemoteRunner, node storage.Server, p fsm.Params) error {
	args := []string{"plan", "execute",
		"--phase", p.PhaseID,
		"--operation-id", p.OperationID,
	}
	if e.DebugMode {
		args = append(args, "--debug")
	}
	if e.Insecure {
		args = append(args, "--insecure")
	}
	if p.Force {
		args = append(args, "--force")
	}
	return runner.Run(ctx, node, args...)
}

// Complete is called to mark operation complete
func (e *fsmEngine) Complete(fsmErr error) error {
	plan, err := e.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	if fsm.IsCompleted(plan) {
		err = ops.CompleteOperation(e.OperationKey, e.Operator)
	} else {
		var message string
		if fsmErr != nil {
			message = trace.Unwrap(fsmErr).Error()
		}
		err = ops.FailOperation(e.OperationKey, e.Operator, message)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	e.WithFields(logrus.Fields{
		constants.FieldSuccess: fsm.IsCompleted(plan),
		constants.FieldError:   fsmErr,
	}).Debug("Marked operation complete.")
	return nil
}

// UpdateProgress reports operation progress to the cluster's operator
func (e *fsmEngine) UpdateProgress(ctx context.Context, p fsm.Params) error {
	plan, err := e.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	phase, err := fsm.FindPhase(plan, p.PhaseID)
	if err != nil {
		return trace.Wrap(err)
	}
	entry := ops.ProgressEntry{
		SiteDomain:  e.OperationKey.SiteDomain,
		OperationID: e.OperationKey.OperationID,
		Completion:  100 / len(fsm.FlattenPlan(plan)) * phase.Step,
		Step:        phase.Step,
		State:       ops.ProgressStateInProgress,
		Message:     phase.Description,
		Created:     time.Now().UTC(),
	}
	err = e.Operator.CreateProgressEntry(e.OperationKey, entry)
	if err != nil {
		e.Warnf("Failed to create progress entry %v: %v.", entry,
			trace.DebugReport(err))
	}
	return nil
}
