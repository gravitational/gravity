package localenv

import (
	"github.com/gravitational/gravity/lib/defaults"
)

// NewLocalWizardEnvironment creates a new local environment to access
// wizard-specific state
func (r *LocalEnvironment) NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	args := LocalEnvironmentArgs{
		StateDir:         defaults.GravityInstallDir(),
		LocalKeyStoreDir: defaults.GravityInstallDir(),
		Reporter:         r.Reporter,
	}
	return NewLocalEnvironment(args)
}

// NewLocalWizardEnvironment creates a new local environment to access
// wizard-specific state
func NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	args := LocalEnvironmentArgs{
		StateDir:         defaults.GravityInstallDir(),
		LocalKeyStoreDir: defaults.GravityInstallDir(),
	}
	return NewLocalEnvironment(args)
}
