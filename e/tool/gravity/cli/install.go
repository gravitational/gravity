// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cli

import (
	"context"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/install"
	"github.com/gravitational/gravity/e/lib/process"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	ossinstall "github.com/gravitational/gravity/lib/install"
	installerclient "github.com/gravitational/gravity/lib/install/client"
	clinstall "github.com/gravitational/gravity/lib/install/engine/cli"
	"github.com/gravitational/gravity/lib/install/engine/interactive"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"
	cliutils "github.com/gravitational/gravity/lib/utils/cli"
	"github.com/gravitational/gravity/tool/gravity/cli"

	"github.com/gravitational/trace"
)

func startInstall(env *environment.Local, config InstallConfig) error {
	if err := config.BootstrapSELinux(context.TODO(), env); err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Starting enterprise installer")
	if err := config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if config.FromService {
		err := startInstallFromService(env.LocalEnvironment, config)
		if utils.IsContextCancelledError(err) {
			return trace.Wrap(err, "installer interrupted")
		}
		return trace.Wrap(err)
	}
	if err := config.RunLocalChecks(); err != nil {
		return trace.Wrap(err)
	}
	strategy, err := newInstallerConnectStrategy(env.LocalEnvironment, config)
	if err != nil {
		return trace.Wrap(err)
	}
	err = cli.InstallerClient(env.LocalEnvironment, installerclient.Config{
		ConnectStrategy: strategy,
		Lifecycle: &installerclient.AutomaticLifecycle{
			Aborter:            cli.AborterForMode(strategy.ServiceName, config.Mode, env.LocalEnvironment),
			Completer:          cli.InstallerCompleteOperation(strategy.ServiceName, env.LocalEnvironment),
			DebugReportPath:    cli.DebugReportPath(),
			LocalDebugReporter: cli.InstallerGenerateLocalReport(env.LocalEnvironment),
		},
	})
	if utils.IsContextCancelledError(err) {
		// We only end up here if the initialization has not been successful - clean up the state
		if err := cli.InstallerCleanup(strategy.ServiceName); err != nil {
			log.Warnf("Failed to clean up installer: %v.", err)
		}
		return trace.Wrap(err, "installer interrupted")
	}
	return trace.Wrap(err)
}

func newInstallerConnectStrategy(env *localenv.LocalEnvironment, config InstallConfig) (*installerclient.InstallerStrategy, error) {
	commandArgs := cliutils.CommandArgs{
		Parser: cliutils.ArgsParserFunc(parseArgs),
	}
	licensePath, err := config.LicenseFilePath()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// Make sure that only the --license-file flag is present in the service
	// command-line.
	if licensePath != "" {
		commandArgs.FlagsToAdd = []cliutils.Flag{cliutils.NewFlag("license-file", licensePath)}
		commandArgs.FlagsToRemove = []string{"license", "license-file"}
	}
	strategy, err := cli.NewInstallerConnectStrategy(env, config.InstallConfig, commandArgs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return strategy, nil
}

func startInstallFromService(env *localenv.LocalEnvironment, config InstallConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(ctx, cancel, cli.InterruptSignals)
	defer interrupt.Close()
	go cli.TerminationHandler(interrupt, env)
	socketPath := state.GravityInstallDirAt(config.StateDir, defaults.GravityRPCInstallerSocketName)
	listener, err := cli.NewServiceListener(socketPath)
	if err != nil {
		return trace.Wrap(utils.NewPreconditionFailedError(err))
	}
	defer func() {
		if err != nil {
			listener.Close()
		}
	}()
	installerConfig, err := newInstallerConfig(ctx, env, config)
	if err != nil {
		return trace.Wrap(utils.NewPreconditionFailedError(err))
	}
	var installer *ossinstall.Installer
	switch config.Mode {
	case constants.InstallModeCLI:
		installer, err = newCLInstaller(ctx, installerConfig)
	case constants.InstallModeInteractive:
		installer, err = newWizardInstaller(ctx, installerConfig)
	default:
		err = trace.BadParameter("unknown installer mode %q", config.Mode)
	}
	if err != nil {
		return trace.Wrap(utils.NewPreconditionFailedError(err))
	}
	interrupt.AddStopper(installer)
	return trace.Wrap(installer.Run(listener))
}

func newInstallerConfig(ctx context.Context, env *localenv.LocalEnvironment, config InstallConfig) (*install.Config, error) {
	processConfig, err := config.NewProcessConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	process, err := ossinstall.InitProcess(ctx, *processConfig, process.NewProcess)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wizard, err := localenv.LoginWizard(processConfig.WizardAddr(), config.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = wizard.WaitForOperator(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installerConfig, err := config.NewInstallerConfig(env, wizard, process)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return installerConfig, nil
}

func newCLInstaller(ctx context.Context, config *install.Config) (*ossinstall.Installer, error) {
	planner := &install.Planner{
		FieldLogger:       config.WithField(trace.Component, "planner"),
		Packages:          config.Packages,
		PlanBuilderGetter: &config.Config,
		PreflightChecks:   true,
		OpsTunnelToken:    config.OpsTunnelToken,
		OpsSNIHost:        config.OpsSNIHost,
		RemoteOpsURL:      config.RemoteOpsURL,
		RemoteOpsToken:    config.RemoteOpsToken,
	}
	engine, err := clinstall.New(clinstall.Config{
		FieldLogger: config.WithField("mode", "cli"),
		Operator:    config.Operator,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installer, err := ossinstall.New(ctx, ossinstall.RuntimeConfig{
		Config:     config.Config,
		FSMFactory: install.NewFSMFactory(*config),
		ClusterFactory: install.NewClusterFactory(
			*config,
			ossinstall.NewClusterFactory(config.Config),
		),
		Planner: planner,
		Engine:  engine,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return installer, nil
}

func newWizardInstaller(ctx context.Context, config *install.Config) (*ossinstall.Installer, error) {
	planner := &install.Planner{
		FieldLogger:       config.WithField(trace.Component, "planner"),
		Packages:          config.Packages,
		PlanBuilderGetter: &config.Config,
		OpsTunnelToken:    config.OpsTunnelToken,
		OpsSNIHost:        config.OpsSNIHost,
		RemoteOpsURL:      config.RemoteOpsURL,
		RemoteOpsToken:    config.RemoteOpsToken,
	}
	engine, err := interactive.New(interactive.Config{
		FieldLogger:   config.WithField("mode", "wizard"),
		Operator:      config.Operator,
		AdvertiseAddr: config.GetWizardAddr(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installer, err := ossinstall.New(ctx, ossinstall.RuntimeConfig{
		Config:     config.Config,
		FSMFactory: install.NewFSMFactory(*config),
		ClusterFactory: install.NewClusterFactory(
			*config,
			ossinstall.NewClusterFactory(config.Config),
		),
		Planner: planner,
		Engine:  engine,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return installer, nil
}
