package gravity

import (
	"context"
	"net/http/httptest"
	"testing"

	_ "github.com/gravitational/gravity/e/lib/modules"
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/e/lib/ops/handler"
	"github.com/gravitational/gravity/e/lib/ops/service"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func TestGravityResources(t *testing.T) { check.TestingT(t) }

type GravityResourcesSuite struct {
	s       *gravity.Suite
	r       *Resources
	cluster *ops.Site
	server  *httptest.Server
}

var _ = check.Suite(&GravityResourcesSuite{
	s: &gravity.Suite{},
})

func (s *GravityResourcesSuite) SetUpSuite(c *check.C) {
	s.s.SetUp(c)
	// start up ops server using enterprise ops handler
	s.server = httptest.NewServer(handler.NewWebHandler(s.s.Handler,
		service.New(s.s.Services.Operator)))
	// create the ops client that uses admin agent creds
	client, err := client.NewBearerClient(s.server.URL, s.s.Creds.Password)
	c.Assert(err, check.IsNil)
	// create the resource control that uses this ops client
	ossResources, err := gravity.New(gravity.Config{
		Operator: client.Client,
		Silent:   localenv.Silent(false),
	})
	c.Assert(err, check.IsNil)
	s.r, err = New(Config{
		Resources: ossResources,
		Operator:  client,
	})
	c.Assert(err, check.IsNil)
	s.cluster, err = client.GetLocalSite(context.TODO())
	c.Assert(err, check.IsNil)
}

func (s *GravityResourcesSuite) TearDownSuite(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
	s.s.TearDown(c)
}

func (s *GravityResourcesSuite) TestRole(c *check.C) {
	err := s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, role)})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindRole, Name: role.GetName()})
	c.Assert(err, check.IsNil)
	err = role.CheckAndSetDefaults()
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &roleCollection{[]teleservices.Role{role}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindRole, Name: role.GetName()})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindRole, Name: role.GetName()})
	if !trace.IsNotFound(err) {
		c.Error("Expected the error to be of type NotFound.")
	}
}

func (s *GravityResourcesSuite) TestOIDCConnectorResource(c *check.C) {
	err := s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, oidcConnector)})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindOIDCConnector, WithSecrets: true})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &oidcCollection{[]teleservices.OIDCConnector{oidcConnector}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindOIDCConnector, Name: oidcConnector.GetName()})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindOIDCConnector})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &oidcCollection{[]teleservices.OIDCConnector{}})
}

func (s *GravityResourcesSuite) TestSAMLConnectorResource(c *check.C) {
	err := s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, samlConnector)})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindSAMLConnector, WithSecrets: true})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &samlCollection{[]teleservices.SAMLConnector{samlConnector}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindSAMLConnector, Name: samlConnector.GetName()})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindSAMLConnector})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &samlCollection{[]teleservices.SAMLConnector{}})
}

func (s *GravityResourcesSuite) TestAuthConnectorResource(c *check.C) {
	err := s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, oidcConnector)})
	c.Assert(err, check.IsNil)
	err = s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, githubConnector)})
	c.Assert(err, check.IsNil)
	err = s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, samlConnector)})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindAuthConnector, WithSecrets: true})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &authConnectorCollection{[]teleservices.Resource{oidcConnector, githubConnector, samlConnector}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindOIDCConnector, Name: oidcConnector.GetName()})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindAuthConnector, WithSecrets: true})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &authConnectorCollection{[]teleservices.Resource{githubConnector, samlConnector}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindGithubConnector, Name: githubConnector.GetName()})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindAuthConnector, WithSecrets: true})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &authConnectorCollection{[]teleservices.Resource{samlConnector}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindSAMLConnector, Name: samlConnector.GetName()})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindAuthConnector, WithSecrets: true})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &authConnectorCollection{[]teleservices.Resource{}})
}

func toUnknown(c *check.C, resource teleservices.Resource) teleservices.UnknownResource {
	unknown, err := utils.ToUnknownResource(resource)
	c.Assert(err, check.IsNil)
	return *unknown
}

var (
	role = &teleservices.RoleV3{
		Kind:    teleservices.KindRole,
		Version: teleservices.V3,
		Metadata: teleservices.Metadata{
			Name:      "test",
			Namespace: teledefaults.Namespace,
		},
		Spec: teleservices.RoleSpecV3{
			Allow: teleservices.RoleConditions{
				Namespaces: []string{defaults.Namespace},
				Logins:     []string{"root"},
				Rules: []teleservices.Rule{{
					Resources: []string{teleservices.Wildcard},
					Verbs:     []string{teleservices.Wildcard},
				}},
			}},
	}

	oidcConnector = storage.NewOIDCConnector("oidc-connector", teleservices.OIDCConnectorSpecV2{
		RedirectURL:  "https://ops.example.com/portalapi/v1/oidc/callback",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		IssuerURL:    "https://example.com",
		ClaimsToRoles: []teleservices.ClaimMapping{
			{
				Claim: "roles",
				Value: "gravitational/admins",
				Roles: []string{"@teleadmin"},
			},
		},
	})

	samlConnector = storage.NewSAMLConnector("saml-connector", teleservices.SAMLConnectorSpecV2{
		Issuer:                   "http://example.com",
		SSO:                      "https://example.com/saml/sso",
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

	githubConnector = storage.NewGithubConnector("github-connector", teleservices.GithubConnectorSpecV3{
		RedirectURL:  "https://ops.example.com/portalapi/v1/github/callback",
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		TeamsToLogins: []teleservices.TeamMapping{
			{
				Organization: "gravitational",
				Team:         "dev",
				Logins:       []string{"@teleadmin"},
			},
		},
	})
)
