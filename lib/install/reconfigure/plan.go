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
func NewPlanner(getter install.PlanBuilderGetter, cluster storage.Site) *Planner {
	return &Planner{
		PlanBuilderGetter: getter,
		Cluster:           cluster,
	}
}

// GetOperationPlan creates operation plan for the reconfigure operation.
func (p *Planner) GetOperationPlan(operator ops.Operator, cluster ops.Site, operation ops.SiteOperation) (*storage.OperationPlan, error) {
	masters, _ := fsm.SplitServers(operation.Servers)
	if len(masters) == 0 {
		return nil, trace.BadParameter(
			"at least one master server is required: %v", operation.Servers)
	}

	teleportPackage, err := cluster.App.Manifest.Dependencies.ByName(
		constants.TeleportPackage)
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
	}

	plan := &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       append(builder.Masters, builder.Nodes...),
		DNSConfig:     cluster.DNSConfig,
	}

	builder.AddPreCleanupPhase(plan)
	builder.AddChecksPhase(plan)
	builder.AddConfigurePhase(plan)
	builder.AddPullPhase(plan)
	builder.AddMastersPhase(plan)
	builder.AddWaitPhase(plan)
	builder.AddHealthPhase(plan)
	builder.AddPostCleanupPhase(plan)

	return plan, nil
}

// Planner creates operation plans for the reconfigure operation.
type Planner struct {
	// PlanBuilderGetter allows to retrieve install plan builder.
	install.PlanBuilderGetter
	// Cluster is the installed cluster.
	Cluster storage.Site
}
