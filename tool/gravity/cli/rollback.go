package cli

import (
	"time"

	"github.com/gravitational/gravity/lib/localenv"
)

type rollbackParams struct {
	// phaseID is the ID of the phase to rollback
	phaseID string
	// force allows to force phase rollback
	force bool
	// skipVersionCheck allows to override gravity version compatibility check
	skipVersionCheck bool
	// timeout is phase rollback timeout
	timeout time.Duration
}

func rollbackOperationPhase(env *localenv.LocalEnvironment, updateEnv *localenv.LocalEnvironment, p rollbackParams) error {
	if hasUpdateOperation(updateEnv) {
		return rollbackUpgradePhase(updateEnv, p)
	}
	return rollbackInstallPhase(env, p)
}
