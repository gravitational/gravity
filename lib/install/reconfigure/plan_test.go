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

	"github.com/gravitational/gravity/lib/install"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/install/reconfigure/phases"
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
	planner := NewPlanner(s.suite.PlanBuilderGetter(), ops.ConvertOpsSite(*cluster))
	plan, err := planner.GetOperationPlan(s.suite.Services().Operator, *cluster, *operation)
	c.Assert(err, check.IsNil)

	expected := []struct {
		phaseID  string
		verifier func(*check.C, storage.OperationPhase)
	}{
		{phases.PreCleanupPhase, s.verifyPreCleanupPhase},
		{installphases.ChecksPhase, s.verifyChecksPhase},
		{installphases.ConfigurePhase, s.verifyConfigurePhase},
		{installphases.PullPhase, s.verifyPullPhase},
		{installphases.MastersPhase, s.suite.VerifyMastersPhase},
		{installphases.WaitPhase, s.verifyWaitPhase},
		{installphases.HealthPhase, s.verifyHealthPhase},
		{phases.PostCleanupPhase, s.verifyPostCleanupPhase},
	}

	c.Assert(len(expected), check.Equals, len(plan.Phases))
	for i, phase := range plan.Phases {
		c.Assert(phase.ID, check.Equals, expected[i].phaseID, check.Commentf(
			"expected phase number %v to be %v but got %v", i, expected[i].phaseID, phase.ID))
		expected[i].verifier(c, phase)
	}
}

func (s *ReconfiguratorSuite) verifyPreCleanupPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.PreCleanupPhase,
		Phases: []storage.OperationPhase{
			{
				ID:   fmt.Sprintf("%v%v", phases.PreCleanupPhase, phases.NetworkPhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
			{
				ID:   fmt.Sprintf("%v%v", phases.PreCleanupPhase, phases.PackagesPhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
			{
				ID:   fmt.Sprintf("%v%v", phases.PreCleanupPhase, phases.DirectoriesPhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
		},
	}, phase)
}

func (s *ReconfiguratorSuite) verifyPostCleanupPhase(c *check.C, phase storage.OperationPhase) {
	master := s.suite.Master()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID:       phases.PostCleanupPhase,
		Requires: []string{installphases.HealthPhase},
		Phases: []storage.OperationPhase{
			{
				ID:   fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.StatePhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
			{
				ID:   fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.TokensPhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
			{
				ID:   fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.NodePhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
			{
				ID:   fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.PodsPhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
			{
				ID:   fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.GravityPhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
			{
				ID:   fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.PackagesPhase),
				Data: &storage.OperationPhaseData{Server: &master},
			},
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
