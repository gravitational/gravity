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

package opsservice

import (
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

type OperationGroupSuite struct {
	operator *Operator
	cluster  *ops.Site
}

var _ = check.Suite(&OperationGroupSuite{})

func (s *OperationGroupSuite) SetUpTest(c *check.C) {
	services := SetupTestServices(c)
	s.operator = services.Operator

	suite := &suite.OpsSuite{}
	app, err := suite.SetUpTestPackage(services.Apps, services.Packages, c)
	c.Assert(err, check.IsNil)

	account, err := s.operator.CreateAccount(ops.NewAccountRequest{
		Org: "operationgroup.test",
	})
	c.Assert(err, check.IsNil)

	s.cluster, err = s.operator.CreateSite(ops.NewSiteRequest{
		AccountID:  account.ID,
		AppPackage: app.String(),
		Provider:   schema.ProvisionerOnPrem,
		DomainName: "operationgroup.test",
	})
	c.Assert(err, check.IsNil)
}

// Makes sure operation group does not allow to create an expand operation for a not installed cluster
func (s *OperationGroupSuite) TestExpandNotInstalled(c *check.C) {
	group := s.operator.getOperationGroup(s.cluster.Key())

	s.assertClusterState(c, ops.SiteStateNotInstalled)

	// cluster is not installed initially so can't be expanded
	_, err := group.createSiteOperation(ops.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationExpand,
		State:      ops.OperationStateExpandInitiated,
	})
	c.Assert(err, check.NotNil)
}

// Makes sure operation group does not allow to create too many expand operations
func (s *OperationGroupSuite) TestExpandMaxConcurrency(c *check.C) {
	group := s.operator.getOperationGroup(s.cluster.Key())

	// initiate and finalize the install operation
	key, err := group.createSiteOperation(ops.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationInstall,
		State:      ops.OperationStateInstallInitiated,
	})
	c.Assert(err, check.IsNil)
	c.Assert(key, check.NotNil)
	s.assertClusterState(c, ops.SiteStateInstalling)

	_, err = group.compareAndSwapOperationState(context.TODO(), swap{
		key:            *key,
		expectedStates: []string{ops.OperationStateInstallInitiated},
		newOpState:     ops.OperationStateCompleted,
	})
	c.Assert(err, check.IsNil)
	s.assertClusterState(c, ops.SiteStateActive)

	// make sure we have enough servers so simultaneous expands are allowed
	var servers []storage.Server
	for i := 0; i < 3; i++ {
		servers = append(servers, storage.Server{Hostname: fmt.Sprintf("node-%v", i)})
	}
	err = group.addClusterStateServers(servers)
	c.Assert(err, check.IsNil)

	// create a few expand operations, up to the allowed limit
	for i := 0; i < defaults.MaxExpandConcurrency; i++ {
		_, err = group.createSiteOperation(ops.SiteOperation{
			AccountID:  s.cluster.AccountID,
			SiteDomain: s.cluster.Domain,
			Type:       ops.OperationExpand,
			State:      ops.OperationStateExpandInitiated,
			InstallExpand: &storage.InstallExpandOperationState{
				Profiles: map[string]storage.ServerProfile{
					"node": storage.ServerProfile{
						ServiceRole: string(schema.ServiceRoleNode),
					},
				},
			},
			Servers: []storage.Server{{Hostname: fmt.Sprintf("node-%v", i), Role: "node"}},
		})
		c.Assert(err, check.IsNil)
		s.assertClusterState(c, ops.SiteStateExpanding)
	}

	// should prohibit next expand operation creation
	_, err = group.createSiteOperation(ops.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationExpand,
		State:      ops.OperationStateExpandInitiated,
		InstallExpand: &storage.InstallExpandOperationState{
			Profiles: map[string]storage.ServerProfile{
				"node": storage.ServerProfile{
					ServiceRole: string(schema.ServiceRoleNode),
				},
			},
		},
		Servers: []storage.Server{{Hostname: "node-fail", Role: "node"}},
	})
	c.Assert(err, check.NotNil)
	s.assertClusterState(c, ops.SiteStateExpanding)
}

// Makes sure cannot expand shrinking cluster
func (s *OperationGroupSuite) TestFailsToExpandShrinkingCluster(c *check.C) {
	group := s.operator.getOperationGroup(s.cluster.Key())

	// initiate and finalize the install operation
	key, err := group.createSiteOperation(ops.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationInstall,
		State:      ops.OperationStateInstallInitiated,
	})
	c.Assert(err, check.IsNil)
	c.Assert(key, check.NotNil)
	s.assertClusterState(c, ops.SiteStateInstalling)

	_, err = group.compareAndSwapOperationState(context.TODO(), swap{
		key:            *key,
		expectedStates: []string{ops.OperationStateInstallInitiated},
		newOpState:     ops.OperationStateCompleted,
	})
	c.Assert(err, check.IsNil)
	s.assertClusterState(c, ops.SiteStateActive)

	// create shrink operation
	_, err = group.createSiteOperation(ops.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationShrink,
		State:      ops.OperationStateShrinkInProgress,
	})
	c.Assert(err, check.IsNil)
	s.assertClusterState(c, ops.SiteStateShrinking)

	// expand creation should fail
	_, err = group.createSiteOperation(ops.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationExpand,
		State:      ops.OperationStateExpandInitiated,
	})
	c.Assert(err, check.NotNil)
	s.assertClusterState(c, ops.SiteStateShrinking)
}

