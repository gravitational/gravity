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

package phases

import (
	"context"
	"path/filepath"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

type PhaseMigrationSuite struct {
	backend   storage.Backend
	cluster   storage.Site
	backupDir string
}

var _ = check.Suite(&PhaseMigrationSuite{})

func (s *PhaseMigrationSuite) SetUpSuite(c *check.C) {
	services := opsservice.SetupTestServices(c)
	s.backend = services.Backend
	cluster, err := s.backend.CreateSite(storage.Site{
		AccountID: uuid.New(),
		Domain:    "example.com",
		Created:   services.Clock.Now(),
	})
	c.Assert(err, check.IsNil)
	s.cluster = *cluster
	s.backupDir = c.MkDir()
}

func (s *PhaseMigrationSuite) TestMigrateRoles(c *check.C) {
	getBackupBackend := func(string) (storage.Backend, error) {
		return keyval.NewBolt(keyval.BoltConfig{
			Path: filepath.Join(s.backupDir, "backup.db"),
		})
	}
	phase := newPhaseMigrateRoles(
		storage.OperationPlan{
			OperationID: "op-1",
			ClusterName: s.cluster.Domain,
		},
		s.backend, logrus.StandardLogger(), getBackupBackend)

	role, err := users.NewSystemRole(constants.RoleAdmin, teleservices.RoleSpecV3{
		Allow: teleservices.RoleConditions{
			Rules: []teleservices.Rule{
				{
					Resources: []string{teleservices.Wildcard},
					Verbs:     []string{teleservices.Wildcard},
					Actions: []string{storage.AssignKubernetesGroupsExpr{
						Groups: users.GetAdminKubernetesGroups(),
					}.String()},
				},
			},
		},
	})
	c.Assert(err, check.IsNil)

	err = s.backend.UpsertRole(role, storage.Forever)
	c.Assert(err, check.IsNil)

	err = phase.Execute(context.TODO())
	c.Assert(err, check.IsNil)

	convertedRole, err := s.backend.GetRole(role.GetName())
	c.Assert(err, check.IsNil)
	c.Assert(convertedRole.GetKubeGroups(teleservices.Allow), check.DeepEquals,
		users.GetAdminKubernetesGroups())

	err = phase.Rollback(context.TODO())
	c.Assert(err, check.IsNil)

	rolledBackRole, err := s.backend.GetRole(role.GetName())
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, rolledBackRole, role)
}

func (s *PhaseMigrationSuite) TestMigrateLinks(c *check.C) {
	phase, err := NewPhaseMigrateLinks(
		storage.OperationPlan{ClusterName: s.cluster.Domain},
		s.backend,
		logrus.StandardLogger())
	c.Assert(err, check.IsNil)

	// insert a few links
	links := []storage.OpsCenterLink{
		{
			SiteDomain: s.cluster.Domain,
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
			SiteDomain: s.cluster.Domain,
			Hostname:   "ops.example.com",
			Type:       storage.OpsCenterUpdateLink,
			RemoteAddr: "ops.example.com:3024",
			APIURL:     "https://ops.example.com:443",
			Enabled:    true,
		},
		{
			SiteDomain: s.cluster.Domain,
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

	linksAfterExecute, err := s.backend.GetOpsCenterLinks(s.cluster.Domain)
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

// newPhaseMigrateRoles returns a new roles migration executor
func newPhaseMigrateRoles(plan storage.OperationPlan, backend storage.Backend, logger logrus.FieldLogger, getBackupBackend func(string) (storage.Backend, error)) *phaseMigrateRoles {
	return &phaseMigrateRoles{
		FieldLogger:      logger,
		Backend:          backend,
		ClusterName:      plan.ClusterName,
		OperationID:      plan.OperationID,
		getBackupBackend: getBackupBackend,
	}
}
