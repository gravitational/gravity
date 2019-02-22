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
	libclusterconfig "github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/clusterconfig"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// ResetConfig executes the loop to reset cluster configuration to defaults
func ResetConfig(localEnv, updateEnv *localenv.LocalEnvironment, manual, confirmed bool) error {
	config := libclusterconfig.NewEmpty()
	bytes, err := libclusterconfig.Marshal(config)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(updateConfig(context.TODO(), localEnv, updateEnv, bytes, manual, confirmed))
}

// UpdateConfig executes the loop to update cluster configuration.
// resource specifies the new configuration to apply.
func UpdateConfig(localEnv, updateEnv *localenv.LocalEnvironment, resource []byte, manual, confirmed bool) error {
	return trace.Wrap(updateConfig(context.TODO(), localEnv, updateEnv, resource, manual, confirmed))
}

func updateConfig(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, resource []byte, manual, confirmed bool) error {
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
	updater, err := newConfigUpdater(ctx, localEnv, updateEnv, resource)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if !manual {
		err = updater.Run(ctx, false)
		return trace.Wrap(err)
	}
	localEnv.Println(updateConfigManualOperationBanner)
	return nil
}

func newConfigUpdater(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, resource []byte) (*update.Updater, error) {
	clusterConfig, err := libclusterconfig.Unmarshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	init := configInitializer{
		resource: resource,
		config:   clusterConfig,
	}
	return newUpdater(ctx, localEnv, updateEnv, init, false)
}

func executeConfigPhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	err = updater.RunPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func rollbackConfigPhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	err = updater.RollbackPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func completeConfigPlan(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	return trace.Wrap(updater.Complete(nil))
}

func getConfigUpdater(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) (*update.Updater, error) {
	clusterEnv, err := env.NewClusterEnvironment()
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
			Silent:       env.Silent,
			FieldLogger: logrus.WithFields(logrus.Fields{
				trace.Component: "update:clusterconfig",
				"operation":     operation,
			}),
		},
		Apps:            clusterEnv.Apps,
		Client:          clusterEnv.Client,
		ClusterPackages: clusterEnv.ClusterPackages,
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
	key, err := operator.CreateUpdateConfigOperation(
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
) error {
	_, err := clusterconfig.NewOperationPlan(operator, operation, r.config, cluster.ClusterState.Servers)
	return trace.Wrap(err)
}

func (configInitializer) newUpdater(
	ctx context.Context,
	operator ops.Operator,
	operation ops.SiteOperation,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	runner fsm.AgentRepository,
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
		Apps:            clusterEnv.Apps,
		Client:          clusterEnv.Client,
		ClusterPackages: clusterEnv.ClusterPackages,
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

See https://gravitational.com/gravity/docs/cluster/#managing-an-ongoing-operation for details on working with operation plan.`
)
