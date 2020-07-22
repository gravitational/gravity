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

package fsm

import (
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

type FSMUtilsSuite struct {
	planner *testPlanner
}

var _ = check.Suite(&FSMUtilsSuite{
	planner: &testPlanner{},
})

func (s *FSMUtilsSuite) TestIsCompleted(c *check.C) {
	plan := s.planner.newPlan(
		s.planner.initPhase(storage.OperationPhaseStateCompleted),
		s.planner.bootstrapPhase(
			s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateCompleted),
			s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateCompleted)),
		s.planner.upgradePhase(storage.OperationPhaseStateCompleted))
	c.Assert(IsCompleted(plan), check.Equals, true)
}

func (s *FSMUtilsSuite) TestIsRolledBack(c *check.C) {
	plan := s.planner.newPlan(
		s.planner.initPhase(storage.OperationPhaseStateRolledBack),
		s.planner.bootstrapPhase(
			s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateRolledBack),
			s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateUnstarted)),
		s.planner.upgradePhase(storage.OperationPhaseStateUnstarted))
	c.Assert(IsRolledBack(plan), check.Equals, true)
}

func (s *FSMUtilsSuite) TestGetPlanProgress(c *check.C) {
	plan := s.planner.newPlan(
		s.planner.initPhase(storage.OperationPhaseStateUnstarted),
		s.planner.bootstrapPhase(
			s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateUnstarted)),
		s.planner.upgradePhase(storage.OperationPhaseStateUnstarted))
	c.Assert(GetPlanProgress(*plan), compare.DeepEquals, []storage.PlanChange(nil))

	plan = s.planner.newPlan(
		s.planner.initPhase(storage.OperationPhaseStateCompleted),
		s.planner.bootstrapPhase(
			s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateCompleted),
			s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateFailed)),
		s.planner.upgradePhase(storage.OperationPhaseStateUnstarted))
	c.Assert(GetPlanProgress(*plan), compare.DeepEquals, []storage.PlanChange{
		{
			PhaseID:    "/init",
			PhaseIndex: 0,
			NewState:   storage.OperationPhaseStateCompleted,
		},
		{
			PhaseID:    "/bootstrap/node-1",
			PhaseIndex: 1,
			NewState:   storage.OperationPhaseStateCompleted,
		},
		{
			PhaseID:    "/bootstrap/node-2",
			PhaseIndex: 2,
			NewState:   storage.OperationPhaseStateFailed,
		},
	})
}

func (s *FSMUtilsSuite) TestDiffPlan(c *check.C) {
	prevPlan := s.planner.newPlan(
		s.planner.initPhase(storage.OperationPhaseStateUnstarted),
		s.planner.bootstrapPhase(
			s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateUnstarted),
			s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateUnstarted)),
		s.planner.upgradePhase(storage.OperationPhaseStateUnstarted))

	nextPlan := s.planner.newPlan(
		s.planner.initPhase(storage.OperationPhaseStateCompleted),
		s.planner.bootstrapPhase(
			s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateCompleted),
			s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateFailed)),
		s.planner.upgradePhase(storage.OperationPhaseStateUnstarted))

	diff, err := DiffPlan(prevPlan, *nextPlan)
	c.Assert(err, check.IsNil)
	c.Assert(diff, compare.DeepEquals, []storage.PlanChange{
		{
			PhaseID:    "/init",
			PhaseIndex: 0,
			NewState:   storage.OperationPhaseStateCompleted,
		},
		{
			PhaseID:    "/bootstrap/node-1",
			PhaseIndex: 1,
			NewState:   storage.OperationPhaseStateCompleted,
		},
		{
			PhaseID:    "/bootstrap/node-2",
			PhaseIndex: 2,
			NewState:   storage.OperationPhaseStateFailed,
		},
	})
}

func (s *FSMUtilsSuite) TestDiffPlanNoPrevious(c *check.C) {
	nextPlan := s.planner.newPlan(
		s.planner.initPhase(storage.OperationPhaseStateCompleted),
		s.planner.bootstrapPhase(
			s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateCompleted),
			s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateFailed)),
		s.planner.upgradePhase(storage.OperationPhaseStateUnstarted))

	diff, err := DiffPlan(nil, *nextPlan)
	c.Assert(err, check.IsNil)
	c.Assert(diff, compare.DeepEquals, []storage.PlanChange{
		{
			PhaseID:    "/init",
			PhaseIndex: 0,
			NewState:   storage.OperationPhaseStateCompleted,
		},
		{
			PhaseID:    "/bootstrap/node-1",
			PhaseIndex: 1,
			NewState:   storage.OperationPhaseStateCompleted,
		},
		{
			PhaseID:    "/bootstrap/node-2",
			PhaseIndex: 2,
			NewState:   storage.OperationPhaseStateFailed,
		},
	})
}
