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
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	helmclt "github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/schema"
	statusapi "github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/update"
	clusterupdate "github.com/gravitational/gravity/lib/update/cluster"

	"github.com/coreos/go-semver/semver"
	"github.com/fatih/color"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
)

func updateCheck(env *localenv.LocalEnvironment, updatePackage string) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite(context.TODO())
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
	manual, noValidateVersion, force bool,
) error {
	ctx := context.TODO()
	updater, err := newClusterUpdater(ctx, localEnv, updateEnv, updatePackage, manual, noValidateVersion, force)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if !manual {
		// The cluster is updating in background
		return nil
	}
	localEnv.Println(updateClusterManualOperationBanner)
	return nil
}

func newClusterUpdater(
	ctx context.Context,
	localEnv, updateEnv *localenv.LocalEnvironment,
	updatePackage string,
	manual, noValidateVersion, force bool,
) (updater, error) {
	init := &clusterInitializer{
		updatePackage: updatePackage,
		unattended:    !manual,
		force:         force,
	}

	if err := checkStatus(ctx, localEnv, force); err != nil {
		return nil, trace.Wrap(err)
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

// checkStatus returns an error if the cluster is degraded.
func checkStatus(ctx context.Context, env *localenv.LocalEnvironment, ignoreWarnings bool) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := operator.GetLocalSite(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	status, err := statusapi.FromCluster(ctx, operator, *cluster, "")
	if err != nil {
		return trace.Wrap(err)
	}

	var failedProbes []string
	var warningProbes []string
	for _, node := range status.Agent.Nodes {
		failedProbes = append(failedProbes, node.FailedProbes...)
		warningProbes = append(warningProbes, node.WarnProbes...)
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)

	if len(failedProbes) > 0 {
		fmt.Println("The upgrade is prohibited because some cluster nodes are currently degraded.")
		printAgentStatus(*status.Agent, w)
		if err := w.Flush(); err != nil {
			log.WithError(err).Warn("Failed to flush to stdout.")
		}
		fmt.Println("Please make sure the cluster is healthy before re-attempting the upgrade.")
		return trace.BadParameter("failed to start upgrade operation")
	}

	if !ignoreWarnings && len(warningProbes) > 0 {
		fmt.Println("Some cluster nodes have active warnings:")
		printAgentStatus(*status.Agent, w)
		if err := w.Flush(); err != nil {
			log.WithError(err).Warn("Failed to flush to stdout.")
		}
		fmt.Println("You can provide the --force flag to suppress this message and launch the upgrade anyways.")
		return trace.BadParameter("failed to start upgrade operation")
	}

	return nil
}

func executeUpdatePhase(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	operation, err := getActiveOperation(env, environ, params.OperationID)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("no active update operation found")
		}
		return trace.Wrap(err)
	}
	if operation.Type != ops.OperationUpdate {
		return trace.NotFound("no active update operation found")
	}

	return executeUpdatePhaseForOperation(env, environ, params, operation.SiteOperation)
}

func executeUpdatePhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getClusterUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if !params.SkipVersionCheck {
		if err := validateBinaryVersion(updater); err != nil {
			return trace.Wrap(err)
		}
	}
	return executeOrForkPhase(env, updater, params, operation)
}

// gravityResumeServiceName is the name of systemd service that executes
// the gravity resume command.
const gravityResumeServiceName = "gravity-resume.service"

// executeOrForkPhase either directly executes the specified operation phase,
// or launches a one-shot systemd service that executes it in the background.
func executeOrForkPhase(env *localenv.LocalEnvironment, updater updater, params PhaseParams, operation ops.SiteOperation) error {
	if params.isResume() {
		if err := verifyOrDeployAgents(env); err != nil {
			// Continue operation in case gravity-site or etcd is down. In these
			// cases the agent status may not be retrievable.
			log.WithError(err).Warn("Failed to verify or deploy agents.")
		}
	}

	// If given the --block flag, we're running as a systemd unit (or a user
	// requested the command to execute in foreground), so proceed to perform
	// the command (resume or single phase) directly.
	if params.Block {
		return updater.RunPhase(context.TODO(),
			params.PhaseID,
			params.Timeout,
			params.Force)
	}
	// Before launching the service, perform a few prechecks, for example to
	// make sure that the operation is being resumed from the correct node.
	//
	// TODO(r0mant): Also, make sure agents are running on the cluster nodes:
	// https://github.com/gravitational/gravity/issues/1667
	if err := updater.Check(params.toFSM()); err != nil {
		return trace.Wrap(err)
	}
	// Make sure to launch the unit command with the --block flag.
	args := append(os.Args[1:], "--debug", "--block")
	env.PrintStep("Starting %v service", gravityResumeServiceName)
	if err := launchOneshotService(gravityResumeServiceName, args); err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep(`Service %[1]v has been launched.

To monitor the operation progress:

  sudo gravity plan --operation-id=%[2]v --tail

To monitor the service logs:

  sudo journalctl -u %[1]v -f
`, gravityResumeServiceName, operation.ID)
	return nil
}

