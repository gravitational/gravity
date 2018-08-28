package gravity

import (
	"net/http/httptest"
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func TestGravityResources(t *testing.T) { check.TestingT(t) }

type GravityResourcesSuite struct {
	s      *Suite
	r      *Resources
	server *httptest.Server
}

var _ = check.Suite(&GravityResourcesSuite{
	s: &Suite{},
})

func (s *GravityResourcesSuite) SetUpSuite(c *check.C) {
	s.s.SetUp(c)
	// start up ops server using configured ops handler
	s.server = httptest.NewServer(s.s.Handler)
	// create the ops client that uses admin agent creds
	client, err := opsclient.NewBearerClient(s.server.URL, s.s.Creds.Password)
	c.Assert(err, check.IsNil)
	// create the resource control that uses this ops client
	s.r, err = New(Config{
		Operator: client,
		Silent:   localenv.Silent(false),
	})
	c.Assert(err, check.IsNil)
}

func (s *GravityResourcesSuite) TearDownSuite(c *check.C) {
	if s.server != nil {
		s.server.Close()
	}
	s.s.TearDown(c)
}

func (s *GravityResourcesSuite) TestGithubConnectorResource(c *check.C) {
	err := s.r.Create(resources.CreateRequest{Resource: toUnknown(c, githubConnector)})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{Kind: teleservices.KindGithubConnector, WithSecrets: true})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &githubCollection{[]teleservices.GithubConnector{githubConnector}})

	err = s.r.Remove(resources.RemoveRequest{Kind: teleservices.KindGithubConnector, Name: "github"})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{Kind: teleservices.KindGithubConnector})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &githubCollection{[]teleservices.GithubConnector{}})
}

func (s *GravityResourcesSuite) TestUser(c *check.C) {
	err := s.r.Create(resources.CreateRequest{Resource: toUnknown(c, user)})
	c.Assert(err, check.IsNil)

	collectionI, err := s.r.GetCollection(resources.ListRequest{Kind: "user", Name: "test"})
	c.Assert(err, check.IsNil)
	collection, ok := collectionI.(*userCollection)
	c.Assert(ok, check.Equals, true)
	c.Assert(len(collection.users), check.Equals, 1)
	user.SetCreatedBy(collection.users[0].GetCreatedBy())
	user.SetRawObject(collection.users[0].GetRawObject())
	compare.DeepCompare(c, collection, &userCollection{[]teleservices.User{user}})

	err = s.r.Remove(resources.RemoveRequest{Kind: "user", Name: "test"})
	c.Assert(err, check.IsNil)

	collectionI, err = s.r.GetCollection(resources.ListRequest{Kind: "user", Name: "test"})
	c.Assert(err, check.FitsTypeOf, trace.NotFound(""))
}

func (s *GravityResourcesSuite) TestToken(c *check.C) {
	token := storage.NewToken("test", s.s.Creds.Email)

	err := s.r.Create(resources.CreateRequest{Resource: toUnknown(c, token)})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{Kind: "token", Name: "test", User: s.s.Creds.Email})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &tokenCollection{[]storage.Token{token}})

	err = s.r.Remove(resources.RemoveRequest{Kind: "token", Name: "test", User: s.s.Creds.Email})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{Kind: "token", Name: "test", User: s.s.Creds.Email})
	c.Assert(err, check.FitsTypeOf, trace.NotFound(""))
}

func toUnknown(c *check.C, resource teleservices.Resource) teleservices.UnknownResource {
	unknown, err := utils.ToUnknownResource(resource)
	c.Assert(err, check.IsNil)
	return *unknown
}

var (
	githubConnector = teleservices.NewGithubConnector("github", teleservices.GithubConnectorSpecV3{
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

	user = storage.NewUser("test", storage.UserSpecV2{
		AccountID: defaults.SystemAccountID,
		Type:      storage.AgentUser,
		Roles:     []string{"@teleadmin"},
	})
)
