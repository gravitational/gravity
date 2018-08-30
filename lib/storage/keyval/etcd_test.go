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

package keyval

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/suite"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/coreos/etcd/client"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	. "gopkg.in/check.v1"
)

func TestETCD(t *testing.T) { TestingT(t) }

type ESuite struct {
	backend *tempBackend
	suite   suite.StorageSuite
}

var _ = Suite(&ESuite{})

// tempBackend helps to create and destroy ad-hock
// databases in Etcd
type tempBackend struct {
	api     client.KeysAPI
	prefix  string
	clock   clockwork.FakeClock
	backend storage.Backend
}

func (t *tempBackend) Delete() error {
	var err error
	if t.api != nil {
		_, err = t.api.Delete(context.Background(), t.prefix, &client.DeleteOptions{Recursive: true, Dir: true})
		err = convertErr(err)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		return nil
	}
	return nil
}

func newBackend(configJSON string) (*tempBackend, error) {
	if configJSON == "" {
		return nil, trace.BadParameter("missing ETCD configuration")
	}
	fakeClock := clockwork.NewFakeClock()
	cfg := ETCDConfig{Clock: fakeClock}
	err := json.Unmarshal([]byte(configJSON), &cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := teleutils.CryptoRandomHex(6)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cfg.Key = fmt.Sprintf("%v/%v", cfg.Key, token)

	b, err := NewETCD(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &tempBackend{prefix: cfg.Key, api: b.api(), clock: fakeClock, backend: b}, nil
}

func (s *ESuite) SetUpTest(c *C) {
	log.SetOutput(os.Stderr)

	testETCD := os.Getenv(defaults.TestETCD)

	if ok, _ := strconv.ParseBool(testETCD); !ok {
		c.Skip("Skipping test suite for ETCD")
		return
	}

	var err error
	s.backend, err = newBackend(os.Getenv(defaults.TestETCDConfig))
	c.Assert(err, IsNil)

	s.suite.Backend = s.backend.backend
	s.suite.Clock = s.backend.clock
}

func (s *ESuite) TearDownTest(c *C) {
	if s.backend != nil {
		err := s.backend.Delete()
		if err != nil {
			log.Error(trace.DebugReport(err))
		}
		c.Assert(err, IsNil)
	}
}

func (s *ESuite) TestAccountsCRUD(c *C) {
	s.suite.AccountsCRUD(c)
}

func (s *ESuite) TestRepositoriesCRUD(c *C) {
	s.suite.RepositoriesCRUD(c)
}

func (s *ESuite) TestSitesCRUD(c *C) {
	s.suite.SitesCRUD(c)
}

func (s *ESuite) TestProgressEntriesCRUD(c *C) {
	s.suite.ProgressEntriesCRUD(c)
}

func (s *ESuite) TestConnectorsCRUD(c *C) {
	s.suite.ConnectorsCRUD(c)
}

func (s *ESuite) TestUsersCRUD(c *C) {
	s.suite.UsersCRUD(c)
}

func (s *ESuite) TestUserTokensCRUD(c *C) {
	s.suite.UserTokensCRUD(c)
}

func (s *ESuite) TestProvisioningTokensCRUD(c *C) {
	s.suite.ProvisioningTokensCRUD(c)
}

func (s *ESuite) TestAPIKeys(c *C) {
	s.suite.APIKeysCRUD(c)
}

func (s *ESuite) TestUserInvites(c *C) {
	s.suite.UserInvitesCRUD(c)
}

func (s *ESuite) TestLoginEntriesCRUD(c *C) {
	s.suite.LoginEntriesCRUD(c)
}

func (s *ESuite) TestPermissionsCRUD(c *C) {
	s.suite.PermissionsCRUD(c)
}

func (s *ESuite) TestOperationsCRUD(c *C) {
	s.suite.OperationsCRUD(c)
}

func (s *ESuite) TestCreatesApplication(c *C) {
	s.suite.CreatesApplication(c)
}

func (s *ESuite) TestDeletesApplication(c *C) {
	s.suite.DeletesApplication(c)
}

func (s *ESuite) TestRetrievesApplications(c *C) {
	s.suite.RetrievesApplications(c)
}

func (s *ESuite) TestCreatesAppImportOperation(c *C) {
	s.suite.CreatesAppImportOperation(c)
}

func (s *ESuite) TestUpdatesAppImportOperation(c *C) {
	s.suite.UpdatesAppImportOperation(c)
}

func (s *ESuite) TestConnectors(c *C) {
	s.suite.ConnectorsCRUD(c)
}

func (s *ESuite) TestWebSessions(c *C) {
	s.suite.WebSessionsCRUD(c)
}

func (s *ESuite) TestAuthoritiesCRUD(c *C) {
	s.suite.AuthoritiesCRUD(c)
}

func (s *ESuite) TestNodesCRUD(c *C) {
	s.suite.NodesCRUD(c)
}

func (s *ESuite) TestReverseTunnelsCRUD(c *C) {
	s.suite.ReverseTunnelsCRUD(c)
}

func (s *ESuite) TestLocksCRUD(c *C) {
	s.suite.LocksCRUD(c)
}

func (s *ESuite) TestPeersCRUD(c *C) {
	s.suite.PeersCRUD(c)
}

func (s *ESuite) TestObjectsCRUD(c *C) {
	s.suite.ObjectsCRUD(c)
}

func (s *ESuite) TestChangesetsCRUD(c *C) {
	s.suite.ChangesetsCRUD(c)
}

func (s *ESuite) TestOpsCenterLinksCRUD(c *C) {
	s.suite.OpsCenterLinksCRUD(c)
}

func (s *ESuite) TestRolesCRUD(c *C) {
	s.suite.RolesCRUD(c)
}

func (s *ESuite) TestNamespacesCRUD(c *C) {
	s.suite.NamespacesCRUD(c)
}

func (s *ESuite) TestLoginAttempts(c *C) {
	s.suite.LoginAttempts(c)
}

func (s *ESuite) TestLocalCluster(c *C) {
	s.suite.LocalCluster(c)
}

func (s *ESuite) TestSAMLCRUD(c *C) {
	s.suite.SAMLCRUD(c)
}

func (s *ESuite) TestClusterAgentCreds(c *C) {
	s.suite.ClusterAgentCreds(c)
}

func (s *ESuite) TestClusterLogin(c *C) {
	s.suite.ClusterLogin(c)
}
