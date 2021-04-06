/*
Copyright 2018-2020 Gravitational, Inc.

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

package gravity

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

func TestGravityResources(t *testing.T) { check.TestingT(t) }

type GravityResourcesSuite struct {
	s       *Suite
	r       *Resources
	cluster *ops.Site
	server  *httptest.Server
	clock   clockwork.Clock
}

var _ = check.Suite(&GravityResourcesSuite{
	s:     &Suite{},
	clock: clockwork.NewFakeClock(),
})

func (s *GravityResourcesSuite) SetUpSuite(c *check.C) {
	s.s.SetUp(c)
	// start up ops server using configured ops handler
	s.server = httptest.NewTLSServer(s.s.Handler)
	// create the ops client that uses admin agent creds
	client, err := opsclient.NewBearerClient(s.server.URL, s.s.Creds.Password,
		opsclient.HTTPClient(s.server.Client()))
	c.Assert(err, check.IsNil)
	// create the resource control that uses this ops client
	s.r, err = New(Config{
		Operator: client,
		Silent:   localenv.Silent(false),
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

func (s *GravityResourcesSuite) TestGithubConnectorResource(c *check.C) {
	err := s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, githubConnector)})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindGithubConnector, WithSecrets: true})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &githubCollection{[]teleservices.GithubConnector{githubConnector}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindGithubConnector, Name: "github"})
	c.Assert(err, check.IsNil)

	collection, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: teleservices.KindGithubConnector})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &githubCollection{[]teleservices.GithubConnector{}})
}

func (s *GravityResourcesSuite) TestUser(c *check.C) {
	err := s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, user)})
	c.Assert(err, check.IsNil)

	collectionI, err := s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: "user", Name: "test"})
	c.Assert(err, check.IsNil)
	collection, ok := collectionI.(*userCollection)
	c.Assert(ok, check.Equals, true)
	c.Assert(len(collection.users), check.Equals, 1)
	user.SetCreatedBy(collection.users[0].GetCreatedBy())
	user.SetRawObject(collection.users[0].GetRawObject())
	user.SetTraits(collection.users[0].GetTraits())
	compare.DeepCompare(c, collection, &userCollection{[]teleservices.User{user}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: "user", Name: "test"})
	c.Assert(err, check.IsNil)

	_, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: "user", Name: "test"})
	if !trace.IsNotFound(err) {
		c.Errorf("Expected err of type NotFound but got %T", err)
	}
}

func (s *GravityResourcesSuite) TestToken(c *check.C) {
	token := storage.NewToken("test", s.s.Creds.Email)

	err := s.r.Create(context.TODO(), resources.CreateRequest{SiteKey: s.cluster.Key(), Resource: toUnknown(c, token)})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: "token", Name: "test", User: s.s.Creds.Email})
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, collection, &tokenCollection{[]storage.Token{token}})

	err = s.r.Remove(context.TODO(), resources.RemoveRequest{SiteKey: s.cluster.Key(), Kind: "token", Name: "test", Owner: s.s.Creds.Email})
	c.Assert(err, check.IsNil)

	_, err = s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: "token", Name: "test", User: s.s.Creds.Email})
	if !trace.IsNotFound(err) {
		c.Errorf("Expected err of type NotFound but got %T", err)
	}
}

func (s *GravityResourcesSuite) TestOperation(c *check.C) {
	node1 := storage.Server{AdvertiseIP: "192.168.1.1", Hostname: "node-1", Role: "master"}
	node2 := storage.Server{AdvertiseIP: "192.168.1.2", Hostname: "node-2", Role: "master"}
	node3 := storage.Server{AdvertiseIP: "192.168.1.3", Hostname: "node-3", Role: "worker"}

	installOp, err := s.s.Services.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationInstall,
		State:      ops.OperationStateCompleted,
		Servers:    storage.Servers{node1, node2},
		Created:    s.clock.Now(),
	})
	c.Assert(err, check.IsNil)

	expandOp, err := s.s.Services.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationExpand,
		State:      ops.OperationStateCompleted,
		Servers:    storage.Servers{node3},
		Created:    s.clock.Now().Add(time.Hour),
	})
	c.Assert(err, check.IsNil)

	shrinkOp, err := s.s.Services.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationShrink,
		State:      ops.OperationStateCompleted,
		Shrink: &storage.ShrinkOperationState{
			Servers: storage.Servers{node2},
		},
		Created: s.clock.Now().Add(2 * time.Hour),
	})
	c.Assert(err, check.IsNil)

	upgradeOp, err := s.s.Services.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationUpdate,
		State:      ops.OperationStateCompleted,
		Update: &storage.UpdateOperationState{
			UpdatePackage: "gravitational.io/example:0.0.2",
		},
		Created: s.clock.Now().Add(3 * time.Hour),
	})
	c.Assert(err, check.IsNil)

	runtimeOp, err := s.s.Services.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationUpdateRuntimeEnviron,
		State:      ops.OperationStateCompleted,
		UpdateEnviron: &storage.UpdateEnvarsOperationState{
			Env: map[string]string{"a": "b"},
		},
		Created: s.clock.Now().Add(4 * time.Hour),
	})
	c.Assert(err, check.IsNil)

	configOp, err := s.s.Services.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationUpdateConfig,
		State:      ops.OperationStateCompleted,
		UpdateConfig: &storage.UpdateConfigOperationState{
			Config: []byte("config"),
		},
		Created: s.clock.Now().Add(5 * time.Hour),
	})
	c.Assert(err, check.IsNil)

	reconfigureOp, err := s.s.Services.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationReconfigure,
		State:      ops.OperationStateCompleted,
		Reconfigure: &storage.ReconfigureOperationState{
			AdvertiseAddr: "192.168.100.1",
		},
		Created: s.clock.Now().Add(6 * time.Hour),
	})
	c.Assert(err, check.IsNil)

	collection, err := s.r.GetCollection(resources.ListRequest{SiteKey: s.cluster.Key(), Kind: storage.KindOperation})
	c.Assert(err, check.IsNil)
	// Should be returned in newest-to-oldest order.
	compare.DeepCompare(c, collection, &operationsCollection{[]storage.Operation{
		toOperation(c, reconfigureOp),
		toOperation(c, configOp),
		toOperation(c, runtimeOp),
		toOperation(c, upgradeOp),
		toOperation(c, shrinkOp),
		toOperation(c, expandOp),
		toOperation(c, installOp),
	}})
}

func toOperation(c *check.C, operation *storage.SiteOperation) storage.Operation {
	resource, err := ops.NewOperation(*operation)
	c.Assert(err, check.IsNil)
	return resource
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
