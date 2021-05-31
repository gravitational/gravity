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
	"context"
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

func TestFSM(t *testing.T) { check.TestingT(t) }

type FSMSuite struct {
	planner *testPlanner
	clock   clockwork.Clock
}

var _ = check.Suite(&FSMSuite{
	planner: &testPlanner{},
	clock:   clockwork.NewFakeClock(),
})

// TestExecutePlan executes an unstarted plan and makes sure all phases have
// been executed in correct order.
func (s *FSMSuite) TestExecutePlan(c *check.C) {
	engine := newTestEngine(func() storage.OperationPlan {
		return *(s.planner.newPlan(
			s.planner.initPhase(storage.OperationPhaseStateUnstarted),
			s.planner.bootstrapPhase(
				s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateUnstarted),
				s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateUnstarted)),
			s.planner.upgradePhase(storage.OperationPhaseStateUnstarted)))
	})

	fsm, err := New(Config{Engine: engine})
	c.Assert(err, check.IsNil)

	err = fsm.ExecutePlan(context.TODO(), utils.DiscardProgress)
	c.Assert(err, check.IsNil)

	plan, err := fsm.GetPlan()
	c.Assert(err, check.IsNil)
	// Make sure plan is completed now.
	c.Assert(IsCompleted(plan), check.Equals, true)
	// Make sure phases were executed in correct order.
	s.checkChangelog(c, engine.changelog, s.planner.newChangelog(
		s.planner.initChange(storage.OperationPhaseStateInProgress),
		s.planner.initChange(storage.OperationPhaseStateCompleted),
		s.planner.bootstrapSubChange("node-1", storage.OperationPhaseStateInProgress),
		s.planner.bootstrapSubChange("node-1", storage.OperationPhaseStateCompleted),
		s.planner.bootstrapSubChange("node-2", storage.OperationPhaseStateInProgress),
		s.planner.bootstrapSubChange("node-2", storage.OperationPhaseStateCompleted),
		s.planner.upgradeChange(storage.OperationPhaseStateInProgress),
		s.planner.upgradeChange(storage.OperationPhaseStateCompleted),
	))
}

// TestRollbackPlan rolls back a failed plan and makes sure all phases have been
// rolled back in correct order.
func (s *FSMSuite) TestRollbackPlan(c *check.C) {
	engine := newTestEngine(func() storage.OperationPlan {
		return *(s.planner.newPlan(
			s.planner.initPhase(storage.OperationPhaseStateCompleted),
			s.planner.bootstrapPhase(
				s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateCompleted),
				s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateCompleted)),
			s.planner.upgradePhase(storage.OperationPhaseStateFailed)))
	})

	fsm, err := New(Config{Engine: engine})
	c.Assert(err, check.IsNil)

	err = fsm.RollbackPlan(context.TODO(), utils.DiscardProgress, false)
	c.Assert(err, check.IsNil)

	plan, err := fsm.GetPlan()
	c.Assert(err, check.IsNil)
	// Make sure plan is rolled back now.
	c.Assert(IsRolledBack(plan), check.Equals, true)
	// Make sure phases were rolled back in correct order.
	s.checkChangelog(c, engine.changelog, s.planner.newChangelog(
		s.planner.upgradeChange(storage.OperationPhaseStateInProgress),
		s.planner.upgradeChange(storage.OperationPhaseStateRolledBack),
		s.planner.bootstrapSubChange("node-2", storage.OperationPhaseStateInProgress),
		s.planner.bootstrapSubChange("node-2", storage.OperationPhaseStateRolledBack),
		s.planner.bootstrapSubChange("node-1", storage.OperationPhaseStateInProgress),
		s.planner.bootstrapSubChange("node-1", storage.OperationPhaseStateRolledBack),
		s.planner.initChange(storage.OperationPhaseStateInProgress),
		s.planner.initChange(storage.OperationPhaseStateRolledBack),
	))
}

// TestRollbackPlanSkip rolls back a plan with some rolled back / unstarted phases
// and makes sure such phases are being skipped during rollback.
func (s *FSMSuite) TestRollbackPlanSkip(c *check.C) {
	engine := newTestEngine(func() storage.OperationPlan {
		return *(s.planner.newPlan(
			s.planner.initPhase(storage.OperationPhaseStateCompleted),
			s.planner.bootstrapPhase(
				s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateRolledBack),
				s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateCompleted)),
			s.planner.upgradePhase(storage.OperationPhaseStateUnstarted)))
	})

	fsm, err := New(Config{Engine: engine})
	c.Assert(err, check.IsNil)

	err = fsm.RollbackPlan(context.TODO(), utils.DiscardProgress, false)
	c.Assert(err, check.IsNil)

	plan, err := fsm.GetPlan()
	c.Assert(err, check.IsNil)
	// Make sure plan is rolled back now.
	c.Assert(IsRolledBack(plan), check.Equals, true)
	// Make sure unstarted/rolled back phases were skipped over.
	s.checkChangelog(c, engine.changelog, s.planner.newChangelog(
		s.planner.bootstrapSubChange("node-2", storage.OperationPhaseStateInProgress),
		s.planner.bootstrapSubChange("node-2", storage.OperationPhaseStateRolledBack),
		s.planner.initChange(storage.OperationPhaseStateInProgress),
		s.planner.initChange(storage.OperationPhaseStateRolledBack),
	))
}

// TestRollbackPlanDryRun make sure that rollback in dry-run mode does not
// rollback any of the phases.
func (s *FSMSuite) TestRollbackPlanDryRun(c *check.C) {
	engine := newTestEngine(func() storage.OperationPlan {
		return *(s.planner.newPlan(
			s.planner.initPhase(storage.OperationPhaseStateCompleted),
			s.planner.bootstrapPhase(
				s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateCompleted),
				s.planner.bootstrapSubPhase("node-2", storage.OperationPhaseStateCompleted)),
			s.planner.upgradePhase(storage.OperationPhaseStateUnstarted)))
	})

	fsm, err := New(Config{Engine: engine})
	c.Assert(err, check.IsNil)

	err = fsm.RollbackPlan(context.TODO(), utils.DiscardProgress, true)
	c.Assert(err, check.IsNil)

	plan, err := fsm.GetPlan()
	c.Assert(err, check.IsNil)
	// Make sure plan is still not rolled back.
	c.Assert(IsRolledBack(plan), check.Equals, false)
	// Make sure changelog is empty.
	s.checkChangelog(c, engine.changelog, s.planner.newChangelog())
}

func (s *FSMSuite) checkChangelog(c *check.C, actual, expected storage.PlanChangelog) {
	c.Assert(len(actual), check.Equals, len(expected))
	for i := 0; i < len(actual); i++ {
		actual[i].Created = expected[i].Created // Do not compare timestamps.
		compare.DeepCompare(c, actual[i], expected[i])
	}
}
