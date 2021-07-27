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

package expand

import (
	"fmt"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
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
	// GravityPackage is the gravity package to install
	GravityPackage loc.Locator
	// TeleportPackage is the teleport package to install
	TeleportPackage loc.Locator
	// PlanetPackage is the planet package to install
	PlanetPackage loc.Locator
	// JoiningNode is the node that's joining to the cluster
	JoiningNode storage.Server
	// ClusterNodes is the list of existing cluster nodes
	ClusterNodes storage.Servers
	// Peer is the IP:port of the cluster node this peer is joining to
	Peer string
	// Master is one of the cluster's existing master nodes
	Master storage.Server
	// AdminAgent is the cluster agent with admin privileges
	AdminAgent storage.LoginEntry
	// RegularAgent is the cluster agent with non-admin privileges
	RegularAgent storage.LoginEntry
	// ServiceUser is the cluster system user
	ServiceUser storage.OSUser
	// DNSConfig specifies the custom cluster DNS configuration
	DNSConfig storage.DNSConfig
}

// AddInitPhase appends initialization phase to the plan.
func (b *planBuilder) AddInitPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.InitPhase,
		Description: "Initialize operation on the joining node",
		Data: &storage.OperationPhaseData{
			Server:  &b.JoiningNode,
			Master:  &b.Master,
			Package: &b.Application.Package,
		},
	})
}

// AddBootstrapSELinuxPhase appends the phase to configure SELinux on a node
func (b *planBuilder) AddBootstrapSELinuxPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.BootstrapSELinuxPhase,
		Description: "Configure SELinux",
		Data: &storage.OperationPhaseData{
			Server:     &b.JoiningNode,
			ExecServer: &b.JoiningNode,
			Package:    &b.Application.Package,
		},
	})
}

// AddChecksPhase appends preflight checks phase to the plan.
func (b *planBuilder) AddChecksPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          ChecksPhase,
		Description: "Execute pre-flight checks on the joining node",
		Data: &storage.OperationPhaseData{
			Server: &b.JoiningNode,
			Master: &b.Master,
		},
		Requires: fsm.RequireIfPresent(*plan,
			installphases.BootstrapSELinuxPhase,
			installphases.InitPhase,
			StartAgentPhase),
	})
}

// AddConfigurePhase appends package configuration phase to the plan
func (b *planBuilder) AddConfigurePhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.ConfigurePhase,
		Description: "Configure packages for the joining node",
		Data: &storage.OperationPhaseData{
			ExecServer: &b.JoiningNode,
		},
		Requires: []string{ChecksPhase},
	})
}

// AddBootstrapPhase appends local node bootstrap phase to the plan
func (b *planBuilder) AddBootstrapPhase(plan *storage.OperationPlan) {
	agent := &b.AdminAgent
	if !b.JoiningNode.IsMaster() {
		agent = &b.RegularAgent
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.BootstrapPhase,
		Description: "Bootstrap the joining node",
		Data: &storage.OperationPhaseData{
			Server:      &b.JoiningNode,
			ExecServer:  &b.JoiningNode,
			Package:     &b.Application.Package,
			Agent:       agent,
			ServiceUser: &b.ServiceUser,
		},
	})
}

// AddPullPhase appends package pull phase to the plan
func (b *planBuilder) AddPullPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.PullPhase,
		Description: "Pull packages on the joining node",
		Data: &storage.OperationPhaseData{
			Server:      &b.JoiningNode,
			ExecServer:  &b.JoiningNode,
			Package:     &b.Application.Package,
			ServiceUser: &b.ServiceUser,
			Pull: &storage.PullData{
				Packages: []loc.Locator{
					b.GravityPackage,
					b.TeleportPackage,
					b.PlanetPackage,
				},
			},
		},
		Requires: []string{installphases.ConfigurePhase, installphases.BootstrapPhase},
	})
}

// AddPreHookPhase appends pre-expand hook phase to the plan
func (b *planBuilder) AddPreHookPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          PreHookPhase,
		Description: fmt.Sprintf("Execute the application's %v hook", schema.HookNodeAdding),
		Data: &storage.OperationPhaseData{
			ExecServer:  &b.JoiningNode,
			Package:     &b.Application.Package,
			ServiceUser: &b.ServiceUser,
		},
		Requires: []string{installphases.PullPhase},
	})
}

// AddSystemPhase appends teleport/planet installation phase to the plan
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
					Server:      &b.JoiningNode,
					ExecServer:  &b.JoiningNode,
					Package:     &b.TeleportPackage,
					ServiceUser: &b.ServiceUser,
				},
				Requires: []string{installphases.PullPhase},
			},
			{
				ID: fmt.Sprintf("%v/planet", SystemPhase),
				Description: fmt.Sprintf("Install system package %v:%v",
					b.PlanetPackage.Name, b.PlanetPackage.Version),
				Data: &storage.OperationPhaseData{
					Server:      &b.JoiningNode,
					ExecServer:  &b.JoiningNode,
					Package:     &b.PlanetPackage,
					ServiceUser: &b.ServiceUser,
					Labels:      pack.RuntimePackageLabels,
				},
				Requires: []string{installphases.PullPhase},
			},
		},
	})
}

// AddStartAgentPhase appends phase that starts agent on a master node
func (b *planBuilder) AddStartAgentPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID: StartAgentPhase,
		Description: fmt.Sprintf("Start RPC agent on the master node %v",
			b.Master.AdvertiseIP),
		Data: &storage.OperationPhaseData{
			ExecServer: &b.JoiningNode,
			Server:     &b.Master,
			Agent: &storage.LoginEntry{
				Email:        b.AdminAgent.Email,
				Password:     b.AdminAgent.Password,
				OpsCenterURL: fmt.Sprintf("https://%v", b.Peer),
			},
		},
	})
}

