package localenv

import (
	"github.com/gravitational/gravity/lib/state"

	"github.com/gravitational/trace"
)

// NewLocalWizardEnvironment creates a new local environment to access
// wizard-specific state
func NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	stateDir, err := state.GravityInstallDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	args := LocalEnvironmentArgs{
		StateDir:         stateDir,
		LocalKeyStoreDir: stateDir,
	}
	return NewLocalEnvironment(args)
}
