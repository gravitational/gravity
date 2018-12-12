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

package opshandler

import (
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/teleport"
	teleservices "github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestOpsHandler(t *testing.T) { TestingT(t) }

type OpsHandlerSuite struct {
	backend     storage.Backend
	suite       suite.OpsSuite
	webServer   *httptest.Server
	users       users.Identity
	clock       *timetools.FreezedTime
	adminUser   string
	testAppPath string
	testApp     loc.Locator
	client      *opsclient.Client

	dir string
}

var _ = Suite(&OpsHandlerSuite{
	clock: &timetools.FreezedTime{
		CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
	},
})

func (s *OpsHandlerSuite) SetUpSuite(c *C) {
	log.SetOutput(os.Stderr)
	log.SetFormatter(&trace.TextFormatter{})
}

func (s *OpsHandlerSuite) SetUpTest(c *C) {
	services := opsservice.SetupTestServices(c)

	s.backend = services.Backend
	s.users = services.Users

	s.adminUser = "admina@example.com"
	role, err := users.NewAdminRole()
	c.Assert(err, IsNil)

	err = s.users.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)
	err = s.users.UpsertUser(storage.NewUser(s.adminUser, storage.UserSpecV2{
		Password: "admin-password",
		Type:     storage.AdminUser,
		Roles:    []string{role.GetName()},
	}))
	c.Assert(err, IsNil)

	_, err = s.suite.SetUpTestPackage(services.Apps, services.Packages, c)
	c.Assert(err, IsNil)

	handler, err := NewWebHandler(WebHandlerConfig{
		Users:        s.users,
		Operator:     services.Operator,
		Applications: services.Apps,
		Packages:     services.Packages,
	})
	c.Assert(err, IsNil)
	s.webServer = httptest.NewServer(handler)

	// for regular test, let's be admins, so tests
	// won't be affected by auth issues
	s.client, err = opsclient.NewAuthenticatedClient(
		s.webServer.URL, s.adminUser, "admin-password")

	s.suite.O = s.client
	s.suite.U = s.users
}

func (s *OpsHandlerSuite) TearDownTest(c *C) {
	if s.webServer != nil {
		s.webServer.Close()
	}
	if s.backend != nil {
		c.Assert(s.backend.Close(), IsNil)
	}
}

func (s *OpsHandlerSuite) TestAccountsCRUD(c *C) {
	s.suite.AccountsCRUD(c)
}

func (s *OpsHandlerSuite) TestSitesCRUD(c *C) {
	s.suite.SitesCRUD(c)
}

func (s *OpsHandlerSuite) TestInstallInstructions(c *C) {
	s.suite.InstallInstructions(c)
}

func (s *OpsHandlerSuite) TestGithubConnector(c *C) {
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	connectors, err := s.client.GetGithubConnectors(key, true)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, connectors, []teleservices.GithubConnector{})

	withSecrets := true
	connector := storage.NewGithubConnector("github", teleservices.GithubConnectorSpecV3{
		ClientID:     "id1",
		ClientSecret: "secret",
		RedirectURL:  "https://gravity",
		TeamsToLogins: []teleservices.TeamMapping{{
			Organization: "example.com",
			Team:         "developers",
			Logins:       []string{"admin"},
		}},
	})

	ttl := s.clock.UtcNow().Add(24 * time.Hour)
	connector.SetExpiry(ttl)

	err = s.client.UpsertGithubConnector(key, connector)
	c.Assert(err, IsNil)

	out, err := s.client.GetGithubConnector(key, connector.GetName(), withSecrets)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, out, connector)

	connectors, err = s.client.GetGithubConnectors(key, withSecrets)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, connectors, []teleservices.GithubConnector{connector})

	connectorWithoutSecrets := connector
	connectorWithoutSecrets.Spec.ClientSecret = ""
	out, err = s.client.GetGithubConnector(key, connector.GetName(), !withSecrets)
	compare.DeepCompare(c, out, connectorWithoutSecrets)

	err = s.client.DeleteGithubConnector(key, connector.GetName())
	c.Assert(err, IsNil)

	_, err = s.client.GetGithubConnector(key, connector.GetName(), withSecrets)
	c.Assert(trace.IsNotFound(err), Equals, true)
}

func (s *OpsHandlerSuite) TestUser(c *C) {
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	user := storage.NewUser("testuser@test.com", storage.UserSpecV2{
		Type: storage.AgentUser,
	})

	ttl := time.Now().UTC().Add(24 * time.Hour)
	user.SetExpiry(ttl)

	err := s.client.UpsertUser(key, user)
	c.Assert(err, IsNil)

	out, err := s.client.GetUser(key, user.GetName())
	c.Assert(err, IsNil)
	c.Assert(user.Equals(out), Equals, true, Commentf(compare.Diff(user, out)))

	role, err := users.NewAdminRole()
	c.Assert(err, IsNil)
	err = s.users.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)

	user.SetRoles([]string{role.GetName()})

	err = s.client.UpsertUser(key, user)
	c.Assert(err, IsNil)

	out, err = s.client.GetUser(key, user.GetName())
	c.Assert(err, IsNil)
	c.Assert(user.Equals(out), Equals, true, Commentf(compare.Diff(user, out)))

	users, err := s.client.GetUsers(key)
	c.Assert(err, IsNil)
	mapped := make(map[string]teleservices.User)
	for i := range users {
		mapped[users[i].GetName()] = users[i]
	}
	userout, ok := mapped[user.GetName()]
	c.Assert(ok, Equals, true)
	c.Assert(user.Equals(userout), Equals, true, Commentf(compare.Diff(user, userout)))

	err = s.client.DeleteUser(key, user.GetName())
	c.Assert(err, IsNil)

	_, err = s.client.GetUser(key, user.GetName())
	c.Assert(trace.IsNotFound(err), Equals, true)
}

