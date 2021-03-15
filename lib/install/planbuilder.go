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

package install

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	resourceutil "github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/runtime"
)

// PlanBuilder builds operation plan phases
type PlanBuilder struct {
	// Cluster is the cluster being installed
	Cluster storage.Site
	// Operation is the operation the builder is for
	Operation ops.SiteOperation
	// Application is the app being installed
	Application app.Application
	// Runtime is the Runtime of the app being installed
	Runtime app.Application
	// TeleportPackage is the runtime teleport package
	TeleportPackage loc.Locator
	// RBACPackage is the runtime rbac app package
	RBACPackage loc.Locator
	// GravitySitePackage is the gravity-site app package
	GravitySitePackage loc.Locator
	// GravityPackage is the gravity binary package
	GravityPackage loc.Locator
	// DNSAppPackage is the dns-app app package
	DNSAppPackage loc.Locator
	// Masters is the list of master nodes
	Masters []storage.Server
	// Nodes is the list of regular nodes
	Nodes []storage.Server
	// Master is one of the master nodes
	Master storage.Server
	// AdminAgent is the cluster agent with admin privileges
	AdminAgent storage.LoginEntry
	// RegularAgent is the cluster agent with non-admin privileges
	RegularAgent storage.LoginEntry
	// ServiceUser is the cluster system user
	ServiceUser storage.OSUser
	// env specifies optional cluster environment variables to add during install
	env map[string]string
	// config specifies the optional cluster configuration
	config []byte
	// resources specifies the optional Kubernetes resources to create
	resources []byte
	// gravityResources specifies the optional Gravity resources to create upon successful install
	gravityResources []storage.UnknownResource
	// InstallerTrustedCluster represents the trusted cluster for installer process
	InstallerTrustedCluster storage.TrustedCluster
	// PersistentStorage is persistent storage resource optionally provided by
	// user at install time.
	PersistentStorage storage.PersistentStorage
}

// NumParallel limits the number of parallel phases that can be run during install
const NumParallel = 10

// AddInitPhase appends initialization phase to the provided plan
func (b *PlanBuilder) AddInitPhase(plan *storage.OperationPlan) {
	var initPhases []storage.OperationPhase
	allNodes := append(b.Masters, b.Nodes...)
	for i, node := range allNodes {
		initPhases = append(initPhases, storage.OperationPhase{
			ID:          fmt.Sprintf("%v/%v", phases.InitPhase, node.Hostname),
			Description: fmt.Sprintf("Initialize operation on node %v", node.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &allNodes[i],
				ExecServer: &allNodes[i],
				Package:    &b.Application.Package,
			},
			Step: 0,
		})
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:            phases.InitPhase,
		Description:   "Initialize operation on all nodes",
		Phases:        initPhases,
		LimitParallel: NumParallel,
		Step:          0,
	})
}

// AddBootstrapSELinuxPhase appends the phase to configure SELinux on a node
func (b *PlanBuilder) AddBootstrapSELinuxPhase(plan *storage.OperationPlan) {
	var bootstrapPhases []storage.OperationPhase
	allNodes := append(b.Masters, b.Nodes...)
	for i, node := range allNodes {
		if !node.SELinux {
			continue
		}
		bootstrapPhases = append(bootstrapPhases, storage.OperationPhase{
			ID:          fmt.Sprintf("%v/%v", phases.BootstrapSELinuxPhase, node.Hostname),
			Description: fmt.Sprintf("Configure SELinux on node %v", node.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &allNodes[i],
				ExecServer: &allNodes[i],
				Package:    &b.Application.Package,
			},
			Step: 0,
		})
	}
	if len(bootstrapPhases) == 0 {
		return
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:            phases.BootstrapSELinuxPhase,
		Description:   "Configure SELinux",
		Phases:        bootstrapPhases,
		LimitParallel: NumParallel,
	})
}

