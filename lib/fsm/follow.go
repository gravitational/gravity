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
	"github.com/sirupsen/logrus"
)

// GetPlanFunc is a function that returns an operation plan.
type GetPlanFunc func() (*storage.OperationPlan, error)

// PlanEvent represents an operation plan event.
type PlanEvent interface{}

// PlanChanged is sent when plan phases change states.
type PlanChanged struct {
	// Change is a plan phase change.
	Change storage.PlanChange
	// Plan is the resolved operation plan.
	Plan storage.OperationPlan
}

// PlanFinished is sent when the plan is fully completed or rolled back.
type PlanFinished struct {
	// Plan is the resolved operation plan.
	Plan storage.OperationPlan
}

// FollowOperationPlan returns a channel that receives phase updates for the
// specified plan.
func FollowOperationPlan(ctx context.Context, getPlan GetPlanFunc) (<-chan PlanEvent, error) {
	previousPlan, err := getPlan()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ch := make(chan PlanEvent, 100)
	go func() {
		// Send initial events to the channel, excluding unstarted phases.
		for _, change := range GetPlanProgress(*previousPlan) {
			ch <- &PlanChanged{Plan: *previousPlan, Change: change}
		}
		if IsCompleted(previousPlan) || IsRolledBack(previousPlan) {
			ch <- &PlanFinished{Plan: *previousPlan}
			return
		}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				newPlan, err := getPlan()
				if err != nil {
					logrus.WithError(err).Error("Failed to reload plan.")
					continue
				}
				diff, err := DiffPlan(*previousPlan, *newPlan)
				if err != nil {
					logrus.WithError(err).Error("Failed to diff plans.")
					continue
				}
				for _, change := range diff {
					ch <- &PlanChanged{Plan: *newPlan, Change: change}
				}
				if IsCompleted(newPlan) || IsRolledBack(newPlan) {
					ch <- &PlanFinished{Plan: *newPlan}
					return
				}
				previousPlan = newPlan
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
