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

package update

import (
	"fmt"
	"path"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport/lib/services"
)

func (r phaseBuilder) init(installed, update loc.Locator) *phase {
	phase := root(phase{
		ID:          "init",
		Executor:    updateInit,
		Description: "Initialize update operation",
		Data: &storage.OperationPhaseData{
			Package:          &update,
			InstalledPackage: &installed,
		},
	})
	return &phase
}

func (r phaseBuilder) checks(installed, update loc.Locator) *phase {
	phase := root(phase{
		ID:          "checks",
		Executor:    updateChecks,
		Description: "Run preflight checks",
		Data: &storage.OperationPhaseData{
			Package:          &update,
			InstalledPackage: &installed,
		},
	})

	return &phase
}

func (r phaseBuilder) bootstrap(servers []storage.Server, installed, update loc.Locator) *phase {
	root := root(phase{
		ID:          "bootstrap",
		Description: "Bootstrap update operation on nodes",
	})

	for i, server := range servers {
		root.AddParallel(phase{
			ID:          root.ChildLiteral(server.Hostname),
			Executor:    updateBootstrap,
			Description: fmt.Sprintf("Bootstrap node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:           &servers[i],
				Package:          &update,
				InstalledPackage: &installed,
			},
		})
	}
	return &root
}

func (r phaseBuilder) preUpdate(appPackage loc.Locator) *phase {
	phase := root(phase{
		ID:          "pre-update",
		Description: "Run pre-update application hook",
		Executor:    preUpdate,
		Data: &storage.OperationPhaseData{
			Package: &appPackage,
		},
	})
	return &phase
}

func (r phaseBuilder) app(updates []loc.Locator) *phase {
	root := root(phase{
		ID:          "app",
		Description: "Update installed application",
	})

	for i, update := range updates {
		root.AddParallel(phase{
			ID:          update.Name,
			Executor:    updateApp,
			Description: fmt.Sprintf("Update application %q to %v", update.Name, update.Version),
			Data: &storage.OperationPhaseData{
				Package: &updates[i],
			},
		})
	}
	return &root
}

// migration constructs a migration phase based on the plan params.
//
// If there are no migrations to perform, returns nil.
func (r phaseBuilder) migration(leadMaster storage.Server, p newPlanParams) *phase {
	root := root(phase{
		ID:          "migration",
		Description: "Perform system database migrations",
	})

	var subphases []phase

	// do we need to migrate links to trusted clusters?
	if len(p.links) != 0 && len(p.trustedClusters) == 0 {
		subphases = append(subphases, phase{
			ID:          root.ChildLiteral("links"),
			Description: "Migrate remote Ops Center links to trusted clusters",
			Executor:    migrateLinks,
		})
	}

	// Update / reset the labels during upgrade
	subphases = append(subphases, phase{
		ID:          root.ChildLiteral("labels"),
		Description: "Update node labels",
		Executor:    updateLabels,
	})

	// migrate roles
	if needMigrateRoles(p.roles) {
		subphases = append(subphases, phase{
			ID:          root.ChildLiteral("roles"),
			Description: "Migrate cluster roles to a new format",
			Executor:    migrateRoles,
			Data: &storage.OperationPhaseData{
				ExecServer: &leadMaster,
			},
		})
	}

	// no migrations needed
	if len(subphases) == 0 {
		return nil
	}

	root.AddParallel(subphases...)
	return &root
}

// needMigrateRoles returns true if the provided cluster roles need to be
// migrated to a new format
func needMigrateRoles(roles []services.Role) bool {
	for _, role := range roles {
		if needMigrateRole(role) {
			return true
		}
	}
	return false
}

// needMigrateRole returns true if the provided cluster role needs to be
// migrated to a new format
func needMigrateRole(role services.Role) bool {
	// if the role has "assignKubernetesGroups" action, it needs to
	// be migrated to the new KubeGroups property
	for _, rule := range append(role.GetRules(services.Allow), role.GetRules(services.Deny)...) {
		for _, action := range rule.Actions {
			if strings.HasPrefix(action, constants.AssignKubernetesGroupsFnName) {
				return true
			}
		}
	}
	return false
}

