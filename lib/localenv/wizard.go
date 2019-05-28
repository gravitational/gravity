package localenv

import (
	"github.com/gravitational/gravity/lib/state"

	"github.com/gravitational/trace"
)

// NewLocalWizardEnvironment creates a new local environment to access
// wizard-specific state
func NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	const failImmediatelyIfLocked = -1
	stateDir, err := state.GravityInstallDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	args := LocalEnvironmentArgs{
		StateDir:         stateDir,
		LocalKeyStoreDir: stateDir,
		BoltOpenTimeout:  failImmediatelyIfLocked,
	}
	return NewLocalEnvironment(args)
}
