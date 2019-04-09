package localenv

import (
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/tool/common"
)

// NewLocalWizardEnvironment creates a new local environment to acces
// wizard-specific state
func NewLocalWizardEnvironment() (*LocalEnvironment, error) {
	args := LocalEnvironmentArgs{
		StateDir:         defaults.GravityInstallDir,
		Reporter:         common.ProgressReporter(false),
		LocalKeyStoreDir: defaults.GravityInstallDir,
	}
	return NewLocalEnvironment(args)
}
