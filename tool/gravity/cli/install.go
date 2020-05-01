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
	"os/exec"
	"strings"
	"time"

	autoscaleaws "github.com/gravitational/gravity/lib/autoscale/aws"
	cloudaws "github.com/gravitational/gravity/lib/cloudprovider/aws"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/expand"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/configure"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func startInstall(env *localenv.LocalEnvironment, i InstallConfig) error {
	env.PrintStep("Starting installer")

	err := CheckLocalState(env)
	if err != nil {
		return trace.Wrap(err)
	}

	err = i.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	installerConfig, err := i.ToInstallerConfig(env, resources.ValidateFunc(gravity.Validate))
	if err != nil {
		return trace.Wrap(err)
	}

	processConfig, err := install.MakeProcessConfig(*installerConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	env.PrintStep("Preparing for installation...")

	installerConfig.Process, err = install.InitProcess(context.TODO(),
		*installerConfig, *processConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	installer, err := install.Init(context.TODO(), *installerConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	err = installer.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	err = installer.Wait()
	if utils.IsContextCancelledError(err) {
		return nil
	}
	return trace.Wrap(err)
}

func Join(env, joinEnv *localenv.LocalEnvironment, j JoinConfig) error {
	err := CheckLocalState(env)
	if err != nil {
		return trace.Wrap(err)
	}

	err = j.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	peerConfig, err := j.ToPeerConfig(env, joinEnv)
	if err != nil {
		return trace.Wrap(err)
	}

	peer, err := expand.NewPeer(*peerConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	err = peer.Init()
	if err != nil {
		return trace.Wrap(err)
	}

	err = peer.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	err = peer.Wait()
	if utils.IsContextCancelledError(err) {
		return nil
	}
	return trace.Wrap(err)

}

type leaveConfig struct {
	force     bool
	confirmed bool
}

func leave(env *localenv.LocalEnvironment, c leaveConfig) error {
	err := tryLeave(env, c)
	if err != nil {
		if !c.force || isCancelledError(err) {
			return trace.Wrap(err)
		}
		log.Warnf("failed to leave cluster, forcing: %v",
			trace.DebugReport(err))
		err := systemUninstall(env, true)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func tryLeave(env *localenv.LocalEnvironment, c leaveConfig) error {
	if err := checkRunningAsRoot(); err != nil {
		return trace.Wrap(err)
	}

	err := httplib.InGravity(env.DNS.Addr())
	if err != nil {
		return trace.NotFound(
			"no running cluster detected, please use --force flag to clean up the local state")
	}

	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	server, err := ops.FindLocalServer(site.ClusterState)
	if err != nil {
		return trace.NotFound(
			"this server is not a part of the running cluster, please use --force flag to clean up the local state")
	}

	if !c.confirmed {
		err = enforceConfirmation(
			"Please confirm removing %v (%v) from the cluster", server.Hostname, server.AdvertiseIP)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err = remove(env, removeConfig{
		server:    server.Hostname,
		confirmed: true,
		force:     c.force,
	})
	if err != nil {
		return trace.BadParameter(
			"error launching shrink operation, please use --force flag to force delete: %v", err)
	}

	return nil
}

func (r *removeConfig) checkAndSetDefaults() error {
	if r.server == "" {
		return trace.BadParameter("server flag is required")
	}
	return nil
}

type removeConfig struct {
	server    string
	force     bool
	confirmed bool
}

func remove(env *localenv.LocalEnvironment, c removeConfig) error {
	if err := checkRunningAsRoot(); err != nil {
		return trace.Wrap(err)
	}

	if err := c.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	server, err := ops.FindServer(site.ClusterState, []string{c.server})
	if err != nil {
		return trace.Wrap(err)
	}

	if !c.confirmed {
		err = enforceConfirmation(
			"Please confirm removing %v (%v) from the cluster", server.Hostname, server.AdvertiseIP)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	key, err := operator.CreateSiteShrinkOperation(
		ops.CreateSiteShrinkOperationRequest{
			AccountID:  site.AccountID,
			SiteDomain: site.Domain,
			Servers:    []string{server.Hostname},
			Force:      c.force,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("launched operation %q, use 'gravity status' to poll its progress\n", key.OperationID)
	return nil
}

type autojoinConfig struct {
	systemLogFile string
	userLogFile   string
	clusterName   string
	role          string
	systemDevice  string
	dockerDevice  string
	mounts        map[string]string
}

func autojoin(env, joinEnv *localenv.LocalEnvironment, d autojoinConfig) error {
	if err := checkRunningAsRoot(); err != nil {
		return trace.Wrap(err)
	}

	instance, err := cloudaws.NewLocalInstance()
	if err != nil {
		return trace.BadParameter("autojoin only supports AWS")
	}

	autoscaler, err := autoscaleaws.New(autoscaleaws.Config{
		ClusterName: d.clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	joinToken, err := autoscaler.GetJoinToken(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	serviceURL, err := autoscaler.GetServiceURL(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("auto joining to cluster %q via %v\n", d.clusterName, serviceURL)

	return Join(env, joinEnv, JoinConfig{
		SystemLogFile: d.systemLogFile,
		UserLogFile:   d.userLogFile,
		AdvertiseAddr: instance.PrivateIP,
		PeerAddrs:     serviceURL,
		Token:         joinToken,
		Role:          d.role,
		SystemDevice:  d.systemDevice,
		DockerDevice:  d.dockerDevice,
		Mounts:        d.mounts,
	})
}

func (r *agentConfig) checkAndSetDefaults() (err error) {
	if r.serviceUID == "" {
		return trace.BadParameter("service user ID is required")
	}
	if r.serviceGID == "" {
		return trace.BadParameter("service group ID is required")
	}
	if r.packageAddr == "" {
		return trace.BadParameter("package service address is required")
	}
	r.cloudProvider, err = install.ValidateCloudProvider(r.cloudProvider)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type agentConfig struct {
	systemLogFile string
	userLogFile   string
	advertiseAddr string
	serverAddr    string
	packageAddr   string
	token         string
	vars          configure.KeyVal
	serviceUID    string
	serviceGID    string
	cloudProvider string
}

func agent(env *localenv.LocalEnvironment, config agentConfig, serviceName string) error {
	if err := checkRunningAsRoot(); err != nil {
		return trace.Wrap(err)
	}

	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	serviceUser, err := install.EnsureServiceUserAndBinary(config.serviceUID, config.serviceGID)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Service user: %+v.", serviceUser)

	gravityPath, err := exec.LookPath(defaults.GravityBin)
	if err != nil {
		return trace.Wrap(err, "failed to lookup gravity binary")
	}

	if serviceName != "" {
		command := []string{gravityPath, "--debug", "ops", "agent",
			config.packageAddr,
			"--advertise-addr", config.advertiseAddr,
			"--server-addr", config.serverAddr,
			"--token", config.token,
			"--system-log-file", config.systemLogFile,
			"--log-file", config.userLogFile,
			"--vars", config.vars.String(),
			"--service-uid", config.serviceUID,
			"--service-gid", config.serviceGID,
			"--cloud-provider", config.cloudProvider,
		}
		spec := systemservice.ServiceSpec{
			StartCommand: strings.Join(command, " "),
		}
		log.Infof("Installing service with spec %+v.", spec)
		err := installOneshotServiceFromSpec(env, serviceName, nil, spec)
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Printf("Agent service %v started.\n", serviceName)
		return nil
	}

	runtimeConfig := pb.RuntimeConfig{
		Token:     config.token,
		KeyValues: config.vars,
	}
	if err = install.FetchCloudMetadata(config.cloudProvider, &runtimeConfig); err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watchCh := make(chan rpcserver.WatchEvent, 1)
	agent, err := install.NewAgent(ctx, install.AgentConfig{
		PackageAddr:   config.packageAddr,
		AdvertiseAddr: config.advertiseAddr,
		ServerAddr:    config.serverAddr,
		RuntimeConfig: runtimeConfig,
	}, log.WithField("addr", config.advertiseAddr), watchCh)
	if err != nil {
		return trace.Wrap(err)
	}

	watchReconnects(ctx, cancel, watchCh)
	utils.WatchTerminationSignals(ctx, cancel, agent, logrus.StandardLogger())

	return trace.Wrap(agent.Serve())
}

func executeInstallPhase(localEnv *localenv.LocalEnvironment, p PhaseParams) error {
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	if wizardEnv.Operator == nil {
		return trace.BadParameter(NoOperationStateBanner)
	}
	operation, err := ops.GetWizardOperation(wizardEnv.Operator)
	if err != nil {
		if trace.IsConnectionProblem(err) {
			if err2 := CheckInstallOperationComplete(localEnv); err2 != nil {
				return trace.Wrap(err, "unable to connect to installer. Is the installer process running?")
			}
			return trace.BadParameter("installation already completed")
		}
		return trace.Wrap(err)
	}
	return executeInstallPhaseForOperation(localEnv, p, *operation)
}

// CheckInstallOperationComplete verifies whether there's a completed install operation.
// Returns nil if there is a completed install operation
func CheckInstallOperationComplete(localEnv *localenv.LocalEnvironment) error {
	operations, err := getLastOperation(localEnv, nil, "")
	if err != nil {
		return trace.Wrap(err)
	}
	if len(operations) == 1 && operations[0].Type == ops.OperationInstall && operations[0].IsCompleted() {
		return nil
	}
	return trace.NotFound("no operation found")
}

func executeInstallPhaseForOperation(localEnv *localenv.LocalEnvironment, p PhaseParams, operation ops.SiteOperation) error {
	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	installFSM, err := install.NewFSM(install.FSMConfig{
		OperationKey:       operation.Key(),
		Packages:           wizardEnv.Packages,
		Apps:               wizardEnv.Apps,
		Operator:           wizardEnv.Operator,
		LocalClusterClient: localEnv.SiteOperator,
		LocalPackages:      localEnv.Packages,
		LocalApps:          localApps,
		LocalBackend:       localEnv.Backend,
		Insecure:           localEnv.Insecure,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()
	progress := utils.NewProgress(ctx, fmt.Sprintf("Executing install phase %q", p.PhaseID), -1, false)
	defer progress.Stop()

	if p.PhaseID == fsm.RootPhase {
		return trace.Wrap(ResumeInstall(ctx, installFSM, progress))
	}

	err = installFSM.ExecutePhase(ctx, fsm.Params{
		PhaseID:  p.PhaseID,
		Force:    p.Force,
		Progress: progress,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func executeJoinPhase(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, p PhaseParams) error {
	operation, err := getActiveOperation(localEnv, environ, p.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	if operation.Type != ops.OperationExpand {
		return trace.NotFound("no active expand operation found")
	}
	return executeJoinPhaseForOperation(localEnv, environ, p, operation.SiteOperation)
}

func executeJoinPhaseForOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, p PhaseParams, operation ops.SiteOperation) error {
	joinEnv, err := environ.NewJoinEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer joinEnv.Close()
	operator, err := joinEnv.CurrentOperator(httplib.WithInsecure())
	if err != nil {
		if trace.IsAccessDenied(err) {
			log.WithError(err).Warn("Failed to connect to cluster service.")
			return trace.BadParameter("unable to connect to the cluster service. Is the command being run from the joining node?")
		}
		return trace.Wrap(err)
	}
	apps, err := joinEnv.CurrentApps(httplib.WithInsecure())
	if err != nil {
		return trace.Wrap(err)
	}
	packages, err := joinEnv.CurrentPackages(httplib.WithInsecure())
	if err != nil {
		return trace.Wrap(err)
	}
	joinFSM, err := expand.NewFSM(expand.FSMConfig{
		OperationKey:  operation.Key(),
		Operator:      operator,
		Apps:          apps,
		Packages:      packages,
		LocalBackend:  localEnv.Backend,
		LocalPackages: localEnv.Packages,
		LocalApps:     localEnv.Apps,
		JoinBackend:   joinEnv.Backend,
		DebugMode:     localEnv.Debug,
		Insecure:      localEnv.Insecure,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()
	progress := utils.NewProgress(ctx, fmt.Sprintf("Executing join phase %q", p.PhaseID), -1, false)
	defer progress.Stop()
	if p.PhaseID == fsm.RootPhase {
		return trace.Wrap(ResumeInstall(ctx, joinFSM, progress))
	}
	return joinFSM.ExecutePhase(ctx, fsm.Params{
		PhaseID:  p.PhaseID,
		Force:    p.Force,
		Progress: progress,
	})
}

func rollbackJoinPhaseForOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, p PhaseParams, operation ops.SiteOperation) error {
	joinEnv, err := environ.NewJoinEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer joinEnv.Close()
	operator, err := joinEnv.CurrentOperator(httplib.WithInsecure(), httplib.WithTimeout(5*time.Second))
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := joinEnv.CurrentApps(httplib.WithInsecure(), httplib.WithTimeout(5*time.Second))
	if err != nil {
		return trace.Wrap(err)
	}
	packages, err := joinEnv.CurrentPackages(httplib.WithInsecure(), httplib.WithTimeout(5*time.Second))
	if err != nil {
		return trace.Wrap(err)
	}
	joinFSM, err := expand.NewFSM(expand.FSMConfig{
		OperationKey:  operation.Key(),
		Operator:      operator,
		Apps:          apps,
		Packages:      packages,
		LocalBackend:  localEnv.Backend,
		LocalPackages: localEnv.Packages,
		LocalApps:     localEnv.Apps,
		JoinBackend:   joinEnv.Backend,
		DebugMode:     localEnv.Debug,
		Insecure:      localEnv.Insecure,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()
	progress := utils.NewProgress(ctx, fmt.Sprintf("Rolling back join phase %q", p.PhaseID), -1, false)
	defer progress.Stop()
	return joinFSM.RollbackPhase(ctx, fsm.Params{
		PhaseID:  p.PhaseID,
		Force:    p.Force,
		Progress: progress,
	})
}

func ResumeInstall(ctx context.Context, machine *fsm.FSM, progress utils.Progress) error {
	fsmErr := machine.ExecutePlan(ctx, progress)
	if fsmErr != nil {
		return trace.Wrap(fsmErr)
	}

	err := machine.Complete(fsmErr)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func rollbackInstallPhaseForOperation(localEnv *localenv.LocalEnvironment, p PhaseParams, operation ops.SiteOperation) error {
	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	installFSM, err := install.NewFSM(install.FSMConfig{
		OperationKey:       operation.Key(),
		Packages:           wizardEnv.Packages,
		Apps:               wizardEnv.Apps,
		Operator:           wizardEnv.Operator,
		LocalClusterClient: localEnv.SiteOperator,
		LocalPackages:      localEnv.Packages,
		LocalApps:          localApps,
		LocalBackend:       localEnv.Backend,
		Insecure:           localEnv.Insecure,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()
	progress := utils.NewProgress(ctx, fmt.Sprintf("Rolling back install phase %q", p.PhaseID), -1, false)
	defer progress.Stop()

	return installFSM.RollbackPhase(ctx, fsm.Params{
		PhaseID:  p.PhaseID,
		Force:    p.Force,
		Progress: progress,
	})
}

func completeInstallPlanForOperation(localEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	installFSM, err := install.NewFSM(install.FSMConfig{
		OperationKey:  operation.Key(),
		Packages:      wizardEnv.Packages,
		Apps:          wizardEnv.Apps,
		Operator:      wizardEnv.Operator,
		LocalPackages: localEnv.Packages,
		LocalApps:     localApps,
		LocalBackend:  localEnv.Backend,
		Insecure:      localEnv.Insecure,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = installFSM.Complete(trace.Errorf("completed manually"))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func completeJoinPlanForOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operation ops.SiteOperation) error {
	joinEnv, err := environ.NewJoinEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer joinEnv.Close()
	operator, err := joinEnv.CurrentOperator(httplib.WithInsecure())
	if err != nil {
		if !trace.IsAccessDenied(err) {
			log.WithError(err).Warn("Failed to query cluster operator.")
		}
		return completeJoinPlanFromExistingNode(localEnv, operation)
	}
	apps, err := joinEnv.CurrentApps(httplib.WithInsecure())
	if err != nil {
		return trace.Wrap(err)
	}
	packages, err := joinEnv.CurrentPackages(httplib.WithInsecure())
	if err != nil {
		return trace.Wrap(err)
	}
	joinFSM, err := expand.NewFSM(expand.FSMConfig{
		OperationKey:  operation.Key(),
		Operator:      operator,
		Apps:          apps,
		Packages:      packages,
		LocalBackend:  localEnv.Backend,
		LocalPackages: localEnv.Packages,
		LocalApps:     localEnv.Apps,
		JoinBackend:   joinEnv.Backend,
		DebugMode:     localEnv.Debug,
		Insecure:      localEnv.Insecure,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return joinFSM.Complete(trace.Errorf("completed manually"))
}

// completeJoinPlanFromExistingNode completes the specifies expand operation
// from a existing cluster node in case the joining node (and its state) is not
// available to perform the operation.
func completeJoinPlanFromExistingNode(localEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	const manualCompletedError = "completed manually"
	plan, err := clusterEnv.Operator.GetOperationPlan(operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if plan != nil {
		return fsm.CompleteOrFailOperation(plan, clusterEnv.Operator, manualCompletedError)
	}
	// No operation plan created for the operation - fail the operation directly
	return ops.FailOperation(operation.Key(), clusterEnv.Operator, manualCompletedError)
}

func isCancelledError(err error) bool {
	if err == nil {
		return false
	}
	return trace.IsCompareFailed(err) && strings.Contains(err.Error(), "cancelled")
}

// CheckLocalState performs a local environment sanity check to make sure
// that install/join on this node can proceed without issues
func CheckLocalState(env *localenv.LocalEnvironment) error {
	// make sure that there are no packages in the local state left from
	// some improperly cleaned up installation
	packages, err := env.Packages.GetPackages(defaults.SystemAccountOrg)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(packages) != 0 {
		return trace.BadParameter("detected previous installation state in %v, "+
			"please clean it up using `gravity leave --force` before proceeding "+
			"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
			env.StateDir)
	}
	return nil
}

func watchReconnects(ctx context.Context, cancel context.CancelFunc, watchCh <-chan rpcserver.WatchEvent) {
	go func() {
		for event := range watchCh {
			if event.Error == nil {
				continue
			}
			log.Warnf("Failed to reconnect to %v: %v.", event.Peer, event.Error)
			cancel()
			return
		}
	}()
}
