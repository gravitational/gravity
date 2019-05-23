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

package cli

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	clusterupdate "github.com/gravitational/gravity/lib/update/cluster"
	"github.com/gravitational/version"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

func updateCheck(env *localenv.LocalEnvironment, updatePackage string) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = checkForUpdate(env, operator, cluster.App.Package, updatePackage)
	return trace.Wrap(err)
}

func updateTrigger(
	localEnv *localenv.LocalEnvironment,
	updateEnv *localenv.LocalEnvironment,
	updatePackage string,
	manual, noValidateVersion bool,
) error {
	ctx := context.TODO()
	updater, err := newClusterUpdater(ctx, localEnv, updateEnv, updatePackage, manual, noValidateVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if !manual {
		return nil
	}
	localEnv.Println(updateClusterManualOperationBanner)
	return nil
}

func newClusterUpdater(
	ctx context.Context,
	localEnv, updateEnv *localenv.LocalEnvironment,
	updatePackage string,
	manual, noValidateVersion bool,
) (updater, error) {
	init := &clusterInitializer{
		updatePackage: updatePackage,
		unattended:    !manual,
	}
	updater, err := newUpdater(ctx, localEnv, updateEnv, init)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if noValidateVersion {
		return updater, nil
	}
	if err := validateBinaryVersion(updater); err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

func executeUpdatePhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	updater, err := getClusterUpdater(env, updateEnv, operation, params.SkipVersionCheck)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	err = updater.RunPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func rollbackUpdatePhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	updater, err := getClusterUpdater(env, updateEnv, operation, params.SkipVersionCheck)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	err = updater.RollbackPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func completeUpdatePlan(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	updater, err := getClusterUpdater(env, updateEnv, operation, true)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	return trace.Wrap(updater.Complete(nil))
}

func getClusterUpdater(localEnv, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation, noValidateVersion bool) (*update.Updater, error) {
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operator := clusterEnv.Operator

	creds, err := libfsm.GetClientCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)

	updater, err := clusterupdate.New(context.TODO(), clusterupdate.Config{
		Config: update.Config{
			Operation:    &operation,
			Operator:     operator,
			Backend:      clusterEnv.Backend,
			LocalBackend: updateEnv.Backend,
			Runner:       runner,
			Silent:       localEnv.Silent,
		},
		Apps:              clusterEnv.Apps,
		Client:            clusterEnv.Client,
		Packages:          clusterEnv.Packages,
		ClusterPackages:   clusterEnv.ClusterPackages,
		HostLocalBackend:  localEnv.Backend,
		HostLocalPackages: localEnv.Packages,
		Users:             clusterEnv.Users,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if noValidateVersion {
		return updater, nil
	}
	if err := validateBinaryVersion(updater); err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

func (r *clusterInitializer) validatePreconditions(localEnv *localenv.LocalEnvironment, operator ops.Operator, cluster ops.Site) error {
	updateApp, err := checkForUpdate(localEnv, operator, cluster.App.Package, r.updatePackage)
	if err != nil {
		return trace.Wrap(err)
	}
	err = checkCanUpdate(cluster, operator, updateApp.Manifest)
	if err != nil {
		return trace.Wrap(err)
	}
	r.updateLoc = updateApp.Package
	return nil
}

func (r clusterInitializer) newOperation(operator ops.Operator, cluster ops.Site) (*ops.SiteOperationKey, error) {
	return operator.CreateSiteAppUpdateOperation(context.TODO(), ops.CreateSiteAppUpdateOperationRequest{
		AccountID:  cluster.AccountID,
		SiteDomain: cluster.Domain,
		App:        r.updateLoc.String(),
	})
}

func (r clusterInitializer) newOperationPlan(
	ctx context.Context,
	operator ops.Operator,
	cluster ops.Site,
	operation ops.SiteOperation,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
) (*storage.OperationPlan, error) {
	plan, err := clusterupdate.InitOperationPlan(
		ctx, localEnv, updateEnv, clusterEnv, operation.Key(),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

func (clusterInitializer) newUpdater(
	ctx context.Context,
	operator ops.Operator,
	operation ops.SiteOperation,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	runner fsm.AgentRepository,
) (*update.Updater, error) {
	config := clusterupdate.Config{
		Config: update.Config{
			Operation:    &operation,
			Operator:     clusterEnv.Operator,
			Backend:      clusterEnv.Backend,
			LocalBackend: updateEnv.Backend,
			Runner:       runner,
		},
		HostLocalBackend:  localEnv.Backend,
		HostLocalPackages: localEnv.Packages,
		Packages:          clusterEnv.Packages,
		ClusterPackages:   clusterEnv.ClusterPackages,
		Apps:              clusterEnv.Apps,
		Client:            clusterEnv.Client,
		Users:             clusterEnv.Users,
	}
	return clusterupdate.New(ctx, config)
}

func (r clusterInitializer) updateDeployRequest(req deployAgentsRequest) deployAgentsRequest {
	if r.unattended {
		req.leaderParams = constants.RPCAgentUpgradeFunction
	}
	return req
}

type clusterInitializer struct {
	updateLoc     loc.Locator
	updatePackage string
	unattended    bool
}

const (
	updateClusterManualOperationBanner = `The operation has been created in manual mode.

See https://gravitational.com/gravity/docs/cluster/#managing-an-ongoing-operation for details on working with operation plan.`
)

func checkCanUpdate(cluster ops.Site, operator ops.Operator, manifest schema.Manifest) error {
	existingGravityPackage, err := cluster.App.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	supportsUpdate, err := supportsUpdate(*existingGravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	if !supportsUpdate {
		return trace.BadParameter(`
Installed runtime version (%q) is too old and cannot be updated by this package.
Please update this installation to a minimum required runtime version (%q) before using this update.`,
			existingGravityPackage.Version, defaults.BaseUpdateVersion)
	}
	return nil
}

// checkForUpdate determines if there is an updatePackage for the cluster's application
// and returns a reference to it if available.
// updatePackage specifies an optional (potentially incomplete) package name of the update package.
// If unspecified, the currently installed application package is used.
// Returns the reference to the update application
func checkForUpdate(
	env *localenv.LocalEnvironment,
	operator ops.Operator,
	installedPackage loc.Locator,
	updatePackage string,
) (updateApp *app.Application, err error) {
	// if app package was not provided, default to the latest version of
	// the currently installed app
	if updatePackage == "" {
		updatePackage = installedPackage.Name
	}

	updateLoc, err := loc.MakeLocator(updatePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	apps, err := env.AppService(
		defaults.GravityServiceURL,
		localenv.AppConfig{},
		httplib.WithLocalResolver(env.DNS.Addr()),
		httplib.WithInsecure())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateApp, err = apps.GetApp(*updateLoc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = pack.CheckUpdatePackage(installedPackage, updateApp.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	env.Printf("updating %v from %v to %v\n",
		updateApp.Package.Name, installedPackage.Version, updateApp.Package.Version)

	return updateApp, nil
}

func supportsUpdate(gravityPackage loc.Locator) (supports bool, err error) {
	ver, err := gravityPackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}
	return defaults.BaseUpdateVersion.Compare(*ver) <= 0, nil
}

func validateBinaryVersion(updater *update.Updater) error {
	plan, err := updater.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkBinaryVersion(plan.GravityPackage); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// checkBinaryVersion makes sure that the plan phase is being executed with
// the proper gravity binary
func checkBinaryVersion(gravityPackage loc.Locator) error {
	ourVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return trace.Wrap(err, "failed to parse this binary version: %v",
			version.Get().Version)
	}

	requiredVersion, err := gravityPackage.SemVer()
	if err != nil {
		return trace.Wrap(err, "failed to parse required binary version: %v",
			gravityPackage)
	}

	if !ourVersion.Equal(*requiredVersion) {
		return trace.BadParameter(
			`Current operation plan should be executed with the gravity binary of version %q while this binary is of version %q.

Please use the gravity binary from the upgrade installer tarball to execute the plan, or download appropriate version from the Ops Center (curl https://get.gravitational.io/telekube/install/%v | bash).
`, requiredVersion, ourVersion, gravityPackage.Version)
	}

	return nil
}
