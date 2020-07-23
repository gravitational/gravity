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

package expand

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	check "gopkg.in/check.v1"
)

func TestJoin(t *testing.T) { check.TestingT(t) }

type PlanSuite struct {
	services        opsservice.TestServices
	peer            *Peer
	clusterNodes    storage.Servers
	masterNode      storage.Server
	joiningNode     storage.Server
	adminAgent      *storage.LoginEntry
	regularAgent    *storage.LoginEntry
	teleportPackage *loc.Locator
	planetPackage   *loc.Locator
	gravityPackage  *loc.Locator
	appPackage      loc.Locator
	serviceUser     storage.OSUser
	cluster         *ops.Site
	installOpKey    *ops.SiteOperationKey
	installOp       *ops.SiteOperation
	joinOpKey       *ops.SiteOperationKey
	joinOp          *ops.SiteOperation
	dnsConfig       storage.DNSConfig
}

var _ = check.Suite(&PlanSuite{})

func (s *PlanSuite) SetUpSuite(c *check.C) {
	s.services = opsservice.SetupTestServices(c)
	account, err := s.services.Operator.CreateAccount(
		ops.NewAccountRequest{
			ID:  defaults.SystemAccountID,
			Org: defaults.SystemAccountOrg,
		})
	c.Assert(err, check.IsNil)
	s.appPackage = suite.SetUpTestPackage(c, s.services.Apps,
		s.services.Packages)
	app, err := s.services.Apps.GetApp(s.appPackage)
	c.Assert(err, check.IsNil)
	s.teleportPackage, err = app.Manifest.Dependencies.ByName(constants.TeleportPackage)
	c.Assert(err, check.IsNil)
	s.gravityPackage, err = app.Manifest.Dependencies.ByName(constants.GravityPackage)
	c.Assert(err, check.IsNil)
	s.dnsConfig = storage.DNSConfig{
		Addrs: []string{"127.0.0.3"},
		Port:  10053,
	}
	s.cluster, err = s.services.Operator.CreateSite(
		ops.NewSiteRequest{
			AccountID:  account.ID,
			DomainName: "example.com",
			AppPackage: s.appPackage.String(),
			Provider:   schema.ProviderAWS,
			DNSConfig:  s.dnsConfig,
		})
	c.Assert(err, check.IsNil)
	_, err = s.services.Users.CreateClusterAdminAgent(s.cluster.Domain,
		storage.NewUser(storage.ClusterAdminAgent(s.cluster.Domain), storage.UserSpecV2{
			AccountID: defaults.SystemAccountID,
		}))
	c.Assert(err, check.IsNil)
	s.masterNode = storage.Server{
		AdvertiseIP: "10.10.0.1",
		Hostname:    "node-1",
		Role:        "node",
		ClusterRole: string(schema.ServiceRoleMaster),
	}
	s.cluster.ClusterState = storage.ClusterState{
		Servers: []storage.Server{s.masterNode},
	}
	s.clusterNodes = storage.Servers(s.cluster.ClusterState.Servers)
	s.joiningNode = storage.Server{
		AdvertiseIP: "10.10.0.2",
		Hostname:    "node-2",
		Role:        "node",
		ClusterRole: string(schema.ServiceRoleMaster),
	}
	s.planetPackage, err = app.Manifest.RuntimePackageForProfile(s.joiningNode.Role)
	c.Assert(err, check.IsNil)
	s.adminAgent, err = s.services.Operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   account.ID,
		ClusterName: s.cluster.Domain,
		Admin:       true,
	})
	c.Assert(err, check.IsNil)
	s.regularAgent, err = s.services.Operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   account.ID,
		ClusterName: s.cluster.Domain,
	})
	c.Assert(err, check.IsNil)
	s.installOpKey, err = s.services.Operator.CreateSiteInstallOperation(context.TODO(),
		ops.CreateSiteInstallOperationRequest{
			AccountID:   account.ID,
			SiteDomain:  s.cluster.Domain,
			Provisioner: schema.ProvisionerAWSTerraform,
		})
	c.Assert(err, check.IsNil)
	s.installOp, err = s.services.Operator.GetSiteOperation(*s.installOpKey)
	c.Assert(err, check.IsNil)
	err = s.services.Operator.UpdateInstallOperationState(
		*s.installOpKey, ops.OperationUpdateRequest{
			Profiles: map[string]storage.ServerProfileRequest{
				s.masterNode.Role: {Count: 1}},
			Servers: []storage.Server{s.masterNode},
		})
	c.Assert(err, check.IsNil)
	err = s.services.Operator.SetOperationState(*s.installOpKey,
		ops.SetOperationStateRequest{
			State: ops.OperationStateCompleted,
			Progress: &ops.ProgressEntry{
				SiteDomain:  s.cluster.Domain,
				OperationID: s.installOpKey.OperationID,
				State:       ops.ProgressStateCompleted,
				Completion:  constants.Completed,
				Created:     time.Now(),
			},
		})
	c.Assert(err, check.IsNil)
	s.joinOpKey, err = s.services.Operator.CreateSiteExpandOperation(context.TODO(),
		ops.CreateSiteExpandOperationRequest{
			AccountID:   account.ID,
			SiteDomain:  s.cluster.Domain,
			Servers:     map[string]int{s.joiningNode.Role: 1},
			Provisioner: schema.ProvisionerAWSTerraform,
		})
	c.Assert(err, check.IsNil)
	s.joinOp, err = s.services.Operator.GetSiteOperation(*s.joinOpKey)
	c.Assert(err, check.IsNil)
	err = s.services.Operator.UpdateExpandOperationState(
		*s.joinOpKey, ops.OperationUpdateRequest{
			Profiles: map[string]storage.ServerProfileRequest{
				s.joiningNode.Role: {Count: 1}},
			Servers: []storage.Server{s.joiningNode},
		})
	c.Assert(err, check.IsNil)
	s.serviceUser = storage.OSUser{
		Name: defaults.ServiceUser,
		UID:  "999",
		GID:  "999",
	}
	s.cluster.ServiceUser = s.serviceUser
	s.peer = &Peer{
		PeerConfig: PeerConfig{
			FieldLogger: logrus.WithField(trace.Component, "join-suite"),
			RuntimeConfig: proto.RuntimeConfig{
				Role: s.joiningNode.Role,
			},
		},
	}
}

