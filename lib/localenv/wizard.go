package localenv

import (
	"github.com/gravitational/gravity/lib/state"
)

// NewLocalWizardEnvironment creates a new local environment to access
// wizard-specific state
func NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	const failImmediatelyIfLocked = -1
	stateDir := state.GravityInstallDir()
	args := LocalEnvironmentArgs{
		StateDir:         stateDir,
		LocalKeyStoreDir: stateDir,
		BoltOpenTimeout:  failImmediatelyIfLocked,
	}
	return NewLocalEnvironment(args)
}
