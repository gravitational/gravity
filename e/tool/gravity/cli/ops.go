package cli

import (
	"io/ioutil"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
)

// generateInstaller generates a standalone installer in the specified
// directory dir for the application given with appPackage
func generateInstaller(env *environment.Local, appPackage loc.Locator, dir, caCertPath, encryptionKey, opsCenterURL string) error {
	operator, err := env.OperatorService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	accounts, err := operator.GetAccounts()
	if err != nil {
		return trace.Wrap(err)
	}

	var accountID string
	for _, account := range accounts {
		if account.Org == defaults.SystemAccountOrg {
			accountID = account.ID
			break
		}
	}

	if accountID == "" {
		return trace.NotFound("no system account found")
	}

	var caCert []byte
	if caCertPath != "" {
		caCert, err = ioutil.ReadFile(caCertPath)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	tarball, err := operator.GetAppInstaller(ops.AppInstallerRequest{
		AccountID:     accountID,
		Application:   appPackage,
		CACert:        string(caCert),
		EncryptionKey: encryptionKey,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer tarball.Close()

	err = archive.Extract(tarball, dir)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
