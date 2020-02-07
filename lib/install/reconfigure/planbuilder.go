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

package reconfigure

import (
	"fmt"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/install/reconfigure/phases"
	"github.com/gravitational/gravity/lib/storage"
)

// PlanBuilder builds plan for the reconfigure operation.
type PlanBuilder struct {
	// PlanBuilder is the embedded installer plan builder.
	*install.PlanBuilder
}

// AddChecksPhase adds the preflight checks phase to the plan.
func (b *PlanBuilder) AddChecksPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.ChecksPhase,
		Description: "Execute preflight checks",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddPreCleanupPhase adds the phase that does pre-reconfiguration cleanups
// on the node to the plan.
func (b *PlanBuilder) AddPreCleanupPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.PreCleanupPhase,
		Description: "Perform pre advertise IP change cleanups on the node",
		Phases: []storage.OperationPhase{
			{
				ID:          fmt.Sprintf("%v%v", phases.PreCleanupPhase, phases.NetworkPhase),
				Description: "Clean up old network interfaces",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
			{
				ID:          fmt.Sprintf("%v%v", phases.PreCleanupPhase, phases.PackagesPhase),
				Description: "Clean up packages in the local state",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
			{
				ID:          fmt.Sprintf("%v%v", phases.PreCleanupPhase, phases.DirectoriesPhase),
				Description: "Clean up local directories",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
		},
	})
}

// AddPostCleanupPhase adds the phase that does post-reconfiguration cleanups
// on the node to the plan.
func (b *PlanBuilder) AddPostCleanupPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.PostCleanupPhase,
		Description: "Perform post advertise IP change cleanups on the node",
		Requires:    fsm.RequireIfPresent(plan, installphases.HealthPhase),
		Phases: []storage.OperationPhase{
			{
				ID:          fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.StatePhase),
				Description: "Update cluster state",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
			{
				ID:          fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.TokensPhase),
				Description: "Clean up old Kubernetes service account tokens",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
			{
				ID:          fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.NodePhase),
				Description: "Clean up old Kubernetes node",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
			{
				ID:          fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.PodsPhase),
				Description: "Clean up old Kubernetes pods",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
			{
				ID:          fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.GravityPhase),
				Description: "Wait for Gravity to become available",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
			{
				ID:          fmt.Sprintf("%v%v", phases.PostCleanupPhase, phases.PackagesPhase),
				Description: "Clean up packages in the cluster state",
				Data: &storage.OperationPhaseData{
					Server: &b.Master,
				},
			},
		},
	})
}
