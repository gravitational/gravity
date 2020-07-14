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

package cluster

import (
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/kubectl"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// AutomaticUpgrade starts automatic upgrade process
func AutomaticUpgrade(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment) (err error) {
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	operation, err := storage.GetLastOperation(updateEnv.Backend)
	if err != nil {
		return trace.Wrap(err)
	}
	creds, err := fsm.GetClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	runner := fsm.NewAgentRunner(creds)
	err = waitForAgents(ctx, clusterEnv, runner)
	if err != nil {
		return trace.Wrap(err)
	}
	config := Config{
		Config: update.Config{
			Operation:    (*ops.SiteOperation)(operation),
			Operator:     clusterEnv.Operator,
			Backend:      clusterEnv.Backend,
			LocalBackend: updateEnv.Backend,
			Runner:       runner,
			Silent:       localEnv.Silent,
		},
		HostLocalBackend:  localEnv.Backend,
		HostLocalPackages: localEnv.Packages,
		Packages:          clusterEnv.Packages,
		ClusterPackages:   clusterEnv.ClusterPackages,
		Apps:              clusterEnv.Apps,
		Client:            clusterEnv.Client,
		Users:             clusterEnv.Users,
	}
	fsm, err := New(ctx, config)
	if err != nil {
		return trace.Wrap(err, "failed to load or initialize upgrade plan")
	}
	defer fsm.Close()

	fsmErr := fsm.Run(ctx)
	if fsmErr != nil {
		log.WithError(fsmErr).Warn("Failed to execute plan.")
		// fallthrough
	}

	err = fsm.Complete(ctx, fsmErr)
	if err != nil {
		return trace.Wrap(err)
	}

	if fsmErr == nil {
		err = ShutdownClusterAgents(ctx, runner)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return trace.Wrap(fsmErr)
}

func waitForAgents(ctx context.Context, clusterEnv *localenv.ClusterEnvironment, runner rpc.RemoteRunner) error {
	cluster, err := clusterEnv.Operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	logger := log.WithField(trace.Component, "automatic")
	// TODO(r0mant): Parallelize this?
	return utils.RetryFor(ctx, defaults.AgentDeployTimeout, func() error {
		var unreachable storage.Servers
		for _, node := range cluster.ClusterState.Servers {
			localCtx, cancel := context.WithTimeout(ctx, defaults.AgentHealthCheckTimeout)
			defer cancel()
			if err := runner.CanExecute(localCtx, node); err != nil {
				logger.WithError(err).WithFields(node.Fields()).Warn("Agent is not running.")
				unreachable = append(unreachable, node)
			} else {
				logger.WithFields(node.Fields()).Info("Agent is running.")
			}
		}
		if len(unreachable) > 0 {
			return utils.Continue("Some agents aren't running yet: %s.", unreachable)
		}
		logger.Info("All agents are running.")
		return nil
	})
}

// ShutdownClusterAgents submits a shutdown request to all agents
func ShutdownClusterAgents(ctx context.Context, remote rpc.AgentRepository) error {
	nodes, err := kubectl.GetNodesAddr(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = rpc.ShutdownAgents(ctx, nodes, log.StandardLogger(), remote)
	return trace.Wrap(err)
}

// AgentStatus requests the statuses of all agents
func AgentStatus(ctx context.Context, remote rpc.AgentRepository) ([]rpc.GravityAgentStatus, error) {
	nodes, err := kubectl.GetNodesAddr(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	statuses, err := rpc.AgentStatus(ctx, nodes, log.StandardLogger(), remote)
	return statuses, trace.Wrap(err)
}
