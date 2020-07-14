/*
Copyright 2019 Gravitational, Inc.

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

	"github.com/gravitational/gravity/lib/fsm"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	libclusterconfig "github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/clusterconfig"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// resetConfig executes the loop to reset cluster configuration to defaults
func resetConfig(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, manual, confirmed bool) error {
	config := libclusterconfig.NewEmpty()
	return trace.Wrap(updateConfig(ctx, localEnv, updateEnv, config, manual, confirmed))
}

func updateConfig(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, config libclusterconfig.Interface, manual, confirmed bool) error {
	if err := validateCloudConfig(localEnv, config); err != nil {
		return trace.Wrap(err)
	}
	if !confirmed {
		if manual {
			localEnv.Println(updateConfigBannerManual)
		} else {
			localEnv.Println(updateConfigBanner)
		}
		resp, err := confirm()
		if err != nil {
			return trace.Wrap(err)
		}
		if !resp {
			localEnv.Println("Action cancelled by user.")
			return nil
		}
	}
	updater, err := newConfigUpdater(ctx, localEnv, updateEnv, config)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if !manual {
		err = updater.Run(ctx)
		return trace.Wrap(err)
	}
	localEnv.Println(updateConfigManualOperationBanner)
	return nil
}

func newConfigUpdater(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, config libclusterconfig.Interface) (*update.Updater, error) {
	configBytes, err := libclusterconfig.Marshal(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	init := configInitializer{
		resource: configBytes,
		config:   config,
	}
	return newUpdater(ctx, localEnv, updateEnv, init)
}

func executeConfigPhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	err = updater.RunPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func setConfigPhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params SetPhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	return updater.SetPhase(context.TODO(), params.PhaseID, params.State)
}

func rollbackConfigPhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	err = updater.RollbackPhase(context.TODO(), fsm.Params{
		PhaseID: params.PhaseID,
		Force:   params.Force,
		DryRun:  params.DryRun,
	}, params.Timeout)
	return trace.Wrap(err)
}

func completeConfigPlanForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if err := updater.Complete(nil); err != nil {
		return trace.Wrap(err)
	}
	if err := updater.Activate(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getConfigUpdater(localEnv, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) (*update.Updater, error) {
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

	updater, err := clusterconfig.New(context.TODO(), clusterconfig.Config{
		Config: update.Config{
			Operation:    &operation,
			Operator:     operator,
			Backend:      clusterEnv.Backend,
			LocalBackend: updateEnv.Backend,
			Runner:       runner,
			Silent:       localEnv.Silent,
			FieldLogger: logrus.WithFields(logrus.Fields{
				trace.Component: "update:clusterconfig",
				"operation":     operation,
			}),
		},
		Apps:              clusterEnv.Apps,
		Client:            clusterEnv.Client,
		ClusterPackages:   clusterEnv.ClusterPackages,
		HostLocalPackages: localEnv.Packages,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

func (r configInitializer) validatePreconditions(*localenv.LocalEnvironment, ops.Operator, ops.Site) error {
	return nil
}

func (r configInitializer) newOperation(operator ops.Operator, cluster ops.Site) (*ops.SiteOperationKey, error) {
	key, err := operator.CreateUpdateConfigOperation(context.TODO(),
		ops.CreateUpdateConfigOperationRequest{
			ClusterKey: cluster.Key(),
			Config:     r.resource,
		},
	)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotImplemented(
				"cluster operator does not implement the API required for updating configuration. " +
					"Please make sure you're running the command on a compatible cluster.")
		}
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (r configInitializer) newOperationPlan(
	ctx context.Context,
	operator ops.Operator,
	cluster ops.Site,
	operation ops.SiteOperation,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	leader *storage.Server,
) (*storage.OperationPlan, error) {
	plan, err := clusterconfig.NewOperationPlan(operator, clusterEnv.Apps, operation, r.config, cluster.ClusterState.Servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

func (configInitializer) newUpdater(
	ctx context.Context,
	operator ops.Operator,
	operation ops.SiteOperation,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	runner rpc.AgentRepository,
) (*update.Updater, error) {
	config := clusterconfig.Config{
		Config: update.Config{
			Operation:    &operation,
			Operator:     operator,
			Backend:      clusterEnv.Backend,
			LocalBackend: updateEnv.Backend,
			Runner:       runner,
			Silent:       localEnv.Silent,
			FieldLogger: logrus.WithFields(logrus.Fields{
				trace.Component: "update:clusterconfig",
				"operation":     operation,
			}),
		},
		Apps:              clusterEnv.Apps,
		Client:            clusterEnv.Client,
		ClusterPackages:   clusterEnv.ClusterPackages,
		HostLocalPackages: localEnv.Packages,
	}
	return clusterconfig.New(ctx, config)
}

func (configInitializer) updateDeployRequest(req deployAgentsRequest) deployAgentsRequest {
	return req
}

type configInitializer struct {
	resource []byte
	config   libclusterconfig.Interface
}

func validateCloudConfig(localEnv *localenv.LocalEnvironment, config libclusterconfig.Interface) error {
	if newGlobalConfig := config.GetGlobalConfig(); !isCloudConfigEmpty(newGlobalConfig) {
		// TODO(dmitri): require cloud provider if cloud-config is being updated
		// This is more a sanity check than a hard requirement so users are explicit about changes
		// in the cloud configuration
		if newGlobalConfig.CloudConfig != "" && newGlobalConfig.CloudProvider == "" {
			return trace.BadParameter("cloud provider is required when updating cloud configuration")
		}
	}
	operator, err := localEnv.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterConfig, err := operator.GetClusterConfiguration(cluster.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	globalConfig := clusterConfig.GetGlobalConfig()
	if isCloudConfigEmpty(globalConfig) {
		if newGlobalConfig := config.GetGlobalConfig(); !isCloudConfigEmpty(newGlobalConfig) {
			return trace.BadParameter("cannot change cloud configuration: cluster does not have cloud provider configured")
		}
	}
	if globalConfig != nil {
		if newGlobalConfig := config.GetGlobalConfig(); newGlobalConfig != nil {
			if newGlobalConfig.CloudProvider != "" && globalConfig.CloudProvider != newGlobalConfig.CloudProvider {
				return trace.BadParameter("changing cloud provider is not supported (%q -> %q)",
					newGlobalConfig.CloudProvider, globalConfig.CloudProvider)
			}
			if globalConfig.CloudProvider == "" && newGlobalConfig.CloudConfig != "" {
				return trace.BadParameter("cannot set cloud configuration: cluster does not have cloud provider configured")
			}
		}
	}
	return nil
}

func isCloudConfigEmpty(global *libclusterconfig.Global) bool {
	return global == nil || (global.CloudProvider == "" && global.CloudConfig == "")
}

const (
	updateConfigBanner = `Updating cluster configuration might require restart of runtime containers on master nodes.
The operation might take a few minutes to complete.

The operation will start automatically once you approve it.
If you want to review the operation plan first or execute it manually step by step,
run the operation in manual mode by specifying '--manual' flag.

Are you sure?`
	updateConfigBannerManual = `Updating cluster configuration might require restart of runtime containers on master nodes.
The operation might take a few minutes to complete.

"Are you sure?`
	updateConfigManualOperationBanner = `The operation has been created in manual mode.

See https://gravitational.com/gravity/docs/cluster/#managing-operations for details on working with operation plan.`
)
