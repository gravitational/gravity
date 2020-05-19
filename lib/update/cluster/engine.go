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

package cluster

import (
	"bytes"
	"context"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// New returns new updater for the specified configuration
func New(ctx context.Context, config Config) (*update.Updater, error) {
	machine, err := newMachine(ctx, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updater, err := update.NewUpdater(ctx, config.Config, machine)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

// checkAndSetDefaults validates FSM config and sets defaults
func (c *Config) checkAndSetDefaults() error {
	if err := c.Config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Spec == nil {
		c.Spec = fsmSpec(*c)
	}
	return nil
}

// Config is the FSM configuration
type Config struct {
	update.Config
	// HostLocalBackend is the host-local backend that stores bootstrap configuration
	// like DNS, logins etc.
	HostLocalBackend storage.Backend
	// Packages is the local package service
	Packages pack.PackageService
	// ClusterPackages is the package service that talks to cluster API
	ClusterPackages pack.PackageService
	// HostLocalPackages is the host-local package service that contains package
	// metadata used for updates
	HostLocalPackages update.LocalPackageService
	// Apps is the cluster apps service
	Apps app.Applications
	// Client is the cluster Kubernetes client
	Client *kubernetes.Clientset
	// Users is the cluster identity service
	Users users.Identity
	// Spec is used to retrieve a phase executor, allows
	// plugging different phase executors during tests
	Spec fsm.FSMSpecFunc
}

// newMachine returns a new FSM instance
func newMachine(ctx context.Context, c Config) (*fsm.FSM, error) {
	err := c.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := logrus.WithFields(logrus.Fields{
		trace.Component: "fsm:update",
	})
	engine, err := newEngine(ctx, c, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fsm, err := fsm.New(fsm.Config{
		Engine: engine,
		Logger: logger,
		Runner: c.Runner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fsm, nil
}

// engine is the update FSM engine
type engine struct {
	// Config is the state machine configuration
	Config
	// FieldLogger is used for logging
	logrus.FieldLogger
	// plan is the update operation plan
	plan       storage.OperationPlan
	reconciler update.Reconciler
}

// newEngine returns a new instance of FSM engine for update
func newEngine(ctx context.Context, config Config, logger logrus.FieldLogger) (*engine, error) {
	plan, err := loadPlan(config.LocalBackend, config.Operation.Key(), logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reconciler := update.NewDefaultReconciler(
		config.Backend, config.LocalBackend,
		plan.ClusterName, plan.OperationID,
		logger)
	reconciledPlan, err := reconciler.ReconcilePlan(ctx, *plan)
	if err != nil {
		// This is not critical and will be retried during the operation
		logger.WithError(err).Warn("Failed to reconcile operation plan.")
		reconciledPlan = plan
	}
	engine := &engine{
		Config: config,
		FieldLogger: &fsm.Logger{
			Operator:    config.Operator,
			Key:         fsm.OperationKey(*plan),
			FieldLogger: logger,
		},
		plan:       *reconciledPlan,
		reconciler: reconciler,
	}
	return engine, nil
}

// GetExecutor returns the appropriate update phase executor based on the
// provided parameters
func (f *engine) GetExecutor(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
	return f.Spec(p, remote)
}

// RunCommand executes the phase specified by params on the specified server
// using the provided runner
func (f *engine) RunCommand(ctx context.Context, runner rpc.RemoteRunner, server storage.Server, p fsm.Params) error {
	command := "execute"
	if p.Rollback {
		command = "rollback"
	}
	args := []string{"plan", command,
		"--phase", p.PhaseID,
		"--operation-id", f.plan.OperationID,
	}
	if p.Force {
		args = append(args, "--force")
	}
	return runner.Run(ctx, server, args...)
}

// PreExecute is no-op for the update engine
func (f *engine) PreExecute(ctx context.Context, p fsm.Params) error {
	return nil
}

// PostExecute is no-op for the update engine
func (f *engine) PostExecute(ctx context.Context, p fsm.Params) error {
	return nil
}

// PreRollback is no-op for the update engine
func (f *engine) PreRollback(ctx context.Context, p fsm.Params) error {
	return nil
}

// Complete marks the provided update operation as completed or failed
// and moves the cluster into active state
func (f *engine) Complete(ctx context.Context, fsmErr error) error {
	plan, err := f.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}

	opKey := fsm.OperationKey(*plan)
	op, err := f.Operator.GetSiteOperation(opKey)
	if err != nil {
		return trace.Wrap(err)
	}

	stateSetter := fsm.OperationStateSetter(opKey, f.Operator, f.LocalBackend)
	completed := fsm.IsCompleted(plan)
	if completed {
		err = ops.CompleteOperation(ctx, opKey, stateSetter)
	} else {
		err = ops.FailOperation(ctx, opKey, stateSetter, trace.Unwrap(fsmErr).Error())
	}
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := f.Backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	if completed {
		if err := f.commitClusterChanges(cluster, *op); err != nil {
			return trace.Wrap(err)
		}
	}

	if completed || fsm.IsRolledBack(plan) {
		if err := f.activateCluster(*cluster); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetPlan returns an up-to-date plan
func (f *engine) GetPlan() (*storage.OperationPlan, error) {
	return &f.plan, nil
}

func (f *engine) commitClusterChanges(cluster *storage.Site, op ops.SiteOperation) error {
	updateAppLoc, err := op.Update.Package()
	if err != nil {
		return trace.Wrap(err)
	}

	updateApp, err := f.Apps.GetApp(*updateAppLoc)
	if err != nil {
		return trace.Wrap(err)
	}

	var updateBaseApp *app.Application
	if updateApp.Manifest.Base() != nil {
		updateBaseApp, err = f.Apps.GetApp(*updateApp.Manifest.Base())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	cluster.App = updateApp.PackageEnvelope.ToPackage()
	if updateBaseApp != nil {
		cluster.App.Base = updateBaseApp.PackageEnvelope.ToPackagePtr()
	}

	checks.OverrideDockerConfig(&cluster.ClusterState.Docker,
		checks.DockerConfigFromSchema(updateApp.Manifest.SystemOptions.DockerConfig()))

	return nil
}

func (f *engine) activateCluster(cluster storage.Site) error {
	f.WithFields(logrus.Fields{
		"cluster": cluster.Domain,
		"state":   cluster.State,
	}).Debug("Activating cluster.")
	cluster.State = ops.SiteStateActive
	if _, err := f.Backend.UpdateSite(cluster); err != nil {
		return trace.Wrap(err)
	}
	if _, err := f.LocalBackend.UpdateSite(cluster); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (f *engine) ChangePhaseState(ctx context.Context, change fsm.StateChange) error {
	f.WithField("change", change).Debug("Apply.")

	_, err := f.LocalBackend.CreateOperationPlanChange(storage.PlanChange{
		ID:          uuid.New(),
		ClusterName: f.plan.ClusterName,
		OperationID: f.plan.OperationID,
		PhaseID:     change.Phase,
		NewState:    change.State,
		Error:       utils.ToRawTrace(change.Error),
		Created:     time.Now().UTC(),
	})
	if err != nil {
		f.WithError(err).Warnf("Error recording phase state change %+v.", change)
		return trace.Wrap(err)
	}

	err = f.reconcilePlan(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (f *engine) reconcilePlan(ctx context.Context) error {
	plan, err := f.reconciler.ReconcilePlan(ctx, f.plan)
	if err != nil {
		return trace.Wrap(err)
	}
	f.plan = *plan
	var buf bytes.Buffer
	fsm.FormatOperationPlanText(&buf, f.plan)
	f.Debugf("Reconciled plan: %v.", buf.String())
	return nil
}

func loadPlan(backend storage.Backend, opKey ops.SiteOperationKey, logger logrus.FieldLogger) (*storage.OperationPlan, error) {
	plan, err := backend.GetOperationPlan(opKey.SiteDomain, opKey.OperationID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if plan == nil {
		return nil, trace.NotFound("operation %v doesn't have a plan", opKey.OperationID)
	}
	return plan, nil
}
