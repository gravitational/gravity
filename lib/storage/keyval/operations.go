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

package keyval

import (
	"sort"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// CreateSiteOperation creates a new site operation
func (b *backend) CreateSiteOperation(op storage.SiteOperation) (*storage.SiteOperation, error) {
	err := op.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if op.ID == "" {
		op.ID = uuid.New()
	}
	if _, err := b.GetSite(op.SiteDomain); err != nil {
		return nil, trace.Wrap(err)
	}
	err = b.createVal(b.key(sitesP, op.SiteDomain, operationsP, op.ID, valP), op, forever)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err, "operation(%v) already exists", op.ID)
		}
		return nil, trace.Wrap(err)
	}
	return &op, nil
}

// GetSiteOperation returns the operation identified by the operation id
// and site id
func (b *backend) GetSiteOperation(siteDomain, operationID string) (*storage.SiteOperation, error) {
	if siteDomain == "" {
		return nil, trace.BadParameter("missing parameter SiteDomain")
	}
	if operationID == "" {
		return nil, trace.BadParameter("missing parameter OperationID")
	}

	b.cachedCompleteOperationsMutex.RLock()
	if op, ok := b.cachedCompleteOperations[operationID]; ok {
		b.cachedCompleteOperationsMutex.RUnlock()
		return op, nil
	}
	b.cachedCompleteOperationsMutex.RUnlock()

	var op storage.SiteOperation
	if err := b.getVal(b.key(sitesP, siteDomain, operationsP, operationID, valP), &op); err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("operation(%v, %v) not found", siteDomain, operationID)
		}
		return nil, trace.Wrap(err)
	}
	utils.UTC(&op.Created)
	utils.UTC(&op.Updated)

	// Operations that are not expected to change in the future are the only operations that are safe to cache
	if op.State == ops.OperationStateCompleted {
		b.cachedCompleteOperationsMutex.Lock()
		b.cachedCompleteOperations[operationID] = &op
		b.cachedCompleteOperationsMutex.Unlock()
	}

	return &op, nil
}

type operationsSorter []storage.SiteOperation

func (s operationsSorter) Len() int {
	return len(s)
}

func (s operationsSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s operationsSorter) Less(i, j int) bool {
	return s[i].Created.After(s[j].Created)
}

// GetSiteOperations returns a list of operations performed on this
// site sorted by time (latest operations come first)
func (b *backend) GetSiteOperations(siteDomain string) ([]storage.SiteOperation, error) {
	if siteDomain == "" {
		return nil, trace.BadParameter("missing parameter SiteDomain")
	}
	ids, err := b.getKeys(b.key(sitesP, siteDomain, operationsP))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, nil
		}
		return nil, trace.Wrap(err)
	}

	var out []storage.SiteOperation
	var uncachedOperations []string

	b.cachedCompleteOperationsMutex.RLock()
	for _, id := range ids {
		if op, ok := b.cachedCompleteOperations[id]; ok {
			out = append(out, *op)
		} else {
			uncachedOperations = append(uncachedOperations, id)
		}
	}
	b.cachedCompleteOperationsMutex.RUnlock()

	for _, id := range uncachedOperations {
		op, err := b.GetSiteOperation(siteDomain, id)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}

		out = append(out, *op)
	}

	sort.Sort(operationsSorter(out))
	return out, nil
}

// UpdateSiteOperation updates site operation state
func (b *backend) UpdateSiteOperation(op storage.SiteOperation) (*storage.SiteOperation, error) {
	if err := op.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	err := b.updateVal(b.key(sitesP, op.SiteDomain, operationsP, op.ID, valP), op, forever)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &op, nil
}

// DeleteSiteOperation deletes an inactive site operation
func (b *backend) DeleteSiteOperation(siteDomain, operationID string) error {
	if siteDomain == "" {
		return trace.BadParameter("missing parameter SiteDomain")
	}
	if operationID == "" {
		return trace.BadParameter("missing parameter OperationID")
	}
	err := b.deleteDir(b.key(sitesP, siteDomain, operationsP, operationID))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "operation(%v, %v) not found", siteDomain, operationID)
		}
		return trace.Wrap(err)
	}
	return nil
}

// CreateOperationPlan saves a new operation plan
func (b *backend) CreateOperationPlan(plan storage.OperationPlan) (*storage.OperationPlan, error) {
	err := plan.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = b.Now().UTC()
	}
	err = b.createVal(b.key(
		sitesP, plan.ClusterName, operationsP, plan.OperationID, planP), plan, forever)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &plan, nil
}

// GetOperationPlan returns plan for the specified operation
func (b *backend) GetOperationPlan(clusterName, operationID string) (*storage.OperationPlan, error) {
	var plan storage.OperationPlan
	err := b.getVal(b.key(sitesP, clusterName, operationsP, operationID, planP), &plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &plan, nil
}

// CreateOperationPlanChange creates a new state transition entry for a plan
func (b *backend) CreateOperationPlanChange(ch storage.PlanChange) (*storage.PlanChange, error) {
	if ch.ID == "" {
		ch.ID = uuid.New()
	}
	err := b.upsertVal(b.key(
		sitesP, ch.ClusterName, operationsP, ch.OperationID, changelogP, ch.ID, valP), ch, forever)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ch, nil
}

// GetOperationPlanChangelog returns all state transition entries for a plan
func (b *backend) GetOperationPlanChangelog(clusterName, operationID string) (storage.PlanChangelog, error) {
	ids, err := b.getKeys(b.key(sitesP, clusterName, operationsP, operationID, changelogP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.PlanChange
	var cacheMisses []string

	b.cachedPlanChangeMutex.RLock()
	for _, id := range ids {
		if cached, ok := b.cachedPlanChange[id]; ok {
			out = append(out, *cached)
			continue
		}

		cacheMisses = append(cacheMisses, id)
	}
	b.cachedPlanChangeMutex.RUnlock()

	for _, id := range cacheMisses {
		var ch storage.PlanChange
		err = b.getVal(b.key(
			sitesP, clusterName, operationsP, operationID, changelogP, id, valP), &ch)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		utils.UTC(&ch.Created)

		b.cachedCompleteOperationsMutex.Lock()
		b.cachedPlanChange[id] = &ch
		b.cachedCompleteOperationsMutex.Unlock()

		out = append(out, ch)
	}
	return storage.PlanChangelog(out), nil
}

// CreateAppOperation creates a new application operation
func (b *backend) CreateAppOperation(op storage.AppOperation) (*storage.AppOperation, error) {
	err := op.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if op.ID == "" {
		op.ID = uuid.New()
	}
	err = b.createVal(b.key(appOperationsP, op.ID, valP), op, forever)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err, "operation(%v) already exists", op.ID)
		}
		return nil, trace.Wrap(err)
	}
	return &op, nil
}

// GetAppOperation queries an existing operation
func (b *backend) GetAppOperation(id string) (*storage.AppOperation, error) {
	var op storage.AppOperation
	if err := b.getVal(b.key(appOperationsP, id, valP), &op); err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("operation(%v) not found", id)
		}
		return nil, trace.Wrap(err)
	}
	utils.UTC(&op.Created)
	utils.UTC(&op.Updated)
	return &op, nil
}

// UpdateAppOperation updates an existing application operation
func (b *backend) UpdateAppOperation(op storage.AppOperation) (*storage.AppOperation, error) {
	if err := op.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	key := b.key(appOperationsP, op.ID, valP)
	err := b.updateVal(key, op, forever)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &op, nil
}