// AddEtcdBackupPhase appends etcd data backup phase to the plan
func (b *planBuilder) AddEtcdBackupPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID: EtcdBackupPhase,
		Description: fmt.Sprintf("Backup etcd data on the master node %v",
			b.Master.AdvertiseIP),
		Data: &storage.OperationPhaseData{
			Server:     &b.Master,
			ExecServer: &b.JoiningNode,
		},
		Requires: []string{StartAgentPhase},
	})
}

// AddEtcdPhase appends etcd member addition phase to the plan
func (b *planBuilder) AddEtcdPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          EtcdPhase,
		Description: "Add the joining node to the etcd cluster",
		Data: &storage.OperationPhaseData{
			Server:     &b.JoiningNode,
			ExecServer: &b.JoiningNode,
			Master:     &b.Master,
		},
		Requires: fsm.RequireIfPresent(*plan, SystemPhase, EtcdBackupPhase),
	})
}

// AddWaitPhase appends planet startup wait phase to the plan
func (b *planBuilder) AddWaitPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          installphases.WaitPhase,
		Description: "Wait for the node to join the cluster",
		Phases: []storage.OperationPhase{
			{
				ID:          WaitPlanetPhase,
				Description: "Wait for the planet to start",
				Data: &storage.OperationPhaseData{
					Server:     &b.JoiningNode,
					ExecServer: &b.JoiningNode,
				},
				Requires: fsm.RequireIfPresent(*plan, SystemPhase, EtcdPhase),
			},
			{
				ID:          WaitK8sPhase,
				Description: "Wait for the node to join Kubernetes cluster",
				Data: &storage.OperationPhaseData{
					Server:     &b.JoiningNode,
					ExecServer: &b.JoiningNode,
				},
				Requires: []string{WaitPlanetPhase},
			},
			{
				ID:          WaitTeleportPhase,
				Description: "Wait for the Teleport node to join cluster",
				Data: &storage.OperationPhaseData{
					Server:     &b.JoiningNode,
					ExecServer: &b.JoiningNode,
				},
				Requires: []string{WaitPlanetPhase},
			},
		},
	})
}

// AddStopAgentPhase appends phase that stops RPC agent on a master node
func (b *planBuilder) AddStopAgentPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID: StopAgentPhase,
		Description: fmt.Sprintf("Stop RPC agent on the master node %v",
			b.Master.AdvertiseIP),
		Data: &storage.OperationPhaseData{
			ExecServer: &b.JoiningNode,
			Server:     &b.Master,
		},
		Requires: []string{installphases.WaitPhase},
	})
}

// AddPostHookPhase appends post-expand hook phase to the plan
func (b *planBuilder) AddPostHookPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          PostHookPhase,
		Description: fmt.Sprintf("Execute the application's %v hook", schema.HookNodeAdded),
		Data: &storage.OperationPhaseData{
			ExecServer:  &b.JoiningNode,
			Package:     &b.Application.Package,
			ServiceUser: &b.ServiceUser,
		},
		Requires: []string{installphases.WaitPhase},
	})
}

// AddElectPhase appends phase that enables leader election to the plan
func (b *planBuilder) AddElectPhase(plan *storage.OperationPlan) {
	phase := storage.OperationPhase{
		ID:          ElectPhase,
		Description: "Enable leader election on the joined node",
		Data: &storage.OperationPhaseData{
			Server:     &b.JoiningNode,
			ExecServer: &b.JoiningNode,
		},
		Requires: []string{installphases.WaitPhase},
	}
	if !b.JoiningNode.IsMaster() {
		phase.Description = "Disable leader election on the joined node"
	}
	plan.Phases = append(plan.Phases, phase)
}

func (p *Peer) getPlanBuilder(ctx operationContext) (*planBuilder, error) {
	application, err := ctx.Apps.GetApp(ctx.Cluster.App.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	base := application.Manifest.Base()
	if base == nil {
		return nil, trace.NotFound("application %v does not have a runtime",
			ctx.Cluster.App.Package)
	}
	runtime, err := ctx.Apps.GetApp(*base)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gravityPackage, err := application.Manifest.Dependencies.ByName(
		constants.GravityPackage)
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
		GravityPackage:  *gravityPackage,
		TeleportPackage: *teleportPackage,
		PlanetPackage:   *planetPackage,
		JoiningNode:     operation.Servers[0],
		ClusterNodes:    ctx.Cluster.ClusterState.Servers,
		Peer:            ctx.Peer,
		Master:          ctx.Cluster.ClusterState.Servers.Masters()[0],
		AdminAgent:      *adminAgent,
		RegularAgent:    *regularAgent,
		ServiceUser:     ctx.Cluster.ServiceUser,
		DNSConfig:       ctx.Cluster.DNSConfig,
	}, nil
}

// fillSteps assigns each phase of the provided plan a step number that will
// be used in the UI to display a progress bar.
//
// The UI currently only supports a fixed number of steps (specified by the
// provided max number) so the plan's phase numbers will be calculated to
// fit within the specified interval.
func fillSteps(plan *storage.OperationPlan, maxSteps int) {
	numPhases := fsm.GetNumPhases(*plan)
	var step int
	fsm.VisitPlanRef(plan, func(phase *storage.OperationPhase) bool {
		phase.Step = calcStep(maxSteps, numPhases, step)
		step += 1
		return true
	})
}

// calcStep adjusts the provided step number so it does not exceed the specified
// maximum number.
func calcStep(maxSteps, actualSteps, stepNumber int) int {
	return int(float64(maxSteps) / float64(actualSteps) * float64(stepNumber+1))
}
