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
	"github.com/gravitational/gravity/lib/checks"
	cloudaws "github.com/gravitational/gravity/lib/cloudprovider/aws"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/ops"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	systemstate "github.com/gravitational/gravity/lib/system/state"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/sirupsen/logrus"

	"github.com/fatih/color"
	"github.com/gravitational/configure"
	"github.com/gravitational/trace"
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

	installerConfig, err := i.ToInstallerConfig(env)
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

	return installer.Wait()
}

type JoinConfig struct {
	SystemLogFile     string
	UserLogFile       string
	AdvertiseAddr     string
	ServerAddr        string
	PeerAddrs         string
	Token             string
	Role              string
	SystemDevice      string
	DockerDevice      string
	Mounts            map[string]string
	ExistingOperation bool
	ServiceUID        string
	ServiceGID        string
	CloudProvider     string
}

func Join(env *localenv.LocalEnvironment, j JoinConfig) error {
	err := CheckLocalState(env)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := j.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := checkRunningAsRoot(); err != nil {
		return trace.Wrap(err)
	}

	if err := install.InitLogging(j.SystemLogFile); err != nil {
		return trace.Wrap(err)
	}

	peers, err := utils.ParseAddrList(j.PeerAddrs)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(peers) == 0 {
		return trace.BadParameter("required argument peer-addr not provided")
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}

	err = systemstate.ConfigureStateDirectory(stateDir, j.SystemDevice)
	if err != nil {
		return trace.Wrap(err)
	}

	runtimeConfig := pb.RuntimeConfig{
		Token:        j.Token,
		Role:         j.Role,
		SystemDevice: j.SystemDevice,
		DockerDevice: j.DockerDevice,
		Mounts:       convertMounts(j.Mounts),
	}
	if err = install.FetchCloudMetadata(j.CloudProvider, &runtimeConfig); err != nil {
		return trace.Wrap(err)
	}

	if !j.ExistingOperation {
		return trace.Wrap(joinLoop(env, j, peers, runtimeConfig))
	}

	serviceUser, err := install.EnsureServiceUserAndBinary(j.ServiceUID, j.ServiceGID)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Service user: %+v.", serviceUser)

	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	entry, err := wizardEnv.LoginWizard(peers[0])
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := ops.GetWizardCluster(wizardEnv.Operator)
	if err != nil {
		return trace.Wrap(err)
	}

	operation, _, err := ops.GetInstallOperation(cluster.Key(), wizardEnv.Operator)
	if err != nil {
		return trace.Wrap(err)
	}

	err = checks.RunLocalChecks(checks.LocalChecksRequest{
		Manifest: cluster.App.Manifest,
		Role:     j.Role,
		Options: &validationpb.ValidateOptions{
			VxlanPort:     int32(operation.GetVars().OnPrem.VxlanPort),
			DnsListenAddr: operation.GetVars().OnPrem.DNSListenAddr,
		},
		AutoFix: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	watchCh := make(chan rpcserver.WatchEvent, 1)
	agent, err := install.NewAgent(context.TODO(), install.AgentConfig{
		PackageAddr:   entry.OpsCenterURL,
		AdvertiseAddr: j.AdvertiseAddr,
		ServerAddr:    j.ServerAddr,
		RuntimeConfig: runtimeConfig,
	}, log.WithField("role", j.Role), watchCh)
	if err != nil {
		return trace.Wrap(err)
	}

	watchReconnects(ctx, cancel, watchCh)
	utils.WatchTerminationSignals(ctx, cancel, agent, logrus.StandardLogger())

	return trace.Wrap(agent.Serve())
}

func (j *JoinConfig) checkAndSetDefaults() (err error) {
	j.CloudProvider, err = install.ValidateCloudProvider(j.CloudProvider)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func joinLoop(env *localenv.LocalEnvironment, j JoinConfig, peers []string, runtimeConfig pb.RuntimeConfig) error {
	env.PrintStep("Joining cluster")

	if j.CloudProvider != schema.ProviderOnPrem {
		env.PrintStep("Enabling %q cloud provider integration",
			j.CloudProvider)
	}

	ctx, cancel := context.WithCancel(context.Background())
	watchCh := make(chan rpcserver.WatchEvent, 1)
	eventsC := make(chan install.Event, 100)
	peer, err := install.NewPeer(install.PeerConfig{
		Peers:         peers,
		Context:       ctx,
		AdvertiseAddr: j.AdvertiseAddr,
		ServerAddr:    j.ServerAddr,
		EventsC:       eventsC,
		WatchCh:       watchCh,
		RuntimeConfig: runtimeConfig,
	}, logrus.WithFields(logrus.Fields{
		"role": j.Role,
		"addr": j.AdvertiseAddr,
	}))
	if err != nil {
		return trace.Wrap(err)
	}

	watchReconnects(ctx, cancel, watchCh)

	start := time.Now()
	if err := peer.Start(); err != nil {
		return trace.Wrap(err)
	}

	utils.WatchTerminationSignals(ctx, cancel, peer, peer.FieldLogger)

	for {
		select {
		case <-peer.Done():
			log.Info("Agent shut down.")
			return nil
		case event := <-eventsC:
			if event.Error != nil {
				if isContextCancelledError(event.Error) {
					// Ignore
					return nil
				}
				return trace.Wrap(event.Error)
			}
			progress := event.Progress
			if progress.Message != "" {
				env.PrintStep(progress.Message)
			}
			if progress.State == ops.ProgressStateCompleted {
				env.PrintStep(color.GreenString("Joined cluster in %v", time.Now().Sub(start)))
				return nil
			}
			if progress.State == ops.ProgressStateFailed {
				env.Println(color.RedString("Installation failed."))
				env.Printf("---\nAgent process will keep running so you can re-run certain installation steps.\n" +
					"Once no longer needed, this process can be shutdown using Ctrl-C.\n")
			}
		}
	}
}

type leaveConfig struct {
	force         bool
	confirmed     bool
	systemLogFile string
	userLogFile   string
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

	if err := install.InitLogging(c.systemLogFile); err != nil {
		return trace.Wrap(err)
	}

	err := checkInCluster()
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

	server, err := findLocalServer(*site)
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
		server:        server.Hostname,
		confirmed:     true,
		force:         c.force,
		systemLogFile: c.systemLogFile,
		userLogFile:   c.userLogFile,
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
	server        string
	force         bool
	confirmed     bool
	systemLogFile string
	userLogFile   string
}

func remove(env *localenv.LocalEnvironment, c removeConfig) error {
	if err := checkRunningAsRoot(); err != nil {
		return trace.Wrap(err)
	}

	if err := c.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if err := install.InitLogging(c.systemLogFile); err != nil {
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

	server, err := findServer(*site, []string{c.server})
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
	serviceUID    string
	serviceGID    string
}

func autojoin(env *localenv.LocalEnvironment, d autojoinConfig) error {
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

	return Join(env, JoinConfig{
		SystemLogFile: d.systemLogFile,
		UserLogFile:   d.userLogFile,
		AdvertiseAddr: instance.PrivateIP,
		PeerAddrs:     serviceURL,
		Token:         joinToken,
		Role:          d.role,
		SystemDevice:  d.systemDevice,
		DockerDevice:  d.dockerDevice,
		Mounts:        d.mounts,
		ServiceUID:    d.serviceUID,
		ServiceGID:    d.serviceGID,
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

	if err := install.InitLogging(config.systemLogFile); err != nil {
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

// findServer searches the provided cluster's state for a server that matches one of the provided
// tokens, where a token can be the server's advertise IP, hostname or AWS internal DNS name
func findServer(site ops.Site, tokens []string) (*storage.Server, error) {
	for _, server := range site.ClusterState.Servers {
		for _, token := range tokens {
			if token == "" {
				continue
			}
			switch token {
			case server.AdvertiseIP, server.Hostname, server.Nodename:
				return &server, nil
			}
		}
	}
	return nil, trace.NotFound("could not find server matching %v among registered cluster nodes",
		tokens)
}

// findLocalServer searches the provided cluster's state for the server that matches the one
// the current command is being executed from
func findLocalServer(site ops.Site) (*storage.Server, error) {
	// collect the machines's IP addresses and search by them
	ifaces, err := systeminfo.NetworkInterfaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(ifaces) == 0 {
		return nil, trace.NotFound("no network interfaces found")
	}

	var ips []string
	for _, iface := range ifaces {
		ips = append(ips, iface.IPv4)
	}

	server, err := findServer(site, ips)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return server, nil
}

func convertMounts(mounts map[string]string) (result []*pb.Mount) {
	result = make([]*pb.Mount, 0, len(mounts))
	for name, source := range mounts {
		result = append(result, &pb.Mount{Name: name, Source: source})
	}
	return result
}

// InstallPhaseParams is a set of parameters for a single phase execution
type InstallPhaseParams struct {
	// PhaseID is the ID of the phase to execute
	PhaseID string
	// Force allows to force phase execution
	Force bool
	// Timeout is phase execution timeout
	Timeout time.Duration
}

func executeInstallPhase(localEnv *localenv.LocalEnvironment, p InstallPhaseParams) error {
	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	op, err := ops.GetWizardOperation(wizardEnv.Operator)
	if err != nil {
		return trace.Wrap(err)
	}

	installFSM, err := install.NewFSM(install.FSMConfig{
		OperationKey:  op.Key(),
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

	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()
	progress := utils.NewProgress(ctx, fmt.Sprintf("Executing install phase %q", p.PhaseID), -1, false)
	defer progress.Stop()

	if p.PhaseID == fsm.RootPhase {
		return trace.Wrap(ResumeInstall(ctx, installFSM, progress, p.Force))
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

func ResumeInstall(ctx context.Context, machine *fsm.FSM, progress utils.Progress, force bool) error {
	fsmErr := machine.ExecutePlan(ctx, progress, force)
	if fsmErr != nil {
		return trace.Wrap(fsmErr)
	}

	err := machine.Complete(fsmErr)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func rollbackInstallPhase(localEnv *localenv.LocalEnvironment, p rollbackParams) error {
	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	op, err := ops.GetWizardOperation(wizardEnv.Operator)
	if err != nil {
		return trace.Wrap(err)
	}

	installFSM, err := install.NewFSM(install.FSMConfig{
		OperationKey:  op.Key(),
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

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	progress := utils.NewProgress(ctx, fmt.Sprintf("Rolling back install phase %q", p.phaseID), -1, false)
	defer progress.Stop()

	return installFSM.RollbackPhase(ctx, fsm.Params{
		PhaseID:  p.phaseID,
		Force:    p.force,
		Progress: progress,
	})
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
			"(see https://gravitational.com/telekube/docs/cluster/#deleting-a-cluster for more details)",
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

func isContextCancelledError(err error) bool {
	origErr := trace.Unwrap(err)
	if origErr == context.Canceled {
		return true
	}
	// FIXME: ConnectionProblemError should properly implement Error.OrigError
	if connErr, ok := origErr.(*trace.ConnectionProblemError); ok {
		return connErr.Err == context.Canceled
	}
	return false
}
