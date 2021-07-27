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
	"fmt"
	"sort"
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
		return s.planner.newPlan(
			s.planner.initPhase(storage.OperationPhaseStateUnstarted),
			s.planner.bootstrapPhase(
				s.planner.bootstrapSubPhase("node-1", storage.OperationPhaseStateUnstarted)),
			s.planner.upgradePhase(storage.OperationPhaseStateUnstarted))
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
		return s.planner.newPlan(
			s.planner.upgradePhase(storage.OperationPhaseStateUnstarted))
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

// TestFollowOperationPlanStress verifies that the plan execution follow loop
// delivers events to completion.
func (s *FSMSuite) TestFollowOperationPlanStress(c *check.C) {
	const planSize = 5000
	plan := make([]storage.OperationPhase, 0, planSize)
	for i := 0; i < planSize; i++ {
		plan = append(plan, s.planner.phase(i, storage.OperationPhaseStateUnstarted))
	}

	engine := newTestEngine(func() storage.OperationPlan {
		return s.planner.newPlan(plan...)
	})

	ts := s.clock.Now()
	for id := range plan {
		engine.changePhaseStateWithTimestamp(StateChange{
			Phase: fmt.Sprint("/phase", id),
			State: storage.OperationPhaseStateInProgress,
		}, ts)
		ts = ts.Add(1 * time.Second)
	}
	s.clock.Advance(planSize * time.Second)

	done := make(chan struct{})
	go func() {
		defer close(done)
		ts := s.clock.Now()
		for id := range plan {
			engine.changePhaseStateWithTimestamp(StateChange{
				Phase: fmt.Sprint("/phase", id),
				State: storage.OperationPhaseStateCompleted,
			}, ts)
			ts = ts.Add(1 * time.Second)
		}
		s.clock.Advance(planSize * time.Second)
	}()

	events := make([]PlanEvent, 0, planSize)
	// Launch the plan watcher.
	for eventPayload := range FollowOperationPlan(context.Background(), engine.GetPlan) {
		if len(events) < cap(events) {
			events = append(events, eventPayload)
		} else {
			copy(events[:len(events)-1], events[1:])
			events[len(events)-1] = eventPayload
		}
	}

	c.Assert(events, check.HasLen, planSize)
	// Verify that the collected events are mostly completed.
	// There might be a part of events that have not
	// been captured as plan execution and follow loops execute
	// concurrently and some phases could have already been
	// marked as completed.
	assertEventsMostlyCompleted(events, c)
	<-done
}

func assertEventsMostlyCompleted(events []PlanEvent, c *check.C) {
	var pivot *int
	for i, eventPayload := range events {
		switch event := eventPayload.(type) {
		case *PlanChangedEvent:
			if event.Change.NewState == storage.OperationPhaseStateCompleted {
				pivot = &i
				break
			}
		}
	}
	if pivot == nil {
		c.Fatal("Expected completed phases.")
	}
	// The last event should be a terminal event
	if !events[len(events)-1].isTerminalEvent() {
		c.Fatal("Expected terminal event at the end.")
	}
	// I'd assume this to hold, but it's dependent on the environment
	// and can be flaky.
	// if *pivot >= len(events)/2 {
	//	c.Error("Expected events to have mostly completed phases.")
	// }
	completedEvents := events[*pivot : len(events)-1]
	for _, eventPayload := range completedEvents {
		switch event := eventPayload.(type) {
		case *PlanChangedEvent:
			if event.Change.NewState != storage.OperationPhaseStateCompleted {
				c.Errorf("Phase with unexpected state %q.", event.Change.NewState)
			}
		}
	}
	changes := make([]*PlanChangedEvent, 0, len(completedEvents))
	for _, eventPayload := range completedEvents {
		switch event := eventPayload.(type) {
		case *PlanChangedEvent:
			changes = append(changes, event)
		}
	}
	if !sort.SliceIsSorted(changes, func(i, j int) bool {
		return changes[j].Change.Created.After(changes[i].Change.Created)
	}) {
		c.Error("Expected completed events to be sorted by time.")
	}
}
