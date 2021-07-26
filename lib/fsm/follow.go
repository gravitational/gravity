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
//
// The watch will stop upon entering one of the terminal operation states, for
// example if the obtained plan is completed or fully rolled back.
func FollowOperationPlan(ctx context.Context, getPlan GetPlanFunc) <-chan PlanEvent {
	ch := make(chan PlanEvent, 100)
	// Send an initial batch of events from the initial state of the plan.
	plan, err := sendNextPlan(ctx, nil, getPlan, ch)
	if err == nil && plan == nil {
		// Done
		return ch
	}
	// Then launch a goroutine that will be monitoring the progress.
	go func() {
		ticker := backoff.NewTicker(getFollowStepPolicy())
		tickerC := ticker.C
		var errorTicker *backoff.Ticker
		defer func() {
			if ticker != nil {
				ticker.Stop()
			}
			if errorTicker != nil {
				errorTicker.Stop()
			}
		}()
		defer logrus.Info("Operation plan watcher done.")
		for {
			select {
			case <-tickerC:
				nextPlan, err := sendNextPlan(ctx, plan, getPlan, ch)
				if err == nil {
					if nextPlan == nil {
						// Done
						return
					}
					// Update the current plan for comparison on the next cycle and
					// reset the backoff so the ticker keeps ticking every second
					// as long as there are no errors.
					plan = nextPlan
					if ticker == nil {
						errorTicker.Stop()
						errorTicker = nil
						ticker = backoff.NewTicker(getFollowStepPolicy())
						tickerC = ticker.C
					}
					continue
				}
				ticker.Stop()
				ticker = nil
				errorTicker = backoff.NewTicker(getFollowBackoffPolicy())
				tickerC = errorTicker.C
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func sendNextPlan(ctx context.Context, plan *storage.OperationPlan, getter GetPlanFunc, ch chan<- PlanEvent) (*storage.OperationPlan, error) {
	nextPlan, err := getter()
	if err != nil {
		logrus.WithError(err).Error("Failed to reload plan.")
		return nil, trace.Wrap(err)
	}
	changes, err := DiffPlan(plan, *nextPlan)
	if err != nil {
		logrus.WithError(err).Error("Failed to diff plans.")
		return nil, trace.Wrap(err)
	}
	for _, event := range getPlanEvents(changes, *nextPlan) {
		select {
		case ch <- event:
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		}
		if event.isTerminalEvent() {
			return nil, nil
		}
	}
	return nextPlan, nil
}

// getPlanEvents returns a list of plan events from the provided list of
// changes and the current state of the plan.
func getPlanEvents(changes []storage.PlanChange, plan storage.OperationPlan) (events []PlanEvent) {
	for _, change := range changes {
		events = append(events, &PlanChangedEvent{Change: change})
	}
	if IsCompleted(&plan) {
		events = append(events, &PlanCompletedEvent{})
	} else if IsRolledBack(&plan) {
		events = append(events, &PlanRolledBackEvent{})
	}
	return events
}

// getFollowBackoffPolicy returns retry backoff policy for the plan follower.
//
// Backoff triggers when plan reload fails.
func getFollowBackoffPolicy() backoff.BackOff {
	return &backoff.ExponentialBackOff{
		InitialInterval: time.Second,
		Multiplier:      backoff.DefaultMultiplier,
		MaxInterval:     5 * time.Second,
		Clock:           backoff.SystemClock,
	}
}

// getFollowStepPolicy returns the pacing policy for the plan follower
// on the happy path
func getFollowStepPolicy() backoff.BackOff {
	return &backoff.ConstantBackOff{Interval: time.Second}
}
