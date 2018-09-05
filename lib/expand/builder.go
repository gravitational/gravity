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

package expand

import (
	"fmt"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// planBuilder builds expand operation plan
type planBuilder struct {
	// Application is the app being installed
	Application app.Application
	// Runtime is the Runtime of the app being installed
	Runtime app.Application
	// TeleportPackage is the teleport package to install
	TeleportPackage loc.Locator
	// PlanetPackage is the planet package to install
	PlanetPackage loc.Locator
	// Node is the node that's joining to the cluster
	Node storage.Server
	// Nodes is the list of existing cluster nodes
	Nodes []storage.Server
	// AdminAgent is the cluster agent with admin privileges
	AdminAgent storage.LoginEntry
	// RegularAgent is the cluster agent with non-admin privileges
	RegularAgent storage.LoginEntry
	// ServiceUser is the cluster system user
	ServiceUser storage.OSUser
}

func (b *planBuilder) AddConfigurePhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.ConfigurePhase,
		Description: "Configure packages for the joining node",
	})
}

func (b *planBuilder) AddBootstrapPhase(plan *storage.OperationPlan) {
	agent := &b.AdminAgent
	if b.Node.ClusterRole != string(schema.ServiceRoleMaster) {
		agent = &b.RegularAgent
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.BootstrapPhase,
		Description: "Bootstrap the joining node",
		Data: &storage.OperationPhaseData{
			Server:      &b.Node,
			Package:     &b.Application.Package,
			Agent:       agent,
			ServiceUser: &b.ServiceUser,
		},
	})
}

func (b *planBuilder) AddPullPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.PullPhase,
		Description: "Pull packages on the joining node",
		Data: &storage.OperationPhaseData{
			Server:      &b.Node,
			Package:     &b.Application.Package,
			ServiceUser: &b.ServiceUser,
		},
	})
}

func (b *planBuilder) AddPreHookPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          PreHookPhase,
		Description: fmt.Sprintf("Execute the application's %v hook", schema.HookNodeAdding),
		Data: &storage.OperationPhaseData{
			Package:     &b.Application.Package,
			ServiceUser: &b.ServiceUser,
		},
	})
}

func (b *planBuilder) AddEtcdPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          EtcdPhase,
		Description: "Add the joining node to the etcd cluster",
	})
}

func (b *planBuilder) AddSystemPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          SystemPhase,
		Description: "Install system software on the joining node",
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/teleport", SystemPhase),
				Description: fmt.Sprintf("Install system package %v:%v",
					b.TeleportPackage.Name, b.TeleportPackage.Version),
				Data: &storage.OperationPhaseData{
					Server:  &b.Node,
					Package: &b.TeleportPackage,
				},
			},
			{
				ID: fmt.Sprintf("%v/planet", SystemPhase),
				Description: fmt.Sprintf("Install system package %v:%v",
					b.PlanetPackage.Name, b.PlanetPackage.Version),
				Data: &storage.OperationPhaseData{
					Server:  &b.Node,
					Package: &b.PlanetPackage,
					Labels:  pack.RuntimePackageLabels,
				},
			},
		},
	})
}

func (b *planBuilder) AddWaitPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.WaitPhase,
		Description: "Wait for the node to join the cluster",
		Phases: []storage.OperationPhase{
			{
				ID:          WaitPlanetPhase,
				Description: "Wait for the planet to start",
				Data: &storage.OperationPhaseData{
					Server: &b.Node,
				},
			},
			{
				ID:          WaitK8sPhase,
				Description: "Wait for the node to join Kubernetes cluster",
				Data: &storage.OperationPhaseData{
					Server: &b.Node,
				},
			},
		},
	})
}

func (b *planBuilder) AddPostHookPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          PostHookPhase,
		Description: fmt.Sprintf("Execute the application's %v hook", schema.HookNodeAdded),
		Data: &storage.OperationPhaseData{
			Package:     &b.Application.Package,
			ServiceUser: &b.ServiceUser,
		},
	})
}

func (b *planBuilder) AddElectPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          ElectPhase,
		Description: "Enable leader election on the joined node",
		Data: &storage.OperationPhaseData{
			Server: &b.Node,
		},
	})
}

func (p *Peer) getPlanBuilder(ctx operationContext) (*planBuilder, error) {
	application, err := ctx.Apps.GetApp(ctx.Site.App.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	base := application.Manifest.Base()
	if base == nil {
		return nil, trace.NotFound("application %v does not have a runtime",
			ctx.Site.App.Package)
	}
	runtime, err := ctx.Apps.GetApp(*base)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	teleportPackage, err := application.Manifest.Dependencies.ByName(
		constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	planetPackage, err := application.Manifest.RuntimePackageForProfile(p.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	adminAgent, err := ctx.Operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   ctx.Operation.AccountID,
		ClusterName: ctx.Operation.SiteDomain,
		Admin:       true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	regularAgent, err := ctx.Operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   ctx.Operation.AccountID,
		ClusterName: ctx.Operation.SiteDomain,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := ctx.Operator.GetSiteOperation(ctx.Operation.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(operation.Servers) == 0 {
		return nil, trace.NotFound("operation does not have servers: %v",
			operation)
	}
	return &planBuilder{
		Application:     *application,
		Runtime:         *runtime,
		TeleportPackage: *teleportPackage,
		PlanetPackage:   *planetPackage,
		Node:            operation.Servers[0],
		Nodes:           ctx.Site.ClusterState.Servers,
		AdminAgent:      *adminAgent,
		RegularAgent:    *regularAgent,
		ServiceUser:     ctx.Site.ServiceUser,
	}, nil
}
