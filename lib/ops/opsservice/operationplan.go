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
	plan, err := o.backend().GetOperationPlan(key.SiteDomain, key.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	changelog, err := o.backend().GetOperationPlanChangelog(key.SiteDomain, key.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return fsm.ResolvePlan(*plan, changelog), nil
}
