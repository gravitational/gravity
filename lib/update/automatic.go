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
func AutomaticUpgrade(ctx context.Context, updateEnv *localenv.LocalEnvironment) (err error) {
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
		Backend:         clusterEnv.Backend,
		Packages:        clusterEnv.Packages,
		ClusterPackages: clusterEnv.ClusterPackages,
		Apps:            clusterEnv.Apps,
		Client:          clusterEnv.Client,
		Operator:        clusterEnv.Operator,
		Users:           clusterEnv.Users,
		LocalBackend:    updateEnv.Backend,
		Remote:          runner,
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
