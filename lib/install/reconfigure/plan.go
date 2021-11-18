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
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// NewPlanner returns reconfigure operation plan builder.
func NewPlanner(getter install.PlanBuilderGetter, cluster storage.Site, role string) *Planner {
	return &Planner{
		PlanBuilderGetter: getter,
		Cluster:           cluster,
		nodeRole:          role,
	}
}

// GetOperationPlan creates operation plan for the reconfigure operation.
func (p *Planner) GetOperationPlan(operator ops.Operator, cluster ops.Site, operation ops.SiteOperation) (*storage.OperationPlan, error) {
	// Advertise IP can only be reconfigured on single-node clusters atm.
	masters, nodes := fsm.SplitServers(operation.Servers)
	if len(masters) != 1 || len(nodes) != 0 {
		return nil, trace.BadParameter("the reconfigure operation only supports single-node clusters, but got: %s", operation.Servers)
	}

	teleportPackage, err := cluster.App.Manifest.Dependencies.ByName(constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	runtimePackage, err := cluster.App.Manifest.RuntimePackageForProfile(p.nodeRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The "reconfigure" operation reuses a lot of the install fsm phases.
	builder := &PlanBuilder{
		PlanBuilder: &install.PlanBuilder{
			Cluster:   ops.ConvertOpsSite(cluster),
			Operation: operation,
			Application: app.Application{
				Package:         cluster.App.Package,
				PackageEnvelope: cluster.App.PackageEnvelope,
				Manifest:        cluster.App.Manifest,
			},
			Masters:         masters,
			Master:          masters[0],
			ServiceUser:     p.Cluster.ServiceUser,
			TeleportPackage: *teleportPackage,
		},
		runtimePackage: *runtimePackage,
	}

	plan := &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       append(builder.Masters, builder.Nodes...),
		DNSConfig:     cluster.DNSConfig,
	}

	builder.AddNetworkPhase(plan)
	builder.AddLocalPackagesPhase(plan)
	builder.AddChecksPhase(plan)
	builder.AddConfigurePhase(plan)
	builder.AddPullPhase(plan)
	if err := builder.AddMastersPhase(plan); err != nil {
		return nil, trace.Wrap(err)
	}
	builder.AddWaitPhase(plan)
	builder.AddHealthPhase(plan)
	builder.AddEtcdPhase(plan)
	builder.AddStatePhase(plan)
	builder.AddTokensPhase(plan)
	builder.AddCorednsPhase(plan)
	builder.AddNodePhase(plan)
	builder.AddDirectoriesPhase(plan)
	builder.AddPodsPhase(plan)
	builder.AddRestartPhase(plan)
	builder.AddGravityPhase(plan)
	builder.AddClusterPackagesPhase(plan)

	return plan, nil
}

// Planner creates operation plans for the reconfigure operation.
type Planner struct {
	// PlanBuilderGetter allows to retrieve install plan builder.
	install.PlanBuilderGetter
	// Cluster is the installed cluster.
	Cluster storage.Site
	// nodeRole specifies the node's application role
	nodeRole string
}
