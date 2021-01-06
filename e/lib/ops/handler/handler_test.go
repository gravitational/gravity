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

package handler

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/e/lib/ops/service"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opshandler"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/teleport/lib/fixtures"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	check "gopkg.in/check.v1"
)

func TestHandler(t *testing.T) { check.TestingT(t) }

type HandlerSuite struct {
	services opsservice.TestServices
	suite    suite.OpsSuite
	server   *httptest.Server
	client   *client.Client
}

var _ = check.Suite(&HandlerSuite{
	suite: suite.OpsSuite{
		C: &timetools.FreezedTime{
			CurrentTime: time.Date(1984, 4, 4, 13, 0, 0, 0, time.UTC),
		},
	},
})

func (s *HandlerSuite) TestRole(c *check.C) {
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	role, err := teleservices.NewRole("test", teleservices.RoleSpecV3{
		Options: teleservices.RoleOptions{
			MaxSessionTTL: teleservices.NewDuration(time.Hour),
		},
		Allow: teleservices.RoleConditions{},
	})
	c.Assert(err, check.IsNil)

	ttl := s.suite.C.UtcNow().Add(24 * time.Hour)
	role.SetExpiry(ttl)

	err = s.client.UpsertRole(key, role)
	c.Assert(err, check.IsNil)

	out, err := s.client.GetRole(key, role.GetName())
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, out, role)

	err = s.client.DeleteRole(key, role.GetName())
	c.Assert(err, check.IsNil)

	_, err = s.client.GetRole(key, role.GetName())
	c.Assert(trace.IsNotFound(err), check.Equals, true)
}

// TestSystemRoleAccessDenied makes sure that it's impossible to create
// roles with system labels via ACL controllers
func (s *HandlerSuite) TestSystemRoleAccessDenied(c *check.C) {
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	role, err := users.NewClusterAgentRole("test", key.SiteDomain)
	c.Assert(err, check.IsNil)

	err = s.client.UpsertRole(key, role)
	c.Assert(trace.IsAccessDenied(err), check.Equals, true)
}

func (s *HandlerSuite) TestOIDCConnector(c *check.C) {
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	connectors, err := s.client.GetOIDCConnectors(key, true)
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, connectors, []teleservices.OIDCConnector{})

	withSecrets := true
	connector := storage.NewOIDCConnector("google", teleservices.OIDCConnectorSpecV2{
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id1",
		ClientSecret: "secret",
		RedirectURL:  "https://gravity",
	})

	ttl := s.suite.C.UtcNow().Add(24 * time.Hour)
	connector.SetExpiry(ttl)

	err = s.client.UpsertOIDCConnector(key, connector)
	c.Assert(err, check.IsNil)

	out, err := s.client.GetOIDCConnector(key, connector.GetName(), withSecrets)
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, out, connector)

	connectors, err = s.client.GetOIDCConnectors(key, withSecrets)
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, connectors, []teleservices.OIDCConnector{connector})

	connectorWithoutSecrets := connector
	connectorWithoutSecrets.Spec.ClientSecret = ""
	out, err = s.client.GetOIDCConnector(key, connector.GetName(), !withSecrets)
	compare.DeepCompare(c, out, connectorWithoutSecrets)

	err = s.client.DeleteOIDCConnector(key, connector.GetName())
	c.Assert(err, check.IsNil)

	_, err = s.client.GetOIDCConnector(key, connector.GetName(), withSecrets)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
}

func (s *HandlerSuite) TestSAMLConnector(c *check.C) {
	key := ops.SiteKey{AccountID: "a", SiteDomain: "b"}

	connectors, err := s.client.GetSAMLConnectors(key, true)
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, connectors, []teleservices.SAMLConnector{})

	withSecrets := true
	connector := storage.NewSAMLConnector("okta", teleservices.SAMLConnectorSpecV2{
		Issuer: "http://example.com",
		SSO:    "https://example.com/saml/sso",
		AssertionConsumerService: "https://localhost/acs",
		Audience:                 "https://localhost/aud",
		ServiceProviderIssuer:    "https://localhost/iss",
		AttributesToRoles: []teleservices.AttributeMapping{
			{
				Name:  "groups",
				Value: "admin",
				Roles: []string{"@teleadmin"},
			},
		},
		Cert: fixtures.SigningCertPEM,
		SigningKeyPair: &teleservices.SigningKeyPair{
			PrivateKey: fixtures.SigningKeyPEM,
			Cert:       fixtures.SigningCertPEM,
		},
	})

	ttl := s.suite.C.UtcNow().Add(24 * time.Hour)
	connector.SetExpiry(ttl)

	err = s.client.UpsertSAMLConnector(key, connector)
	c.Assert(err, check.IsNil)

	out, err := s.client.GetSAMLConnector(key, connector.GetName(), withSecrets)
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, out, connector)

	connectors, err = s.client.GetSAMLConnectors(key, withSecrets)
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, connectors, []teleservices.SAMLConnector{connector})

	connectorWithoutSecrets := connector
	connectorWithoutSecrets.Spec.SigningKeyPair.PrivateKey = ""
	out, err = s.client.GetSAMLConnector(key, connector.GetName(), !withSecrets)
	compare.DeepCompare(c, out, connectorWithoutSecrets)

	err = s.client.DeleteSAMLConnector(key, connector.GetName())
	c.Assert(err, check.IsNil)

	_, err = s.client.GetSAMLConnector(key, connector.GetName(), withSecrets)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
}

func (s *HandlerSuite) SetUpTest(c *check.C) {
	s.services = opsservice.SetupTestServices(c)

	role, err := users.NewAdminRole()
	c.Assert(err, check.IsNil)

	err = s.services.Users.UpsertRole(role, storage.Forever)
	c.Assert(err, check.IsNil)

	err = s.services.Users.UpsertUser(storage.NewUser(adminUser,
		storage.UserSpecV2{
			Password: adminPass,
			Type:     storage.AdminUser,
			Roles:    []string{role.GetName()},
		}))
	c.Assert(err, check.IsNil)

	_, err = s.suite.SetUpTestPackage(s.services.Apps, s.services.Packages, c)
	c.Assert(err, check.IsNil)

	ossHandler, err := opshandler.NewWebHandler(
		opshandler.WebHandlerConfig{
			Users:        s.services.Users,
			Operator:     s.services.Operator,
			Applications: s.services.Apps,
			Packages:     s.services.Packages,
		})
	c.Assert(err, check.IsNil)

	s.server = httptest.NewServer(NewWebHandler(
		ossHandler, service.New(s.services.Operator)))

	s.client, err = client.NewAuthenticatedClient(
		s.server.URL, adminUser, adminPass)
}

func (s *HandlerSuite) TearDownTest(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
	s.services.Backend.Close()
	os.RemoveAll(s.services.Dir)
}

const (
	adminUser = "admin@example.com"
	adminPass = "admin-password"
)
