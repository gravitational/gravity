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
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/logrus"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"google.golang.org/grpc/credentials"
)

// FSMConfig is the expand FSM configuration
type FSMConfig struct {
	// Operator is operator of the cluster the node is joining to
	Operator ops.Operator
	// OperationKey is the key of the join operation
	OperationKey ops.SiteOperationKey
	// Apps is apps service of the cluster the node is joining to
	Apps app.Applications
	// Packages is package service of the cluster the node is joining to
	Packages pack.PackageService
	// LocalBackend is local backend of the joining node
	LocalBackend storage.Backend
	// LocalApps is local apps service of the joining node
	LocalApps app.Appliations
	// LocalPackages is local package service of the joining node
	LocalPackages pack.PackageService
	// Etcd is client to the cluster's etcd members API
	Etcd etcd.MembersAPI
	// Spec is the FSM spec
	Spec fsm.FSMSpecFunc
	// Credentials is the credentials for gRPC agents
	Credentials credentials.TransportCredentials
	// Debug turns on FSM debug mode
	Debug bool
	// Insecure turns on FSM insecure mode
	Insecure bool
}

// CheckAndSetDefaults validates expand FSM configuration and sets defaults
func (c *FSMConfig) CheckAndSetDefaults() error {
	if c.Operator == nil {
		return trace.BadParameter("missing Operator")
	}
	if err := c.OperationKey.Check(); err != nil {
		return trace.Wrap(err)
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
	if c.Etcd == nil {
		return trace.BadParameter("missing Etcd")
	}
	if c.Spec == nil {
		c.Spec = FSMSpec(*c)
	}
	if c.Credentials == nil {
		c.Credentials, err = rpc.ClientCredentials(defaults.RPCAgentSecretsDir)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NewFSM returns a new state machine for expand operation
func NewFSM(config FSMConfig) (*fsm.FSM, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := config.Operator.GetSiteOperation(config.OperationKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if operation.Type != ops.OperationExpand {
		return nil, trace.BadParameter("not an expand operation: %v", operation)
	}
	logger := logrus.WithField(trace.Component, "fsm:join")
	engine := &fsmEngine{
		FSMConfig:   config,
		FieldLogger: logger,
	}
	fsm, err := fsm.New(fsm.Config{
		Engine: engine,
		Runner: fsm.NewAgentRunner(config.Credentials),
		Logger: logger,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fsm.SetPreExec(engine.UpdateProgress)
	return fs, nil
}

// fsmEngine is the expand FSM engine
type fsmEngine struct {
	// FSMConfig is the expand FSM configuration
	FSMConfig
	// operation is the ongoing expand operation
	operation ops.SiteOperation
	// FieldLogger is used for logging
	logrus.FieldLogger
}

// GetExecutor returns a new executor based on the provided parameters
func (e *fsmEngine) GetExecutor(p ExecutorParams, remote Remote) (PhaseExecutor, error) {
	return e.Spec(p, remote)
}

// ChangePhaseState updates the phase state based on the provided parameters
func (e *fsmEngine) ChangePhaseState(ctx context.Context, change fsm.StateChange) error {
	err := e.Operator.CreateOperationPlanChange(e.OperationKey, storage.PlanChange{
		ID:          uuid.New(),
		ClusterName: e.OperationKey.SiteDomain,
		OperationID: e.OperationKey.OperationID,
		PhaseID:     change.Phase,
		NewState:    change.State,
		Error:       utils.ToRawTrace(change.Error),
		Created:     time.Now().UTC(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	e.Debugf("Applied %s.", change)
	return nil
}

// GetPlan returns the up-to-date operation plan
func (e *fsmEngine) GetPlan() (*storage.OperationPlan, error) {
	return e.Operator.GetOperationPlan(e.OperationKey)
}

// RunCommand executes the phase specified by params on the specified
// server using the provided runner
func (e *fsmEngine) RunCommand(ctx context.Context, runner fsm.RemoteRunner, node storage.Server, p fsm.Params) error {
	args := []string{"join", "--phase", p.PhaseID, fmt.Sprintf("--force=%v", p.Force)}
	if e.Debug {
		args = append([]string{"--debug"}, args...)
	}
	if e.Insecure {
		args = append([]string{"--insecure"}, args...)
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
		err = ops.FailOperation(e.OperationKey, e.Operator, trace.Unwrap(fsmErr).Error())
	}
	if err != nil {
		return trace.Wrap(err)
	}
	e.Debug("Marked operation complete.")
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
		Completion:  100 / utils.Max(len(plan.Phases), 1) * phase.Step,
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
