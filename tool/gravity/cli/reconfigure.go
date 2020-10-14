/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/client"
	"github.com/gravitational/gravity/lib/install/reconfigure"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/cli"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const reconfigureMessage = `This action will launch the operation of reconfiguring this node to use a new advertise address %v.
Would you like to proceed? You can launch the command with --confirm flag to suppress this prompt in future.`

// reconfigureCluster starts the cluster reconfiguration operation.
//
// Currently, the reconfiguration operation only allows to change advertise
// address for single-node clusters.
func reconfigureCluster(env *localenv.LocalEnvironment, config reconfigureConfig, confirmed bool) error {
	// Validate that the operation is ok to launch.
	localState, err := validateReconfiguration(env, config.InstallConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Local cluster state: %#v.", localState)
	config.Apply(localState.Cluster, localState.InstallOperation)
	if err := config.CheckAndSetDefaults(resources.ValidateFunc(gravity.Validate)); err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Using config: %#v.", config)
	if config.FromService {
		err := startReconfiguratorFromService(env, config, localState)
		if utils.IsContextCancelledError(err) {
			return trace.Wrap(err, "reconfigurator interrupted")
		}
		return trace.Wrap(err)
	}
	if !confirmed {
		env.Println(color.YellowString(reconfigureMessage, config.AdvertiseAddr))
		confirmed, err := confirm()
		if err != nil {
			return trace.Wrap(err)
		}
		if !confirmed {
			env.Println("Action cancelled by user.")
			return nil
		}
	}
	env.PrintStep("Starting reconfiguration to advertise address %v", config.AdvertiseAddr)
	baseDir := utils.Exe.WorkingDir
	stateDir := state.GravityInstallDirAt(baseDir)
	strategy, err := newReconfiguratorConnectStrategy(env, baseDir, config, cli.CommandArgs{
		Parser: cli.ArgsParserFunc(parseArgs),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = InstallerClient(env, client.Config{
		ConnectStrategy: strategy,
		Lifecycle: &client.AutomaticLifecycle{
			Aborter:   AborterForMode(stateDir, config.Mode, env),
			Completer: InstallerCompleteOperation(stateDir, env),
		},
	})
	if utils.IsContextCancelledError(err) {
		if err := InstallerCleanup(stateDir); err != nil {
			logrus.WithError(err).Error("Failed to clean up installer.")
		}
		return trace.Wrap(err, "reconfigurator interrupted")
	}
	return trace.Wrap(err)
}

func startReconfiguratorFromService(env *localenv.LocalEnvironment, config reconfigureConfig, localState *localenv.LocalState) error {
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.NewInterruptHandler(ctx, cancel, InterruptSignals)
	defer interrupt.Close()
	go TerminationHandler(interrupt, env)
	socketPath := state.GravityInstallDirAt(config.StateDir, defaults.GravityRPCInstallerSocketName)
	listener, err := NewServiceListener(socketPath)
	if err != nil {
		return trace.Wrap(utils.NewPreconditionFailedError(err))
	}
	defer func() {
		if err != nil {
			listener.Close()
		}
	}()
	installerConfig, err := newInstallerConfig(ctx, env, config.InstallConfig)
	if err != nil {
		return trace.Wrap(utils.NewPreconditionFailedError(err))
	}
	installer, err := newReconfigurator(ctx, installerConfig, localState)
	if err != nil {
		return trace.Wrap(utils.NewPreconditionFailedError(err))
	}
	interrupt.AddStopper(installer)
	return trace.Wrap(installer.Run(listener))
}

func newReconfigurator(ctx context.Context, config *install.Config, state *localenv.LocalState) (*install.Installer, error) {
	engine, err := reconfigure.NewEngine(reconfigure.Config{
		Operator: config.Operator,
		State:    state,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.LocalAgent = false // To make sure agent does not get launched on this node.
	installer, err := install.New(ctx, install.RuntimeConfig{
		Config:         *config,
		Planner:        reconfigure.NewPlanner(config, state.Cluster),
		FSMFactory:     reconfigure.NewFSMFactory(*config),
		ClusterFactory: install.NewClusterFactory(*config),
		Engine:         engine,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return installer, nil
}

// validateReconfiguration determines if reconfiguration can be launched on this
// node.
//
// If all checks pass, returns information about the existing cluster state such
// as the cluster object and the original install operation.
func validateReconfiguration(env *localenv.LocalEnvironment, config InstallConfig) (*localenv.LocalState, error) {
	// The cluster should be installed but not running.
	err := localenv.DetectCluster(context.TODO(), env)
	if err != nil && trace.IsNotFound(err) {
		return nil, trace.BadParameter("Gravity doesn't appear to be installed on this node.")
	}
	if err == nil {
		return nil, trace.BadParameter(`Gravity appears to be running on this node. Please stop it using "gravity stop" first.`)
	}
	localState, err := env.GetLocalState()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Only single-node clusters are supported.
	servers := localState.Cluster.ClusterState.Servers
	if len(servers) != 1 {
		return nil, trace.BadParameter("Only single-node clusters can be reconfigured.")
	}
	advertiseIP := servers[0].AdvertiseIP
	if advertiseIP == config.AdvertiseAddr {
		return nil, trace.BadParameter("This node is already using advertise address %v.", advertiseIP)
	}
	return localState, nil
}