// AddChecksPhase appends preflight checks phase to the provided plan
func (b *PlanBuilder) AddChecksPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.ChecksPhase,
		Description: "Execute pre-flight checks",
		Requires:    []string{phases.InitPhase},
		Data: &storage.OperationPhaseData{
			Package: &b.Application.Package,
		},
		Step: 0,
	})
}

// AddConfigurePhase appends package configuration phase to the provided plan
func (b *PlanBuilder) AddConfigurePhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.ConfigurePhase,
		Description: "Configure packages for all nodes",
		Requires:    fsm.RequireIfPresent(plan, phases.InstallerPhase, phases.DecryptPhase),
		Data: &storage.OperationPhaseData{
			Install: &storage.InstallOperationData{
				Env:    b.env,
				Config: b.config,
			},
		},
		Step: 3,
	})
}

// AddBootstrapPhase appends nodes bootstrap phase to the provided plan
func (b *PlanBuilder) AddBootstrapPhase(plan *storage.OperationPlan) {
	var bootstrapPhases []storage.OperationPhase
	allNodes := append(b.Masters, b.Nodes...)
	for i, node := range allNodes {
		var description string
		var agent *storage.LoginEntry
		if node.ClusterRole == string(schema.ServiceRoleMaster) {
			description = "Bootstrap master node %v"
			agent = &b.AdminAgent
		} else {
			description = "Bootstrap node %v"
			agent = &b.RegularAgent
		}
		bootstrapPhases = append(bootstrapPhases, storage.OperationPhase{
			ID:          fmt.Sprintf("%v/%v", phases.BootstrapPhase, node.Hostname),
			Description: fmt.Sprintf(description, node.Hostname),
			Data: &storage.OperationPhaseData{
				Server:      &allNodes[i],
				ExecServer:  &allNodes[i],
				Package:     &b.Application.Package,
				Agent:       agent,
				ServiceUser: &b.ServiceUser,
			},
			Step: 3,
		})
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:            phases.BootstrapPhase,
		Description:   "Bootstrap all nodes",
		Phases:        bootstrapPhases,
		LimitParallel: NumParallel,
		Step:          3,
	})
}

// AddPullPhase appends package download phase to the provided plan
func (b *PlanBuilder) AddPullPhase(plan *storage.OperationPlan) error {
	var pullPhases []storage.OperationPhase
	allNodes := append(b.Masters, b.Nodes...)
	for i, node := range allNodes {
		var description string
		if node.ClusterRole == string(schema.ServiceRoleMaster) {
			description = "Pull packages on master node %v"
		} else {
			description = "Pull packages on node %v"
		}
		pullData, err := b.getPullData(node)
		if err != nil {
			return trace.Wrap(err)
		}
		pullPhases = append(pullPhases, storage.OperationPhase{
			ID:          fmt.Sprintf("%v/%v", phases.PullPhase, node.Hostname),
			Description: fmt.Sprintf(description, node.Hostname),
			Data: &storage.OperationPhaseData{
				Server:      &allNodes[i],
				ExecServer:  &allNodes[i],
				Package:     &b.Application.Package,
				ServiceUser: &b.ServiceUser,
				Pull:        pullData,
			},
			Requires: fsm.RequireIfPresent(plan, phases.ConfigurePhase, phases.BootstrapPhase),
			Step:     3,
		})
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:            phases.PullPhase,
		Description:   "Pull configured packages",
		Phases:        pullPhases,
		Requires:      fsm.RequireIfPresent(plan, phases.ConfigurePhase, phases.BootstrapPhase),
		LimitParallel: NumParallel,
		Step:          3,
	})
	return nil
}

