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

package webpack

import (
	"bytes"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/pack/suite"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/users/usersservice"

	"github.com/gravitational/roundtrip"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestWebpack(t *testing.T) { TestingT(t) }

type WebpackSuite struct {
	backend   storage.Backend
	suite     suite.PackageSuite
	webServer *httptest.Server
	users     users.Identity
	clock     *timetools.FreezedTime

	agentUser storage.User
	adminUser storage.User

	dir string
}

var _ = Suite(&WebpackSuite{
	clock: &timetools.FreezedTime{
		CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
	},
})

func (s *WebpackSuite) SetUpSuite(c *C) {
	log.SetOutput(os.Stderr)
}

func (s *WebpackSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.backend, err = keyval.NewBolt(keyval.BoltConfig{Path: filepath.Join(s.dir, "bolt.db")})
	c.Assert(err, IsNil)

	objects, err := fs.New(s.dir)
	c.Assert(err, IsNil)

	s.users, err = usersservice.New(
		usersservice.Config{Backend: s.backend})
	c.Assert(err, IsNil)

	role, err := users.NewAdminRole()
	c.Assert(err, IsNil)
	err = s.users.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)

	s.adminUser = storage.NewUser("admin@a.example.com", storage.UserSpecV2{
		Password: "admin-password",
		Type:     storage.AdminUser,
		Roles:    []string{role.GetName()},
	})
	err = s.users.UpsertUser(s.adminUser)
	c.Assert(err, IsNil)

	service, err := localpack.New(localpack.Config{
		Backend:     s.backend,
		UnpackedDir: filepath.Join(s.dir, defaults.UnpackedDir),
		Clock:       s.clock,
		Objects:     objects,
	})
	c.Assert(err, IsNil)
	webHandler, err := NewHandler(Config{
		Users:    s.users,
		Packages: service,
	})
	c.Assert(err, IsNil)
	mux := http.NewServeMux()
	mux.Handle("/pack/", webHandler)

	// It is important that we launch TLS server as authentication
	// middleware on the handler expects TLS connections.
	s.webServer = httptest.NewTLSServer(mux)

	// for regular test, let's be admins, so tests
	// won't be affected by auth issues
	s.suite.S, err = NewAuthenticatedClient(
		s.webServer.URL, s.adminUser.GetName(), "admin-password",
		roundtrip.HTTPClient(s.webServer.Client()))
	c.Assert(err, IsNil)

	s.suite.O = objects
	s.suite.C = s.clock
}

