// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
