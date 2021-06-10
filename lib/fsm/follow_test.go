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

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

// TestFollowOperationPlan verifies the operation plan watcher receives proper
// plan update events.
func (s *FSMSuite) TestFollowOperationPlan(c *check.C) {
	// Make sure to cap the test execution in case something's not working.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	engine.changePhaseStateWithTimestamp(StateChange{
		Phase: "/init",
		State: storage.OperationPhaseStateCompleted,
	}, tsInit)
	tsBootstrap := s.clock.Now().Add(time.Minute)
	engine.changePhaseStateWithTimestamp(StateChange{
		Phase: "/bootstrap/node-1",
		State: storage.OperationPhaseStateCompleted,
	}, tsBootstrap)

	// Launch the plan watcher.
	eventsCh := FollowOperationPlan(ctx, func() (*storage.OperationPlan, error) {
		return engine.GetPlan()
	})

	// Change a phase state after the watch has been established as well.
	tsUpgrade := s.clock.Now().Add(2 * time.Minute)
	engine.changePhaseStateWithTimestamp(StateChange{
		Phase: "/upgrade",
		State: storage.OperationPhaseStateCompleted,
	}, tsUpgrade)

	// All received plan events will be collected here after the watch stops.
	var events []PlanEvent

L:
	for {
		select {
		case event := <-eventsCh:
			events = append(events, event)
			if event.isTerminalEvent() {
				break L
			}
		case <-ctx.Done():
			c.Fatalf("Timeout following operation plan")
		}
	}

	c.Assert(events, compare.DeepEquals, []PlanEvent{
		&PlanChangedEvent{
			Change: storage.PlanChange{
				PhaseID:    "/init",
				PhaseIndex: 0,
				NewState:   storage.OperationPhaseStateCompleted,
				Created:    tsInit,
			},
		},
		&PlanChangedEvent{
			Change: storage.PlanChange{
				PhaseID:    "/bootstrap/node-1",
				PhaseIndex: 1,
				NewState:   storage.OperationPhaseStateCompleted,
				Created:    tsBootstrap,
			},
		},
		&PlanChangedEvent{
			Change: storage.PlanChange{
				PhaseID:    "/upgrade",
				PhaseIndex: 2,
				NewState:   storage.OperationPhaseStateCompleted,
				Created:    tsUpgrade,
			},
		},
		&PlanCompletedEvent{},
	})
}

// TestFollowOperationPlanFailure makes sure the operation plan watcher
// retries in case of failures.
func (s *FSMSuite) TestFollowOperationPlanFailure(c *check.C) {
	// Make sure to cap the test execution in case something's not working.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Generate a very simple single-phase plan and complete it.
	engine := newTestEngine(func() storage.OperationPlan {
		return *(s.planner.newPlan(
			s.planner.upgradePhase(storage.OperationPhaseStateUnstarted)))
	})

	tsUpgrade := s.clock.Now()
	engine.changePhaseStateWithTimestamp(StateChange{
		Phase: "/upgrade",
		State: storage.OperationPhaseStateCompleted,
	}, tsUpgrade)

	// Launch the plan watcher, make sure getPlan returns error first couple of times.
	counter := 0
	eventsCh := FollowOperationPlan(ctx, func() (*storage.OperationPlan, error) {
		if counter < 2 {
			counter++
			return nil, trace.BadParameter("plan reload test failure")
		}
		return engine.GetPlan()
	})

	// Make sure the watch finishes and events are received.
	var events []PlanEvent

L:
	for {
		select {
		case event := <-eventsCh:
			events = append(events, event)
			if event.isTerminalEvent() {
				break L
			}
		case <-ctx.Done():
			c.Fatalf("Timeout following operation plan")
		}
	}

	c.Assert(events, compare.DeepEquals, []PlanEvent{
		&PlanChangedEvent{
			Change: storage.PlanChange{
				PhaseID:    "/upgrade",
				PhaseIndex: 0,
				NewState:   storage.OperationPhaseStateCompleted,
				Created:    tsUpgrade,
			},
		},
		&PlanCompletedEvent{},
	})
}