// Makes sure cluster stays in proper state after expand operation finishes
func (s *OperationGroupSuite) TestMultiExpandClusterState(c *check.C) {
	group := s.operator.getOperationGroup(s.cluster.Key())

	// initiate and finalize the install operation
	key, err := group.createSiteOperation(ops.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationInstall,
		State:      ops.OperationStateInstallInitiated,
	})
	c.Assert(err, check.IsNil)
	c.Assert(key, check.NotNil)
	s.assertClusterState(c, ops.SiteStateInstalling)

	_, err = group.compareAndSwapOperationState(context.TODO(), swap{
		key:            *key,
		expectedStates: []string{ops.OperationStateInstallInitiated},
		newOpState:     ops.OperationStateCompleted,
	})
	c.Assert(err, check.IsNil)
	s.assertClusterState(c, ops.SiteStateActive)

	// make sure we have enough servers so simultaneous expands are allowed
	var servers []storage.Server
	for i := 0; i < 3; i++ {
		servers = append(servers, storage.Server{Hostname: fmt.Sprintf("node-%v", i)})
	}
	err = group.addClusterStateServers(servers)
	c.Assert(err, check.IsNil)

	// create two expand operations
	keys := make([]*ops.SiteOperationKey, 2)
	for i := 0; i < 2; i++ {
		keys[i], err = group.createSiteOperation(ops.SiteOperation{
			AccountID:  s.cluster.AccountID,
			SiteDomain: s.cluster.Domain,
			Type:       ops.OperationExpand,
			State:      ops.OperationStateExpandInitiated,
			InstallExpand: &storage.InstallExpandOperationState{
				Profiles: map[string]storage.ServerProfile{
					"node": storage.ServerProfile{
						ServiceRole: string(schema.ServiceRoleNode),
					},
				},
			},
			Servers: []storage.Server{{Hostname: fmt.Sprintf("node-%v", i), Role: "node"}},
		})
		c.Assert(err, check.IsNil)
		s.assertClusterState(c, ops.SiteStateExpanding)
	}

	// finish one of expands and make sure the cluster is still expanding
	_, err = group.compareAndSwapOperationState(context.TODO(), swap{
		key:            *keys[0],
		expectedStates: []string{ops.OperationStateExpandInitiated},
		newOpState:     ops.OperationStateCompleted,
	})
	c.Assert(err, check.IsNil)
	s.assertClusterState(c, ops.SiteStateExpanding)

	// finish the second one and make sure the cluster became active
	_, err = group.compareAndSwapOperationState(context.TODO(), swap{
		key:            *keys[1],
		expectedStates: []string{ops.OperationStateExpandInitiated},
		newOpState:     ops.OperationStateCompleted,
	})
	c.Assert(err, check.IsNil)
	s.assertClusterState(c, ops.SiteStateActive)
}

// Makes sure operations that modify cluster state servers behave correctly
func (s *OperationGroupSuite) TestClusterStateModifications(c *check.C) {
	group := s.operator.getOperationGroup(s.cluster.Key())

	err := group.addClusterStateServers([]storage.Server{{Hostname: "node-1"}, {Hostname: "node-2"}})
	c.Assert(err, check.IsNil)
	s.assertServerCount(c, 2)

	err = group.addClusterStateServers([]storage.Server{{Hostname: "node-2"}})
	c.Assert(err, check.NotNil)
	s.assertServerCount(c, 2)

	err = group.addClusterStateServers([]storage.Server{{Hostname: "node-3"}, {Hostname: "node-3"}})
	c.Assert(err, check.NotNil)
	s.assertServerCount(c, 2)

	err = group.addClusterStateServers([]storage.Server{{Hostname: "node-3"}})
	c.Assert(err, check.IsNil)
	s.assertServerCount(c, 3)

	err = group.removeClusterStateServers([]string{"node-2"})
	c.Assert(err, check.IsNil)
	s.assertServerCount(c, 2)

	err = group.removeClusterStateServers([]string{"node-2"})
	c.Assert(err, check.IsNil)
	s.assertServerCount(c, 2)
}

func (s *OperationGroupSuite) assertClusterState(c *check.C, state string) {
	cluster, err := s.operator.GetSite(s.cluster.Key())
	c.Assert(err, check.IsNil)
	c.Assert(cluster.State, check.Equals, state)
}

func (s *OperationGroupSuite) assertServerCount(c *check.C, count int) {
	cluster, err := s.operator.GetSite(s.cluster.Key())
	c.Assert(err, check.IsNil)
	c.Assert(len(cluster.ClusterState.Servers), check.Equals, count)
}
