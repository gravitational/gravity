package localenv

import (
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/pack"
)

// NewLocalWizardEnvironment creates a new local environment to acces
// wizard-specific state
func (r *LocalEnvironment) NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	return NewLocalWizardEnvironment(r.Reporter)
}

// NewLocalWizardEnvironment creates a new local environment to acces
// wizard-specific state
func NewLocalWizardEnvironment(reporter pack.ProgressReporter) (*LocalEnvironment, error) {
	args := LocalEnvironmentArgs{
		StateDir:         defaults.GravityInstallDir(),
		Reporter:         reporter,
		LocalKeyStoreDir: defaults.GravityInstallDir(),
	}
	return NewLocalEnvironment(args)
}
