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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/suite"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

type BSuite struct {
	backend *tempBolt
	suite   suite.StorageSuite
}

var _ = Suite(&BSuite{})

// tempBolt helps to create and destroy ad-hock bolt databases
type tempBolt struct {
	clock   clockwork.FakeClock
	backend storage.Backend
	dir     string
}

func (t *tempBolt) Delete() error {
	var errs []error
	if t.backend != nil {
		errs = append(errs, t.backend.Close())
	}
	if t.dir != "" {
		errs = append(errs, os.RemoveAll(t.dir))
	}
	return trace.NewAggregate(errs...)
}

func newTempBolt() (*tempBolt, error) {
	dir, err := ioutil.TempDir("", "gravity-test")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fakeClock := clockwork.NewFakeClock()
	b, err := NewBolt(BoltConfig{
		Clock: fakeClock,
		Path:  filepath.Join(dir, "bolt.db"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &tempBolt{
		dir:     dir,
		clock:   fakeClock,
		backend: b,
	}, nil
}

func (s *BSuite) SetUpTest(c *C) {
	log.SetOutput(os.Stderr)

	var err error
	s.backend, err = newTempBolt()
	c.Assert(err, IsNil)

	s.suite.Backend = s.backend.backend
	s.suite.Clock = s.backend.clock
}

func (s *BSuite) TearDownTest(c *C) {
	if s.backend != nil {
		err := s.backend.Delete()
		if err != nil {
			log.Error(trace.DebugReport(err))
		}
		c.Assert(err, IsNil)
	}
}

func (s *BSuite) TestAccountsCRUD(c *C) {
	s.suite.AccountsCRUD(c)
}

func (s *BSuite) TestRepositoriesCRUD(c *C) {
	s.suite.RepositoriesCRUD(c)
}

func (s *BSuite) TestSitesCRUD(c *C) {
	s.suite.SitesCRUD(c)
}

func (s *BSuite) TestProgressEntriesCRUD(c *C) {
	s.suite.ProgressEntriesCRUD(c)
}

func (s *BSuite) TestConnectorsCRUD(c *C) {
	s.suite.ConnectorsCRUD(c)
}

func (s *BSuite) TestUsersCRUD(c *C) {
	s.suite.UsersCRUD(c)
}

func (s *BSuite) TestUserTokensCRUD(c *C) {
	s.suite.UserTokensCRUD(c)
}

func (s *BSuite) TestProvisioningTokensCRUD(c *C) {
	s.suite.ProvisioningTokensCRUD(c)
}

func (s *BSuite) TestAPIKeys(c *C) {
	s.suite.APIKeysCRUD(c)
}

func (s *BSuite) TestUserInvites(c *C) {
	s.suite.UserInvitesCRUD(c)
}

func (s *BSuite) TestLoginEntriesCRUD(c *C) {
	s.suite.LoginEntriesCRUD(c)
}

func (s *BSuite) TestPermissionsCRUD(c *C) {
	s.suite.PermissionsCRUD(c)
}

func (s *BSuite) TestOperationsCRUD(c *C) {
	s.suite.OperationsCRUD(c)
}

func (s *BSuite) TestCreatesApplication(c *C) {
	s.suite.CreatesApplication(c)
}

func (s *BSuite) TestDeletesApplication(c *C) {
	s.suite.DeletesApplication(c)
}

func (s *BSuite) TestRetrievesApplications(c *C) {
	s.suite.RetrievesApplications(c)
}

func (s *BSuite) TestCreatesAppImportOperation(c *C) {
	s.suite.CreatesAppImportOperation(c)
}

func (s *BSuite) TestUpdatesAppImportOperation(c *C) {
	s.suite.UpdatesAppImportOperation(c)
}

func (s *BSuite) TestConnectors(c *C) {
	s.suite.ConnectorsCRUD(c)
}

func (s *BSuite) TestWebSessions(c *C) {
	s.suite.WebSessionsCRUD(c)
}

func (s *BSuite) TestAuthoritiesCRUD(c *C) {
	s.suite.AuthoritiesCRUD(c)
}

func (s *BSuite) TestNodesCRUD(c *C) {
	s.suite.NodesCRUD(c)
}

func (s *BSuite) TestReverseTunnelsCRUD(c *C) {
	s.suite.ReverseTunnelsCRUD(c)
}

func (s *BSuite) TestLocksCRUD(c *C) {
	s.suite.LocksCRUD(c)
}

func (s *BSuite) TestPeersCRUD(c *C) {
	s.suite.PeersCRUD(c)
}

func (s *BSuite) TestObjectsCRUD(c *C) {
	s.suite.ObjectsCRUD(c)
}

func (s *BSuite) TestChangesetsCRUD(c *C) {
	s.suite.ChangesetsCRUD(c)
}

func (s *BSuite) TestOpsCenterLinksCRUD(c *C) {
	s.suite.OpsCenterLinksCRUD(c)
}

func (s *BSuite) TestRolesCRUD(c *C) {
	s.suite.RolesCRUD(c)
}

func (s *BSuite) TestNamespacesCRUD(c *C) {
	s.suite.NamespacesCRUD(c)
}

func (s *BSuite) TestLoginAttempts(c *C) {
	s.suite.LoginAttempts(c)
}

func (s *BSuite) TestLocalCluster(c *C) {
	s.suite.LocalCluster(c)
}

func (s *BSuite) TestSAMLCRUD(c *C) {
	s.suite.SAMLCRUD(c)
}

func (s *BSuite) TestClusterAgentCreds(c *C) {
	s.suite.ClusterAgentCreds(c)
}

func (s *BSuite) TestClusterLogin(c *C) {
	s.suite.ClusterLogin(c)
}

func (s *BSuite) TestIndexFile(c *C) {
	s.suite.IndexFile(c)
}
