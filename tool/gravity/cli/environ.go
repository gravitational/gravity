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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/environ"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// UpdateEnvars executes the loop to update cluster environment variables.
// resource specifies the new environment variables to apply.
func UpdateEnvars(localEnv, updateEnv *localenv.LocalEnvironment, resource teleservices.UnknownResource) error {
	env, err := storage.UnmarshalEnvironmentVariables(resource.Raw)
	if err != nil {
		return trace.Wrap(err)
	}
	teleportClient, err := localEnv.TeleportClient(constants.Localhost)
	if err != nil {
		return trace.Wrap(err, "failed to create a teleport client")
	}
	proxy, err := teleportClient.ConnectToProxy()
	if err != nil {
		return trace.Wrap(err, "failed to connect to teleport proxy")
	}
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	operator := clusterEnv.Operator
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	key, err := operator.CreateUpdateEnvarsOperation(
		ops.CreateUpdateEnvarsOperationRequest{
			SiteKey: cluster.Key(),
			Env:     env.GetKeyValues(),
		},
	)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotImplemented(
				"cluster operator does not implement the API required for updating cluster environment variables. " +
					"Please make sure you're running the command on a compatible cluster.")
		}
		return trace.Wrap(err)
	}
	defer func() {
		r := recover()
		triggered := r == nil
		if !triggered {
			logrus.WithError(err).Warn("Operation failed.")
			var msg string
			if err != nil {
				msg = err.Error()
			}
			if errMark := ops.FailOperation(*key, operator, msg); errMark != nil {
				logrus.WithFields(logrus.Fields{
					logrus.ErrorKey: errMark,
					"operation":     key,
				}).Warn("Failed to mark operation as failed.")
			}
		}
		if r != nil {
			panic(r)
		}
	}()

	operation, err := operator.GetSiteOperation(*key)
	if err != nil {
		return trace.Wrap(err)
	}
	// Create the operation plan so it can be replicated on remote nodes
	_, err = environ.NewOperationPlan(operator, *operation, cluster.ClusterState.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	req := deployAgentsRequest{
		clusterState: cluster.ClusterState,
		clusterName:  cluster.Domain,
		clusterEnv:   clusterEnv,
		proxy:        proxy,
		nodeParams:   []string{constants.RPCAgentSyncPlanFunction},
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaults.AgentDeployTimeout)
	defer cancel()
	creds, err := deployAgents(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)
	config := environ.Config{
		Operator:     operator,
		Operation:    operation,
		Backend:      clusterEnv.Backend,
		LocalBackend: updateEnv.Backend,
		Servers:      cluster.ClusterState.Servers,
		ClusterKey:   cluster.Key(),
		Silent:       localEnv.Silent,
		Runner:       runner,
		Emitter:      localEnv,
	}
	updater, err := environ.New(config)
	if err != nil {
		return trace.Wrap(err)
	}

	err = updater.Run(context.Background(), false)
	return trace.Wrap(err)
}

func updateEnvarsPhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams) error {
	updater, err := getUpdater(env, updateEnv)
	if err != nil {
		return trace.Wrap(err)
	}

	err = updater.RunPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func rollbackUpdateEnvarsPhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams) error {
	updater, err := getUpdater(env, updateEnv)
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.RollbackPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func getUpdater(env, updateEnv *localenv.LocalEnvironment) (*environ.Updater, error) {
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operator := clusterEnv.Operator

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operation, _, err := ops.GetLastOperation(cluster.Key(), operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := libfsm.GetClientCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)

	updater, err := environ.New(environ.Config{
		Operator:     operator,
		Operation:    operation,
		Backend:      clusterEnv.Backend,
		LocalBackend: env.Backend,
		Servers:      cluster.ClusterState.Servers,
		ClusterKey:   cluster.Key(),
		Silent:       env.Silent,
		Runner:       runner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}
