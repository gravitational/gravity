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

package install

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
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

// fsmEngine is the install FSM engine
type fsmEngine struct {
	// FSMConfig is the install FSM config
	FSMConfig
	// FieldLogger is used for logging
	logrus.FieldLogger
	// operation is the current operation
	operation *ops.SiteOperation
}

// FSMConfig is the install FSM config
type FSMConfig struct {
	// OperationKey is the install operation key
	OperationKey ops.SiteOperationKey
	// Packages is authenticated installer pack client
	Packages pack.PackageService
	// Apps is authenticated installer apps client
	Apps app.Applications
	// Operator is authenticated installer ops client
	Operator ops.Operator
	// LocalClusterClient is a factory for creating a client to the installed cluster.
	LocalClusterClient func(...httplib.ClientOption) (*opsclient.Client, error)
	// LocalPackages is the machine-local pack service
	LocalPackages *localpack.PackageServer
	// LocalApps is the machine-local apps service
	LocalApps app.Applications
	// LocalBackend is the machine-local backend
	LocalBackend storage.LocalBackend
	// Spec is the FSM spec
	Spec fsm.FSMSpecFunc
	// Credentials is the credentials for gRPC agents
	Credentials credentials.TransportCredentials
	// Insecure allows to turn off cert validation in dev mode
	Insecure bool
	// UserLogFile is the user-friendly install log file
	UserLogFile string
	// ReportProgress controls whether engine should report progress to Operator
	ReportProgress bool
	// DNSConfig specifies the DNS configuration to use
	DNSConfig storage.DNSConfig
}

// CheckAndSetDefaults validates install FSM config and sets some defaults
func (c *FSMConfig) CheckAndSetDefaults() (err error) {
	err = c.OperationKey.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	if c.Packages == nil {
		return trace.BadParameter("missing Packages")
	}
	if c.Apps == nil {
		return trace.BadParameter("missing Apps")
	}
	if c.Operator == nil {
		return trace.BadParameter("missing Operator")
	}
	if c.LocalPackages == nil {
		return trace.BadParameter("missing LocalPackages")
	}
	if c.LocalClusterClient == nil {
		return trace.BadParameter("missing LocalClusterClient")
	}
	if c.LocalApps == nil {
		return trace.BadParameter("missing LocalApps")
	}
	if c.LocalBackend == nil {
		return trace.BadParameter("missing LocalBackend")
	}
	if c.Spec == nil {
		c.Spec = FSMSpec(*c)
	}
	if c.Credentials == nil {
		c.Credentials, err = ClientCredentials(c.Packages)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// NewFSM returns a new install FSM instance
func NewFSM(config FSMConfig) (*fsm.FSM, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	op, err := config.Operator.GetSiteOperation(config.OperationKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if op.Type != ops.OperationInstall && op.Type != ops.OperationReconfigure {
		return nil, trace.BadParameter("expected %v to be install or reconfigure operation, not %v",
			config.OperationKey, op.Type)
	}
	logger := logrus.WithFields(logrus.Fields{
		trace.Component:            "fsm:install",
		constants.FieldOperationID: config.OperationKey.OperationID,
	})
	engine := &fsmEngine{
		FSMConfig:   config,
		FieldLogger: logger,
		operation:   op,
	}
	runner := fsm.NewAgentRunner(config.Credentials)
	fsm, err := fsm.New(fsm.Config{
		Engine:   engine,
		Runner:   runner,
		Insecure: config.Insecure,
		Logger:   logger,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// if this machine should report progress, set the hook that will
	// be submitting progress entries to the configured Operator when
	// a phase executes
	if config.ReportProgress {
		fsm.SetPreExec(engine.UpdateProgress)
	}
	return fsm, nil
}

// UpdateProgress creates an appropriate progress entry in the operator
func (f *fsmEngine) UpdateProgress(ctx context.Context, p fsm.Params) error {
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	phase, err := fsm.FindPhase(*plan, p.PhaseID)
	if err != nil {
		return trace.Wrap(err)
	}
	entry := ops.ProgressEntry{
		SiteDomain:  f.OperationKey.SiteDomain,
		OperationID: f.OperationKey.OperationID,
		Completion:  100 / utils.Max(len(plan.Phases), 1) * phase.Step,
		Step:        phase.Step,
		State:       ops.ProgressStateInProgress,
		Message:     phase.Description,
		Created:     time.Now().UTC(),
	}
	err = f.Operator.CreateProgressEntry(f.OperationKey, entry)
	if err != nil {
		f.Warnf("Failed to create progress entry %v: %v.", entry,
			trace.DebugReport(err))
	}
	return nil
}

// Complete marks the install operation as either completed or failed based
// on the state of the operation plan
func (f *fsmEngine) Complete(ctx context.Context, fsmErr error) error {
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	if fsmErr == nil {
		fsmErr = trace.Errorf("completed manually")
	}
	return fsm.CompleteOrFailOperation(ctx, *plan, f.Operator, fsmErr.Error())
}

// ChangePhaseState creates an operation plan changelog entry
func (f *fsmEngine) ChangePhaseState(ctx context.Context, change fsm.StateChange) error {
	err := f.Operator.CreateOperationPlanChange(f.operation.Key(),
		storage.PlanChange{
			ID:          uuid.New(),
			ClusterName: f.operation.SiteDomain,
			OperationID: f.operation.ID,
			PhaseID:     change.Phase,
			NewState:    change.State,
			Error:       utils.ToRawTrace(change.Error),
			Created:     time.Now().UTC(),
		})
	if err != nil {
		return trace.Wrap(err)
	}
	f.Debugf("Applied %s.", change)
	return nil
}

// GetExecutor returns the appropriate install phase executor based on the
// provided parameters
func (f *fsmEngine) GetExecutor(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    f.Operator,
	}
	executor, err := f.Spec(p, remote)
	if err != nil {
		logger.Warnf("Failed to initialize phase: %v.", err)
		return nil, trace.Wrap(err)
	}
	return executor, nil
}

// RunCommand executes the phase specified by params on the specified server
// using the provided runner
func (f *fsmEngine) RunCommand(ctx context.Context, runner rpc.RemoteRunner, server storage.Server, p fsm.Params) error {
	args := []string{"plan", "execute",
		"--phase", p.PhaseID,
		"--operation-id", p.OperationID,
	}
	if p.Force {
		args = append(args, "--force")
	}
	if f.Insecure {
		args = append(args, "--debug", "--insecure")
	}
	return runner.Run(ctx, server, args...)
}

// GetPlan returns the most up-to-date operation plan
func (f *fsmEngine) GetPlan() (*storage.OperationPlan, error) {
	plan, err := f.Operator.GetOperationPlan(f.operation.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}
