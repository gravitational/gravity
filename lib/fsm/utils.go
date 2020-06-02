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

package fsm

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// CanRollback checks if specified phase can be rolled back
func CanRollback(plan *storage.OperationPlan, phaseID string) error {
	phase, err := FindPhase(plan, phaseID)
	if err != nil {
		return trace.Wrap(err)
	}
	if phase.IsUnstarted() {
		return trace.BadParameter(
			"phase %q hasn't been executed yet", phase.ID)
	}
	if phase.IsRolledBack() {
		return trace.BadParameter(
			"phase %q has already been rolled back", phase.ID)
	}
	return nil
}

// IsCompleted returns true if all phases of the provided plan are completed
func IsCompleted(plan *storage.OperationPlan) bool {
	for _, phase := range plan.GetLeafPhases() {
		if !phase.IsCompleted() {
			return false
		}
	}
	return true
}

// IsRolledBack returns true if the provided plan is rolled back.
func IsRolledBack(plan *storage.OperationPlan) bool {
	for _, phase := range plan.GetLeafPhases() {
		if !phase.IsRolledBack() && !phase.IsUnstarted() {
			return false
		}
	}
	return true
}

// MarkCompleted marks all phases of the plan as completed
func MarkCompleted(plan *storage.OperationPlan) {
	allPhases := FlattenPlan(plan)
	for i := range allPhases {
		allPhases[i].State = storage.OperationPhaseStateCompleted
	}
}

// HasFailed returns true if the provided plan has at least one failed phase
func HasFailed(plan *storage.OperationPlan) bool {
	for _, phase := range FlattenPlan(plan) {
		if phase.IsFailed() {
			return true
		}
	}
	return false
}

// IsFailed returns true if all phases of the provided plan are either rolled back or unstarted
func IsFailed(plan *storage.OperationPlan) bool {
	for _, phase := range FlattenPlan(plan) {
		if !phase.IsFailed() && !phase.IsRolledBack() && !phase.IsUnstarted() {
			return false
		}
	}
	return true
}

// FindPhase finds a phase with the specified id in the provided plan
func FindPhase(plan *storage.OperationPlan, phaseID string) (*storage.OperationPhase, error) {
	allPhases := FlattenPlan(plan)
	for i, phase := range allPhases {
		if phase.ID == phaseID {
			return allPhases[i], nil
		}
	}
	return nil, trace.NotFound("phase %q not found", phaseID)
}

// FlattenPlan returns a slice of pointers to all phases of the provided plan
func FlattenPlan(plan *storage.OperationPlan) []*storage.OperationPhase {
	var result []*storage.OperationPhase
	for i := range plan.Phases {
		addPhases(&plan.Phases[i], &result)
	}
	return result
}

// SplitServers splits the specified server list into servers with master cluster role
// and regular nodes.
func SplitServers(servers []storage.Server) (masters, nodes []storage.Server) {
	for _, server := range servers {
		switch server.ClusterRole {
		case string(schema.ServiceRoleMaster):
			masters = append(masters, server)
		case string(schema.ServiceRoleNode):
			nodes = append(nodes, server)
		}
	}
	return masters, nodes
}

// GetOperationPlan returns plan for the specified operation
func GetOperationPlan(backend storage.Backend, opKey ops.SiteOperationKey) (*storage.OperationPlan, error) {
	plan, err := backend.GetOperationPlan(opKey.SiteDomain, opKey.OperationID)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no operation plan for operation %v found",
				opKey.OperationID)
		}
		return nil, trace.Wrap(err)
	}
	changelog, err := backend.GetOperationPlanChangelog(opKey.SiteDomain, opKey.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ResolvePlan(*plan, changelog), nil
}

// ResolvePlan applies changelog to the provided plan and returns the resulting plan
func ResolvePlan(plan storage.OperationPlan, changelog storage.PlanChangelog) *storage.OperationPlan {
	allPhases := FlattenPlan(&plan)
	for i, phase := range allPhases {
		latest := changelog.Latest(phase.ID)
		if latest != nil {
			allPhases[i].State = latest.NewState
			allPhases[i].Updated = latest.Created
			allPhases[i].Error = latest.Error
		}
	}
	return &plan
}

// DiffChangelog returns a list of changelog entries from "local" that are missing from "remote"
func DiffChangelog(local, remote storage.PlanChangelog) []storage.PlanChange {
	remoteEntries := make(map[string]struct{})
	for _, remoteEntry := range remote {
		remoteEntries[remoteEntry.ID] = struct{}{}
	}
	var missingEntries []storage.PlanChange
	for _, localEntry := range local {
		_, ok := remoteEntries[localEntry.ID]
		if !ok {
			missingEntries = append(missingEntries, localEntry)
		}
	}
	return missingEntries
}

// RequireIfPresent takes a list of phase IDs and returns those that are
// present in the provided plan
func RequireIfPresent(plan *storage.OperationPlan, phaseIDs ...string) []string {
	var present []string
	for _, id := range phaseIDs {
		_, err := FindPhase(plan, id)
		if trace.IsNotFound(err) {
			continue
		}
		present = append(present, id)
	}
	return present
}

// OperationStateSetter returns the handler to set operation state both in the given operator
// as well as the specified backend
func OperationStateSetter(key ops.SiteOperationKey, operator ops.Operator, backend storage.Backend) ops.OperationStateFunc {
	return func(ctx context.Context, key ops.SiteOperationKey, req ops.SetOperationStateRequest) error {
		err := operator.SetOperationState(ctx, key, req)
		if err != nil {
			return trace.Wrap(err)
		}
		op, err := operator.GetSiteOperation(key)
		if err != nil {
			return trace.Wrap(err)
		}
		backendOp, err := backend.GetSiteOperation(key.SiteDomain, key.OperationID)
		if err != nil {
			return trace.Wrap(err)
		}
		backendOp.State = op.State
		_, err = backend.UpdateSiteOperation(*backendOp)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
}

// OperationKey returns the operation key for the specified operation plan
func OperationKey(plan storage.OperationPlan) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   plan.AccountID,
		SiteDomain:  plan.ClusterName,
		OperationID: plan.OperationID,
	}
}

// CompleteOrFailOperation completes or fails the operation given by the plan in the specified operator.
// planErr optionally specifies the error to record in the failed message and record operation failure
func CompleteOrFailOperation(ctx context.Context, plan *storage.OperationPlan, operator ops.Operator, planErr string) (err error) {
	key := OperationKey(*plan)
	if IsCompleted(plan) {
		err = ops.CompleteOperation(ctx, key, operator)
	} else {
		err = ops.FailOperation(ctx, key, operator, planErr)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	logrus.WithFields(logrus.Fields{
		constants.FieldSuccess: IsCompleted(plan),
		constants.FieldError:   planErr,
	}).Debug("Marked operation complete.")
	return nil
}

func addPhases(phase *storage.OperationPhase, result *[]*storage.OperationPhase) {
	// Add the phase itself and all its subphases recursively.
	*result = append(*result, phase)
	for i := range phase.Phases {
		addPhases(&phase.Phases[i], result)
	}
}
