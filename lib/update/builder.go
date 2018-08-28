package update

import (
	"fmt"
	"path"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
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

func (r phaseBuilder) bootstrap(servers []storage.Server, update loc.Locator) *phase {
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
				Server:  &servers[i],
				Package: &update,
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
func (r phaseBuilder) migration(p newPlanParams) *phase {
	root := root(phase{
		ID:          "migration",
		Description: "Perform system database migration",
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

	// no migrations needed
	if len(subphases) == 0 {
		return nil
	}

	root.AddParallel(subphases...)
	return &root
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
func (r phaseBuilder) masters(leadMaster storage.Server, otherMasters []storage.Server, supportsTaints bool,
	application loc.Locator) *phase {
	root := root(phase{
		ID:          "masters",
		Description: "Update master nodes",
	})

	node := r.node(leadMaster, root, "Update system software on master node %q")
	if len(otherMasters) != 0 {
		node.AddSequential(phase{
			ID:          "kubelet-permissions",
			Executor:    kubeletPermissions,
			Description: fmt.Sprintf("Add permissions to kubelet on %q", leadMaster.Hostname),
			Data: &storage.OperationPhaseData{
				Server: &leadMaster,
			}})

		// election - stepdown first node we will upgrade
		enable := []storage.Server{}
		disable := []storage.Server{leadMaster}
		node.AddSequential(setLeaderElection(enable, disable, leadMaster, "stepdown", "Step down %q as Kubernetes leader"))
	}
	node.AddSequential(r.commonNode(leadMaster, leadMaster, supportsTaints,
		waitsForEndpoints(len(otherMasters) == 0), application)...)
	root.AddSequential(node)

	if len(otherMasters) != 0 {
		// election - force election to first upgraded node
		enable := []storage.Server{leadMaster}
		disable := otherMasters
		root.AddSequential(setLeaderElection(enable, disable, leadMaster, "elect", "Make node %q Kubernetes leader"))
	}

	for _, server := range otherMasters {
		node = r.node(server, root, "Update system software on master node %q")
		node.AddSequential(r.commonNode(server, leadMaster, supportsTaints, waitsForEndpoints(true), application)...)

		// election - enable election on the upgraded node
		enable := []storage.Server{server}
		disable := []storage.Server{}
		node.AddSequential(setLeaderElection(enable, disable, server, "enable", "Enable leader election on node %q"))
		root.AddSequential(node)
	}
	return &root
}

func (r phaseBuilder) nodes(leadMaster storage.Server, nodes []storage.Server, supportsTaints bool, application loc.Locator) *phase {
	root := root(phase{
		ID:          "nodes",
		Description: "Update regular nodes",
	})

	for _, server := range nodes {
		node := r.node(server, root, "Update system software on node %q")
		node.AddSequential(r.commonNode(server, leadMaster, supportsTaints,
			waitsForEndpoints(true), application)...)
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
func (r phaseBuilder) commonNode(server storage.Server, leadMaster storage.Server, supportsTaints bool,
	waitsForEndpoints waitsForEndpoints, application loc.Locator) []phase {
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
				Server:  &server,
				Package: &application,
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
