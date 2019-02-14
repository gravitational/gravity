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

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// RemoveEnvars executes the loop to clear cluster environment variables.
func RemoveEnvars(localEnv, updateEnv *localenv.LocalEnvironment, manual, confirmed bool) error {
	env := storage.NewEnvironment(nil)
	ctx := context.TODO()
	return trace.Wrap(updateEnvars(ctx, localEnv, updateEnv, env, manual, confirmed))
}

// UpdateEnvars executes the loop to update cluster environment variables.
// resource specifies the new environment variables to apply.
func UpdateEnvars(localEnv, updateEnv *localenv.LocalEnvironment, resource []byte, manual, confirmed bool) error {
	env, err := storage.UnmarshalEnvironmentVariables(resource)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx := context.TODO()
	return trace.Wrap(updateEnvars(ctx, localEnv, updateEnv, env, manual, confirmed))
}

func updateEnvars(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, env storage.EnvironmentVariables, manual, confirmed bool) error {
	if !confirmed {
		if manual {
			localEnv.Println(updateEnvarsBannerManual)
		} else {
			localEnv.Println(updateEnvarsBanner)
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
	updater, err := newUpdater(ctx, localEnv, updateEnv, env)
	if err != nil {
		return trace.Wrap(err)
	}
	if !manual {
		err = updater.Run(ctx, false)
		return trace.Wrap(err)
	}
	localEnv.Println(updateEnvarsManualOperationBanner)
	return nil
}

func newUpdater(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, env storage.EnvironmentVariables) (*environ.Updater, error) {
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
	key, err := operator.CreateUpdateEnvarsOperation(
		ops.CreateUpdateEnvarsOperationRequest{
			ClusterKey: cluster.Key(),
			Env:        env.GetKeyValues(),
		},
	)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotImplemented(
				"cluster operator does not implement the API required for updating cluster runtime environment variables. " +
					"Please make sure you're running the command on a compatible cluster.")
		}
		return nil, trace.Wrap(err)
	}
	defer func() {
		r := recover()
		panicked := r != nil
		if err != nil || panicked {
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
		return nil, trace.Wrap(err)
	}
	// Create the operation plan so it can be replicated on remote nodes
	_, err = environ.NewOperationPlan(operator, *operation, cluster.ClusterState.Servers)
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
	config := environ.Config{
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
	updater, err := environ.New(ctx, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

func executeEnvarsPhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	updater, err := getUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}

	err = updater.RunPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func rollbackEnvarsPhase(env, updateEnv *localenv.LocalEnvironment, params PhaseParams, operation ops.SiteOperation) error {
	updater, err := getUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.RollbackPhase(context.TODO(), params.PhaseID, params.Timeout, params.Force)
	return trace.Wrap(err)
}

func completeEnvarsPlan(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	updater, err := getUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(updater.Complete())
}

func getUpdateEnvarsOperationPlan(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) (*storage.OperationPlan, error) {
	updater, err := getUpdater(env, updateEnv, operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plan, err := updater.GetPlan()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

func getUpdater(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) (*environ.Updater, error) {
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

	updater, err := environ.New(context.TODO(), environ.Config{
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
	updateEnvarsBanner = `Updating cluster runtime environment requires restart of runtime containers on all nodes.
The operation might take several minutes to complete depending on the cluster size.

The operation will start automatically once you approve it.
If you want to review the operation plan first or execute it manually step by step,
run the operation in manual mode by specifying '--manual' flag.

Are you sure?`
	updateEnvarsBannerManual = `Updating cluster runtime environment requires restart of runtime containers on all nodes.
The operation might take several minutes to complete depending on the cluster size.

"Are you sure?`
	// TODO(dmitri): provide a link to the documentation that describes common CLI workflow
	// for doing operations manually one it has been added
	updateEnvarsManualOperationBanner = `The operation has been created in manual mode.

To view the operation plan, run:

$ sudo gravity plan

The plan is a tree of operational steps (phases).
To execute a specific phase, use the full path as shown in the plan

$ sudo gravity plan execute --phase=/<root-phase>/<sub-phase>/...

To rollback a phase, execute:

$ sudo gravity plan rollback --phase=/<root-phase>/<sub-phase>/...

To resume operation from any point, run:

$ sudo gravity plan resume

Resume will automatically complete the operation.
To complete the operation manually (i.e. after rolling back), run:

$ sudo gravity plan complete`
)
