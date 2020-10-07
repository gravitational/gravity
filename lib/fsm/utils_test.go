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
	"fmt"

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

func (s *FSMUtilsSuite) TestNonLeafRollback(c *check.C) {
	tests := []struct {
		comment    string
		phases     []*phaseBuilder
		rollbackID string
		expected   string
	}{
		{
			comment: "Rollback non-leaf phase",
			phases: []*phaseBuilder{
				s.phaseBuilder("/non-leaf", storage.OperationPhaseStateCompleted).
					withSubphases(
						s.phaseBuilder("/leaf", storage.OperationPhaseStateCompleted)),
			},
			rollbackID: "/non-leaf",
			expected:   "rolling back phases that have sub-phases is not supported. Please rollback individual phases",
		},
	}
	for _, tc := range tests {
		comment := check.Commentf(tc.comment)

		// build plan
		phases := make([]storage.OperationPhase, len(tc.phases))
		for i, phase := range tc.phases {
			phases[i] = phase.build()
		}
		plan := &storage.OperationPlan{Phases: phases}

		err := CanRollback(plan, tc.rollbackID)
		c.Assert(trace.UserMessage(err), check.Equals, tc.expected, comment)
	}

}

func (s *FSMUtilsSuite) TestCanRollback(c *check.C) {
	tests := []struct {
		comment    string
		phases     []*phaseBuilder
		rollbackID string
		expected   string
	}{
		{
			comment: "Rollback latest phase",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted),
			},
			rollbackID: "/init",
		},
		{
			comment: "A subsequent phase is in progress",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted),
				s.phaseBuilder("/startAgent", storage.OperationPhaseStateInProgress).
					withRequires("/init"),
			},
			rollbackID: "/init",
			expected:   rollbackDependentsErrorMsg("/init", []string{"/startAgent"}),
		},
		{
			comment: "All dependent phases have been rolled back or are unstarted",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted),
				s.phaseBuilder("/startAgent", storage.OperationPhaseStateRolledBack).
					withRequires("/init"),
				s.phaseBuilder("/checks", storage.OperationPhaseStateUnstarted).
					withRequires("/startAgent"),
			},
			rollbackID: "/init",
		},
		{
			comment: "Dependent phase is in progress",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted),
				s.phaseBuilder("/masters", storage.OperationPhaseStateInProgress).
					withRequires("/init").
					withSubphases(
						s.phaseBuilder("/node-1", storage.OperationPhaseStateRolledBack),
						s.phaseBuilder("/node-2", storage.OperationPhaseStateUnstarted).
							withRequires("/masters/node-1"),
					),
			},
			rollbackID: "/init",
			expected:   rollbackDependentsErrorMsg("/init", []string{"/masters"}),
		},
		{
			comment: "Rollback after a dependent phase was previously rolled back forcefully",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted),
				s.phaseBuilder("/startAgent", storage.OperationPhaseStateRolledBack).
					withRequires("/init"),
				s.phaseBuilder("/checks", storage.OperationPhaseStateFailed).
					withRequires("/startAgent"),
			},
			rollbackID: "/init",
			expected:   rollbackDependentsErrorMsg("/init", []string{"/checks"}),
		},
		{
			comment: "Rollback after a dependent phase has been executed out of band",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted),
				s.phaseBuilder("/startAgent", storage.OperationPhaseStateUnstarted).
					withRequires("/init"),
				s.phaseBuilder("/checks", storage.OperationPhaseStateCompleted).
					withRequires("/startAgent"),
			},
			rollbackID: "/init",
			expected:   rollbackDependentsErrorMsg("/init", []string{"/checks"}),
		},
		{
			comment: "Top level phase has dependent phases that have not been rolled back",
			phases: []*phaseBuilder{
				s.phaseBuilder("/masters", storage.OperationPhaseStateCompleted).
					withSubphases(
						s.phaseBuilder("/node-1", storage.OperationPhaseStateCompleted)),
				s.phaseBuilder("/nodes", storage.OperationPhaseStateCompleted).
					withRequires("/masters").
					withSubphases(
						s.phaseBuilder("node-2", storage.OperationPhaseStateCompleted),
						s.phaseBuilder("node-3", storage.OperationPhaseStateCompleted).
							withRequires("/nodes/node-2")),
			},
			rollbackID: "/masters/node-1",
			expected:   rollbackDependentsErrorMsg("/masters/node-1", []string{"/nodes"}),
		},
		{
			comment: "Rollback parallel phase",
			phases: []*phaseBuilder{
				s.phaseBuilder("/parallel", storage.OperationPhaseStateCompleted).
					withSubphases(
						s.phaseBuilder("/masters", storage.OperationPhaseStateCompleted),
						s.phaseBuilder("/nodes", storage.OperationPhaseStateCompleted)),
			},
			rollbackID: "/parallel/masters",
		},
		{
			comment: "Rollback with multiple requires",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted).
					withSubphases(
						s.phaseBuilder("/node-1", storage.OperationPhaseStateCompleted),
						s.phaseBuilder("/node-2", storage.OperationPhaseStateCompleted),
						s.phaseBuilder("/node-3", storage.OperationPhaseStateCompleted),
					),
				s.phaseBuilder("/checks", storage.OperationPhaseStateCompleted).
					withRequires("/init"),
				s.phaseBuilder("/pre-update", storage.OperationPhaseStateCompleted).
					withRequires("/init", "/checks"),
			},
			rollbackID: "/init/node-1",
			expected:   rollbackDependentsErrorMsg("/init/node-1", []string{"/checks", "/pre-update"}),
		},
		{
			comment: "Invalid rollback with multi-level deep subphases",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted),
				s.phaseBuilder("/masters", storage.OperationPhaseStateCompleted).
					withRequires("/init").
					withSubphases(
						s.phaseBuilder("/node-1", storage.OperationPhaseStateCompleted).
							withSubphases(
								s.phaseBuilder("/drain", storage.OperationPhaseStateCompleted),
								s.phaseBuilder("/system-upgrade", storage.OperationPhaseStateCompleted).
									withRequires("/masters/node-1/drain")),
						s.phaseBuilder("/node-2", storage.OperationPhaseStateCompleted).
							withRequires("/masters/node-1").
							withSubphases(
								s.phaseBuilder("/drain", storage.OperationPhaseStateCompleted),
								s.phaseBuilder("/system-upgrade", storage.OperationPhaseStateCompleted).
									withRequires("/masters/node-2/drain"))),
				s.phaseBuilder("/nodes", storage.OperationPhaseStateCompleted).
					withRequires("/masters").
					withSubphases(
						s.phaseBuilder("node-3", storage.OperationPhaseStateCompleted).
							withSubphases(
								s.phaseBuilder("/drain", storage.OperationPhaseStateCompleted),
								s.phaseBuilder("/system-upgrade", storage.OperationPhaseStateCompleted).
									withRequires("/nodes/node-3/drain"))),
				s.phaseBuilder("/etcd", storage.OperationPhaseStateCompleted).
					withSubphases(
						s.phaseBuilder("/backup", storage.OperationPhaseStateCompleted)),
				s.phaseBuilder("/runtime", storage.OperationPhaseStateCompleted).
					withRequires("/masters").
					withSubphases(
						s.phaseBuilder("/monitoring", storage.OperationPhaseStateCompleted),
						s.phaseBuilder("/site", storage.OperationPhaseStateCompleted)),
				s.phaseBuilder("/gc", storage.OperationPhaseStateCompleted).
					withRequires("/runtime"),
			},
			rollbackID: "/masters/node-1/drain",
			expected: rollbackDependentsErrorMsg("/masters/node-1/drain", []string{
				"/masters/node-1/system-upgrade",
				"/masters/node-2",
				"/nodes",
				"/runtime",
				"/gc",
			}),
		},
		{
			comment: "Valid rollback with multi-level deep subphases",
			phases: []*phaseBuilder{
				s.phaseBuilder("/init", storage.OperationPhaseStateCompleted),
				s.phaseBuilder("/masters", storage.OperationPhaseStateCompleted).
					withRequires("/init").
					withSubphases(
						s.phaseBuilder("/node-1", storage.OperationPhaseStateCompleted).
							withSubphases(
								s.phaseBuilder("/drain", storage.OperationPhaseStateCompleted),
								s.phaseBuilder("/system-upgrade", storage.OperationPhaseStateRolledBack).
									withRequires("/masters/node-1/drain")),
						s.phaseBuilder("/node-2", storage.OperationPhaseStateRolledBack).
							withRequires("/masters/node-1").
							withSubphases(
								s.phaseBuilder("/drain", storage.OperationPhaseStateRolledBack),
								s.phaseBuilder("/system-upgrade", storage.OperationPhaseStateRolledBack).
									withRequires("/masters/node-2/drain"))),
				s.phaseBuilder("/nodes", storage.OperationPhaseStateRolledBack).
					withRequires("/masters").
					withSubphases(
						s.phaseBuilder("node-3", storage.OperationPhaseStateRolledBack).
							withSubphases(
								s.phaseBuilder("/drain", storage.OperationPhaseStateRolledBack),
								s.phaseBuilder("/system-upgrade", storage.OperationPhaseStateRolledBack).
									withRequires("/nodes/node-3/drain"))),
				s.phaseBuilder("/etcd", storage.OperationPhaseStateCompleted).
					withSubphases(
						s.phaseBuilder("/backup", storage.OperationPhaseStateCompleted)),
				s.phaseBuilder("/runtime", storage.OperationPhaseStateRolledBack).
					withRequires("/masters").
					withSubphases(
						s.phaseBuilder("/monitoring", storage.OperationPhaseStateRolledBack),
						s.phaseBuilder("/site", storage.OperationPhaseStateRolledBack)),
				s.phaseBuilder("/gc", storage.OperationPhaseStateUnstarted).
					withRequires("/runtime"),
			},
			rollbackID: "/masters/node-1/drain",
		},
	}
	for _, tc := range tests {
		comment := check.Commentf(tc.comment)

		// build plan
		phases := make([]storage.OperationPhase, len(tc.phases))
		for i, phase := range tc.phases {
			phases[i] = phase.build()
		}
		plan := &storage.OperationPlan{Phases: phases}

		err := CanRollback(plan, tc.rollbackID)
		c.Assert(trace.UserMessage(err), check.Equals, tc.expected, comment)
	}
}

// phaseBuilder returns a new phaseBuilder.
func (s *FSMUtilsSuite) phaseBuilder(id, state string) *phaseBuilder {
	return &phaseBuilder{
		id:    id,
		state: state,
	}
}

// phaseBuilder builds storage.OperationPhase to be used in test cases.
type phaseBuilder struct {
	id       string
	state    string
	phases   []*phaseBuilder
	requires []string
}

// withSubphases appends the provided subphases.
func (r *phaseBuilder) withSubphases(subphases ...*phaseBuilder) *phaseBuilder {
	r.phases = append(r.phases, subphases...)
	return r
}

// withRequires appends the provided required phases.
func (r *phaseBuilder) withRequires(requires ...string) *phaseBuilder {
	r.requires = append(r.requires, requires...)
	return r
}

// build builds the phase.
func (r *phaseBuilder) build() storage.OperationPhase {
	phase := storage.OperationPhase{
		ID:       r.id,
		State:    r.state,
		Phases:   make([]storage.OperationPhase, len(r.phases)),
		Requires: r.requires,
	}
	for i, subphase := range r.phases {
		subphase.id = fmt.Sprintf("%s%s", r.id, subphase.id)
		phase.Phases[i] = subphase.build()
	}
	return phase
}
