package fsm

import (
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
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
	for _, phase := range FlattenPlan(plan) {
		if !phase.IsCompleted() {
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

func addPhases(phase *storage.OperationPhase, result *[]*storage.OperationPhase) {
	// add the phase itself
	*result = append(*result, phase)
	// as well as all its subphases and their subphases recursively
	for i := range phase.Phases {
		addPhases(&phase.Phases[i], result)
	}
}
