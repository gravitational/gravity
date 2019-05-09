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
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/process"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/configure"
	"github.com/gravitational/trace"
)

// InstallerClient runs the client for the installer service.
// The client is responsible for triggering the install operation and observing
// operation progress
func InstallerClient(printer utils.Printer, config installerclient.Config) error {
	printInstallInstructionsBanner(printer)
	// Context to use for cancelling tasks before initialization is complete
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(cancel, clientInterruptSignals)
	defer interrupt.Close()
	clientC := clientTerminationHandler(interrupt, printer)

	config.InterruptHandler = interrupt
	if len(config.Args) == 0 {
		config.Args = installerServiceCommandline(filepath.Dir(utils.Exe.Path))
	}

	printer.PrintStep("Connecting to installer")
	client, err := installerclient.New(ctx, config)
	if err != nil {
		return trace.Wrap(err)
	}
	addCancelInstallationHandler(ctx, client, clientC)
	printer.PrintStep("Connected to installer")
	return trace.Wrap(client.Run(context.Background()))
}

// JoinClient runs the client for the agent service.
// The client is responsible for starting the RPC agent and observing
// operation progress
func JoinClient(printer utils.Printer, config installerclient.Config) error {
	printJoinInstructionsBanner(printer)
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(cancel, clientInterruptSignals)
	defer interrupt.Close()
	clientC := clientTerminationHandler(interrupt, printer)

	config.InterruptHandler = interrupt

	printer.PrintStep("Connecting to agent")
	client, err := installerclient.New(ctx, config)
	if err != nil {
		return trace.Wrap(err)
	}
	addCancelInstallationHandler(ctx, client, clientC)
	printer.PrintStep("Connected to agent")
	return trace.Wrap(client.Run(context.Background()))
}

// TODO: auto mode can be implemented in such a way that the server also runs a client
func Join(env, joinEnv *localenv.LocalEnvironment, config JoinConfig) error {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	log.WithField(trace.Component, "agent").Info("Running in service mode.")
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, env)
	listener, err := NewServiceListener()
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
	interrupt.AddStopper(peer)
	return trace.Wrap(peer.Run(listener))
}

// InterruptSignals lists signals installer service considers interrupts
var InterruptSignals = signals.WithSignals(
	os.Interrupt,
	syscall.SIGTERM,
	syscall.SIGQUIT,
)

// NewServiceListener returns a new listener for the installer service
func NewServiceListener() (net.Listener, error) {
	return net.Listen("unix", installpb.SocketPath(defaults.GravityEphemeralDir))
}

// clientInterruptSignals lists signals installer client considers interrupts
var clientInterruptSignals = signals.WithSignals(
	os.Interrupt,
)

// clientTerminationHandler implements the default interrupt handler for the installer client
func clientTerminationHandler(interrupt *signals.InterruptHandler, printer utils.Printer) chan<- *installerclient.Client {
	const abortTimeout = 5 * time.Second
	clientC := make(chan *installerclient.Client, 1)
	go func() {
		var timerC <-chan time.Time
		// number of consecutive interrupts
		var interrupts int
		var client *installerclient.Client
		for {
			select {
			case sig := <-interrupt.C:
				if sig != os.Interrupt || client == nil || client.Completed() {
					// Fast path
					interrupt.Cancel()
					return
				}
				// Interrupt signaled
				interrupts += 1
				if interrupts > 1 {
					printer.Println("Received", sig, "signal. Aborting the installer gracefully, please wait.")
					interrupt.Abort()
					return
				}
				printer.Println("Press Ctrl+C again to abort the installation.")
				// If the interrupt signal is not re-triggered within the allotted time,
				// the signal is dropped
				timerC = time.After(abortTimeout)
			case <-timerC:
				// Drop this interrupt signal
				interrupts = 0
				timerC = nil
			case client = <-clientC:
			case <-interrupt.Done():
				return
			}
		}
	}()
	return clientC
}