// getPullData returns package and application locators that should be pulled
// during the operation on the provided node.
func (b *PlanBuilder) getPullData(node storage.Server) (*storage.PullData, error) {
	// Master nodes pull the entire application to be able to invoke an
	// install hook from any master node local state.
	if node.ClusterRole == string(schema.ServiceRoleMaster) {
		return &storage.PullData{
			Apps: []loc.Locator{
				b.Application.Package,
			},
		}, nil
	}
	// Regular nodes pull only packages required for runtime such as planet
	// or teleport. The planet package also depends on the node role.
	planetPackage, err := b.Application.Manifest.RuntimePackageForProfile(node.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &storage.PullData{
		Packages: []loc.Locator{
			b.GravityPackage,
			b.TeleportPackage,
			*planetPackage,
		},
	}, nil
}

// AddMastersPhase appends master nodes system installation phase to the provided plan
func (b *PlanBuilder) AddMastersPhase(plan *storage.OperationPlan) error {
	var masterPhases []storage.OperationPhase
	for i, node := range b.Masters {
		planetPackage, err := b.Application.Manifest.RuntimePackageForProfile(node.Role)
		if err != nil {
			return trace.Wrap(err)
		}
		masterPhases = append(masterPhases, storage.OperationPhase{
			ID: fmt.Sprintf("%v/%v", phases.MastersPhase, node.Hostname),
			Description: fmt.Sprintf("Install system software on master node %v",
				node.Hostname),
			Phases: []storage.OperationPhase{
				{
					ID: fmt.Sprintf("%v/%v/teleport", phases.MastersPhase, node.Hostname),
					Description: fmt.Sprintf("Install system package %v:%v on master node %v",
						b.TeleportPackage.Name, b.TeleportPackage.Version, node.Hostname),
					Data: &storage.OperationPhaseData{
						Server:      &b.Masters[i],
						ExecServer:  &b.Masters[i],
						Package:     &b.TeleportPackage,
						ServiceUser: &b.ServiceUser,
					},
					Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, node.Hostname)},
					Step:     4,
				},
				{
					ID: fmt.Sprintf("%v/%v/planet", phases.MastersPhase, node.Hostname),
					Description: fmt.Sprintf("Install system package %v:%v on master node %v",
						planetPackage.Name, planetPackage.Version, node.Hostname),
					Data: &storage.OperationPhaseData{
						Server:      &b.Masters[i],
						ExecServer:  &b.Masters[i],
						Package:     planetPackage,
						Labels:      pack.RuntimePackageLabels,
						ServiceUser: &b.ServiceUser,
					},
					Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, node.Hostname)},
					Step:     4,
				},
			},
			Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, node.Hostname)},
			Step:     4,
		})
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:            phases.MastersPhase,
		Description:   "Install system software on master nodes",
		Phases:        masterPhases,
		Requires:      []string{phases.PullPhase},
		LimitParallel: NumParallel,
		Step:          4,
	})
	return nil
}

// AddNodesPhase appends regular nodes system installation phase to the provided plan
func (b *PlanBuilder) AddNodesPhase(plan *storage.OperationPlan) error {
	var nodePhases []storage.OperationPhase
	for i, node := range b.Nodes {
		planetPackage, err := b.Application.Manifest.RuntimePackageForProfile(node.Role)
		if err != nil {
			return trace.Wrap(err)
		}
		nodePhases = append(nodePhases, storage.OperationPhase{
			ID: fmt.Sprintf("%v/%v", phases.NodesPhase, node.Hostname),
			Description: fmt.Sprintf("Install system software on node %v",
				node.Hostname),
			Phases: []storage.OperationPhase{
				{
					ID: fmt.Sprintf("%v/%v/teleport", phases.NodesPhase, node.Hostname),
					Description: fmt.Sprintf("Install system package %v:%v on node %v",
						b.TeleportPackage.Name, b.TeleportPackage.Version, node.Hostname),
					Data: &storage.OperationPhaseData{
						Server:      &b.Nodes[i],
						ExecServer:  &b.Nodes[i],
						Package:     &b.TeleportPackage,
						ServiceUser: &b.ServiceUser,
					},
					Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, node.Hostname)},
					Step:     4,
				},
				{
					ID: fmt.Sprintf("%v/%v/planet", phases.NodesPhase, node.Hostname),
					Description: fmt.Sprintf("Install system package %v:%v on node %v",
						planetPackage.Name, planetPackage.Version, node.Hostname),
					Data: &storage.OperationPhaseData{
						Server:      &b.Nodes[i],
						ExecServer:  &b.Nodes[i],
						Package:     planetPackage,
						Labels:      pack.RuntimePackageLabels,
						ServiceUser: &b.ServiceUser,
					},
					Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, node.Hostname)},
					Step:     4,
				},
			},
			Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, node.Hostname)},
			Step:     4,
		})
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:            phases.NodesPhase,
		Description:   "Install system software on regular nodes",
		Phases:        nodePhases,
		Requires:      []string{phases.PullPhase},
		LimitParallel: NumParallel,
		Step:          4,
	})
	return nil
}

