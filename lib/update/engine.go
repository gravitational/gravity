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
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
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
	plan *storage.OperationPlan
	// useEtcd indicated whether the engine should attempt to use etcd or not
	useEtcd bool
}

// NewUpdateEngine returns a new FSM instance
func NewUpdateEngine(c FSMConfig) (*fsmUpdateEngine, error) {
	logger := logrus.WithFields(logrus.Fields{
		trace.Component: "engine:update",
	})
	engine := &fsmUpdateEngine{
		FSMConfig:   c,
		FieldLogger: logger,
		useEtcd:     true,
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
	args := []string{"upgrade", "--phase", p.PhaseID, fmt.Sprintf("--force=%v", p.Force)}
	return runner.Run(ctx, server, args...)
}

// PreExecute is no-op for the update engine
func (f *fsmUpdateEngine) PreExecute(ctx context.Context, p fsm.Params) error {
	return nil
}

// PostExecute reconciles the operation plan after phase execution
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
	if !fsm.IsCompleted(f.plan) && !fsm.IsFailed(f.plan) {
		return trace.BadParameter(
			"to complete the operation, all phases must be either completed or failed / rolled back / unstarted, check 'gravity plan'")
	}

	op, err := storage.GetLastOperation(f.Backend)
	if err != nil {
		return trace.Wrap(err)
	}

	if fsmErr != nil {
		_, err := f.Backend.CreateProgressEntry(storage.ProgressEntry{
			SiteDomain:  op.SiteDomain,
			OperationID: op.ID,
			Step:        constants.FinalStep,
			Completion:  constants.Completed,
			State:       ops.ProgressStateFailed,
			Message:     trace.Unwrap(fsmErr).Error(),
			Created:     time.Now().UTC(),
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if fsm.IsCompleted(f.plan) {
		op.State = ops.OperationStateCompleted
	} else {
		op.State = ops.OperationStateFailed
	}

	_, err = f.Backend.UpdateSiteOperation(*op)
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := f.Backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	updateAppLoc, err := loc.ParseLocator(op.Update.UpdatePackage)
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

	cluster.State = ops.SiteStateActive
	if op.State == ops.OperationStateCompleted {
		cluster.App = updateApp.PackageEnvelope.ToPackage()
		if updateBaseApp != nil {
			cluster.App.Base = updateBaseApp.PackageEnvelope.ToPackagePtr()
		}
	}

	_, err = f.Backend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = f.LocalBackend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetPlan returns an up-to-date plan
func (f *fsmUpdateEngine) GetPlan() (*storage.OperationPlan, error) {
	return f.plan, nil
}

func (f *fsmUpdateEngine) loadPlan() error {
	op, err := storage.GetLastOperation(f.LocalBackend)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		f.Error(trace.DebugReport(err))
		execPath, err := os.Executable()
		if err != nil {
			return trace.Wrap(err)
		}

		return trace.NotFound("Please run `sudo %[1]v upgrade` or `sudo %[1]v upgrade --manual` first", execPath)
	}

	plan, err := f.LocalBackend.GetOperationPlan(op.SiteDomain, op.ID)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if plan == nil {
		return trace.NotFound("operation %v (%v) doesn't have a plan",
			op.Type, op.ID)
	}

	f.plan = plan

	return nil
}

func (f *fsmUpdateEngine) reconcilePlan(ctx context.Context) error {
	err := f.trySyncChangelogFromEtcd(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	// Always use the local plan as authoritative
	local, err := f.LocalBackend.GetOperationPlanChangelog(f.plan.ClusterName, f.plan.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}

	f.plan = fsm.ResolvePlan(*f.plan, local)

	var buf bytes.Buffer
	fsm.FormatOperationPlanText(&buf, *f.plan)
	f.Debugf("Reconciled plan: %v.", buf.String())
	return nil
}

func (f *fsmUpdateEngine) trySyncChangelogToEtcd(ctx context.Context) error {
	shouldSync, err := f.isEtcdAvailable(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if shouldSync {
		return trace.Wrap(f.syncChangelog(f.LocalBackend, f.Backend))
	}

	return nil
}

func (f *fsmUpdateEngine) trySyncChangelogFromEtcd(ctx context.Context) error {
	shouldSync, err := f.isEtcdAvailable(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if shouldSync {
		return trace.Wrap(f.syncChangelog(f.Backend, f.LocalBackend))
	}

	return nil
}

// syncChangelog will sync changelog entries from src to dst storage
func (f *fsmUpdateEngine) syncChangelog(src storage.Backend, dst storage.Backend) error {
	return trace.Wrap(syncChangelog(src, dst, f.plan.ClusterName, f.plan.OperationID))
}

// syncChangelog will sync changelog entries from src to dst storage
func syncChangelog(src storage.Backend, dst storage.Backend, clusterName string, operationID string) error {
	srcChangeLog, err := src.GetOperationPlanChangelog(clusterName, operationID)
	if err != nil {
		return trace.Wrap(err)
	}

	dstChangeLog, err := dst.GetOperationPlanChangelog(clusterName, operationID)
	if err != nil {
		return trace.Wrap(err)
	}

	diff := fsm.DiffChangelog(srcChangeLog, dstChangeLog)
	for _, entry := range diff {
		_, err = dst.CreateOperationPlanChange(entry)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// isEtdAvailable checks the local backend, and checks if we're in an upgrade phase where we expect etcd to be available
func (f *fsmUpdateEngine) isEtcdAvailable(ctx context.Context) (bool, error) {
	if !f.useEtcd {
		return false, nil
	}
	_, err := utils.RunCommand(ctx, f.FieldLogger, utils.PlanetCommandArgs(defaults.EtcdCtlBin, "cluster-health")...)
	if err != nil {
		// etcdctl uses an exit code if the health cannot be checked
		// so we don't need to return an error
		if _, ok := trace.Unwrap(err).(*exec.ExitError); ok {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	return true, nil
}

func (f *fsmUpdateEngine) ChangePhaseState(ctx context.Context, change fsm.StateChange) error {
	f.Debugf("%s.", change)

	id := uuid.New()
	_, err := f.LocalBackend.CreateOperationPlanChange(storage.PlanChange{
		ID:          id,
		ClusterName: f.plan.ClusterName,
		OperationID: f.plan.OperationID,
		PhaseID:     change.Phase,
		NewState:    change.State,
		Error:       utils.ToRawTrace(change.Error),
		Created:     time.Now().UTC(),
	})
	if err != nil {
		f.Errorf("Error recording phase state change %+v: %v.",
			change, err)
		return trace.Wrap(err)
	}

	err = f.trySyncChangelogToEtcd(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = f.reconcilePlan(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
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

Please use the gravity binary from the upgrade installer tarball to execute the plan, or download appropriate version from the Telekube Distribution Ops Center (curl https://get.gravitational.io/telekube/install/%v | bash).
`, requiredVersion, ourVersion, plan.GravityPackage.Version)
	}

	return nil
}
