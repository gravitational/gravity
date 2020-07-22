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
	"io"
	"time"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func newUpdater(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, init updateInitializer) (*update.Updater, error) {
	teleportClient, err := localEnv.TeleportClient(constants.Localhost)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a teleport client")
	}
	proxy, err := teleportClient.ConnectToProxy(ctx)
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
	err = init.validatePreconditions(localEnv, operator, *cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := init.newOperation(operator, *cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := logrus.WithField("operation", key)
	defer func() {
		r := recover()
		panicked := r != nil
		if err != nil || panicked {
			logger.WithError(err).Warn("Operation failed.")
			var msg string
			if err != nil {
				msg = err.Error()
			}
			if errReset := ops.FailOperationAndResetCluster(*key, operator, msg); errReset != nil {
				logger.WithError(errReset).Warn("Failed to mark operation as failed.")
			}
			// Depending on where the operation initialization failed, some upgrade
			// agents may have started so we need to shut them down. If they're
			// not running, it will be no-op so there's no harm in running this
			// even if we failed before deploying the agents.
			localEnv.PrintStep(color.YellowString("Encountered error, will shutdown agents"))
			if errShutdown := rpcAgentShutdown(localEnv); errShutdown != nil {
				logger.WithError(errShutdown).Warn("Failed to shutdown upgrade agents.")
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
	leader, err := findLocalServer(cluster.ClusterState.Servers)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find local node in cluster state.\n"+
			"Make sure you start the operation from one of the cluster master nodes.")
	}
	// Create the operation plan so it can be replicated on remote nodes
	plan, err := init.newOperationPlan(ctx, operator, *cluster, *operation,
		localEnv, updateEnv, clusterEnv, leader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = update.SyncOperationPlan(clusterEnv.Backend, updateEnv.Backend, *plan,
		(storage.SiteOperation)(*operation))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := init.updateDeployRequest(deployAgentsRequest{
		// Use server list from the operation plan to always have a consistent
		// view of the cluster (i.e. with servers correctly reflecting cluster roles)
		clusterState: clusterStateFromPlan(*plan),
		cluster:      *cluster,
		clusterEnv:   clusterEnv,
		proxy:        proxy,
		leader:       leader,
		nodeParams:   constants.RPCAgentSyncPlanFunction,
	})
	deployCtx, cancel := context.WithTimeout(ctx, defaults.AgentDeployTimeout)
	defer cancel()
	logger.WithField("request", req).Debug("Deploying agents on cluster nodes.")
	localEnv.PrintStep("Deploying agents on cluster nodes")
	creds, err := deployAgents(deployCtx, localEnv, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if bool(localEnv.Silent) {
		// FIXME: keep the legacy behavior of reporting the operation ID in quiet mode.
		// This is still used by robotest to fetch the operation ID
		fmt.Println(key.OperationID)
	}
	runner := libfsm.NewAgentRunner(creds)
	updater, err := init.newUpdater(ctx, operator, *operation, localEnv, updateEnv, clusterEnv, runner)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	//nolint:errcheck
	localEnv.EmitOperationEvent(ctx, *operation)
	return updater, nil
}

type updateInitializer interface {
	validatePreconditions(localEnv *localenv.LocalEnvironment, operator ops.Operator, cluster ops.Site) error
	newOperation(ops.Operator, ops.Site) (*ops.SiteOperationKey, error)
	newOperationPlan(ctx context.Context,
		operator ops.Operator,
		cluster ops.Site,
		operation ops.SiteOperation,
		localEnv, updateEnv *localenv.LocalEnvironment,
		clusterEnv *localenv.ClusterEnvironment,
		leader *storage.Server,
	) (*storage.OperationPlan, error)
	newUpdater(ctx context.Context,
		operator ops.Operator,
		operation ops.SiteOperation,
		localEnv, updateEnv *localenv.LocalEnvironment,
		clusterEnv *localenv.ClusterEnvironment,
		runner rpc.AgentRepository,
	) (*update.Updater, error)
	updateDeployRequest(deployAgentsRequest) deployAgentsRequest
}

type updater interface {
	io.Closer
	Run(ctx context.Context) error
	RunPhase(ctx context.Context, phase string, phaseTimeout time.Duration, force bool) error
	RollbackPhase(ctx context.Context, params fsm.Params, phaseTimeout time.Duration) error
	Complete(error) error
}

func clusterStateFromPlan(plan storage.OperationPlan) (result storage.ClusterState) {
	result.Servers = make([]storage.Server, 0, len(plan.Servers))
	for _, s := range plan.Servers {
		result.Servers = append(result.Servers, s)
	}
	return result
}