func (r phaseBuilder) runtime(updates []loc.Locator, rbacUpdateAvailable bool) *phase {
	root := root(phase{
		ID:          "runtime",
		Description: "Update application runtime",
	})

	for i, update := range updates {
		phase := phase{
			ID:       update.Name,
			Executor: updateApp,
			Description: fmt.Sprintf(
				"Update system application %q to %v", update.Name, update.Version),
			Data: &storage.OperationPhaseData{
				Package: &updates[i],
			},
		}
		phase.ID = root.Child(phase)
		if rbacUpdateAvailable && update.Name != constants.BootstrapConfigPackage {
			phase.RequireLiteral(root.ChildLiteral(constants.BootstrapConfigPackage))
		}
		root.AddParallel(phase)
	}
	return &root
}

// masters returns a new phase for upgrading master servers.
// leadMaster is the master node that is upgraded first and gets to be the leader during the operation.
// otherMasters lists the rest of the master nodes (can be empty)
func (r phaseBuilder) masters(leadMaster runtimeServer, otherMasters runtimeServers,
	supportsTaints bool) *phase {
	root := root(phase{
		ID:          "masters",
		Description: "Update master nodes",
	})

	node := r.node(leadMaster.Server, root, "Update system software on master node %q")
	if len(otherMasters) != 0 {
		node.AddSequential(phase{
			ID:          "kubelet-permissions",
			Executor:    kubeletPermissions,
			Description: fmt.Sprintf("Add permissions to kubelet on %q", leadMaster.Hostname),
			Data: &storage.OperationPhaseData{
				Server: &leadMaster.Server,
			}})

		// election - stepdown first node we will upgrade
		enable := []storage.Server{}
		disable := []storage.Server{leadMaster.Server}
		node.AddSequential(setLeaderElection(enable, disable, leadMaster.Server, "stepdown", "Step down %q as Kubernetes leader"))
	}

	node.AddSequential(r.commonNode(leadMaster.Server, leadMaster.runtime, leadMaster.Server, supportsTaints,
		waitsForEndpoints(len(otherMasters) == 0))...)
	root.AddSequential(node)

	if len(otherMasters) != 0 {
		// election - force election to first upgraded node
		enable := []storage.Server{leadMaster.Server}
		disable := otherMasters.asServers()
		root.AddSequential(setLeaderElection(enable, disable, leadMaster.Server, "elect", "Make node %q Kubernetes leader"))
	}

	for _, server := range otherMasters {
		node = r.node(server.Server, root, "Update system software on master node %q")
		node.AddSequential(r.commonNode(server.Server, server.runtime, leadMaster.Server, supportsTaints,
			waitsForEndpoints(true))...)
		// election - enable election on the upgraded node
		enable := []storage.Server{server.Server}
		disable := []storage.Server{}
		node.AddSequential(setLeaderElection(enable, disable, server.Server, "enable", "Enable leader election on node %q"))
		root.AddSequential(node)
	}
	return &root
}

func (r phaseBuilder) nodes(leadMaster storage.Server, nodes []runtimeServer, supportsTaints bool) *phase {
	root := root(phase{
		ID:          "nodes",
		Description: "Update regular nodes",
	})

	for _, server := range nodes {
		node := r.node(server.Server, root, "Update system software on node %q")
		node.AddSequential(r.commonNode(server.Server, server.runtime, leadMaster, supportsTaints,
			waitsForEndpoints(true))...)
		root.AddParallel(node)
	}
	return &root
}

func (r phaseBuilder) node(server storage.Server, parent phase, format string) phase {
	return phase{
		ID:          parent.ChildLiteral(server.Hostname),
		Description: fmt.Sprintf(format, server.Hostname),
	}
}

