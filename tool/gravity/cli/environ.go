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
// "context"
// "time"

// "github.com/gravitational/gravity/lib/environ"
// libfsm "github.com/gravitational/gravity/lib/fsm"
// "github.com/gravitational/gravity/lib/localenv"
// "github.com/gravitational/gravity/lib/ops"
// "github.com/gravitational/gravity/lib/storage"

// teleclient "github.com/gravitational/teleport/lib/client"
// "github.com/gravitational/trace"
)

/*
func updateEnvars(config environ.Config) error {
	ctx := context.TODO()
	updater, err := newUpdater(ctx, config, env, teleProxy)
	if err != nil {
		return trace.Wrap(err)
	}

	err = updater.Run(ctx, false)
	return trace.Wrap(err)
}

func newUpdater(ctx context.Context, config environ.Config, env storage.EnvironmentVariables, proxy teleclient.ProxyClient) (*environ.Updater, error) {
	key, err := config.Operator.CreateUpdateEnvarsOperation(
		ops.CreateUpdateEnvarsOperationRequest{
			AccountID:   cluster.AccountID,
			ClusterName: cluster.Domain,
			Env:         env,
		},
	)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotImplemented(
				"cluster operator does not implement the API required for updating cluster environment variables. " +
					"Please make sure you're running the command on a compatible cluster.")
		}
		return nil, trace.Wrap(err)
	}

	defer func() {
		r := recover()
		triggered := err == nil && r == nil
		if !triggered {
			if err := ops.FailOperation(operator, key); err != nil {
				log.WithFields(log.Fields{
					log.ErrorKey: err,
					"operation":  key,
				}).Warn("Failed to mark operation as failed.")
			}
		}
		if r != nil {
			panic(r)
		}
	}()

	operation, err := config.Operator.GetSiteOperation(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := deployAgentsRequest{
		clusterState: cluster.ClusterState,
		clusterName:  cluster.Domain,
		clusterEnv:   clusterEnv,
		proxy:        proxy,
	}
	creds, err := deployAgents(ctx, env, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)

	updater, err := environ.New(environ.Config{
		Operator:   operator,
		Operation:  operation,
		Servers:    cluster.ClusterState.Servers,
		ClusterKey: cluster.Key(),
		Silent:     env.Silent,
		Runner:     runner,
		Emitter:    env,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

func updateEnvarsPhase(env *localenv.LocalEnvironment, phase string, phaseTimeout time.Duration, force bool) error {
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	operation, _, err := ops.GetLastOperation(cluster.Key(), operator)
	if err != nil {
		return trace.Wrap(err)
	}

	creds, err := libfsm.GetClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	runner := libfsm.NewAgentRunner(creds)

	updater, err := environ.New(environ.Config{
		Operator:   operator,
		Operation:  operation,
		Servers:    cluster.ClusterState.Servers,
		ClusterKey: cluster.Key(),
		Silent:     env.Silent,
		Runner:     runner,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = updater.RunPhase(context.TODO(), phase, phaseTimeout, force)
	return trace.Wrap(err)
}
*/