// AddWaitPhase appends planet startup wait phase to the provided plan
func (b *PlanBuilder) AddWaitPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.WaitPhase,
		Description: "Wait for Kubernetes to become available",
		Requires:    fsm.RequireIfPresent(plan, phases.MastersPhase, phases.NodesPhase),
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
		Step: 4,
	})
}

// AddHealthPhase appends phase that waits for the cluster to become healthy
func (b *PlanBuilder) AddHealthPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.HealthPhase,
		Description: "Wait for cluster to pass health checks",
		Requires:    fsm.RequireIfPresent(plan, phases.InstallOverlayPhase, phases.ExportPhase),
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
		Step: 4,
	})
}

// AddRBACPhase appends K8s RBAC initialization phase to the provided plan
func (b *PlanBuilder) AddRBACPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.RBACPhase,
		Description: "Bootstrap Kubernetes roles and PSPs",
		Data: &storage.OperationPhaseData{
			Server:  &b.Master,
			Package: &b.RBACPackage,
		},
		Requires: []string{phases.WaitPhase},
		Step:     4,
	})
}

// AddOpenEBSPhase appends phase that creates OpenEBS configuration.
func (b *PlanBuilder) AddOpenEBSPhase(plan *storage.OperationPlan) (err error) {
	var bytes []byte
	if b.PersistentStorage != nil {
		bytes, err = storage.MarshalPersistentStorage(b.PersistentStorage)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.OpenEBSPhase,
		Description: "Create OpenEBS configuration",
		Data: &storage.OperationPhaseData{
			Server:  &b.Master,
			Storage: bytes,
		},
		Requires: []string{phases.RBACPhase},
		Step:     4,
	})
	return nil
}

// AddSystemResourcesPhase appends phase that creates system Kubernetes
// resources to the provided plan.
func (b *PlanBuilder) AddSystemResourcesPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.SystemResourcesPhase,
		Description: "Create system Kubernetes resources",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
		Requires: []string{phases.RBACPhase},
		Step:     4,
	})
}

// AddUserResourcesPhase appends K8s resources initialization phase to the provided plan
func (b *PlanBuilder) AddUserResourcesPhase(plan *storage.OperationPlan) {
	if len(b.resources) == 0 {
		// Nothing to add
		return
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.UserResourcesPhase,
		Description: "Create user-supplied Kubernetes resources",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
			Install: &storage.InstallOperationData{
				Resources: b.resources,
			},
		},
		Requires: []string{phases.RBACPhase},
		Step:     4,
	})
}

// AddGravityResourcesPhase appends Gravity resources initialization phase to the provided plan
func (b *PlanBuilder) AddGravityResourcesPhase(plan *storage.OperationPlan) {
	if len(b.gravityResources) == 0 {
		// Nothing to add
		return
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.GravityResourcesPhase,
		Description: "Create user-supplied Gravity resources",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
			Install: &storage.InstallOperationData{
				GravityResources: b.gravityResources,
			},
		},
		Requires: []string{phases.EnableElectionPhase},
		Step:     10,
	})
}

