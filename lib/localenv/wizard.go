package localenv

import (
	"github.com/gravitational/gravity/lib/state"
)

// NewLocalWizardEnvironment creates a new local environment to access
// wizard-specific state
func NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	args := LocalEnvironmentArgs{
		StateDir:         state.GravityInstallDir(),
		LocalKeyStoreDir: state.GravityInstallDir(),
	}
	return NewLocalEnvironment(args)
}
