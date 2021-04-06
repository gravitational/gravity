/*
Copyright 2018 Gravitational, Inc.

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

package opsservice

import (
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// CreateOperationPlan saves the provided operation plan
func (o *Operator) CreateOperationPlan(key ops.SiteOperationKey, plan storage.OperationPlan) error {
	_, err := o.backend().CreateOperationPlan(plan)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateOperationPlanChange creates a new changelog entry for a plan
func (o *Operator) CreateOperationPlanChange(key ops.SiteOperationKey, change storage.PlanChange) error {
	_, err := o.backend().CreateOperationPlanChange(change)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOperationPlan returns plan for the specified operation
func (o *Operator) GetOperationPlan(key ops.SiteOperationKey) (*storage.OperationPlan, error) {
	return fsm.GetOperationPlan(o.backend(), key)
}