// AddInstallOverlayPhase appends a phase to install a non-flannel overlay network
func (b *PlanBuilder) AddInstallOverlayPhase(plan *storage.OperationPlan, locator *loc.Locator) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.InstallOverlayPhase,
		Description: "Install overlay network",
		Data: &storage.OperationPhaseData{
			Server:      &b.Master,
			Package:     locator,
			ServiceUser: &b.ServiceUser,
		},
		Requires: fsm.RequireIfPresent(plan, phases.ExportPhase),
		Step:     4,
	})
}

// AddCorednsPhase generates default coredns configuration for the cluster
func (b *PlanBuilder) AddCorednsPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.CorednsPhase,
		Description: "Configure CoreDNS",
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
		Requires: []string{phases.WaitPhase},
		Step:     4,
	})
}

// AddExportPhase appends Docker images export phase to the provided plan
func (b *PlanBuilder) AddExportPhase(plan *storage.OperationPlan) {
	var exportPhases []storage.OperationPhase
	for i, node := range b.Masters {
		exportPhases = append(exportPhases, storage.OperationPhase{
			ID: fmt.Sprintf("%v/%v", phases.ExportPhase, node.Hostname),
			Description: fmt.Sprintf("Populate Docker registry on master node %v",
				node.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &b.Masters[i],
				ExecServer: &b.Masters[i],
				Package:    &b.Application.Package,
			},
			Requires: []string{phases.WaitPhase},
			Step:     4,
		})
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:            phases.ExportPhase,
		Description:   "Export applications layers to Docker registries",
		Phases:        exportPhases,
		Requires:      []string{phases.WaitPhase},
		LimitParallel: NumParallel,
		Step:          4,
	})
}

// AddRuntimePhase appends system applications installation phase to the provided plan
func (b *PlanBuilder) AddRuntimePhase(plan *storage.OperationPlan) error {
	runtimeLocators, err := app.GetDirectDeps(b.Runtime)
	if err != nil {
		return trace.Wrap(err)
	}
	var runtimePhases []storage.OperationPhase
	for i, locator := range runtimeLocators {
		if b.skipDependency(locator) {
			continue
		}
		runtimePhases = append(runtimePhases, storage.OperationPhase{
			ID: fmt.Sprintf("%v/%v", phases.RuntimePhase, locator.Name),
			Description: fmt.Sprintf("Install system application %v:%v",
				locator.Name, locator.Version),
			Data: &storage.OperationPhaseData{
				Server:      &b.Master,
				Package:     &runtimeLocators[i],
				ServiceUser: &b.ServiceUser,
			},
			Requires: []string{phases.RBACPhase},
			Step:     5,
		})
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.RuntimePhase,
		Description: "Install system applications",
		Phases:      runtimePhases,
		Requires:    []string{phases.RBACPhase},
		Step:        5,
	})
	return nil
}

// AddApplicationPhase appends user application installation phase to the provided plan
func (b *PlanBuilder) AddApplicationPhase(plan *storage.OperationPlan) error {
	applicationLocators, err := app.GetDirectDeps(b.Application)
	if err != nil {
		return trace.Wrap(err)
	}
	var applicationPhases []storage.OperationPhase
	for i, locator := range applicationLocators {
		applicationPhases = append(applicationPhases, storage.OperationPhase{
			ID: fmt.Sprintf("%v/%v", phases.AppPhase, locator.Name),
			Description: fmt.Sprintf("Install application %v:%v",
				locator.Name, locator.Version),
			Data: &storage.OperationPhaseData{
				Server:      &b.Master,
				Package:     &applicationLocators[i],
				ServiceUser: &b.ServiceUser,
				Values:      b.Operation.GetVars().Values,
			},
			Requires: []string{phases.RuntimePhase},
			Step:     6,
		})
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.AppPhase,
		Description: "Install user application",
		Phases:      applicationPhases,
		Requires:    []string{phases.RuntimePhase},
		Step:        6,
	})
	return nil
}

