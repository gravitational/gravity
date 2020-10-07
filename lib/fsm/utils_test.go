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

	"github.com/gravitational/trace"
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

func (s *FSMUtilsSuite) TestCanRollback(c *check.C) {
	tests := []struct {
		comment  string
		plan     *storage.OperationPlan
		phaseID  string
		expected string
	}{
		{
			comment: "Rollback latest phase",

			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/init",
						State: storage.OperationPhaseStateCompleted,
					},
					{
						ID:       "/startAgent",
						State:    storage.OperationPhaseStateInProgress,
						Requires: []string{"/init"},
					},
				},
			},
			phaseID: "/startAgent",
		},
		{
			comment: "A subsequent phase is in progress",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/init",
						State: storage.OperationPhaseStateCompleted,
					},
					{
						ID:       "/startAgent",
						State:    storage.OperationPhaseStateInProgress,
						Requires: []string{"/init"},
					},
				},
			},
			phaseID:  "/init",
			expected: rollbackDependentsErrorMsg("/init", []string{"/startAgent"}),
		},
		{
			comment: "All later phases have been rolled back",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/init",
						State: storage.OperationPhaseStateCompleted,
					},
					{
						ID:       "/startAgent",
						State:    storage.OperationPhaseStateRolledBack,
						Requires: []string{"/init"},
					},
				},
			},
			phaseID: "/init",
		},
		{
			comment: "All later phases have been rolled back or are unstarted",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/init",
						State: storage.OperationPhaseStateCompleted,
					},
					{
						ID:       "/startAgent",
						State:    storage.OperationPhaseStateRolledBack,
						Requires: []string{"/init"},
					},
					{
						ID:       "/checks",
						State:    storage.OperationPhaseStateUnstarted,
						Requires: []string{"/startAgent"},
					},
				},
			},
			phaseID: "/init",
		},
		{
			comment: "Rollback after a previously forced rollback",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/init",
						State: storage.OperationPhaseStateCompleted,
					},
					{
						ID:       "/startAgent",
						State:    storage.OperationPhaseStateRolledBack,
						Requires: []string{"/init"},
					},
					{
						ID:       "/checks",
						State:    storage.OperationPhaseStateFailed,
						Requires: []string{"/startAgent"},
					},
				},
			},
			phaseID:  "/init",
			expected: rollbackDependentsErrorMsg("/init", []string{"/checks"}),
		},
		{
			comment: "Rollback after a later phase has been executed out of band",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/init",
						State: storage.OperationPhaseStateCompleted,
					},
					{
						ID:       "/startAgent",
						State:    storage.OperationPhaseStateUnstarted,
						Requires: []string{"/init"},
					},
					{
						ID:       "/checks",
						State:    storage.OperationPhaseStateCompleted,
						Requires: []string{"/startAgent"},
					},
					{
						ID:       "/test",
						State:    storage.OperationPhaseStateRolledBack,
						Requires: []string{"/checks"},
					},
				},
			},
			phaseID:  "/init",
			expected: rollbackDependentsErrorMsg("/init", []string{"/checks"}),
		},
		{
			comment: "Rollback subphase",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/masters",
						State: storage.OperationPhaseStateInProgress,
						Phases: []storage.OperationPhase{
							{
								ID:    "/masters/node-1",
								State: storage.OperationPhaseStateCompleted,
							},
							{
								ID:       "/masters/node-2",
								State:    storage.OperationPhaseStateInProgress,
								Requires: []string{"/masters/node-1"},
							},
						},
					},
				},
			},
			phaseID: "/masters/node-2",
		},
		{
			comment: "Rollback non-leaf phase",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/masters",
						State: storage.OperationPhaseStateCompleted,
						Phases: []storage.OperationPhase{
							{
								ID:    "/masters/node-1",
								State: storage.OperationPhaseStateCompleted,
							},
							{
								ID:       "/masters/node-2",
								State:    storage.OperationPhaseStateCompleted,
								Requires: []string{"/masters/node-1"},
							},
						},
					},
				},
			},
			phaseID:  "/masters",
			expected: "rolling back phases that have sub-phases is not supported. Please rollback individual phases",
		},
		{
			comment: "Top level phase has dependent phases that have not been rolled back",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/masters",
						State: storage.OperationPhaseStateCompleted,
						Phases: []storage.OperationPhase{
							{
								ID:    "/masters/node-1",
								State: storage.OperationPhaseStateCompleted,
							},
						},
					},
					{
						ID:    "/nodes",
						State: storage.OperationPhaseStateCompleted,
						Phases: []storage.OperationPhase{
							{
								ID:    "/nodes/node-2",
								State: storage.OperationPhaseStateCompleted,
							},
							{
								ID:       "/nodes/node-3",
								State:    storage.OperationPhaseStateCompleted,
								Requires: []string{"/nodes/node-2"},
							},
						},
						Requires: []string{"/masters"},
					},
				},
			},
			phaseID:  "/masters/node-1",
			expected: rollbackDependentsErrorMsg("/masters/node-1", []string{"/nodes"}),
		},
		{
			comment: "Rollback parallel phase",
			plan: &storage.OperationPlan{
				Phases: []storage.OperationPhase{
					{
						ID:    "/parallel",
						State: storage.OperationPhaseStateCompleted,
						Phases: []storage.OperationPhase{
							{
								ID:    "/parallel/masters",
								State: storage.OperationPhaseStateCompleted,
							},
							{
								ID:    "/parallel/nodes",
								State: storage.OperationPhaseStateCompleted,
							},
						},
					},
				},
			},
			phaseID: "/parallel/masters",
		},
	}
	for _, tc := range tests {
		comment := check.Commentf(tc.comment)
		err := CanRollback(tc.plan, tc.phaseID)
		c.Assert(trace.UserMessage(err), check.Equals, tc.expected, comment)
	}
}
