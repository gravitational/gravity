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
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// rollbackDependentsErrorMsg returns an error message for when a phase is being
// rolled back, but has dependent phases that have not yet been rolled back.
func rollbackDependentsErrorMsg(phaseID string, dependents []string) string {
	const msg = `Phase %[1]s cannot be rolled back because some phases that depend on it haven't been rolled back yet. Please rollback the following phases first:

	%[2]s

You can pass --force flag to override this check and force phase %[1]s rollback.`
	return fmt.Sprintf(msg, phaseID, strings.Join(dependents, "\n\t"))
}

// CanRollback checks if specified phase can be rolled back
func CanRollback(plan storage.OperationPlan, phaseID string) error {
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

	// TODO: Rollback of non-leaf phases is not currently supported.
	// Rollback starts top-down, and not in reverse order.
	if phase.HasSubphases() {
		return trace.BadParameter(
			"rolling back phases that have sub-phases is not supported. Please rollback individual phases").
			AddField("phase", phase.ID)
	}

	requiresRollback := getRequiresRollback(plan, phase.ID)
	if len(requiresRollback) != 0 {
		return trace.BadParameter(rollbackDependentsErrorMsg(phase.ID, requiresRollback))
	}

	return nil
}

// getRequired constructs the initial set of required phases. This set includes
// the phase specified by phaseID and its parent phases. Returns nil if phases
// does not contain a phase with phaseID.
//
// Given a list of phases like:
//
//	/init
//	/masters
//		* /node-1
//			* /system-upgrade
//		* /node-2
//
// and a phaseID "/masters/node-1/system-upgrade" will return the set:
//
// {"/masters", "/masters/node-1", "/masters/node-1/system-upgrade"}
func getRequired(phases []storage.OperationPhase, phaseID string) map[string]struct{} {
	for _, phase := range phases {
		if phase.ID == phaseID {
			return map[string]struct{}{
				phaseID: {},
			}
		}
		required := getRequired(phase.Phases, phaseID)
		if required != nil {
			required[phase.ID] = struct{}{}
			return required
		}
	}
	return nil
}

// getRequiresRollback returns a list of phases that need to be rolled back
// before the phase specified by phaseID can be rolled back.
func getRequiresRollback(plan storage.OperationPlan, phaseID string) (dependents []string) {
	// required will be nil if an invalid phaseID is provided.
	required := getRequired(plan.Phases, phaseID)
	if required == nil {
		return dependents
	}
	return getRequiresRollbackHelper(required, plan.Phases)
}

// getRequiresRollbackHelper is a recursive helper function that returns a list
// of dependent phases that have been started and have not been rolled back.
func getRequiresRollbackHelper(required map[string]struct{}, phases []storage.OperationPhase) (dependents []string) {
	if len(phases) == 0 {
		return dependents
	}

	for _, phase := range phases {
		if isDependent(required, phase) {
			if !phase.IsUnstarted() && !phase.IsRolledBack() {
				// Append phase to list of dependents that need to be rolled back.
				dependents = append(dependents, phase.ID)
			}
			// Add phase to the required set. Phases dependent on this phase are
			// also dependents of the original set of required phases.
			required[phase.ID] = struct{}{}
		}
		// Append any dependent sub phases that need to be rolled back.
		dependents = append(dependents, getRequiresRollbackHelper(required, phase.Phases)...)
	}

	return dependents
}

// isDependent returns true if the phase requires any of the phases contained in
// the required set.
func isDependent(required map[string]struct{}, phase storage.OperationPhase) bool {
	for _, phaseID := range phase.Requires {
		if _, exists := required[phaseID]; exists {
			return true
		}
	}
	return false
}

// IsCompleted returns true if all phases of the provided plan are completed
func IsCompleted(plan storage.OperationPlan) bool {
	for _, phase := range plan.GetLeafPhases() {
		if !phase.IsCompleted() {
			return false
		}
	}
	return true
}