// AddConnectInstallerPhase appends installer/cluster connection phase
func (b *PlanBuilder) AddConnectInstallerPhase(plan *storage.OperationPlan) error {
	bytes, err := storage.MarshalTrustedCluster(b.InstallerTrustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.ConnectInstallerPhase,
		Description: "Connect to installer",
		Data: &storage.OperationPhaseData{
			Server:         &b.Master,
			TrustedCluster: bytes,
		},
		Requires: []string{phases.RuntimePhase},
		Step:     8,
	})
	return nil
}

// AddEnableElectionPhase appends leader election enabling phase to the provided plan
func (b *PlanBuilder) AddEnableElectionPhase(plan *storage.OperationPlan) {
	plan.Phases = append(plan.Phases, storage.OperationPhase{
		ID:          phases.EnableElectionPhase,
		Description: "Enable cluster leader elections",
		Requires:    []string{phases.AppPhase},
		Data: &storage.OperationPhaseData{
			Server: &b.Master,
		},
		Step: 9,
	})
}

// skipDependency returns true if the dependency package specified by dep
// should be skipped when installing the provided application
func (b *PlanBuilder) skipDependency(dep loc.Locator) bool {
	if dep.Name == constants.BootstrapConfigPackage {
		return true // rbac-app is installed separately
	}
	return schema.ShouldSkipApp(b.Application.Manifest, dep)
}

