package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

func executeAutomaticUpgrade(ctx context.Context, upgradeEnv *localenv.LocalEnvironment, args []string) error {
	return trace.Wrap(update.AutomaticUpgrade(ctx, upgradeEnv))
}

// upgradePhaseParams combines parameters for an upgrade phase execution/rollback
type upgradePhaseParams struct {
	// phaseID is the ID of the phase to execute
	phaseID string
	// force allows to force phase execution
	force bool
	// skipVersionCheck allows to override gravity version compatibility check
	skipVersionCheck bool
	// timeout is phase execution timeout
	timeout time.Duration
}

func executeUpgradePhase(upgradeEnv *localenv.LocalEnvironment, p upgradePhaseParams) error {
	clusterEnv, err := localenv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	progress := utils.NewProgress(ctx, fmt.Sprintf("phase %q execution", p.phaseID), -1, false)
	defer progress.Stop()

	creds, err := fsm.GetClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	runner := fsm.NewAgentRunner(creds)

	err = update.ExecutePhase(ctx, update.FSMConfig{
		Backend:         clusterEnv.Backend,
		Packages:        clusterEnv.Packages,
		ClusterPackages: clusterEnv.ClusterPackages,
		Apps:            clusterEnv.Apps,
		Client:          clusterEnv.Client,
		Operator:        clusterEnv.Operator,
		Users:           clusterEnv.Users,
		LocalBackend:    upgradeEnv.Backend,
		Remote:          runner,
	}, fsm.Params{
		PhaseID:  p.phaseID,
		Force:    p.force,
		Progress: progress,
	}, p.skipVersionCheck)

	return trace.Wrap(err)
}

func rollbackUpgradePhase(updateEnv *localenv.LocalEnvironment, p rollbackParams) error {
	clusterEnv, err := localenv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()

	progress := utils.NewProgress(ctx, fmt.Sprintf("phase %q rollback", p.phaseID), -1, false)
	defer progress.Stop()

	creds, err := fsm.GetClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	runner := fsm.NewAgentRunner(creds)

	err = update.RollbackPhase(ctx, update.FSMConfig{
		Backend:         clusterEnv.Backend,
		Packages:        clusterEnv.Packages,
		ClusterPackages: clusterEnv.ClusterPackages,
		Apps:            clusterEnv.Apps,
		Client:          clusterEnv.Client,
		Operator:        clusterEnv.Operator,
		Users:           clusterEnv.Users,
		LocalBackend:    updateEnv.Backend,
		Remote:          runner,
	}, fsm.Params{
		PhaseID:  p.phaseID,
		Force:    p.force,
		Progress: progress,
	}, p.skipVersionCheck)

	return trace.Wrap(err)
}

func completeUpgrade(updateEnv *localenv.LocalEnvironment) error {
	clusterEnv, err := localenv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	creds, err := fsm.GetClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	runner := fsm.NewAgentRunner(creds)

	fsm, err := update.NewFSM(context.TODO(),
		update.FSMConfig{
			Backend:         clusterEnv.Backend,
			Packages:        clusterEnv.Packages,
			ClusterPackages: clusterEnv.ClusterPackages,
			Apps:            clusterEnv.Apps,
			LocalBackend:    updateEnv.Backend,
			Remote:          runner,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	err = fsm.Complete(nil)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaults.RPCAgentShutdownTimeout)
	defer cancel()
	if err = update.ShutdownClusterAgents(ctx, runner); err != nil {
		log.Warnf("Failed to shutdown cluster agents: %v.", trace.DebugReport(err))
	}

	updateEnv.Printf("cluster has been activated\n")
	return nil
}