// IsRolledBack returns true if the provided plan is rolled back.
func IsRolledBack(plan storage.OperationPlan) bool {
	for _, phase := range plan.GetLeafPhases() {
		if !phase.IsRolledBack() && !phase.IsUnstarted() {
			return false
		}
	}
	return true
}

// MarkCompleted marks all phases of the plan as completed
func MarkCompleted(plan storage.OperationPlan) storage.OperationPlan {
	VisitPlanRef(&plan, func(phase *storage.OperationPhase) bool {
		phase.State = storage.OperationPhaseStateCompleted
		return true
	})
	return plan
}

// HasFailed returns true if the provided plan has at least one failed phase
func HasFailed(plan storage.OperationPlan) bool {
	var hasFailedPhase bool
	VisitPlan(plan, func(phase storage.OperationPhase) bool {
		if phase.IsFailed() {
			hasFailedPhase = true
			return false
		}
		return true
	})
	return hasFailedPhase
}

// IsFailed returns true if all phases of the provided plan are either rolled back or unstarted
func IsFailed(plan storage.OperationPlan) bool {
	allFailed := true
	VisitPlan(plan, func(phase storage.OperationPhase) bool {
		if !phase.IsFailed() && !phase.IsRolledBack() && !phase.IsUnstarted() {
			allFailed = false
			return false
		}
		return true
	})
	return allFailed
}

// FindPhase finds a phase with the specified id in the provided plan
func FindPhase(plan storage.OperationPlan, phaseID string) (result *storage.OperationPhase, err error) {
	VisitPlan(plan, func(phase storage.OperationPhase) bool {
		if phase.ID == phaseID {
			result = &phase
			return false
		}
		return true
	})
	if result == nil {
		return nil, trace.NotFound("phase %q not found", phaseID)
	}
	return result, nil
}

// GetNumPhases computes the number of phases in the given plan
func GetNumPhases(plan storage.OperationPlan) (numPhases int) {
	VisitPlan(plan, func(storage.OperationPhase) bool {
		numPhases++
		return true
	})
	return numPhases
}

// VisitPlan executes the specified callback on each phase in the plan
func VisitPlan(plan storage.OperationPlan, cb func(phase storage.OperationPhase) bool) {
	for i := range plan.Phases {
		if !visitPhases(plan.Phases[i], cb) {
			return
		}
	}
}

// VisitPlanRef executes the specified callback on each phase in the plan.
// The callback receives a mutable reference so any changes are persisted
func VisitPlanRef(plan *storage.OperationPlan, cb func(phase *storage.OperationPhase) bool) {
	phases := plan.Phases
	plan.Phases = make([]storage.OperationPhase, len(phases))
	copy(plan.Phases, phases)
	for i := range plan.Phases {
		if !visitPhasesRef(&plan.Phases[i], cb) {
			return
		}
	}
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
	result := ResolvePlan(*plan, changelog)
	return &result, nil
}

// ResolvePlan applies changelog to the provided plan and returns the resulting plan
func ResolvePlan(plan storage.OperationPlan, changelog storage.PlanChangelog) storage.OperationPlan {
	clonedPlan := plan
	VisitPlanRef(&clonedPlan, func(phase *storage.OperationPhase) bool {
		latest := changelog.Latest(phase.ID)
		if latest != nil {
			phase.State = latest.NewState
			phase.Updated = latest.Created
			phase.Error = latest.Error
		}
		return true
	})
	return clonedPlan
}

