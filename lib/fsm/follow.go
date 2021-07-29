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
	"errors"
	"time"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"

	"github.com/cenkalti/backoff"
	"github.com/sirupsen/logrus"
)

// GetPlanFunc is a function that returns an operation plan.
type GetPlanFunc func() (*storage.OperationPlan, error)

// PlanEvent represents an operation plan event.
type PlanEvent interface {
	// isTerminalEvent returns true for terminal plan events, such as if
	// a plan was completed or fully rolled back.
	isTerminalEvent() bool
}

// PlanChangedEvent is sent when plan phases change states.
type PlanChangedEvent struct {
	// Change is a plan phase change.
	Change storage.PlanChange
}

// isTerminalEvent returns false for the plan change event.
func (e *PlanChangedEvent) isTerminalEvent() bool { return false }

// PlanCompletedEvent is sent when the plan is fully completed.
type PlanCompletedEvent struct{}

// isTerminalEvent returns true for the completed plan event.
func (e *PlanCompletedEvent) isTerminalEvent() bool { return true }

// PlanRolledBackEvent is sent when the plan is fully rolled back.
type PlanRolledBackEvent struct{}

// isTerminalEvent returns true for the rolled back plan event.
func (e *PlanRolledBackEvent) isTerminalEvent() bool { return true }

// FollowOperationPlan returns a channel that will be receiving phase updates
// for the specified plan.
// The returned channel must be served to avoid blocking the producer as
// no events are dropped.
//
// The watch will stop upon entering one of the terminal operation states, for
// example if the obtained plan is completed or fully rolled back or when the specified
// context has expired. In any case, the channel will be closed to signal completion
func FollowOperationPlan(ctx context.Context, getPlan GetPlanFunc) <-chan PlanEvent {
	ch := make(chan PlanEvent)
	go func() {
		defer close(ch)
		// Send an initial batch of events from the initial state of the plan.
		plan, err := getPlan()
		if err == nil {
			err = sendPlanChanges(ctx, GetPlanProgress(*plan), *plan, ch)
			if errors.Is(err, errStopUpdates) {
				// Done
				return
			}
		}
		ticker := backoff.NewTicker(getFollowStepPolicy())
		defer ticker.Stop()
		defer logrus.Info("Operation plan watcher done.")
		for {
			select {
			case <-ticker.C:
				nextPlan, err := getPlan()
				if err != nil {
					logrus.WithError(err).Error("Failed to diff plans.")
					continue
				}
				changes, err := diffPlan(plan, *nextPlan)
				if err != nil {
					logrus.WithError(err).Error("Failed to diff plans.")
					continue
				}
				err = sendPlanChanges(ctx, changes, *nextPlan, ch)
				if errors.Is(err, errStopUpdates) {
					// Done
					return
				}
				if err != nil {
					continue
				}
				// Update the current plan for comparison on the next cycle and
				// reset the backoff so the ticker keeps ticking every second
				// as long as there are no errors.
				plan = nextPlan
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func sendPlanChanges(ctx context.Context, changes []storage.PlanChange, plan storage.OperationPlan, ch chan<- PlanEvent) error {
	for _, event := range getPlanEvents(changes, plan) {
		select {
		case ch <- event:
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
		if event.isTerminalEvent() {
			return errStopUpdates
		}
	}
	return nil
}

// errStopUpdates is a special error value that signifies that no further
// plan updates will be sent
var errStopUpdates = errors.New("stop plan updates")

// getPlanEvents returns a list of plan events from the provided list of
// changes and the current state of the plan.
func getPlanEvents(changes []storage.PlanChange, plan storage.OperationPlan) (events []PlanEvent) {
	events = make([]PlanEvent, 0, len(changes)+1)
	for _, change := range changes {
		events = append(events, &PlanChangedEvent{Change: change})
	}
	if IsCompleted(plan) {
		events = append(events, &PlanCompletedEvent{})
	} else if IsRolledBack(plan) {
		events = append(events, &PlanRolledBackEvent{})
	}
	return events
}

// getFollowStepPolicy returns the pacing policy for the plan follower
// on the happy path
func getFollowStepPolicy() backoff.BackOff {
	return &backoff.ConstantBackOff{Interval: 5 * time.Second}
}
