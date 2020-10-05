/*
Copyright 2020 Gravitational, Inc.

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

package reconfigure

import (
	"context"
	"fmt"
	"testing"

	"github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/install"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/install/reconfigure/phases"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

func TestReconfigurator(t *testing.T) { check.TestingT(t) }

type ReconfiguratorSuite struct {
	suite        install.PlanSuite
	operationKey *ops.SiteOperationKey
}

var _ = check.Suite(&ReconfiguratorSuite{})

func (s *ReconfiguratorSuite) SetUpSuite(c *check.C) {
	s.suite.SetUpSuite(c)
	var err error
	s.operationKey, err = s.suite.Services().Operator.CreateClusterReconfigureOperation(
		context.TODO(), ops.CreateClusterReconfigureOperationRequest{
			SiteKey:       s.suite.Cluster().Key(),
			AdvertiseAddr: "192.168.1.2",
			Servers:       []storage.Server{s.suite.Master()},
			InstallExpand: s.suite.Operation(c).InstallExpand,
		})
	c.Assert(err, check.IsNil)
}

func (s *ReconfiguratorSuite) TestPlan(c *check.C) {
	operation, err := s.suite.Services().Operator.GetSiteOperation(*s.operationKey)
	c.Assert(err, check.IsNil)

	cluster := s.suite.Cluster()
	planner := NewPlanner(s.suite.PlanBuilderGetter(), ops.ConvertOpsSite(*cluster), test.NodeProfile)
	plan, err := planner.GetOperationPlan(s.suite.Services().Operator, *cluster, *operation)
	c.Assert(err, check.IsNil)

	expected := []struct {
		phaseID  string
		verifier func(*check.C, storage.OperationPhase)
	}{
		{phases.NetworkPhase, s.verifyNetworkPhase},
		{phases.LocalPackagesPhase, s.verifyLocalPackagesPhase},
		{installphases.ChecksPhase, s.verifyChecksPhase},
		{installphases.ConfigurePhase, s.verifyConfigurePhase},
		{installphases.PullPhase, s.verifyPullPhase},
		{installphases.MastersPhase, s.suite.VerifyMastersPhase},
		{installphases.WaitPhase, s.verifyWaitPhase},
		{installphases.HealthPhase, s.verifyHealthPhase},
		{phases.EtcdPhase, s.verifyEtcdPhase},
		{phases.StatePhase, s.verifyStatePhase},
		{phases.TokensPhase, s.verifyTokensPhase},
		{installphases.CorednsPhase, s.verifyCorednsPhase},
		{phases.NodePhase, s.verifyNodePhase},
		{phases.DirectoriesPhase, s.verifyDirectoriesPhase},
		{phases.PodsPhase, s.verifyPodsPhase},
		{phases.RestartPhase, s.verifyRestartPhase},
		{phases.GravityPhase, s.verifyGravityPhase},
		{phases.ClusterPackagesPhase, s.verifyClusterPackagesPhase},
	}

	c.Assert(len(expected), check.Equals, len(plan.Phases))
	for i, phase := range plan.Phases {
		c.Assert(phase.ID, check.Equals, expected[i].phaseID, check.Commentf(
			"expected phase number %v to be %v but got %v", i, expected[i].phaseID, phase.ID))
		expected[i].verifier(c, phase)
	}
}

func (s *ReconfiguratorSuite) verifyNetworkPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.NetworkPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyLocalPackagesPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.LocalPackagesPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyEtcdPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.EtcdPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyStatePhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.StatePhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyTokensPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.TokensPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyCorednsPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.CorednsPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
		Requires: []string{installphases.WaitPhase},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyNodePhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.NodePhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyDirectoriesPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.DirectoriesPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyPodsPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.PodsPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyRestartPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.RestartPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v%v", phases.RestartPhase, phases.TeleportPhase),
				Data: &storage.OperationPhaseData{
					Server:  &master,
					Package: &loc.Teleport,
				},
			},
			{
				ID: fmt.Sprintf("%v%v", phases.RestartPhase, phases.PlanetPhase),
				Data: &storage.OperationPhaseData{
					Server:  &master,
					Package: &test.PlanetPackage,
				},
			},
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyGravityPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.GravityPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyClusterPackagesPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.ClusterPackagesPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyChecksPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.ChecksPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyConfigurePhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.ConfigurePhase,
		Data: &storage.OperationPhaseData{
			Install: &storage.InstallOperationData{},
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyPullPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.PullPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", installphases.PullPhase, master.Hostname),
				Data: &storage.OperationPhaseData{
					Server:      &master,
					ExecServer:  &master,
					Package:     s.suite.Package(),
					ServiceUser: &s.suite.Cluster().ServiceUser,
					Pull: &storage.PullData{
						Apps: []loc.Locator{
							*(s.suite.Package()),
						},
					},
				},
				Requires: []string{installphases.ConfigurePhase},
			},
		},
		Requires: []string{installphases.ConfigurePhase},
		Parallel: true,
	}, phase)
}

func (s *ReconfiguratorSuite) verifyWaitPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.WaitPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
		Requires: []string{installphases.MastersPhase},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyHealthPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: installphases.HealthPhase,
		Data: &storage.OperationPhaseData{
			Server: &master,
		},
	}, phase)
}
