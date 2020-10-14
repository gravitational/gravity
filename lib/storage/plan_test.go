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

package storage

import (
	"github.com/gravitational/gravity/lib/compare"

	"gopkg.in/check.v1"
)

type PlanSuite struct{}

var _ = check.Suite(&PlanSuite{})

func (*PlanSuite) TestGetLeafPhases(c *check.C) {
	initPhase := OperationPhase{ID: "/init"}
	bootstrapPhase1 := OperationPhase{ID: "/bootstrap/node-1"}
	bootstrapPhase2 := OperationPhase{ID: "/bootstrap/node-2"}
	bootstrapPhase := OperationPhase{
		ID:     "/bootstrap",
		Phases: []OperationPhase{bootstrapPhase1, bootstrapPhase2},
	}
	upgradePhase := OperationPhase{ID: "/upgrade"}
	plan := &OperationPlan{
		Phases: []OperationPhase{initPhase, bootstrapPhase, upgradePhase},
	}
	compare.DeepCompare(c, plan.GetLeafPhases(), []OperationPhase{
		initPhase, bootstrapPhase1, bootstrapPhase2, upgradePhase})
}

func (*PlanSuite) TestGetState(c *check.C) {
	tests := []struct {
		comment  string
		expected string
		phase    OperationPhase
	}{
		{
			comment:  "Phase unstarted",
			expected: OperationPhaseStateUnstarted,
			phase: OperationPhase{
				Phases: []OperationPhase{
					{
						State: OperationPhaseStateUnstarted,
					},
				},
			},
		},
		{
			comment:  "Phase completed",
			expected: OperationPhaseStateCompleted,
			phase: OperationPhase{
				Phases: []OperationPhase{
					{State: OperationPhaseStateCompleted},
				},
			},
		},
		{
			comment:  "Phase failed",
			expected: OperationPhaseStateFailed,
			phase: OperationPhase{
				Phases: []OperationPhase{
					{State: OperationPhaseStateCompleted},
					{State: OperationPhaseStateFailed},
					{State: OperationPhaseStateRolledBack},
					{State: OperationPhaseStateUnstarted},
				},
			},
		},
		{
			comment:  "Phase rolled back",
			expected: OperationPhaseStateRolledBack,
			phase: OperationPhase{
				Phases: []OperationPhase{
					{State: OperationPhaseStateRolledBack},
					{State: OperationPhaseStateUnstarted},
				},
			},
		},
		{
			comment:  "Phase in progress",
			expected: OperationPhaseStateInProgress,
			phase: OperationPhase{
				Phases: []OperationPhase{
					{State: OperationPhaseStateCompleted},
					{State: OperationPhaseStateInProgress},
					{State: OperationPhaseStateUnstarted},
				},
			},
		},
	}
	for _, tc := range tests {
		comment := check.Commentf(tc.comment)
		c.Assert(tc.phase.GetState(), check.Equals, tc.expected, comment)
	}
}