func (s *WebpackSuite) TestPermissionsCRUD(c *C) {
	account, err := s.backend.CreateAccount(storage.Account{
		Org: "example.com",
	})
	c.Assert(err, IsNil)
	// Create application package
	repo, err := s.backend.CreateRepository(storage.NewRepository("test"))
	c.Assert(err, IsNil)
	app, err := s.backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "test",
		Version:    "0.0.1",
		Type:       string(storage.AppUser),
	})
	c.Assert(err, IsNil)

	site, err := s.backend.CreateSite(storage.Site{
		AccountID: account.ID,
		Domain:    "a.example.com",
		Created:   s.clock.UtcNow(),
		App:       *app,
	})
	c.Assert(err, IsNil)

	s.agentUser = storage.NewUser("agent@a.example.com", storage.UserSpecV2{
		ClusterName: site.Domain,
		AccountID:   account.ID,
		Type:        storage.AgentUser,
	})
	err = s.users.UpsertUser(s.agentUser)
	c.Assert(err, IsNil)

	keys, err := s.backend.GetAPIKeys(s.agentUser.GetName())
	c.Assert(err, IsNil)

	newService := func() *Client {
		service, err := NewAuthenticatedClient(
			s.webServer.URL, s.agentUser.GetName(), keys[0].Token,
			roundtrip.HTTPClient(&http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					}}}),
		)
		c.Assert(err, IsNil)
		return service
	}
	service := newService()

	// by default agents can't do anything
	_, err = service.GetRepositories()
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	err = service.UpsertRepository("a.example.com", time.Time{})
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	pack1Data := []byte("hello, world!")
	loc1 := loc.MustParseLocator("a.example.com/package-1:0.0.1")
	repoA := "a.example.com"

	_, err = service.CreatePackage(
		loc1, bytes.NewBuffer(pack1Data))
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))
	// now create permissions to read packages from one repository
	// (e.g. agent)
	c.Assert(s.suite.S.UpsertRepository(repoA, time.Time{}), IsNil)

	_, err = s.suite.S.CreatePackage(
		loc1, bytes.NewBuffer(pack1Data))
	c.Assert(err, IsNil)

	role, err := teleservices.NewRole(s.agentUser.GetName(), teleservices.RoleSpecV3{
		Allow: teleservices.RoleConditions{
			Namespaces: []string{defaults.Namespace},
			Rules: []teleservices.Rule{
				{
					Resources: []string{storage.KindRepository},
					Where: storage.EqualsExpr{
						Left:  storage.ResourceNameExpr,
						Right: storage.StringExpr("a.example.com"),
					}.String(),
					Verbs: []string{teleservices.VerbList, teleservices.VerbRead},
				},
			},
		},
	})
	c.Assert(err, IsNil)
	err = s.users.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)
	roles := []string{role.GetName()}
	err = s.users.UpdateUser(s.agentUser.GetName(), storage.UpdateUserReq{Roles: &roles})
	c.Assert(err, IsNil)

	// now we can read this package
	_, err = service.ReadPackageEnvelope(loc1)
	c.Assert(err, IsNil)

	// but we can't do pretty much anything else
	_, err = service.GetRepositories()
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	err = service.UpsertRepository(repoA, time.Time{})
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	_, err = service.CreatePackage(
		loc1, bytes.NewBuffer(pack1Data))
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	role, err = users.NewClusterAgentRole(s.agentUser.GetName(), site.Domain)
	c.Assert(err, IsNil)
	err = s.users.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)
	roles = []string{role.GetName()}
	err = s.users.UpdateUser(s.agentUser.GetName(), storage.UpdateUserReq{Roles: &roles})
	c.Assert(err, IsNil)

	pack2Data := []byte("hello, world 2!")
	loc2 := loc.MustParseLocator("a.example.com/package-2:0.0.2")

	_, err = s.suite.S.CreatePackage(
		loc2, bytes.NewBuffer(pack2Data))
	c.Assert(err, IsNil)

	err = s.suite.S.DeletePackage(loc2)
	c.Assert(err, IsNil)

	// the last step - unleash all the power!
	role, err = users.NewAdminRole()
	c.Assert(err, IsNil)
	err = s.users.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)
	s.agentUser.SetRoles([]string{role.GetName()})
	err = s.users.UpsertUser(s.agentUser)
	c.Assert(err, IsNil)

	c.Assert(s.suite.S.UpsertRepository("b.example.com", time.Time{}), IsNil)

	repos, err := s.suite.S.GetRepositories()
	c.Assert(err, IsNil)
	// sort to avoid ordering issues when querying the backend
	sort.Strings(repos)
	c.Assert(repos, DeepEquals, []string{"a.example.com", "b.example.com", "test"})

	c.Assert(s.suite.S.DeleteRepository("a.example.com"), IsNil)
}

func (s *WebpackSuite) TearDownTest(c *C) {
	s.webServer.Close()
	c.Assert(s.backend.Close(), IsNil)
}

func (s *WebpackSuite) TestRepositoriesCRUD(c *C) {
	s.suite.RepositoriesCRUD(c)
}

func (s *WebpackSuite) TestPackagesCRUD(c *C) {
	s.suite.PackagesCRUD(c)
}

func (s *WebpackSuite) TestUpsertPackages(c *C) {
	s.suite.UpsertPackages(c)
}

func (s *WebpackSuite) TestDeleteRepository(c *C) {
	s.suite.DeleteRepository(c)
}
