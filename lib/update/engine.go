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

package update

import (
	"bytes"
	"context"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

// fsmUpdateEngine is the update FSM engine
type fsmUpdateEngine struct {
	// FSMConfig is the state machine configuration
	FSMConfig
	// FieldLogger is used for logging
	logrus.FieldLogger
	// plan is the update operation plan
	plan       storage.OperationPlan
	reconciler Reconciler
}

// newUpdateEngine returns a new instance of FSM engine for update
func newUpdateEngine(ctx context.Context, config FSMConfig, logger logrus.FieldLogger) (*fsmUpdateEngine, error) {
	plan, err := loadPlan(config.LocalBackend, config.Operation.Key(), logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reconciler := NewDefaultReconciler(
		config.Backend, config.LocalBackend,
		plan.ClusterName, plan.OperationID,
		logger)
	reconciledPlan, err := reconciler.ReconcilePlan(ctx, *plan)
	if err != nil {
		// This is not critical and will be retried during the operation
		logger.WithError(err).Warn("Failed to reconcile operation plan.")
		reconciledPlan = plan
	}
	engine := &fsmUpdateEngine{
		FSMConfig: config,
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
func (f *fsmUpdateEngine) GetExecutor(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
	return f.Spec(p, remote)
}

// RunCommand executes the phase specified by params on the specified server
// using the provided runner
func (f *fsmUpdateEngine) RunCommand(ctx context.Context, runner fsm.RemoteRunner, server storage.Server, p fsm.Params) error {
	args := []string{"plan", "execute",
		"--phase", p.PhaseID,
		"--operation-id", f.plan.OperationID,
	}
	if p.Force {
		args = append(args, "--force")
	}
	return runner.Run(ctx, server, args...)
}

// PreExecute is no-op for the update engine
func (f *fsmUpdateEngine) PreExecute(ctx context.Context, p fsm.Params) error {
	return nil
}

// PostExecute is no-op for the update engine
func (f *fsmUpdateEngine) PostExecute(ctx context.Context, p fsm.Params) error {
	return nil
}

// PreRollback is no-op for the update engine
func (f *fsmUpdateEngine) PreRollback(ctx context.Context, p fsm.Params) error {
	return nil
}

// Complete marks the provided update operation as completed or failed
// and moves the cluster into active state
func (f *fsmUpdateEngine) Complete(fsmErr error) error {
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
		err = ops.CompleteOperation(opKey, stateSetter)
	} else {
		err = ops.FailOperation(opKey, stateSetter, trace.Unwrap(fsmErr).Error())
	}
	if err != nil {
		return trace.Wrap(err)
	}

	if !completed {
		return nil
	}

	cluster, err := f.Backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	err = f.commitClusterChanges(cluster, *op)
	if err != nil {
		return trace.Wrap(err)
	}

	err = f.activateCluster(*cluster)
	return trace.Wrap(err)
}

// GetPlan returns an up-to-date plan
func (f *fsmUpdateEngine) GetPlan() (*storage.OperationPlan, error) {
	return &f.plan, nil
}

func (f *fsmUpdateEngine) commitClusterChanges(cluster *storage.Site, op ops.SiteOperation) error {
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

func (f *fsmUpdateEngine) activateCluster(cluster storage.Site) error {
	cluster.State = ops.SiteStateActive
	_, err := f.Backend.UpdateSite(cluster)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = f.LocalBackend.UpdateSite(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (f *fsmUpdateEngine) ChangePhaseState(ctx context.Context, change fsm.StateChange) error {
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

func (f *fsmUpdateEngine) reconcilePlan(ctx context.Context) error {
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

// checkBinaryVersion makes sure that the plan phase is being executed with
// the proper gravity binary
func checkBinaryVersion(fsm *fsm.FSM) error {
	ourVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return trace.Wrap(err, "failed to parse this binary version: %v",
			version.Get().Version)
	}

	plan, err := fsm.GetPlan()
	if err != nil {
		return trace.Wrap(err, "failed to obtain operation plan")
	}

	requiredVersion, err := plan.GravityPackage.SemVer()
	if err != nil {
		return trace.Wrap(err, "failed to parse required binary version: %v",
			plan.GravityPackage)
	}

	if !ourVersion.Equal(*requiredVersion) {
		return trace.BadParameter(
			`Current operation plan should be executed with the gravity binary of version %q while this binary is of version %q.

Please use the gravity binary from the upgrade installer tarball to execute the plan, or download appropriate version from the Ops Center (curl https://get.gravitational.io/telekube/install/%v | bash).
`, requiredVersion, ourVersion, plan.GravityPackage.Version)
	}

	return nil
}
