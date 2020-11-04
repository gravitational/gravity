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

	"github.com/gravitational/gravity/lib/install"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/install/reconfigure/phases"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
)

// PlanBuilder builds plan for the reconfigure operation.
type PlanBuilder struct {
	// PlanBuilder is the embedded installer plan builder.
	*install.PlanBuilder
	runtimePackage loc.Locator
}

// AddChecksPhase adds the preflight checks phase to the plan.
func (b *PlanBuilder) AddChecksPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.ChecksPhase,
		Description: "Execute pre-flight checks",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddNetworkPhase adds phase that cleans up old network interfaces.
func (b *PlanBuilder) AddNetworkPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.NetworkPhase,
		Description: "Clean up old network interfaces",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddLocalPackagesPhase adds phase that cleans up packages in the local state.
func (b *PlanBuilder) AddLocalPackagesPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.LocalPackagesPhase,
		Description: "Clean up packages in the local state",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddEtcdPhase updates etcd member's peer advertise URL.
func (b *PlanBuilder) AddEtcdPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.EtcdPhase,
		Description: "Update etcd member peer advertise URL",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddStatePhase adds phase that updates cluster state in the database.
func (b *PlanBuilder) AddStatePhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.StatePhase,
		Description: "Update cluster state",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddTokensPhase adds phase that cleans up old service account tokens.
func (b *PlanBuilder) AddTokensPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.TokensPhase,
		Description: "Clean up old Kubernetes service account tokens",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddNodePhase adds phase that cleans up old node object.
func (b *PlanBuilder) AddNodePhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.NodePhase,
		Description: "Clean up old Kubernetes node",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddDirectoriesPhase adds phase that cleans up old directories.
func (b *PlanBuilder) AddDirectoriesPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.DirectoriesPhase,
		Description: "Clean up local directories",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddPodsPhase adds phase that recreates all pods.
func (b *PlanBuilder) AddPodsPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.PodsPhase,
		Description: "Clean up old Kubernetes pods",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddRestartPhase adds phase that restarts Teleport and Planet units.
func (b *PlanBuilder) AddRestartPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.RestartPhase,
		Description: "Restart Gravity services",
		Phases: []storage.OperationPhase{
			{
				ID:          fmt.Sprintf("%v%v", phases.RestartPhase, phases.TeleportPhase),
				Description: "Restart Teleport",
				Data: &storage.OperationPhaseData{
					Server:  &b.Master,
					Package: &loc.Teleport,
				},
			},
			{
				ID:          fmt.Sprintf("%v%v", phases.RestartPhase, phases.PlanetPhase),
				Description: "Restart Planet",
				Data: &storage.OperationPhaseData{
					Server:  &b.Master,
					Package: &b.runtimePackage,
				},
			},
		},
	})
}

// AddGravityPhase adds phase that waits for gravity to become available.
func (b *PlanBuilder) AddGravityPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.GravityPhase,
		Description: "Wait for Gravity to become available",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}

// AddClusterPackagesPhase adds phase that cleans up packages in the cluster state.
func (b *PlanBuilder) AddClusterPackagesPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.ClusterPackagesPhase,
		Description: "Clean up packages in the cluster state",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
	})
}
