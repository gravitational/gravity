/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package transfer

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	storagesuite "github.com/gravitational/gravity/lib/storage/suite"
	"github.com/gravitational/gravity/lib/users"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	. "gopkg.in/check.v1"
)

func TestTransfer(t *testing.T) { TestingT(t) }

type TransferSuite struct {
	backend1 storage.Backend
	backend2 storage.Backend
}

var _ = Suite(&TransferSuite{})

func (s *TransferSuite) SetUpTest(c *C) {
	testETCD := os.Getenv(defaults.TestETCD)
	if ok, _ := strconv.ParseBool(testETCD); !ok {
		c.Skip("Skipping test suite for ETCD")
		return
	}

	s.backend1 = s.makeBackend(c)
	s.backend2 = s.makeBackend(c)
}

func (s *TransferSuite) TestExportImport(c *C) {
	// Setup
	account, site := s.createSite(c)
	user, keys, roles := s.createUser(*site, c)
	token := s.createToken(*site, user.GetName(), c)
	cluster := storage.NewTrustedCluster("ops.example.com",
		storage.TrustedClusterSpecV2{
			Enabled:              true,
			ProxyAddress:         "ops.example.com:32009",
			ReverseTunnelAddress: "ops.example.com:32024",
			Roles:                []string{constants.RoleAdmin},
			Token:                "secret",
		})

	// Exercise
	exportedFilename := exportSite(site, s.backend1, []storage.TrustedCluster{cluster}, c)
	defer os.Remove(exportedFilename)

	// Verify
	err := ImportSite(exportedFilename, s.backend2)
	c.Assert(err, IsNil)

	accounts, err := s.backend2.GetAccounts()
	c.Assert(err, IsNil)
	c.Assert(accounts, DeepEquals, []storage.Account{*account})

	site.Local = true
	sites, err := s.backend2.GetSites(account.ID)
	c.Assert(err, IsNil)
	c.Assert(sites, DeepEquals, []storage.Site{*site})

	obtainedUser, err := s.backend2.GetUser(user.GetName())
	c.Assert(err, IsNil)
	storagesuite.UsersEquals(c, obtainedUser, user)

	obtainedAPIKeys, err := s.backend2.GetAPIKeys(user.GetName())
	c.Assert(err, IsNil)
	c.Assert(obtainedAPIKeys, DeepEquals, keys)

	obtainedRoles, err := s.backend2.GetUserRoles(user.GetName())
	c.Assert(err, IsNil)
	c.Assert(obtainedRoles, DeepEquals, roles)

	clusters, err := s.backend2.GetTrustedClusters()
	c.Assert(err, IsNil)
	c.Assert(clusters, DeepEquals, []teleservices.TrustedCluster{cluster})

	tokens, err := s.backend2.GetSiteProvisioningTokens(site.Domain)
	c.Assert(err, IsNil)
	c.Assert(tokens, DeepEquals, []storage.ProvisioningToken{token})

	// Import site again: make sure the import is idempotent
	err = ImportSite(exportedFilename, s.backend2)
	c.Assert(err, IsNil)
}

func (s *TransferSuite) createSite(c *C) (*storage.Account, *storage.Site) {
	// Create account
	account, err := s.backend1.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)

	// Create application package
	repo, err := s.backend1.CreateRepository(storage.NewRepository("example.com"))
	c.Assert(err, IsNil)

	app, err := s.backend1.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "app",
		Version:    "0.0.1",
		Type:       string(storage.AppUser),
		Manifest:   []byte("a"),
		Created:    now,
	})
	c.Assert(err, IsNil)

	// Create a site
	site, err := s.backend1.CreateSite(storage.Site{
		AccountID:       account.ID,
		Created:         now,
		Provider:        "virsh",
		State:           "active",
		Domain:          "a.example.com",
		App:             *app,
		NextUpdateCheck: now,
	})
	c.Assert(err, IsNil)

	return account, site
}

func (s *TransferSuite) createToken(site storage.Site, userEmail string, c *C) storage.ProvisioningToken {
	// Create site install operation
	op := storage.SiteOperation{
		AccountID:  site.AccountID,
		SiteDomain: site.Domain,
		Type:       "test",
		Created:    now,
		Updated:    now,
		State:      "new",
	}

	_, err := s.backend1.CreateSiteOperation(op)
	c.Assert(err, IsNil)

	// Create install token for site
	token, err := s.backend1.CreateProvisioningToken(storage.ProvisioningToken{
		Token:       "token",
		Expires:     now.Add(time.Hour),
		Type:        storage.ProvisioningTokenTypeInstall,
		AccountID:   site.AccountID,
		SiteDomain:  site.Domain,
		OperationID: op.ID,
		UserEmail:   userEmail,
	})
	c.Assert(err, IsNil)

	return *token
}

func (s *TransferSuite) createUser(site storage.Site, c *C) (storage.User, []storage.APIKey, []teleservices.Role) {
	role, err := users.NewAdminRole()
	c.Assert(err, IsNil)
	err = s.backend1.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)
	user := storage.NewUser("bob@example.com", storage.UserSpecV2{
		Type:        storage.AgentUser,
		AccountID:   site.AccountID,
		ClusterName: site.Domain,
		Password:    "token 1",
		Roles:       []string{role.GetName()},
	})
	_, err = s.backend1.CreateUser(user)
	c.Assert(err, IsNil)

	key, err := s.backend1.CreateAPIKey(storage.APIKey{
		Token:     "key",
		UserEmail: user.GetName(),
	})
	c.Assert(err, IsNil)

	return user, []storage.APIKey{*key}, []teleservices.Role{role}
}

func (s *TransferSuite) makeBackend(c *C) storage.Backend {
	config := keyval.ETCDConfig{}

	err := json.Unmarshal([]byte(os.Getenv(defaults.TestETCDConfig)), &config)
	c.Assert(err, IsNil)

	token, err := teleutils.CryptoRandomHex(6)
	c.Assert(err, IsNil)

	config.Key = fmt.Sprintf("%v/%v", config.Key, token)
	backend, err := keyval.NewETCD(config)
	c.Assert(err, IsNil)

	return backend
}

func exportSite(site *storage.Site, backend storage.Backend, clusters []storage.TrustedCluster, c *C) string {
	exported, err := ExportSite(site, backend, c.MkDir(), clusters)
	c.Assert(err, IsNil)
	defer exported.Close()
	file, err := ioutil.TempFile("", "test")
	c.Assert(err, IsNil)
	_, err = io.Copy(file, exported)
	c.Assert(err, IsNil)
	c.Assert(file.Close(), IsNil)
	return file.Name()
}

var now = time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC)