func (s *PlanSuite) TestPlan(c *check.C) {
	err := s.peer.initOperationPlan(operationContext{
		Operator:  s.services.Operator,
		Packages:  s.services.Packages,
		Apps:      s.services.Apps,
		Peer:      fmt.Sprintf("%v:%v", s.masterNode.AdvertiseIP, defaults.GravitySiteNodePort),
		Operation: *s.joinOp,
		Cluster:   *s.cluster,
	})
	c.Assert(err, check.IsNil)

	plan, err := s.services.Operator.GetOperationPlan(*s.joinOpKey)
	c.Assert(err, check.IsNil)

	expected := []struct {
		phaseID       string
		phaseVerifier func(*check.C, storage.OperationPhase)
	}{
		{installphases.InitPhase, s.verifyInitPhase},
		{StartAgentPhase, s.verifyStartAgentPhase},
		{ChecksPhase, s.verifyChecksPhase},
		{installphases.ConfigurePhase, s.verifyConfigurePhase},
		{installphases.BootstrapPhase, s.verifyBootstrapPhase},
		{installphases.PullPhase, s.verifyPullPhase},
		{PreHookPhase, s.verifyPreHookPhase},
		{SystemPhase, s.verifySystemPhase},
		{EtcdBackupPhase, s.verifyEtcdBackupPhase},
		{EtcdPhase, s.verifyEtcdPhase},
		{installphases.WaitPhase, s.verifyWaitPhase},
		{StopAgentPhase, s.verifyStopAgentPhase},
		{PostHookPhase, s.verifyPostHookPhase},
		{ElectPhase, s.verifyElectPhase},
	}

	c.Assert(len(expected), check.Equals, len(plan.Phases))

	for i, phase := range plan.Phases {
		c.Assert(phase.ID, check.Equals, expected[i].phaseID, check.Commentf(
			"expected phase number %v to be %v but got %v", i, expected[i].phaseID, phase.ID))
		expected[i].phaseVerifier(c, phase)
	}
}

func (s *PlanSuite) verifyInitPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.InitPhase,
		Data: &storage.OperationPhaseData{
			Server:  &s.joiningNode,
			Master:  &s.masterNode,
			Package: &s.appPackage,
		},
	}, phase)
}

func (s *PlanSuite) verifyChecksPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: ChecksPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.joiningNode,
			Master: &s.masterNode,
		},
		Requires: []string{installphases.InitPhase, StartAgentPhase},
	}, phase)
}

func (s *PlanSuite) verifyConfigurePhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.ConfigurePhase,
		Data: &storage.OperationPhaseData{
			ExecServer: &s.joiningNode,
		},
		Requires: []string{ChecksPhase},
	}, phase)
}

func (s *PlanSuite) verifyBootstrapPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.BootstrapPhase,
		Data: &storage.OperationPhaseData{
			Server:      &s.joiningNode,
			ExecServer:  &s.joiningNode,
			Package:     &s.appPackage,
			Agent:       s.adminAgent,
			ServiceUser: &s.serviceUser,
		},
	}, phase)
}

func (s *PlanSuite) verifyPullPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.PullPhase,
		Data: &storage.OperationPhaseData{
			Server:      &s.joiningNode,
			ExecServer:  &s.joiningNode,
			Package:     &s.appPackage,
			ServiceUser: &s.serviceUser,
			Pull: &storage.PullData{
				Packages: []loc.Locator{
					*s.gravityPackage,
					*s.teleportPackage,
					*s.planetPackage,
				},
			},
		},
		Requires: []string{installphases.ConfigurePhase, installphases.BootstrapPhase},
	}, phase)
}

