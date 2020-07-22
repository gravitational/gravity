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

	"github.com/cenkalti/backoff"
	"github.com/sirupsen/logrus"
)

// GetPlanFunc is a function that returns an operation plan.
type GetPlanFunc func() (*storage.OperationPlan, error)

// PlanEvent represents an operation plan event.
type PlanEvent interface {
	// IsPlanEvent returns true for events representing plan changes.
	IsPlanEvent() bool
}

// PlanChangeEvent is sent when plan phases change states.
type PlanChangeEvent struct {
	// Change is a plan phase change.
	Change storage.PlanChange
	// Plan is the resolved operation plan.
	Plan storage.OperationPlan
}

// IsPlanEvent is for satisfying PlanEvent interface.
func (e *PlanChangeEvent) IsPlanEvent() bool { return true }

// PlanFinishEvent is sent when the plan is fully completed or rolled back.
type PlanFinishEvent struct {
	// Plan is the resolved operation plan.
	Plan storage.OperationPlan
}

// IsPlanEvent is for satisfying PlanEvent interface.
func (e *PlanFinishEvent) IsPlanEvent() bool { return true }

// FollowOperationPlan returns a channel that receives phase updates for the
// specified plan.
func FollowOperationPlan(ctx context.Context, getPlan GetPlanFunc) <-chan PlanEvent {
	ch := make(chan PlanEvent, 100)
	// Send an initial batch of events from the initial state of the plan.
	plan, err := getPlan()
	if err != nil {
		logrus.WithError(err).Error("Failed to load plan.")
	}
	if plan != nil {
		for _, change := range GetPlanProgress(*plan) {
			ch <- &PlanChangeEvent{Change: change, Plan: *plan}
		}
		if IsCompleted(plan) || IsRolledBack(plan) {
			ch <- &PlanFinishEvent{Plan: *plan}
			return ch
		}
	}
	// Then launch a goroutine that will be monitoring the progress.
	go func() {
		tickerBackoff := getFollowBackoffPolicy()
		ticker := backoff.NewTicker(tickerBackoff)
		defer func() {
			ticker.Stop()
			logrus.Info("Operation plan watcher done.")
		}()
		for {
			select {
			case <-ticker.C:
				nextPlan, err := getPlan()
				if err != nil {
					logrus.WithError(err).Error("Failed to reload plan.")
					continue
				}
				changes, err := DiffPlan(plan, *nextPlan)
				if err != nil {
					logrus.WithError(err).Error("Failed to diff plans.")
					continue
				}
				for _, change := range changes {
					ch <- &PlanChangeEvent{Change: change, Plan: *nextPlan}
				}
				if IsCompleted(nextPlan) || IsRolledBack(nextPlan) {
					ch <- &PlanFinishEvent{Plan: *nextPlan}
					return
				}
				// Update the current plan for comparison on the next cycle and
				// reset the backoff so the ticker keeps ticking every second
				// as long as there are no errors.
				plan = nextPlan
				tickerBackoff.Reset()
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// getFollowBackoffPolicy returns retry/backoff policy for the plan follower.
//
// Backoff triggers when plan reload fails. Otherwise, the backoff is reset
// on each cycle to maintain the constant retry at initial interval.
func getFollowBackoffPolicy() backoff.BackOff {
	return &backoff.ExponentialBackOff{
		InitialInterval: time.Second,
		Multiplier:      backoff.DefaultMultiplier,
		MaxInterval:     5 * time.Second,
		Clock:           backoff.SystemClock,
	}
}