// commonNode returns a list of operations required for any node role to upgrade its system software
func (r phaseBuilder) commonNode(server storage.Server, runtimePackage loc.Locator, leadMaster storage.Server, supportsTaints bool,
	waitsForEndpoints waitsForEndpoints) []phase {
	phases := []phase{
		phase{
			ID:          "drain",
			Executor:    drainNode,
			Description: fmt.Sprintf("Drain node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server,
				ExecServer: &leadMaster,
			}},
		phase{
			ID:          "system-upgrade",
			Executor:    updateSystem,
			Description: fmt.Sprintf("Update system software on node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:         &server,
				RuntimePackage: &runtimePackage,
			}},
	}
	if supportsTaints {
		phases = append(phases, phase{
			ID:          "taint",
			Executor:    taintNode,
			Description: fmt.Sprintf("Taint node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server,
				ExecServer: &leadMaster,
			}})
	}
	phases = append(phases, phase{
		ID:          "uncordon",
		Executor:    uncordonNode,
		Description: fmt.Sprintf("Uncordon node %q", server.Hostname),
		Data: &storage.OperationPhaseData{
			Server:     &server,
			ExecServer: &leadMaster,
		}})
	if waitsForEndpoints {
		phases = append(phases, phase{
			ID:          "endpoints",
			Executor:    endpoints,
			Description: fmt.Sprintf("Wait for DNS/cluster endpoints on %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server,
				ExecServer: &leadMaster,
			}})
	}
	if supportsTaints {
		phases = append(phases, phase{
			ID:          "untaint",
			Executor:    untaintNode,
			Description: fmt.Sprintf("Remove taint from node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server,
				ExecServer: &leadMaster,
			}})
	}
	return phases
}

func (r phaseBuilder) cleanup(nodes []storage.Server) *phase {
	root := root(phase{
		ID:          "gc",
		Description: "Run cleanup tasks",
	})

	for _, server := range nodes {
		node := r.node(server, root, "Clean up node %q")
		node.Executor = cleanupNode
		node.Data = &storage.OperationPhaseData{
			Server: &server,
		}
		root.AddParallel(node)
	}
	return &root
}

type phaseBuilder struct{}

// AddSequential will append sub-phases which depend one upon another
func (p *phase) AddSequential(sub ...phase) {
	for i := range sub {
		if len(p.Phases) > 0 {
			sub[i].Require(phase(p.Phases[len(p.Phases)-1]))
		}
		p.Phases = append(p.Phases, storage.OperationPhase(sub[i]))
	}
}

// AddParallel will append sub-phases which depend on parent only
func (p *phase) AddParallel(sub ...phase) {
	p.Phases = append(p.Phases, phases(sub).asPhases()...)
}

// Child adds the specified sub phase as a child of this phase and
// returns the resulting path
func (p *phase) Child(sub phase) string {
	if p == nil {
		return path.Join("/", sub.ID)
	}
	return path.Join(p.ID, sub.ID)
}

// ChildLiteral adds the specified sub phase ID as a child of this phase
// and returns the resulting path
func (p *phase) ChildLiteral(sub string) string {
	if p == nil {
		return path.Join("/", sub)
	}
	return path.Join(p.ID, sub)
}

// Required adds the specified phases reqs as requirements for this phase
func (p *phase) Require(reqs ...phase) *phase {
	for _, req := range reqs {
		p.Requires = append(p.Requires, req.ID)
	}
	return p
}

// RequireLiteral adds the specified phase IDs as requirements for this phase
func (p *phase) RequireLiteral(ids ...string) *phase {
	p.Requires = append(p.Requires, ids...)
	return p
}

// Root makes the specified phase root
func root(sub phase) phase {
	sub.ID = path.Join("/", sub.ID)
	return sub
}

type phase storage.OperationPhase

func (r phases) asPhases() (result []storage.OperationPhase) {
	result = make([]storage.OperationPhase, 0, len(r))
	for _, phase := range r {
		result = append(result, storage.OperationPhase(phase))
	}
	return result
}

type phases []phase

type waitsForEndpoints bool

func (r runtimeServers) asServers() (result []storage.Server) {
	result = make([]storage.Server, 0, len(r))
	for _, server := range r {
		result = append(result, server.Server)
	}
	return result
}

type runtimeServers []runtimeServer

type runtimeServer struct {
	storage.Server
	runtime loc.Locator
}