// GetPlanBuilder returns a new plan builder for this installer and provided
// operation that can be used to build operation plan phases
func (c *Config) GetPlanBuilder(operator ops.Operator, cluster ops.Site, op ops.SiteOperation) (*PlanBuilder, error) {
	// determine which app and runtime are being installed
	base := cluster.App.Manifest.Base()
	if base == nil {
		return nil, trace.BadParameter("application %v does not have a runtime",
			cluster.App.Package)
	}
	runtime, err := c.Apps.GetApp(*base)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// sort out all servers into masters and nodes
	masters, nodes := fsm.SplitServers(op.Servers)
	if len(masters) == 0 {
		return nil, trace.BadParameter(
			"at least one master server is required: %v", op.Servers)
	}
	// pick one of master nodes for executing phases that need to
	// be executed from any master node
	master := masters[0]
	// prepare information about application packages that will be required
	// during plan generation
	teleportPackage, err := cluster.App.Manifest.Dependencies.ByName(
		constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rbacPackage, err := cluster.App.Manifest.Dependencies.ByName(
		constants.BootstrapConfigPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gravitySitePackage, err := cluster.App.Manifest.Dependencies.ByName(
		constants.GravitySitePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gravityPackage, err := cluster.App.Manifest.Dependencies.ByName(
		constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dnsAppPackage, err := cluster.App.Manifest.Dependencies.ByName(
		constants.DNSAppPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// retrieve cluster agents
	adminAgent, err := operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   op.AccountID,
		ClusterName: op.SiteDomain,
		Admin:       true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	regularAgent, err := operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   op.AccountID,
		ClusterName: op.SiteDomain,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedCluster, err := c.getInstallerTrustedCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	builder := &PlanBuilder{
		Cluster:   ops.ConvertOpsSite(cluster),
		Operation: op,
		Application: app.Application{
			Package:         cluster.App.Package,
			PackageEnvelope: cluster.App.PackageEnvelope,
			Manifest:        cluster.App.Manifest,
		},
		Runtime:            *runtime,
		TeleportPackage:    *teleportPackage,
		RBACPackage:        *rbacPackage,
		GravitySitePackage: *gravitySitePackage,
		GravityPackage:     *gravityPackage,
		DNSAppPackage:      *dnsAppPackage,
		Masters:            masters,
		Nodes:              nodes,
		Master:             master,
		AdminAgent:         *adminAgent,
		RegularAgent:       *regularAgent,
		ServiceUser: storage.OSUser{
			Name: c.ServiceUser.Name,
			UID:  strconv.Itoa(c.ServiceUser.UID),
			GID:  strconv.Itoa(c.ServiceUser.GID),
		},
		InstallerTrustedCluster: trustedCluster,
	}
	err = addResources(builder, cluster.Resources, c.RuntimeResources, c.ClusterResources)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return builder, nil
}

// splitServers splits the provided servers into masters and nodes
func splitServers(servers []storage.Server, app app.Application) (masters []storage.Server, nodes []storage.Server, err error) {
	numMasters := 0

	// count the number of servers designated as master by the node profile
	for _, server := range servers {
		profile, err := app.Manifest.NodeProfiles.ByName(server.Role)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if profile.ServiceRole == schema.ServiceRoleMaster {
			numMasters++
		}
	}

	// assign the servers to their rolls
	for _, server := range servers {
		profile, err := app.Manifest.NodeProfiles.ByName(server.Role)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		switch profile.ServiceRole {
		case "":
			if numMasters < defaults.MaxMasterNodes {
				server.ClusterRole = string(schema.ServiceRoleMaster)
				masters = append(masters, server)
				numMasters++
			} else {
				server.ClusterRole = string(schema.ServiceRoleNode)
				nodes = append(nodes, server)
			}
		case schema.ServiceRoleMaster:
			server.ClusterRole = string(schema.ServiceRoleMaster)
			masters = append(masters, server)
			// don't increment numMasters as this server has already been counted above
		case schema.ServiceRoleNode:
			server.ClusterRole = string(schema.ServiceRoleNode)
			nodes = append(nodes, server)
		default:
			return nil, nil, trace.BadParameter(
				"unknown cluster role %q for node profile %q",
				profile.ServiceRole, server.Role)
		}
	}
	return masters, nodes, nil
}

func addResources(builder *PlanBuilder, resourceBytes []byte, runtimeResources []runtime.Object, clusterResources []storage.UnknownResource) error {
	kubernetesResources, gravityResources, err := resourceutil.Split(bytes.NewReader(resourceBytes))
	if err != nil {
		return trace.Wrap(err)
	}
	gravityResources = append(gravityResources, clusterResources...)
	rest := gravityResources[:0]
	for _, res := range gravityResources {
		switch res.Kind {
		case storage.KindRuntimeEnvironment:
			env, err := storage.UnmarshalEnvironmentVariables(res.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := env.CheckAndSetDefaults(); err != nil {
				return trace.Wrap(err)
			}
			builder.env = env.GetKeyValues()
			configmap := opsservice.NewEnvironmentConfigMap(env.GetKeyValues())
			kubernetesResources = append(kubernetesResources, configmap)
		case storage.KindClusterConfiguration:
			builder.config = res.Raw
			configmap := opsservice.NewConfigurationConfigMap(res.Raw)
			kubernetesResources = append(kubernetesResources, configmap)
		case storage.KindPersistentStorage:
			// If custom persistent storage configuration was provided by user,
			// it will get applied to the default configuration during install.
			ps, err := storage.UnmarshalPersistentStorage(res.Raw)
			if err != nil {
				return trace.Wrap(err)
			}
			builder.PersistentStorage = ps
		default:
			// Filter out resources that are created using the regular workflow
			rest = append(rest, res)
		}
	}
	builder.gravityResources = rest
	kubernetesResources = append(kubernetesResources, runtimeResources...)
	if len(kubernetesResources) != 0 {
		var buf bytes.Buffer
		err = resources.NewResource(kubernetesResources...).Encode(&buf)
		if err != nil {
			return trace.Wrap(err)
		}
		builder.resources = buf.Bytes()
	}
	return nil
}
