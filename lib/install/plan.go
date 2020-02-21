/*
Copyright 2018-2019 Gravitational, Inc.

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

package install

import (
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// NewPlanner returns a new instance of Planner with the specified builder getter
func NewPlanner(preflightChecks bool, builderGetter PlanBuilderGetter) *Planner {
	return &Planner{
		PlanBuilderGetter: builderGetter,
		preflightChecks:   preflightChecks,
	}
}

// GetOperationPlan builds a plan for the provided operation
func (r *Planner) GetOperationPlan(operator ops.Operator, cluster ops.Site, operation ops.SiteOperation) (*storage.OperationPlan, error) {
	builder, err := r.GetPlanBuilder(operator, cluster, operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan := &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       append(builder.Masters, builder.Nodes...),
		DNSConfig:     cluster.DNSConfig,
		SELinux:       cluster.SELinux,
	}

	if cluster.SELinux {
		builder.AddBootstrapSELinuxPhase(plan)
	}

	// perform some initialization on all nodes
	builder.AddInitPhase(plan)

	if r.preflightChecks {
		builder.AddChecksPhase(plan)
	}

	// configure packages for all nodes
	builder.AddConfigurePhase(plan)

	// bootstrap each node: setup directories, users, etc.
	builder.AddBootstrapPhase(plan)

	// pull configured packages on each node
	builder.AddPullPhase(plan)

	// install system software on master nodes
	if err := builder.AddMastersPhase(plan); err != nil {
		return nil, trace.Wrap(err)
	}

	// (optional) install system software on regular nodes
	if len(builder.Nodes) > 0 {
		if err := builder.AddNodesPhase(plan); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// perform post system install tasks such as waiting for planet
	// to start up, creating RBAC resources, etc.
	builder.AddWaitPhase(plan)
	builder.AddRBACPhase(plan)
	builder.AddCorednsPhase(plan)

	// create OpenEBS configuration if it's enabled, it has to be done
	// before OpenEBS is installed during the runtime phase
	if cluster.App.Manifest.OpenEBSEnabled() {
		if err := builder.AddOpenEBSPhase(plan); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// create system and user-supplied Kubernetes resources
	builder.AddSystemResourcesPhase(plan)
	builder.AddUserResourcesPhase(plan)

	// export applications to registries
	builder.AddExportPhase(plan)

	if cluster.App.Manifest.HasHook(schema.HookNetworkInstall) {
		builder.AddInstallOverlayPhase(plan, &cluster.App.Package)
	}
	builder.AddHealthPhase(plan)

	// install runtime application
	err = builder.AddRuntimePhase(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// install user application
	err = builder.AddApplicationPhase(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// establish trust b/w installed cluster and installer process
	err = builder.AddConnectInstallerPhase(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// re-enable planet leader elections
	builder.AddEnableElectionPhase(plan)

	// Add a phase to create optional Gravity resources upon successful installation
	builder.AddGravityResourcesPhase(plan)

	return plan, nil
}

// PlanBuilderGetter is a factory for plan builders
type PlanBuilderGetter interface {
	// GetPlanBuilder returns a new plan builder for the specified cluster and operation
	GetPlanBuilder(operator ops.Operator, cluster ops.Site, operation ops.SiteOperation) (*PlanBuilder, error)
}

// Planner builds an install operation plan
type Planner struct {
	PlanBuilderGetter
	preflightChecks bool
}
