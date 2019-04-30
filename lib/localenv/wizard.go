package localenv

import (
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/pack"
)

// NewLocalWizardEnvironment creates a new local environment to access
// wizard-specific state
func (r *LocalEnvironment) NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	return NewLocalWizardEnvironment(r.Reporter)
}

// NewLocalWizardEnvironment creates a new local environment to access
// wizard-specific state
func NewLocalWizardEnvironment(reporter pack.ProgressReporter) (*LocalEnvironment, error) {
	args := LocalEnvironmentArgs{
		StateDir:         defaults.GravityInstallDir(),
		LocalKeyStoreDir: defaults.GravityInstallDir(),
		Reporter:         reporter,
	}
	return NewLocalEnvironment(args)
}
