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

	"github.com/gravitational/gravity/lib/clusterconfig"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// ResetConfig executes the loop to reset cluster configuration to defaults
func ResetConfig(localEnv, updateEnv *localenv.LocalEnvironment, manual, confirmed bool) error {
	// FIXME
	// return trace.Wrap(updateConfig(ctx, localEnv, updateEnv, env, manual, confirmed))
	return trace.NotImplemented("")
}

// UpdateConfig executes the loop to update cluster configuration.
// resource specifies the new configuration to apply.
func UpdateConfig(localEnv, updateEnv *localenv.LocalEnvironment, resource []byte, manual, confirmed bool) error {
	ctx := context.TODO()
	return trace.Wrap(updateConfig(ctx, localEnv, updateEnv, resource, manual, confirmed))
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
	if !manual {
		err = updater.Run(ctx, false)
		return trace.Wrap(err)
	}
	localEnv.Println(updateConfigManualOperationBanner)
	return nil
}

func newConfigUpdater(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, resource []byte) (*clusterconfig.Updater, error) {
	teleportClient, err := localEnv.TeleportClient(constants.Localhost)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a teleport client")
	}
	proxy, err := teleportClient.ConnectToProxy()
	if err != nil {
		return nil, trace.Wrap(err, "failed to connect to teleport proxy")
	}
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if clusterEnv.Client == nil {
		return nil, trace.BadParameter("this operation can only be executed on one of the master nodes")
	}
	operator := clusterEnv.Operator
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := operator.CreateUpdateConfigOperation(
		ops.CreateUpdateConfigOperationRequest{
			ClusterKey: cluster.Key(),
			Config:     resource,
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
	defer func() {
		r := recover()
		panicked := r != nil
		if err != nil || panicked {
			logger := logrus.WithField("operation", key)
			logger.WithError(err).Warn("Operation failed.")
			var msg string
			if err != nil {
				msg = err.Error()
			}
			if errMark := ops.FailOperation(*key, operator, msg); errMark != nil {
				logger.WithError(errMark).Warn("Failed to mark operation as failed.")
			}
		}
		if r != nil {
			panic(r)
		}
	}()
	operation, err := operator.GetSiteOperation(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Create the operation plan so it can be replicated on remote nodes
	_, err = clusterconfig.NewOperationPlan(operator, *operation, cluster.ClusterState.Servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := deployAgentsRequest{
		clusterState: cluster.ClusterState,
		clusterName:  cluster.Domain,
		clusterEnv:   clusterEnv,
		proxy:        proxy,
		nodeParams:   constants.RPCAgentSyncPlanFunction,
	}
	deployCtx, cancel := context.WithTimeout(ctx, defaults.AgentDeployTimeout)
	defer cancel()
	localEnv.Println("Deploying agents on nodes")
	creds, err := deployAgents(deployCtx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)
	config := clusterconfig.Config{
		Operator:        operator,
		Operation:       operation,
		Apps:            clusterEnv.Apps,
		Backend:         clusterEnv.Backend,
		LocalBackend:    updateEnv.Backend,
		ClusterPackages: clusterEnv.ClusterPackages,
		Client:          clusterEnv.Client,
		Servers:         cluster.ClusterState.Servers,
		ClusterKey:      cluster.Key(),
		Silent:          localEnv.Silent,
		Runner:          runner,
	}
	updater, err := clusterconfig.New(ctx, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

func executeConfigPhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}

	err = updater.RunPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func rollbackConfigPhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.RollbackPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func completeConfigPlan(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(updater.Complete())
}

func getUpdateConfigOperationPlan(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) (*storage.OperationPlan, error) {
	updater, err := getConfigUpdater(env, updateEnv, operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plan, err := updater.GetPlan()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

func getConfigUpdater(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) (*clusterconfig.Updater, error) {
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operator := clusterEnv.Operator

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := libfsm.GetClientCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)

	updater, err := clusterconfig.New(context.TODO(), clusterconfig.Config{
		Operator:        operator,
		Operation:       &operation,
		Apps:            clusterEnv.Apps,
		Backend:         clusterEnv.Backend,
		Client:          clusterEnv.Client,
		ClusterPackages: clusterEnv.ClusterPackages,
		LocalBackend:    env.Backend,
		Servers:         cluster.ClusterState.Servers,
		ClusterKey:      cluster.Key(),
		Silent:          env.Silent,
		Runner:          runner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
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

See https://gravitational.com/gravity/docs/cluster/#managing-an-ongoing-operation for details.`
)
