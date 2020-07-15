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
	"time"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

func (s *FSMSuite) TestFollowOperationPlan(c *check.C) {
	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()

	engine := newTestEngine(func() storage.OperationPlan {
		return *(s.planner.newPlan(
			s.planner.initPhase(storage.OperationPhaseStateUnstarted),
			s.planner.bootstrapPhase(
				s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateUnstarted)),
			s.planner.upgradePhase(storage.OperationPhaseStateUnstarted)))
	})

	// Emit a couple of plan changes prior to starting the watch.
	tsInit := s.clock.Now()
	engine.ChangePhaseState(ctx, StateChange{
		Phase:   "/init",
		State:   storage.OperationPhaseStateCompleted,
		created: tsInit,
	})
	tsBootstrap := s.clock.Now().Add(time.Minute)
	engine.ChangePhaseState(ctx, StateChange{
		Phase:   "/bootstrap/node-1",
		State:   storage.OperationPhaseStateCompleted,
		created: tsBootstrap,
	})

	// Save the initial plan state (before watch) for comparison later.
	planAfterBootstrap, err := engine.GetPlan()
	c.Assert(err, check.IsNil)

	// Launch the plan watcher.
	eventsCh, err := FollowOperationPlan(ctx, func() (*storage.OperationPlan, error) {
		return engine.GetPlan()
	})
	c.Assert(err, check.IsNil)

	// Change a phase state after the watch has been established as well.
	tsUpgrade := s.clock.Now().Add(2 * time.Minute)
	engine.ChangePhaseState(ctx, StateChange{
		Phase:   "/upgrade",
		State:   storage.OperationPhaseStateCompleted,
		created: tsUpgrade,
	})

	planAfterUpgrade, err := engine.GetPlan()
	c.Assert(err, check.IsNil)

	// All received plan events will be collected here after the watch stops.
	var events []PlanEvent

L:
	for {
		select {
		case event := <-eventsCh:
			events = append(events, event)
			if _, isFinished := event.(*PlanFinished); isFinished {
				break L
			}
		case <-ctx.Done():
			c.Fatalf("Timeout following operation plan")
		}
	}

	c.Assert([]PlanEvent{
		&PlanChanged{
			Plan: *planAfterBootstrap,
			Change: storage.PlanChange{
				PhaseID:    "/init",
				PhaseIndex: 0,
				NewState:   storage.OperationPhaseStateCompleted,
				Created:    tsInit,
			},
		},
		&PlanChanged{
			Plan: *planAfterBootstrap,
			Change: storage.PlanChange{
				PhaseID:    "/bootstrap/node-1",
				PhaseIndex: 1,
				NewState:   storage.OperationPhaseStateCompleted,
				Created:    tsBootstrap,
			},
		},
		&PlanChanged{
			Plan: *planAfterUpgrade,
			Change: storage.PlanChange{
				PhaseID:    "/upgrade",
				PhaseIndex: 2,
				NewState:   storage.OperationPhaseStateCompleted,
				Created:    tsUpgrade,
			},
		},
		&PlanFinished{
			Plan: *planAfterUpgrade,
		},
	}, compare.DeepEquals, events)
}
