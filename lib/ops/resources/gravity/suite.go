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

package gravity

import (
	"os"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opshandler"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/storage"

	check "gopkg.in/check.v1"
)

// Suite is the gravity resource controller test suite helper
type Suite struct {
	// Services is a set of initialized test services
	Services opsservice.TestServices
	// Handler is the initializes operator web handler
	Handler *opshandler.WebHandler
	// Creds is the credentials for the test cluster admin agent
	Creds *storage.LoginEntry
}

// SetUp prepares services and database for a test
func (s *Suite) SetUp(c *check.C) {
	s.Services = opsservice.SetupTestServices(c)
	// setup test account, application and cluster
	_, err := s.Services.Operator.CreateAccount(ops.NewAccountRequest{
		ID:  defaults.SystemAccountID,
		Org: "test",
	})
	c.Assert(err, check.IsNil)
	app := suite.SetUpTestPackage(c, s.Services.Apps, s.Services.Packages)
	cluster, err := s.Services.Backend.CreateSite(storage.Site{
		Domain:    "example.com",
		AccountID: defaults.SystemAccountID,
		App: storage.Package{
			Repository: app.Repository,
			Name:       app.Name,
			Version:    app.Version,
		},
		Local:   true,
		Created: time.Now().UTC(),
	})
	c.Assert(err, check.IsNil)
	// configure cluster admin agent so it can call the API
	_, err = s.Services.Users.CreateClusterAdminAgent(cluster.Domain,
		storage.NewUser(storage.ClusterAdminAgent(cluster.Domain), storage.UserSpecV2{
			AccountID: defaults.SystemAccountID,
		}))
	c.Assert(err, check.IsNil)
	s.Creds, err = s.Services.Operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   defaults.SystemAccountID,
		ClusterName: cluster.Domain,
		Admin:       true,
	})
	c.Assert(err, check.IsNil)
	// init ops web handler
	s.Handler, err = opshandler.NewWebHandler(opshandler.WebHandlerConfig{
		Operator:     s.Services.Operator,
		Packages:     s.Services.Packages,
		Applications: s.Services.Apps,
		Users:        s.Services.Users,
	})
	c.Assert(err, check.IsNil)
}

// TearDown performs post-test cleanups
func (s *Suite) TearDown(c *check.C) {
	s.Services.Backend.Close()
	os.RemoveAll(s.Services.Dir)
}