func (s *PlanSuite) verifyPreHookPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: PreHookPhase,
		Data: &storage.OperationPhaseData{
			ExecServer:  &s.joiningNode,
			Package:     &s.appPackage,
			ServiceUser: &s.serviceUser,
		},
		Requires: []string{installphases.PullPhase},
	}, phase)
}

func (s *PlanSuite) verifySystemPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: SystemPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/teleport", SystemPhase),
				Data: &storage.OperationPhaseData{
					Server:      &s.joiningNode,
					ExecServer:  &s.joiningNode,
					ServiceUser: &s.serviceUser,
					Package:     s.teleportPackage,
				},
				Requires: []string{installphases.PullPhase},
			},
			{
				ID: fmt.Sprintf("%v/planet", SystemPhase),
				Data: &storage.OperationPhaseData{
					Server:      &s.joiningNode,
					ExecServer:  &s.joiningNode,
					Package:     s.planetPackage,
					ServiceUser: &s.serviceUser,
					Labels:      pack.RuntimePackageLabels,
				},
				Requires: []string{installphases.PullPhase},
			},
		},
	}, phase)
}

func (s *PlanSuite) verifyStartAgentPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: StartAgentPhase,
		Data: &storage.OperationPhaseData{
			Server:     &s.masterNode,
			ExecServer: &s.joiningNode,
			Agent: &storage.LoginEntry{
				Email:        s.adminAgent.Email,
				Password:     s.adminAgent.Password,
				OpsCenterURL: fmt.Sprintf("https://%v:%v", s.masterNode.AdvertiseIP, defaults.GravitySiteNodePort),
			},
		},
	}, phase)
}

func (s PlanSuite) verifyEtcdBackupPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: EtcdBackupPhase,
		Data: &storage.OperationPhaseData{
			Server:     &s.masterNode,
			ExecServer: &s.joiningNode,
		},
		Requires: []string{StartAgentPhase},
	}, phase)
}

func (s PlanSuite) verifyEtcdPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: EtcdPhase,
		Data: &storage.OperationPhaseData{
			Server:     &s.joiningNode,
			ExecServer: &s.joiningNode,
			Master:     &s.masterNode,
		},
		Requires: []string{SystemPhase, EtcdBackupPhase},
	}, phase)
}

func (s *PlanSuite) verifyWaitPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.WaitPhase,
		Phases: []storage.OperationPhase{
			{
				ID: WaitPlanetPhase,
				Data: &storage.OperationPhaseData{
					Server:     &s.joiningNode,
					ExecServer: &s.joiningNode,
				},
				Requires: []string{SystemPhase, EtcdPhase},
			},
			{
				ID: WaitK8sPhase,
				Data: &storage.OperationPhaseData{
					Server:     &s.joiningNode,
					ExecServer: &s.joiningNode,
				},
				Requires: []string{WaitPlanetPhase},
			},
			{
				ID: WaitTeleportPhase,
				Data: &storage.OperationPhaseData{
					Server:     &s.joiningNode,
					ExecServer: &s.joiningNode,
				},
				Requires: []string{WaitPlanetPhase},
			},
		},
	}, phase)
}

func (s *PlanSuite) verifyStopAgentPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: StopAgentPhase,
		Data: &storage.OperationPhaseData{
			Server:     &s.masterNode,
			ExecServer: &s.joiningNode,
		},
		Requires: []string{installphases.WaitPhase},
	}, phase)
}

func (s *PlanSuite) verifyPostHookPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: PostHookPhase,
		Data: &storage.OperationPhaseData{
			ExecServer:  &s.joiningNode,
			Package:     &s.appPackage,
			ServiceUser: &s.serviceUser,
		},
		Requires: []string{installphases.WaitPhase},
	}, phase)
}

func (s *PlanSuite) verifyElectPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: ElectPhase,
		Data: &storage.OperationPhaseData{
			Server:     &s.joiningNode,
			ExecServer: &s.joiningNode,
		},
		Requires: []string{installphases.WaitPhase},
	}, phase)
}

func (s *PlanSuite) TestFillSteps(c *check.C) {
	tests := []struct {
		phasesCount int
		stepNumbers []int
	}{
		{
			phasesCount: 0,
			stepNumbers: nil,
		},
		{
			phasesCount: 1,
			stepNumbers: []int{10},
		},
		{
			phasesCount: 2,
			stepNumbers: []int{5, 10},
		},
		{
			phasesCount: 13,
			stepNumbers: []int{0, 1, 2, 3, 3, 4, 5, 6, 6, 7, 8, 9, 10},
		},
	}
	for _, t := range tests {
		var plan storage.OperationPlan
		for i := 0; i < t.phasesCount; i++ {
			plan.Phases = append(plan.Phases, storage.OperationPhase{})
		}
		fillSteps(&plan, 10)
		var stepNumbers []int
		for _, p := range plan.Phases {
			stepNumbers = append(stepNumbers, p.Step)
		}
		c.Assert(stepNumbers, check.DeepEquals, t.stepNumbers)
	}
}
