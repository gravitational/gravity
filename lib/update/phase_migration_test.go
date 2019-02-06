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

package update

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

type PhaseMigrationSuite struct {
	backend storage.Backend
}

var _ = check.Suite(&PhaseMigrationSuite{})

func (s *PhaseMigrationSuite) SetUpSuite(c *check.C) {
	services := opsservice.SetupTestServices(c)
	s.backend = services.Backend
}

func (s *PhaseMigrationSuite) TestMigrateLinks(c *check.C) {
	cluster, err := s.backend.CreateSite(storage.Site{
		AccountID: uuid.New(),
		Domain:    "example.com",
		Created:   time.Now(),
	})
	c.Assert(err, check.IsNil)

	phase, err := NewPhaseMigrateLinks(
		FSMConfig{Backend: s.backend},
		storage.OperationPlan{ClusterName: cluster.Domain},
		storage.OperationPhase{}, logrus.StandardLogger())
	c.Assert(err, check.IsNil)

	// insert a few links
	links := []storage.OpsCenterLink{
		{
			SiteDomain: cluster.Domain,
			Hostname:   "ops.example.com",
			Type:       storage.OpsCenterRemoteAccessLink,
			RemoteAddr: "ops.example.com:3024",
			APIURL:     "https://ops.example.com:443",
			User: &storage.RemoteAccessUser{
				Email: "agent@ops2.example.com",
				Token: "token1",
			},
			Enabled: true,
		},
		{
			SiteDomain: cluster.Domain,
			Hostname:   "ops.example.com",
			Type:       storage.OpsCenterUpdateLink,
			RemoteAddr: "ops.example.com:3024",
			APIURL:     "https://ops.example.com:443",
			Enabled:    true,
		},
		{
			SiteDomain: cluster.Domain,
			Hostname:   "ops2.example.com",
			Type:       storage.OpsCenterRemoteAccessLink,
			RemoteAddr: "ops2.example.com:3024",
			APIURL:     "https://ops2.example.com:61009",
			Enabled:    true,
			User: &storage.RemoteAccessUser{
				Email: "agent@ops2.example.com",
				Token: "token2",
			},
			Wizard: true,
		},
	}
	for _, l := range links {
		_, err := s.backend.UpsertOpsCenterLink(l, 0)
		c.Assert(err, check.IsNil)
	}

	ctx := context.TODO()
	// execute the phase and make sure links get converted to trusted clusters
	err = phase.Execute(ctx)
	c.Assert(err, check.IsNil)

	linksAfterExecute, err := s.backend.GetOpsCenterLinks(cluster.Domain)
	c.Assert(err, check.IsNil)
	c.Assert(len(linksAfterExecute), check.Equals, 3, check.Commentf(
		"links should not have been removed during migration"))

	clustersAfterExecute, err := s.backend.GetTrustedClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(clustersAfterExecute), check.Equals, 1)

	expectedCluster := storage.NewTrustedCluster("ops.example.com",
		storage.TrustedClusterSpecV2{
			Enabled:              true,
			Token:                "token1",
			ProxyAddress:         "ops.example.com:443",
			ReverseTunnelAddress: "ops.example.com:3024",
			SNIHost:              "ops.example.com",
			Roles:                []string{constants.RoleAdmin},
			PullUpdates:          true,
		})

	obtainedCluster, err := s.backend.GetTrustedCluster("ops.example.com")
	c.Assert(err, check.IsNil)
	c.Assert(obtainedCluster, check.DeepEquals, expectedCluster)

	// rollback and make sure trusted clusters have been removed
	err = phase.Rollback(ctx)
	c.Assert(err, check.IsNil)

	clustersAfterRollback, err := s.backend.GetTrustedClusters()
	c.Assert(err, check.IsNil)
	c.Assert(len(clustersAfterRollback), check.Equals, 0)
}