// TestUserWithNonexistentRole tests scenario when someone tries
// to create a user with a nonexistent role
func (s *OpsHandlerSuite) TestUserWithNonexistentRole(c *C) {
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	user := storage.NewUser("testuser@test.com", storage.UserSpecV2{
		Type: storage.AgentUser,
	})

	user.SetRoles([]string{"nothere"})

	err := s.client.UpsertUser(key, user)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("expected not found, got %T, %v", err, err))
}

func (s *OpsHandlerSuite) TestLogForwarder(c *C) {
	testEnabled := os.Getenv(defaults.TestK8s)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		c.Skip("log forwarders test needs Kubernetes")
	}

	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	forwarder1 := storage.NewLogForwarder("f1", "127.0.0.1:7070", "tcp")
	forwarder2 := storage.NewLogForwarder("f2", "127.0.0.1:8080", "udp")
	for _, f := range []storage.LogForwarder{forwarder1, forwarder2} {
		err := s.client.CreateLogForwarder(key, f)
		c.Assert(err, IsNil)
	}
	s.checkForwarders(c, key, []storage.LogForwarder{forwarder1, forwarder2})

	err := s.client.DeleteLogForwarder(key, forwarder1.GetName())
	c.Assert(err, IsNil)
	s.checkForwarders(c, key, []storage.LogForwarder{forwarder2})

	forwarderV1 := storage.LogForwarderV1{
		Address:  "127.0.0.1:9090",
		Protocol: "tcp",
	}
	err = s.client.UpdateLogForwarders(key, []storage.LogForwarderV1{forwarderV1})
	c.Assert(err, IsNil)
	s.checkForwarders(c, key, []storage.LogForwarder{storage.NewLogForwarderFromV1(forwarderV1)})
}

func (s *OpsHandlerSuite) TestUpdatesClusterEnvironment(c *C) {
	if !utils.RunKubernetesTests() {
		c.Skip("Test requires kubernetes cluster")
	}

	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}
	env := storage.NewEnvironment(map[string]string{
		"FOO": "foo",
		"BAR": "bar",
	})
	err := s.client.UpdateClusterEnvironmentVariables(ops.UpdateClusterEnvironmentVariablesRequest{
		Key: key,
		Env: env,
	})
	c.Assert(err, IsNil)

	clusterEnv, err := s.client.GetClusterEnvironmentVariables(key)
	c.Assert(err, IsNil)
	c.Assert(clusterEnv.GetKeyValues(), DeepEquals, env.GetKeyValues())
}

func (s *OpsHandlerSuite) checkForwarders(c *C, key ops.SiteKey, expected []storage.LogForwarder) {
	forwarders, err := s.client.GetLogForwarders(key)
	c.Assert(err, IsNil)
	c.Assert(len(forwarders), Equals, len(expected))
	forwardersMap := make(map[string]storage.LogForwarder)
	for i := range forwarders {
		forwardersMap[forwarders[i].GetName()] = forwarders[i]
	}
	for _, f := range expected {
		c.Assert(f, DeepEquals, forwardersMap[f.GetName()])
	}
}

func (s *OpsHandlerSuite) TestToken(c *C) {
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	user := storage.NewUser("testuser@test.com", storage.UserSpecV2{
		Type: storage.AgentUser,
	})

	ttl := time.Now().UTC().Add(24 * time.Hour)
	user.SetExpiry(ttl)

	err := s.client.UpsertUser(key, user)
	c.Assert(err, IsNil)

	token := storage.NewToken("secret", user.GetName())

	_, err = s.client.CreateAPIKey(ops.NewAPIKeyRequest{
		Token:     token.GetName(),
		UserEmail: token.GetUser(),
	})
	c.Assert(err, IsNil)

	keys, err := s.client.GetAPIKeys(user.GetName())
	c.Assert(err, IsNil)

	found := false
	for _, k := range keys {
		t := storage.NewTokenFromV1(k)
		if t.GetName() == token.GetName() {
			found = true
			break
		}
	}
	c.Assert(found, Equals, true)

	err = s.client.DeleteAPIKey(user.GetName(), token.GetName())
	c.Assert(err, IsNil)

	keys, err = s.client.GetAPIKeys(user.GetName())
	c.Assert(err, IsNil)

	found = false
	for _, k := range keys {
		t := storage.NewTokenFromV1(k)
		if t.GetName() == token.GetName() {
			found = true
			break
		}
	}
	c.Assert(found, Equals, false)
}

func (s *OpsHandlerSuite) TestClusterAuthConfiguration(c *C) {
	// should not exist
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}
	_, err := s.client.GetClusterAuthPreference(key)
	c.Assert(trace.IsNotFound(err), Equals, true)

	// upsert
	cap, err := teleservices.NewAuthPreference(teleservices.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, IsNil)

	err = s.client.UpsertClusterAuthPreference(key, cap)
	c.Assert(err, IsNil)

	// read
	actual, err := s.client.GetClusterAuthPreference(key)
	c.Assert(err, IsNil)

	c.Assert(actual.GetType(), Equals, cap.GetType())
	c.Assert(actual.GetSecondFactor(), Equals, cap.GetSecondFactor())
}
