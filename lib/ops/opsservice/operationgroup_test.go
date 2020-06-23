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
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"gopkg.in/check.v1"
)

type OperationGroupSuite struct {
	operator *Operator
	backend  storage.Backend
	cluster  *ops.Site
	clock    clockwork.Clock
}

var _ = check.Suite(&OperationGroupSuite{
	clock: clockwork.NewFakeClock(),
})

func (s *OperationGroupSuite) SetUpTest(c *check.C) {
	services := SetupTestServices(c)
	s.operator = services.Operator
	s.backend = services.Backend
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

	_, err = group.compareAndSwapOperationState(swap{
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
					"node": {
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
				"node": {
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

	_, err = group.compareAndSwapOperationState(swap{
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

	_, err = group.compareAndSwapOperationState(swap{
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
					"node": {
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
	_, err = group.compareAndSwapOperationState(swap{
		key:            *keys[0],
		expectedStates: []string{ops.OperationStateExpandInitiated},
		newOpState:     ops.OperationStateCompleted,
	})
	c.Assert(err, check.IsNil)
	s.assertClusterState(c, ops.SiteStateExpanding)

	// finish the second one and make sure the cluster became active
	_, err = group.compareAndSwapOperationState(swap{
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

func (s *OperationGroupSuite) TestCanCreateUpgradeOperation(c *check.C) {
	// Can't create upgrade operation if cluster isn't active.
	s.setClusterState(c, ops.SiteStateUpdating)
	_, err := s.createUpgradeOperation(c, s.clock.Now(), false)
	c.Assert(err, check.NotNil,
		check.Commentf("Shouldn't be able to create upgrade operation for non-active cluster"))

	// First upgrade operation should create fine.
	s.setClusterState(c, ops.SiteStateActive)
	key, err := s.createUpgradeOperation(c, s.clock.Now().Add(time.Second), false)
	c.Assert(err, check.IsNil,
		check.Commentf("Should be able to create first upgrade operation"))

	// Reset the cluster state but the first operation is still in progress.
	s.setClusterState(c, ops.SiteStateActive)
	_, err = s.createUpgradeOperation(c, s.clock.Now().Add(2*time.Second), false)
	c.Assert(err, check.NotNil,
		check.Commentf("Shouldn't be able to create upgrade operation if another one in progress"))

	// Simulate failed operation and force-reset cluster state.
	s.setClusterState(c, ops.SiteStateActive)
	s.setOperationState(c, *key, ops.OperationStateFailed)
	_, err = s.backend.CreateOperationPlanChange(storage.PlanChange{
		ID:          uuid.New(),
		ClusterName: s.cluster.Domain,
		OperationID: key.OperationID,
		PhaseID:     "/init",
		NewState:    storage.OperationPhaseStateFailed,
		Created:     s.clock.Now(),
	})
	c.Assert(err, check.IsNil)
	_, err = s.createUpgradeOperation(c, s.clock.Now().Add(3*time.Second), false)
	c.Assert(err, check.NotNil,
		check.Commentf("Shouldn't be able to create upgrade operation if last failed upgrade plan isn't rolled back"))

	// Rollback the failed operation properly, should be able to create a new one then.
	s.setOperationState(c, *key, ops.OperationStateFailed)
	_, err = s.backend.CreateOperationPlanChange(storage.PlanChange{
		ID:          uuid.New(),
		ClusterName: s.cluster.Domain,
		OperationID: key.OperationID,
		PhaseID:     "/init",
		NewState:    storage.OperationPhaseStateRolledBack,
		Created:     s.clock.Now().Add(time.Second),
	})
	c.Assert(err, check.IsNil)
	_, err = s.createUpgradeOperation(c, s.clock.Now().Add(4*time.Second), false)
	c.Assert(err, check.IsNil,
		check.Commentf("Should be able to create upgrade operation if last upgrade plan is fully rolled back"))

	// Force flag should allow to create operation.
	s.setClusterState(c, ops.SiteStateActive)
	_, err = s.createUpgradeOperation(c, s.clock.Now().Add(5*time.Second), true)
	c.Assert(err, check.IsNil,
		check.Commentf("Should be able to create upgrade operation in force mode"))
}

func (s *OperationGroupSuite) setClusterState(c *check.C, state string) {
	cluster, err := s.backend.GetSite(s.cluster.Domain)
	c.Assert(err, check.IsNil)
	cluster.State = state
	_, err = s.backend.UpdateSite(*cluster)
	c.Assert(err, check.IsNil)
}

func (s *OperationGroupSuite) setOperationState(c *check.C, key ops.SiteOperationKey, state string) {
	op, err := s.backend.GetSiteOperation(key.SiteDomain, key.OperationID)
	c.Assert(err, check.IsNil)
	op.State = state
	_, err = s.backend.UpdateSiteOperation(*op)
	c.Assert(err, check.IsNil)
}

func (s *OperationGroupSuite) createUpgradeOperation(c *check.C, created time.Time, force bool) (*ops.SiteOperationKey, error) {
	group := s.operator.getOperationGroup(s.cluster.Key())
	key, err := group.createSiteOperationWithOptions(ops.SiteOperation{
		AccountID:  s.cluster.AccountID,
		SiteDomain: s.cluster.Domain,
		Type:       ops.OperationUpdate,
		State:      ops.OperationStateUpdateInProgress,
		Created:    created,
	}, createOperationOptions{force: force})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = s.backend.CreateOperationPlan(storage.OperationPlan{
		OperationID:   key.OperationID,
		OperationType: ops.OperationUpdate,
		AccountID:     s.cluster.AccountID,
		ClusterName:   s.cluster.Domain,
		Phases: []storage.OperationPhase{
			{
				ID:    "/init",
				State: storage.OperationPhaseStateUnstarted,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
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