// diffPlan returns the difference between the previous and the next plans in the
// form of a changelog.
func diffPlan(prevPlan *storage.OperationPlan, nextPlan storage.OperationPlan) (diff []storage.PlanChange, err error) {
	// If the current plan is not provided, the diff is all attempted phases
	// from the next plan.
	if prevPlan == nil {
		return GetPlanProgress(nextPlan), nil
	}
	// Quick sanity check that this is the same plan.
	if prevPlan.OperationID != nextPlan.OperationID {
		return nil, trace.BadParameter("could not diff different plans: %v %v (this is a bug)", prevPlan, nextPlan)
	}
	// Since this is the same plan, should be safe to assume they have the
	// same phases with different states.
	prevPhases := prevPlan.GetLeafPhases()
	nextPhases := nextPlan.GetLeafPhases()
	if len(prevPhases) != len(nextPhases) {
		return nil, trace.BadParameter("plans have different lengths: %v %v (this is a bug)", prevPlan, nextPlan)
	}
	for i, prevPhase := range prevPhases {
		nextPhase := nextPhases[i]
		if prevPhase.ID != nextPhase.ID {
			return nil, trace.BadParameter("phase ids don't match: %v %v (this is a bug)", prevPhase, nextPhase)
		}
		if prevPhase.State != nextPhase.State || prevPhase.Updated != nextPhase.Updated {
			diff = append(diff, storage.PlanChange{
				ClusterName: nextPlan.ClusterName,
				OperationID: nextPlan.OperationID,
				PhaseID:     nextPhase.ID,
				PhaseIndex:  i,
				NewState:    nextPhase.State,
				Created:     nextPhase.Updated,
				Error:       nextPhase.Error,
			})
		}
	}
	return diff, nil
}

// GetPlanProgress returns phases of the plan that have been executed so far
// in the form of a changelog.
func GetPlanProgress(plan storage.OperationPlan) (progress []storage.PlanChange) {
	for i, phase := range plan.GetLeafPhases() {
		if !phase.IsUnstarted() {
			progress = append(progress, storage.PlanChange{
				ClusterName: plan.ClusterName,
				OperationID: plan.OperationID,
				PhaseID:     phase.ID,
				PhaseIndex:  i,
				NewState:    phase.State,
				Created:     phase.Updated,
				Error:       phase.Error,
			})
		}
	}
	return progress
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
func RequireIfPresent(plan storage.OperationPlan, phaseIDs ...string) []string {
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
func CompleteOrFailOperation(ctx context.Context, plan storage.OperationPlan, operator ops.Operator, planErr string) (err error) {
	key := OperationKey(plan)
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

// visitPhasesRef walks the specified phase and its subphases executing the given callback
// for each phase as long as it returns true.
func visitPhasesRef(phase *storage.OperationPhase, cb func(phase *storage.OperationPhase) bool) bool {
	if !cb(phase) {
		return false
	}
	if len(phase.Phases) > 0 {
		phases := phase.Phases
		phase.Phases = make([]storage.OperationPhase, len(phases))
		copy(phase.Phases, phases)
		for i := range phase.Phases {
			if !visitPhasesRef(&phase.Phases[i], cb) {
				return false
			}
		}
	}
	return true
}

// visitPhases walks the specified phase and its subphases executing the given callback
// for each phase as long as it returns true.
func visitPhases(phase storage.OperationPhase, cb func(phase storage.OperationPhase) bool) bool {
	if !cb(phase) {
		return false
	}
	for i := range phase.Phases {
		if !visitPhases(phase.Phases[i], cb) {
			return false
		}
	}
	return true
}

// CheckPlanCoordinator ensures that the node this function is invoked on is the
// coordinator node specified in the plan.
//
// This is mainly important for making sure the plan is executed on the lead
// node for a particular plan - for example, for etcd upgrades, where state
// can only be kept in sync on the lead master node itself.
func CheckPlanCoordinator(p *storage.OperationPlan) error {
	if p.OfflineCoordinator == nil {
		return nil
	}
	err := systeminfo.HasInterface(p.OfflineCoordinator.AdvertiseIP)
	if err != nil && trace.IsNotFound(err) {
		return trace.BadParameter("Plan must be resumed on node %v/%v",
			p.OfflineCoordinator.Hostname, p.OfflineCoordinator.AdvertiseIP)
	}
	return trace.Wrap(err)
}
