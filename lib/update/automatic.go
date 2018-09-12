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

package update

import (
	"context"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/kubectl"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// AutomaticUpgrade starts automatic upgrade process
func AutomaticUpgrade(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment) (err error) {
	clusterEnv, err := localenv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	creds, err := fsm.GetClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	runner := fsm.NewAgentRunner(creds)

	config := FSMConfig{
		Backend:          clusterEnv.Backend,
		LocalBackend:     updateEnv.Backend,
		HostLocalBackend: localEnv.Backend,
		Packages:         clusterEnv.Packages,
		ClusterPackages:  clusterEnv.ClusterPackages,
		Apps:             clusterEnv.Apps,
		Client:           clusterEnv.Client,
		Operator:         clusterEnv.Operator,
		Users:            clusterEnv.Users,
		Remote:           runner,
	}

	fsm, err := NewFSM(ctx, config)
	if err != nil {
		return trace.Wrap(err, "failed to load or initialize upgrade plan")
	}
	defer fsm.Close()

	progress := utils.NewProgress(ctx, "automatic upgrade", -1, false)
	defer progress.Stop()

	force := false
	fsmErr := fsm.ExecutePlan(ctx, progress, force)
	if fsmErr != nil {
		return trace.Wrap(err)
	}

	err = fsm.Complete(fsmErr)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ShutdownClusterAgents(ctx, runner)
	return trace.Wrap(err)
}

// ShutdownClusterAgents fetches all nodes in a cluster
// and submits a shutdown request
func ShutdownClusterAgents(ctx context.Context, remote rpc.AgentRepository) error {
	nodes, err := kubectl.GetNodesAddr(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = rpc.ShutdownAgents(ctx, nodes, log.StandardLogger(), remote)
	return trace.Wrap(err)
}