// TerminationHandler implements the default interrupt handler for the installer service
func TerminationHandler(interrupt *signals.InterruptHandler, printer utils.Printer) {
	for {
		select {
		case sig := <-interrupt.C:
			log.Info("Received ", sig, " signal. Terminating the installer gracefully, please wait.")
			if sig == os.Interrupt {
				// Abort for Ctrl+C
				interrupt.Abort()
			} else {
				interrupt.Cancel()
			}
			return
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
	err := InstallerClient(env, installerclient.Config{
		StateChecker: CheckInstallerLocalState(env),
		Printer:      env,
	})
	if utils.IsContextCancelledError(err) {
		return trace.Wrap(err, "installer interrupted")
	}
	return trace.Wrap(err)
}

func startInstallFromService(env *localenv.LocalEnvironment, config InstallConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, env)
	processConfig, err := config.NewProcessConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	process, err := install.InitProcess(ctx, *processConfig, process.NewProcess)
	if err != nil {
		return trace.Wrap(err)
	}
	wizard, err := localenv.LoginWizard(processConfig.WizardAddr(), config.Token)
	if err != nil {
		return trace.Wrap(err)
	}
	installerConfig, err := config.NewInstallerConfig(env, wizard, process, resources.ValidateFunc(gravity.Validate))
	if err != nil {
		return trace.Wrap(err)
	}
	listener, err := NewServiceListener()
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
	interrupt.AddStopper(installer)
	var engine install.Engine
	switch config.Mode {
	case constants.InstallModeCLI:
		engine, err = newCLInstaller(installer)
	case constants.InstallModeInteractive:
		engine, err = newWizardInstaller(installer)
	default:
		return trace.BadParameter("unknown mode %q", config.Mode)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(installer.Run(engine, listener))
}

func newCLInstaller(installer *install.Installer) (*clinstall.Engine, error) {
	enablePreflightChecks := true
	return clinstall.New(clinstall.Config{
		FieldLogger:         installer.WithField("mode", "cli"),
		StateMachineFactory: installer,
		ClusterFactory:      installer,
		Planner:             install.NewPlanner(enablePreflightChecks, installer),
		Operator:            installer.Operator,
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

// ResumeInstall resumes the aborted install operation
func ResumeInstall(env *localenv.LocalEnvironment) error {
	env.PrintStep("Starting installer")

	// TODO(dmitri): this needs to differentiate between running installer (use resume observer mode)
	// and stopped installer - use manual plan execution
	// TODO(dmitri): detect install/join modality based on ephemeral directories and have client
	// connect with the socket from corresponding location.	This will also enable the reuse of 'plan resume'
	// from a joining node in case of expand
	err := InstallerClient(env, installerclient.Config{
		Resume:  true,
		Printer: env,
	})
	if utils.IsContextCancelledError(err) {
		return trace.Wrap(err, "installer interrupted")
	}
	return trace.Wrap(err)
}

func join(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, config JoinConfig) error {
	env.PrintStep("Starting agent")

	if err := config.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if config.FromService {
		joinEnv, err := environ.NewJoinEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer joinEnv.Close()
		return trace.Wrap(Join(env, joinEnv, config))
	}
	err := JoinClient(env, installerclient.Config{
		Args:         joinServiceCommandline(),
		StateChecker: checkAgentLocalState(env),
		Printer:      env,
	})
	if utils.IsContextCancelledError(err) {
		return trace.Wrap(err, "agent interrupted")
	}
	return trace.Wrap(err)
}

func resumeJoin(env *localenv.LocalEnvironment) error {
	env.PrintStep("Starting agent")

	err := JoinClient(env, installerclient.Config{
		Printer: env,
		Resume:  true,
	})
	if utils.IsContextCancelledError(err) {
		return trace.Wrap(err, "agent interrupted")
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
		log.WithError(err).Warn("Failed to leave cluster, forcing.")
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
		err := service.ReinstallOneshot(req)
		if err != nil {
			return trace.Wrap(err)
		}
		env.Printf("Agent service %v started.\n", serviceName)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, env)

	creds, err := loadRPCCredentials(ctx, config.packageAddr, config.token)
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

	interrupt.AddStopper(agent)
	watchReconnects(ctx, cancel, watchCh)

	return trace.Wrap(agent.Serve())
}

func executeJoinPhase(localEnv, joinEnv *localenv.LocalEnvironment, params PhaseParams, operation *ops.SiteOperation) error {
	ctx, cancel := context.WithTimeout(context.Background(), params.Timeout)
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, localEnv)
	client, err := observerclient.New(ctx, observerclient.Config{
		InterruptHandler: interrupt,
		Printer:          localEnv,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := newJoinMachine(localEnv, joinEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(client.Execute(ctx, machine, fsm.Params{
		PhaseID: params.PhaseID,
		Force:   params.Force,
	}))
}

func rollbackJoinPhase(localEnv, joinEnv *localenv.LocalEnvironment, params PhaseParams, operation *ops.SiteOperation) error {
	ctx, cancel := context.WithTimeout(context.Background(), params.Timeout)
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, localEnv)
	client, err := observerclient.New(ctx, observerclient.Config{
		InterruptHandler: interrupt,
		Printer:          localEnv,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := newJoinMachine(localEnv, joinEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(client.Rollback(ctx, machine, fsm.Params{
		PhaseID: params.PhaseID,
		Force:   params.Force,
	}))
}

func completeInstallPlan(localEnv *localenv.LocalEnvironment, operation *ops.SiteOperation) error {
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, localEnv)
	_, err := observerclient.New(ctx, observerclient.Config{
		InterruptHandler: interrupt,
		Printer:          localEnv,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := newInstallMachine(localEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return machine.Complete(trace.Errorf("completed manually"))
	// TODO(dmitri): shut down remote agents
}

func completeJoinPlan(localEnv, joinEnv *localenv.LocalEnvironment, operation *ops.SiteOperation) error {
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, localEnv)
	_, err := observerclient.New(ctx, observerclient.Config{
		InterruptHandler: interrupt,
		Printer:          localEnv,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := newJoinMachine(localEnv, joinEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return machine.Complete(trace.Errorf("completed manually"))
}

func installerServiceCommandline(applicationDir string) (args []string) {
	args = append([]string{utils.Exe.Path}, os.Args[1:]...)
	return append(args, "--from-service", applicationDir)
}

func joinServiceCommandline() (args []string) {
	args = append([]string{utils.Exe.Path}, os.Args[1:]...)
	return append(args, "--from-service")
}

func addCancelInstallationHandler(ctx context.Context, client *installerclient.Client, clientC chan<- *installerclient.Client) {
	select {
	case clientC <- client:
	case <-ctx.Done():
	}
}

// ExecutePhase executes installation phase specified with params.
// Implements Installer
func (defaultInstaller) ExecutePhase(
	localEnv *localenv.LocalEnvironment,
	params PhaseParams,
	operation *ops.SiteOperation,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), params.Timeout)
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, localEnv)
	client, err := observerclient.New(ctx, observerclient.Config{
		InterruptHandler: interrupt,
		Printer:          localEnv,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := newInstallMachine(localEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(client.Execute(ctx, machine, fsm.Params{
		PhaseID: params.PhaseID,
		Force:   params.Force,
	}))
}

// RollbackPhase rolls back installation phase specified with params.
// Implements Installer
func (defaultInstaller) RollbackPhase(
	localEnv *localenv.LocalEnvironment,
	params PhaseParams,
	operation *ops.SiteOperation,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), params.Timeout)
	interrupt := signals.NewInterruptHandler(cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, localEnv)
	client, err := observerclient.New(ctx, observerclient.Config{
		InterruptHandler: interrupt,
		Printer:          localEnv,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := newInstallMachine(localEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(client.Rollback(ctx, machine, fsm.Params{
		PhaseID: params.PhaseID,
		Force:   params.Force,
	}))
}

// Resume resumes aborted installation.
// Implements Installer
func (defaultInstaller) Resume(env *localenv.LocalEnvironment) error {
	if isJoinEnv() {
		return trace.Wrap(resumeJoin(env))
	}
	return trace.Wrap(ResumeInstall(env))
}

type defaultInstaller struct{}

// Installer manages installation-specific tasks
type Installer interface {
	// ExecutePhase executes an installation phase specified with params
	ExecutePhase(*localenv.LocalEnvironment, PhaseParams, *ops.SiteOperation) error
	// RollbackPhase rolls back an installation phase specified with params
	RollbackPhase(*localenv.LocalEnvironment, PhaseParams, *ops.SiteOperation) error
	// Resume resumes aborted installation
	Resume(*localenv.LocalEnvironment) error
}

func newInstallMachine(env *localenv.LocalEnvironment, operation *ops.SiteOperation) (*fsm.FSM, error) {
	localApps, err := env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if operation == nil {
		operation, err = ops.GetWizardOperation(wizardEnv.Operator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	machine, err := install.NewFSM(install.FSMConfig{
		OperationKey:       operation.Key(),
		Packages:           wizardEnv.Packages,
		Apps:               wizardEnv.Apps,
		Operator:           wizardEnv.Operator,
		LocalClusterClient: env.SiteOperator,
		LocalPackages:      env.Packages,
		LocalApps:          localApps,
		LocalBackend:       env.Backend,
		Insecure:           env.Insecure,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return machine, nil
}

func newJoinMachine(env, joinEnv *localenv.LocalEnvironment, operation *ops.SiteOperation) (*fsm.FSM, error) {
	operator, err := joinEnv.CurrentOperator(httplib.WithInsecure())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apps, err := joinEnv.CurrentApps(httplib.WithInsecure())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	packages, err := joinEnv.CurrentPackages(httplib.WithInsecure())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if operation == nil {
		// determine the ongoing expand operation, it should be the only
		// operation present in the local join-specific backend
		operation, err = ops.GetExpandOperation(joinEnv.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	machine, err := expand.NewFSM(expand.FSMConfig{
		OperationKey:  operation.Key(),
		Operator:      operator,
		Apps:          apps,
		Packages:      packages,
		LocalBackend:  env.Backend,
		JoinBackend:   joinEnv.Backend,
		LocalPackages: env.Packages,
		LocalApps:     env.Apps,
		DebugMode:     env.Debug,
		Insecure:      env.Insecure,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return machine, nil
}

// CheckInstallerLocalState performs a local environment sanity check to make sure
// that install on this node can proceed without issues
func CheckInstallerLocalState(env *localenv.LocalEnvironment) func() error {
	return func() error {
		empty, err := utils.IsDirectoryEmpty(defaults.GravityInstallDir())
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err == nil && !empty {
			return trace.BadParameter("detected previous installation state in %v, "+
				"please resume installation with `gravity plan resume` or "+
				"clean it up using `gravity leave --force` before proceeding "+
				"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
				defaults.GravityInstallDir())

		}
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
}

// checkAgentLocalState performs a local environment sanity check to make sure
// that join on this node can proceed without issues
func checkAgentLocalState(env *localenv.LocalEnvironment) func() error {
	return func() error {
		empty, err := utils.IsDirectoryEmpty(defaults.GravityJoinDir())
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err == nil && !empty {
			return trace.BadParameter("detected previous installation state in %v, "+
				"please resume the agent with `gravity join resume` or "+
				"clean it up using `gravity leave --force` before proceeding "+
				"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
				defaults.GravityJoinDir())

		}
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
}

// isJoinEnv implements a simple heuristic to determine if the local host
// is a joining or installer node.
// TODO(dmitri): there should be a better way to detect the context for the
// 'plan resume' command
func isJoinEnv() (ok bool) {
	ok, err := utils.IsDirectoryEmpty(defaults.GravityJoinDir())
	if err == nil && !ok {
		return true
	}
	return false
}

// TODO: different banner for the joining agent as 'gravity plan resume' is only
// meanimgful from the installer node.
func printInstallInstructionsBanner(printer utils.Printer) {
	printer.Println(`
To abort the installation and clean up the system,
press Ctrl+C two times in a row.

If the you get disconnected from the terminal, you can reconnect to the installer
agent by issuing 'gravity plan resume' command.

If the installation fails, use 'gravity plan' to inspect the state and
'gravity plan resume' to continue the operation.
See https://gravitational.com/gravity/docs/cluster/#managing-an-ongoing-operation for details.
`)
}

func printJoinInstructionsBanner(printer utils.Printer) {
	printer.Println(`
To abort the agent and clean up the system,
press Ctrl+C two times in a row.

If the you get disconnected from the terminal, you can reconnect to the installer
agent by issuing 'gravity plan resume' command.
See https://gravitational.com/gravity/docs/cluster/#managing-an-ongoing-operation for details.
`)
}
