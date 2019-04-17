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
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	autoscaleaws "github.com/gravitational/gravity/lib/autoscale/aws"
	cloudaws "github.com/gravitational/gravity/lib/cloudprovider/aws"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/expand"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install"
	installerclient "github.com/gravitational/gravity/lib/install/client/installer"
	observerclient "github.com/gravitational/gravity/lib/install/client/observer"
	clinstall "github.com/gravitational/gravity/lib/install/engine/cli"
	"github.com/gravitational/gravity/lib/install/engine/interactive"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/pack/webpack"
	"github.com/gravitational/gravity/lib/process"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/roundtrip"

	"github.com/gravitational/configure"
	"github.com/gravitational/trace"
)

// InstallerClient runs the client for the installer service.
// The client is responsible for triggering the install operation and observing
// operation progress
func InstallerClient(env *localenv.LocalEnvironment, config InstallConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(ctx, cancel, InterruptSignals)
	defer func() {
		cancel()
		<-interrupt.Done()
	}()
	go TerminationHandler(interrupt, cancel, env)

	env.PrintStep("Connecting to installer")
	args := append([]string{utils.Exe.Path}, os.Args[1:]...)
	args = append(args, "--from-service", config.StateDir)
	client, err := installerclient.New(ctx, installerclient.Config{
		Args:              args,
		StateDir:          env.StateDir,
		OperationStateDir: config.writeStateDir,
		Packages:          env.Packages,
		InterruptHandler:  interrupt,
		Printer:           env,
		ConnectTimeout:    10 * time.Minute,
		ServiceName:       defaults.GravityRPCInstallerServiceName,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Connected to installer")
	return trace.Wrap(client.Run(ctx))
}

// JoinClient runs the client for the agent service.
// The client is responsible for triggering the install operation and observing
// operation progress
func JoinClient(env *localenv.LocalEnvironment, config JoinConfig) error {
	env.PrintStep("Connecting to agent")

	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(ctx, cancel, InterruptSignals)
	defer func() {
		cancel()
		<-interrupt.Done()
	}()
	go TerminationHandler(interrupt, cancel, env)
	args := append([]string{utils.Exe.Path}, os.Args[1:]...)
	args = append(args, "--from-service")
	client, err := installerclient.New(ctx, installerclient.Config{
		Args:              args,
		StateDir:          env.StateDir,
		OperationStateDir: env.StateDir,
		Packages:          env.Packages,
		InterruptHandler:  interrupt,
		Printer:           env,
		ConnectTimeout:    10 * time.Minute,
		ServiceName:       defaults.GravityRPCAgentServiceName,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Connected to agent")
	return trace.Wrap(client.Run(ctx))
}

// TODO: auto mode can be implemented in such a way that the server also runs a client
func Join(env, joinEnv *localenv.LocalEnvironment, config JoinConfig) error {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	log.WithField(trace.Component, "agent").Info("Running in service mode.")
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(ctx, cancel, InterruptSignals)
	defer func() {
		cancel()
		<-interrupt.Done()
	}()
	go TerminationHandler(interrupt, cancel, env)
	listener, err := net.Listen("unix", filepath.Join(joinEnv.StateDir, "installer.sock"))
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			listener.Close()
		}
	}()
	peerConfig, err := config.NewPeerConfig(env, joinEnv)
	if err != nil {
		return trace.Wrap(err)
	}
	peer, err := expand.NewPeer(ctx, *peerConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	interrupt.Add(peer)
	return trace.Wrap(peer.Serve(listener))
}

// InterruptSignals lists signals considered interrupt signals
var InterruptSignals = signals.WithSignals(
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGQUIT,
)

// TerminationHandler implements the default interrupt handler for the install operation
func TerminationHandler(interrupt *signals.InterruptHandler, cancel context.CancelFunc, printer utils.Printer) {
	// timer defines the interrupt cancellation policy.
	// If the interrupt signal is not re-triggered within the allotted time,
	// the signal is dropped
	timer := time.NewTimer(5 * time.Second)
	var timerC <-chan time.Time
	// number of consecutive interrupts
	var interrupts int
	for {
		select {
		case sig := <-interrupt.C:
			if sig != os.Interrupt {
				printer.Println("Received", sig, "signal. Terminating the installer gracefully, please wait.")
				cancel()
				return
			}
			// Interrupt signaled
			interrupts += 1
			if interrupts > 1 {
				printer.Println("Received", sig, "signal. Aborting the installer gracefully, please wait.")
				cancel()
				return
			}
			printer.Println("Press Ctrl+C again to abort the installation.")
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(5 * time.Second)
			timerC = timer.C
		case <-timerC:
			// Drop this interrupt signal
			interrupts = 0
			timerC = nil
		case <-interrupt.Done():
			return
		}
	}
}

func startInstall(env *localenv.LocalEnvironment, config InstallConfig) error {
	env.PrintStep("Starting installer")

	if err := config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if config.FromService {
		err := startInstallFromService(env, config)
		if utils.IsContextCancelledError(err) {
			return trace.Wrap(err, "installer interrupted")
		}
		return trace.Wrap(err)
	}
	err := InstallerClient(env, config)
	if utils.IsContextCancelledError(err) {
		return trace.Wrap(err, "installer interrupted")
	}
	return trace.Wrap(err)
}

func startInstallFromService(env *localenv.LocalEnvironment, config InstallConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(ctx, cancel, InterruptSignals)
	defer func() {
		cancel()
		<-interrupt.Done()
	}()
	go TerminationHandler(interrupt, cancel, env)
	processConfig, err := config.NewProcessConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	process, err := install.InitProcess(ctx, *processConfig, process.NewProcess)
	if err != nil {
		return trace.Wrap(err)
	}
	wizard, err := localenv.LoginWizard(processConfig.WizardAddr())
	if err != nil {
		return trace.Wrap(err)
	}
	installerConfig, err := config.NewInstallerConfig(wizard, process, resources.ValidateFunc(gravity.Validate))
	if err != nil {
		return trace.Wrap(err)
	}
	listener, err := net.Listen("unix", filepath.Join(installerConfig.WriteStateDir, "installer.sock"))
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			listener.Close()
		}
	}()
	installer, err := install.New(ctx, *installerConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	interrupt.Add(installer)
	var engine install.Engine
	switch config.Mode {
	case constants.InstallModeCLI:
		engine, err = newCLInstaller(installer, config.ExcludeHostFromCluster)
	case constants.InstallModeInteractive:
		engine, err = newWizardInstaller(installer)
	default:
		return trace.BadParameter("unknown mode %q", config.Mode)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(installer.Serve(engine, listener))
}

func newCLInstaller(installer *install.Installer, excludeHostFromCluster bool) (*clinstall.Engine, error) {
	enablePreflightChecks := true
	return clinstall.New(clinstall.Config{
		FieldLogger:            installer.WithField("mode", "cli"),
		StateMachineFactory:    installer,
		ClusterFactory:         installer,
		Planner:                install.NewPlanner(enablePreflightChecks, installer),
		Operator:               installer.Operator,
		ExcludeHostFromCluster: excludeHostFromCluster,
	})
}

func newWizardInstaller(installer *install.Installer) (*interactive.Engine, error) {
	disablePreflightChecks := false
	return interactive.New(interactive.Config{
		FieldLogger:         installer.WithField("mode", "wizard"),
		StateMachineFactory: installer,
		Planner:             install.NewPlanner(disablePreflightChecks, installer),
		Operator:            installer.Operator,
		AdvertiseAddr:       installer.Process.Config().Pack.GetAddr().Addr,
	})
}

func join(env, joinEnv *localenv.LocalEnvironment, config JoinConfig) error {
	env.PrintStep("Starting agent")

	if err := config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if config.FromService {
		return trace.Wrap(Join(env, joinEnv, config))
	}
	err := JoinClient(joinEnv, config)
	if utils.IsContextCancelledError(err) {
		return trace.Wrap(err, "agent interrupted")
	}
	return nil
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

	key, err := operator.CreateSiteShrinkOperation(context.TODO(),
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
		log.WithError(err).Warn("Failed to fetch instance metadata on AWS.")
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

	env.Printf("auto joining to cluster %q via %v\n", d.clusterName, serviceURL)

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
		Auto:          true,
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
		req := systemservice.NewServiceRequest{
			ServiceSpec: systemservice.ServiceSpec{
				StartCommand: strings.Join(command, " "),
			},
			NoBlock: true,
			Name:    serviceName,
		}
		log.Infof("Installing service with req %+v.", req)
		err := systemservice.ReinstallOneshotService(req)
		if err != nil {
			return trace.Wrap(err)
		}
		env.Printf("Agent service %v started.\n", serviceName)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(ctx, cancel, InterruptSignals)
	defer func() {
		cancel()
		<-interrupt.Done()
	}()
	go TerminationHandler(interrupt, cancel, env)

	creds, err := loadRPCCredentials(context.TODO(), config.packageAddr, config.token)
	if err != nil {
		return trace.Wrap(err)
	}

	runtimeConfig := pb.RuntimeConfig{
		Token:     config.token,
		KeyValues: config.vars,
	}
	watchCh := make(chan rpcserver.WatchEvent, 1)
	agent, err := install.NewAgent(ctx, install.AgentConfig{
		FieldLogger:   log.WithField("addr", config.advertiseAddr),
		AdvertiseAddr: config.advertiseAddr,
		Credentials:   *creds,
		ServerAddr:    config.serverAddr,
		RuntimeConfig: runtimeConfig,
		WatchCh:       watchCh,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	interrupt.Add(agent)
	watchReconnects(ctx, cancel, watchCh)

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

func executeInstallPhase(localEnv *localenv.LocalEnvironment, p PhaseParams, operation *ops.SiteOperation) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	interrupt := signals.NewInterruptHandler(ctx, cancel, InterruptSignals)
	defer func() {
		cancel()
		<-interrupt.Done()
	}()
	go TerminationHandler(interrupt, cancel, localEnv)
	client, err := observerclient.New(ctx, observerclient.Config{
		StateDir:         defaults.GravityInstallDir(),
		InterruptHandler: interrupt,
		Printer:          localEnv,
		ServiceName:      defaults.GravityRPCInstallerServiceName,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	if operation == nil {
		operation, err = ops.GetWizardOperation(wizardEnv.Operator)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	machine, err := install.NewFSM(install.FSMConfig{
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
	return trace.Wrap(client.ExecutePhase(ctx, machine, fsm.Params{
		PhaseID: p.PhaseID,
		Force:   p.Force,
	}))
}

func executeJoinPhase(localEnv, joinEnv *localenv.LocalEnvironment, p PhaseParams, operation *ops.SiteOperation) error {
	operator, err := joinEnv.CurrentOperator(httplib.WithInsecure())
	if err != nil {
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
	if operation == nil {
		// determine the ongoing expand operation, it should be the only
		// operation present in the local join-specific backend
		operation, err = ops.GetExpandOperation(joinEnv.Backend)
		if err != nil {
			return trace.Wrap(err)
		}
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
	// FIXME: implement with the observer client
	// if p.PhaseID == fsm.RootPhase {
	// 	return trace.Wrap(ResumeInstall(ctx, joinFSM, progress, p.Force))
	// }
	return joinFSM.ExecutePhase(ctx, fsm.Params{
		PhaseID:  p.PhaseID,
		Force:    p.Force,
		Progress: progress,
	})
}

func rollbackJoinPhase(localEnv, joinEnv *localenv.LocalEnvironment, p PhaseParams, operation *ops.SiteOperation) error {
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
	if operation == nil {
		// determine the ongoing expand operation, it should be the only
		// operation present in the local join-specific backend
		operation, err = ops.GetExpandOperation(joinEnv.Backend)
		if err != nil {
			return trace.Wrap(err)
		}
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

func rollbackInstallPhase(localEnv *localenv.LocalEnvironment, p PhaseParams, operation *ops.SiteOperation) error {
	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	if operation == nil {
		operation, err = ops.GetWizardOperation(wizardEnv.Operator)
		if err != nil {
			return trace.Wrap(err)
		}
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

func completeInstallPlan(localEnv *localenv.LocalEnvironment, operation *ops.SiteOperation) error {
	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	if operation == nil {
		operation, err = ops.GetWizardOperation(wizardEnv.Operator)
		if err != nil {
			return trace.Wrap(err)
		}
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

func completeJoinPlan(localEnv, joinEnv *localenv.LocalEnvironment, operation *ops.SiteOperation) error {
	operator, err := joinEnv.CurrentOperator(httplib.WithInsecure())
	if err != nil {
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
	if operation == nil {
		operation, err = ops.GetExpandOperation(joinEnv.Backend)
		if err != nil {
			return trace.Wrap(err)
		}
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

func isCancelledError(err error) bool {
	if err == nil {
		return false
	}
	return trace.IsCompareFailed(err) && strings.Contains(err.Error(), "cancelled")
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

func loadRPCCredentials(ctx context.Context, addr, token string) (*rpcserver.Credentials, error) {
	// Assume addr to be a complete address if it's prefixed with `http`
	if !strings.Contains(addr, "http") {
		host, port := utils.SplitHostPort(addr, strconv.Itoa(defaults.GravitySiteNodePort))
		addr = fmt.Sprintf("https://%v:%v", host, port)
	}
	httpClient := roundtrip.HTTPClient(httplib.GetClient(true))
	packages, err := webpack.NewBearerClient(addr, token, httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	creds, err := install.LoadRPCCredentials(ctx, packages, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}
