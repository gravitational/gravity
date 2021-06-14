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

	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/environ"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func updateEnviron(
	ctx context.Context,
	localEnv, updateEnv *localenv.LocalEnvironment,
	env storage.EnvironmentVariables,
	manual, confirmed bool,
) error {
	if !confirmed {
		if manual {
			localEnv.Println(updateEnvironBannerManual)
		} else {
			localEnv.Println(updateEnvironBanner)
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
	updater, err := newEnvironUpdater(ctx, localEnv, updateEnv, env)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if !manual {
		err = updater.Run(ctx)
		return trace.Wrap(err)
	}
	localEnv.Println(updateEnvironManualOperationBanner)
	return nil
}

func newEnvironUpdater(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, environ storage.EnvironmentVariables) (*update.Updater, error) {
	init := environInitializer{
		environ: environ,
	}
	return newUpdater(ctx, localEnv, updateEnv, init, nil)
}

func executeEnvironPhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getEnvironUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	return executeOrForkPhase(env, updater, params, operation)
}

func setEnvironPhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params SetPhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getEnvironUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	return updater.SetPhase(context.TODO(), params.PhaseID, params.State)
}

func rollbackEnvironPhaseForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getEnvironUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	err = updater.RollbackPhase(context.TODO(), libfsm.Params{
		PhaseID: params.PhaseID,
		Force:   params.Force,
		DryRun:  params.DryRun,
	}, params.Timeout)
	return trace.Wrap(err)
}

func completeEnvironPlanForOperation(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operation ops.SiteOperation) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	updater, err := getEnvironUpdater(env, updateEnv, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	defer updater.Close()
	if err := updater.Complete(context.TODO(), nil); err != nil {
		return trace.Wrap(err)
	}
	if err := updater.Activate(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getEnvironUpdater(env, updateEnv *localenv.LocalEnvironment, operation ops.SiteOperation) (*update.Updater, error) {
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

	updater, err := environ.New(context.TODO(), environ.Config{
		Config: update.Config{
			Operation:    &operation,
			Operator:     operator,
			Backend:      clusterEnv.Backend,
			LocalBackend: updateEnv.Backend,
			Silent:       env.Silent,
			Runner:       runner,
			FieldLogger: logrus.WithFields(logrus.Fields{
				trace.Component: "update:environ",
				"operation":     operation,
			}),
		},
		Apps:              clusterEnv.Apps,
		Client:            clusterEnv.Client,
		ClusterPackages:   clusterEnv.ClusterPackages,
		HostLocalPackages: env.Packages,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

func (r environInitializer) validatePreconditions(*localenv.LocalEnvironment, ops.Operator, ops.Site) error {
	return nil
}

func (r environInitializer) newOperation(operator ops.Operator, cluster ops.Site) (*ops.SiteOperationKey, error) {
	key, err := operator.CreateUpdateEnvarsOperation(context.TODO(),
		ops.CreateUpdateEnvarsOperationRequest{
			ClusterKey: cluster.Key(),
			Env:        r.environ.GetKeyValues(),
		},
	)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotImplemented(
				"cluster operator does not implement the API required for updating runtime environment. " +
					"Please make sure you're running the command on a compatible cluster.")
		}
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (environInitializer) newOperationPlan(
	ctx context.Context,
	operator ops.Operator,
	cluster ops.Site,
	operation ops.SiteOperation,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	leader *storage.Server,
	userConfig interface{},
) (*storage.OperationPlan, error) {
	plan, err := environ.NewOperationPlan(ctx, operator, clusterEnv.Apps, operation, cluster.ClusterState.Servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

func (environInitializer) newUpdater(
	ctx context.Context,
	operator ops.Operator,
	operation ops.SiteOperation,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	runner rpc.AgentRepository,
) (*update.Updater, error) {
	config := environ.Config{
		Config: update.Config{
			Operation:    &operation,
			Operator:     operator,
			Backend:      clusterEnv.Backend,
			LocalBackend: updateEnv.Backend,
			Silent:       localEnv.Silent,
			Runner:       runner,
			FieldLogger: logrus.WithFields(logrus.Fields{
				trace.Component: "update:environ",
				"operation":     operation,
			}),
		},
		Apps:              clusterEnv.Apps,
		Client:            clusterEnv.Client,
		ClusterPackages:   clusterEnv.ClusterPackages,
		HostLocalPackages: localEnv.Packages,
	}
	return environ.New(ctx, config)
}

func (environInitializer) updateDeployRequest(req deployAgentsRequest) deployAgentsRequest {
	return req
}

type environInitializer struct {
	environ storage.EnvironmentVariables
}

const (
	updateEnvironBanner = `Updating cluster runtime environment requires restart of runtime containers on all nodes.
The operation might take several minutes to complete depending on the cluster size.

The operation will start automatically once you approve it.
If you want to review the operation plan first or execute it manually step by step,
run the operation in manual mode by specifying '--manual' flag.

Are you sure?`
	updateEnvironBannerManual = `Updating cluster runtime environment requires restart of runtime containers on all nodes.
The operation might take several minutes to complete depending on the cluster size.

"Are you sure?`
	updateEnvironManualOperationBanner = `The operation has been created in manual mode.

See https://gravitational.com/gravity/docs/cluster/#managing-operations for details on working with operation plan.`
)