// launchOneshotService launches the specified command as a one-shot systemd
// service with the specified name.
func launchOneshotService(name string, args []string) error {
	systemd, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	// See if the service is already running.
	status, err := systemd.StatusService(name)
	if err != nil {
		return trace.Wrap(err)
	}
	// Since we're using a one-shot service, it will be in "activating" state
	// for the duration of the command execution. In fact, one-shot services
	// never reach "active" state, but we're checking it too just in case.
	switch status {
	case systemservice.ServiceStatusActivating, systemservice.ServiceStatusActive:
		return trace.AlreadyExists("service %v is already running", name)
	}
	// Launch the systemd unit that runs the specified command using same binary.
	gravityPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err)
	}
	command := strings.Join(append([]string{gravityPath}, args...), " ")
	return systemd.InstallService(systemservice.NewServiceRequest{
		Name:    name,
		NoBlock: true,
		ServiceSpec: systemservice.ServiceSpec{
			User:         constants.RootUIDString,
			Type:         service.OneshotService,
			StartCommand: command,
		},
	})
}

func rollbackUpdatePhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getClusterUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if !params.SkipVersionCheck {
		if err := validateBinaryVersion(updater); err != nil {
			return trace.Wrap(err)
		}
	}
	err = updater.RollbackPhase(context.TODO(), fsm.Params{
		PhaseID: params.PhaseID,
		Force:   params.Force,
		DryRun:  params.DryRun,
	}, params.Timeout)
	return trace.Wrap(err)
}

func setUpdatePhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params SetPhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getClusterUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	return updater.SetPhase(context.TODO(), params.PhaseID, params.State)
}

func completeUpdatePlanForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getClusterUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if err := updater.Complete(context.TODO(), nil); err != nil {
		return trace.Wrap(err)
	}
	if err := updater.Activate(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getClusterUpdater(localEnv, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) (*update.Updater, error) {
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
	return updater, nil
}

func (r *clusterInitializer) validatePreconditions(localEnv *localenv.LocalEnvironment, operator ops.Operator, cluster ops.Site) error {
	updateApp, err := checkForUpdate(localEnv, operator, cluster.App.Package, r.updatePackage)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := checkCanUpdate(cluster, operator, updateApp.Manifest); err != nil {
		return trace.Wrap(err)
	}
	if err := r.checkTiller(localEnv, updateApp.Manifest); err != nil {
		return trace.Wrap(err)
	}
	if err := r.checkRuntimeEnvironment(localEnv, cluster, operator); err != nil {
		return trace.Wrap(err)
	}
	r.updateLoc = updateApp.Package
	return nil
}

func (r *clusterInitializer) checkRuntimeEnvironment(env *localenv.LocalEnvironment, cluster ops.Site, operator ops.Operator) error {
	runtimeEnv, err := operator.GetClusterEnvironmentVariables(cluster.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	if err := runtimeEnv.CheckAndSetDefaults(); err != nil {
		log.WithError(err).Error("Runtime environment variables validation failed.")
		if r.force {
			env.PrintStep(color.YellowString("Runtime environment variables validation failed: %v", err))
			return nil
		}
		bytes, marshalErr := yaml.Marshal(runtimeEnv)
		if marshalErr != nil {
			return trace.Wrap(err)
		}
		return trace.BadParameter(`There was an issue detected with runtime environment variables:

    %q

This may cause problems during the upgrade. Please review configured environment
variables using "gravity resource get runtimeenvironment" command and update it
appropriately before proceeding with the upgrade:

%s
See https://gravitational.com/gravity/docs/config/#runtime-environment-variables
for more information on managing runtime environment variables.

This warning can be bypassed by providing a --force flag to the upgrade command.`, err, bytes)
	}
	log.Info("Runtime environment variables are valid.")
	return nil
}

// checkTiller verifies tiller server health before kicking off upgrade.
func (r *clusterInitializer) checkTiller(env *localenv.LocalEnvironment, manifest schema.Manifest) error {
	if manifest.CatalogDisabled() {
		log.Info("Tiller server is disabled, not checking its health.")
		return nil
	}
	err := helmclt.Ping(env.DNS.Addr())
	if err != nil {
		log.WithError(err).Error("Failed to ping tiller pod.")
		if r.force {
			env.PrintStep(color.YellowString(`Tiller server health check failed, "helm upgrade" may not work!`))
			return nil
		}
		return trace.BadParameter(`Tiller server health check failed with the following error:

    %q

This means that "helm upgrade" and other Helm commands may not work correctly. If
the application upgrade requires Helm, make sure that Tiller pod is up, running
and reachable before retrying the upgrade.

This warning can be bypassed by providing a --force flag to the upgrade command.`, err)
	}
	log.Info("Tiller server ping success.")
	return nil
}

func (r clusterInitializer) newOperation(operator ops.Operator, cluster ops.Site) (*ops.SiteOperationKey, error) {
	return operator.CreateSiteAppUpdateOperation(context.TODO(), ops.CreateSiteAppUpdateOperationRequest{
		AccountID:  cluster.AccountID,
		SiteDomain: cluster.Domain,
		App:        r.updateLoc.String(),
		Force:      r.force,
	})
}

func (r clusterInitializer) newOperationPlan(
	ctx context.Context,
	operator ops.Operator,
	cluster ops.Site,
	operation ops.SiteOperation,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	leader *storage.Server,
) (*storage.OperationPlan, error) {
	plan, err := clusterupdate.InitOperationPlan(
		ctx, localEnv, updateEnv, clusterEnv, operation.Key(), leader,
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
	runner rpc.AgentRepository,
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
	force         bool
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

	apps, err := env.AppServiceCluster()
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

	env.PrintStep("Upgrading cluster from %v to %v", installedPackage.Version,
		updateApp.Package.Version)

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

Please use the gravity binary from the upgrade installer tarball to execute the plan, or download appropriate version from Gravity Hub (curl https://get.gravitational.io/telekube/install/%v | bash).
`, requiredVersion, ourVersion, gravityPackage.Version)
	}

	return nil
}
