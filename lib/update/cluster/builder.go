/*
Copyright 2019 Gravitational, Inc.

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

package cluster

import (
	"fmt"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	libphase "github.com/gravitational/gravity/lib/update/cluster/phases"

	"github.com/coreos/go-semver/semver"
	teleservices "github.com/gravitational/teleport/lib/services"
)

func (r phaseBuilder) init() *update.Phase {
	phase := update.RootPhase(update.Phase{
		ID:          "init",
		Executor:    updateInit,
		Description: "Initialize update operation",
		Data: &storage.OperationPhaseData{
			Package:          &r.updateApp.Package,
			ExecServer:       &r.leadMaster.Server,
			InstalledPackage: &r.installedApp.Package,
			Update: &storage.UpdateOperationData{
				Servers: r.servers,
			},
		},
	})
	return &phase
}

func (r phaseBuilder) checks() *update.Phase {
	phase := update.RootPhase(update.Phase{
		ID:          "checks",
		Executor:    updateChecks,
		Description: "Run preflight checks",
		Data: &storage.OperationPhaseData{
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
		},
	})

	return &phase
}

func (r phaseBuilder) bootstrap(server storage.UpdateServer) update.Phase {
	return update.Phase{
		ID:          "bootstrap",
		Executor:    updateBootstrap,
		Description: fmt.Sprintf("Bootstrap node %q", server.Hostname),
		Data: &storage.OperationPhaseData{
			ExecServer:       &server.Server,
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
			Update: &storage.UpdateOperationData{
				Servers: []storage.UpdateServer{server},
			},
		},
	}
}

func (r phaseBuilder) preUpdate() *update.Phase {
	phase := update.RootPhase(update.Phase{
		ID:          "pre-update",
		Description: "Run pre-update application hook",
		Executor:    preUpdate,
		Data: &storage.OperationPhaseData{
			Package: &r.updateApp.Package,
		},
	})
	return &phase
}

func (r phaseBuilder) corednsVersionedPhase(version semver.Version) *update.Phase {
	phase := update.RootPhase(update.Phase{
		ID:          "coredns",
		Description: "Provision CoreDNS resources",
		Executor:    coredns,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster.Server,
			CoreDNS: &storage.CoreDNSOperationData{
				Version: version.String(),
			},
		},
	})
	return &phase
}

func (r phaseBuilder) corednsPhase() *update.Phase {
	phase := update.RootPhase(update.Phase{
		ID:          "coredns",
		Description: "Provision CoreDNS resources",
		Executor:    coredns,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster.Server,
		},
	})
	return &phase
}

func (r phaseBuilder) app() update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "app",
		Description: "Update installed application",
	})

	for i, loc := range r.appUpdates {
		root.AddParallel(update.Phase{
			ID:          loc.Name,
			Executor:    updateApp,
			Description: fmt.Sprintf("Update application %q to %v", loc.Name, loc.Version),
			Data: &storage.OperationPhaseData{
				Package: &r.appUpdates[i],
			},
		})
	}
	return root
}

// migration constructs a migration phase based on the plan params.
//
// If there are no migrations to perform, returns nil.
func (r phaseBuilder) migration() *update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "migration",
		Description: "Perform system database migration",
	})

	var subphases []update.Phase

	// do we need to migrate links to trusted clusters?
	if len(r.links) != 0 && len(r.trustedClusters) == 0 {
		subphases = append(subphases, update.Phase{
			ID:          root.ChildLiteral("links"),
			Description: "Migrate remote Ops Center links to trusted clusters",
			Executor:    migrateLinks,
		})
	}

	// Update / reset the labels during upgrade
	subphases = append(subphases, update.Phase{
		ID:          root.ChildLiteral("labels"),
		Description: "Update node labels",
		Executor:    updateLabels,
	})

	// migrate roles
	if libphase.NeedMigrateRoles(r.roles) {
		subphases = append(subphases, update.Phase{
			ID:          root.ChildLiteral("roles"),
			Description: "Migrate cluster roles to a new format",
			Executor:    migrateRoles,
			Data: &storage.OperationPhaseData{
				ExecServer: &r.leadMaster.Server,
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

// config returns phase that pulls system configuration on provided nodes
func (r phaseBuilder) config(nodes []storage.Server) *update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "config",
		Description: "Update system configuration on nodes",
	})
	for i, node := range nodes {
		root.AddParallel(update.Phase{
			ID:       root.ChildLiteral(node.Hostname),
			Executor: config,
			Description: fmt.Sprintf("Update system configuration on node %q",
				node.Hostname),
			Data: &storage.OperationPhaseData{
				Server: &nodes[i],
			},
		})
	}
	return &root
}

func (r phaseBuilder) runtime(updates []loc.Locator) *update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "runtime",
		Description: "Update application runtime",
	})
	for i, loc := range updates {
		phase := update.Phase{
			ID:       loc.Name,
			Executor: updateApp,
			Description: fmt.Sprintf(
				"Update system application %q to %v", loc.Name, loc.Version),
			Data: &storage.OperationPhaseData{
				Package: &updates[i],
			},
		}
		phase.ID = root.Child(phase)
		root.AddSequential(phase)
	}
	return &root
}

// masters returns a new phase for upgrading master servers.
// otherMasters lists the rest of the master nodes (without the leader)
func (r phaseBuilder) masters(leadMaster storage.UpdateServer, otherMasters []storage.UpdateServer, changesetID string) *update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "masters",
		Description: "Update master nodes",
	})
	return r.mastersInternal(leadMaster, otherMasters, &root, changesetID)
}

func (r phaseBuilder) mastersInternal(leadMaster storage.UpdateServer, otherMasters []storage.UpdateServer, root *update.Phase, changesetID string) *update.Phase {
	node := r.node(leadMaster.Server, root, "Update system software on master node %q")
	node.Add(r.bootstrap(leadMaster))
	if len(otherMasters) != 0 {
		node.AddSequential(update.Phase{
			ID:          "kubelet-permissions",
			Executor:    kubeletPermissions,
			Description: fmt.Sprintf("Add permissions to kubelet on %q", leadMaster.Hostname),
			Data: &storage.OperationPhaseData{
				Server: &leadMaster.Server,
			},
		})

		// election - stepdown first node we will upgrade
		node.AddSequential(setLeaderElection(enable(), disable(leadMaster), leadMaster.Server,
			"stepdown", "Step down %q as Kubernetes leader"))
	}

	node.AddSequential(r.commonNode(leadMaster, &leadMaster.Server, waitsForEndpoints(len(otherMasters) == 0), changesetID)...)
	root.AddSequential(node)

	if len(otherMasters) != 0 {
		// election - force election to first upgraded node
		root.AddSequential(setLeaderElection(enable(leadMaster), disable(otherMasters...), leadMaster.Server,
			"elect", "Make node %q Kubernetes leader"))
	}

	for i, server := range otherMasters {
		node = r.node(server.Server, root, "Update system software on master node %q")
		node.Add(r.bootstrap(server))
		node.AddSequential(r.commonNode(otherMasters[i], &leadMaster.Server, waitsForEndpoints(true), changesetID)...)
		// election - enable election on the upgraded node
		node.AddSequential(setLeaderElection(enable(server), disable(), server.Server, "enable", "Enable leader election on node %q"))
		root.AddSequential(node)
	}
	return root
}

func (r phaseBuilder) nodes(leadMaster storage.UpdateServer, nodes []storage.UpdateServer, changesetID string) *update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "nodes",
		Description: "Update regular nodes",
	})
	return r.nodesInternal(leadMaster, nodes, &root, changesetID)
}

func (r phaseBuilder) nodesInternal(leadMaster storage.UpdateServer, nodes []storage.UpdateServer, root *update.Phase, changesetID string) *update.Phase {
	for i, server := range nodes {
		node := r.node(server.Server, root, "Update system software on node %q")
		node.Add(r.bootstrap(server))
		node.AddSequential(r.commonNode(nodes[i], &leadMaster.Server, waitsForEndpoints(true), changesetID)...)
		root.AddParallel(node)
	}
	return root
}

func (r phaseBuilder) etcdPlan(
	otherMasters []storage.Server,
	workers []storage.Server,
	etcd etcdVersion,
) *update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          etcdPhaseName,
		Description: fmt.Sprintf("Upgrade etcd %v to %v", etcd.installed, etcd.update),
	})
	if etcd.installed == "" {
		root.Description = fmt.Sprintf("Upgrade etcd to %v", etcd.update)
	}

	// Backup etcd on each master server
	// Do each master, just in case
	backupEtcd := update.Phase{
		ID:          root.ChildLiteral("backup"),
		Description: "Backup etcd data",
	}
	backupEtcd.AddParallel(r.etcdBackupNode(r.leadMaster.Server, backupEtcd))

	for _, server := range otherMasters {
		p := r.etcdBackupNode(server, backupEtcd)
		backupEtcd.AddParallel(p)
	}

	root.AddSequential(backupEtcd)

	// Shutdown etcd
	// Move data directory to backup location
	shutdownEtcd := update.Phase{
		ID:          root.ChildLiteral("shutdown"),
		Description: "Shutdown etcd cluster",
	}
	shutdownEtcd.AddWithDependency(
		update.DependencyForServer(backupEtcd, r.leadMaster.Server),
		r.etcdShutdownNode(r.leadMaster.Server, shutdownEtcd, true))

	for _, server := range otherMasters {
		p := r.etcdShutdownNode(server, shutdownEtcd, false)
		shutdownEtcd.AddWithDependency(update.DependencyForServer(backupEtcd, server), p)
	}
	for _, server := range workers {
		p := r.etcdShutdownNode(server, shutdownEtcd, false)
		shutdownEtcd.Add(p)
	}

	root.Add(shutdownEtcd)

	// Upgrade servers
	// Replace configuration and data directories, for new version of etcd
	// relaunch etcd on temporary port
	upgradeServers := update.Phase{
		ID:          root.ChildLiteral("upgrade"),
		Description: "Upgrade etcd servers",
	}
	upgradeServers.AddWithDependency(
		update.DependencyForServer(shutdownEtcd, r.leadMaster.Server),
		r.etcdUpgrade(r.leadMaster.Server, upgradeServers))

	for _, server := range otherMasters {
		p := r.etcdUpgrade(server, upgradeServers)
		upgradeServers.AddWithDependency(update.DependencyForServer(shutdownEtcd, server), p)
	}
	for _, server := range workers {
		p := r.etcdUpgrade(server, upgradeServers)
		upgradeServers.AddWithDependency(update.DependencyForServer(shutdownEtcd, server), p)
	}
	root.Add(upgradeServers)

	// Restore kubernetes data
	// migrate to etcd3 store
	// clear kubernetes data from etcd2 store
	restoreData := update.Phase{
		ID:          root.ChildLiteral("restore"),
		Description: "Restore etcd data from backup",
		Executor:    updateEtcdRestore,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster.Server,
		},
	}
	root.AddSequential(restoreData)

	// restart master servers
	// Rolling restart of master servers to listen on normal ports. ETCD outage ends here
	restartMasters := update.Phase{
		ID:          root.ChildLiteral("restart"),
		Description: "Restart etcd servers",
	}
	restartMasters.AddWithDependency(
		update.DependencyForServer(restoreData, r.leadMaster.Server),
		r.etcdRestart(r.leadMaster.Server, restartMasters))

	for _, server := range otherMasters {
		p := r.etcdRestart(server, restartMasters)
		restartMasters.AddWithDependency(update.DependencyForServer(upgradeServers, server), p)
	}
	for _, server := range workers {
		p := r.etcdRestart(server, restartMasters)
		restartMasters.AddWithDependency(update.DependencyForServer(upgradeServers, server), p)
	}

	// also restart gravity-site, so that elections get unbroken
	restartMasters.AddParallel(update.Phase{
		ID:          restartMasters.ChildLiteral(constants.GravityServiceName),
		Description: fmt.Sprint("Restart ", constants.GravityServiceName, " service"),
		Executor:    updateEtcdRestartGravity,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster.Server,
		},
	})
	root.Add(restartMasters)

	return &root
}

func (r phaseBuilder) etcdBackupNode(server storage.Server, parent update.Phase) update.Phase {
	return update.Phase{
		ID:          parent.ChildLiteral(server.Hostname),
		Description: fmt.Sprintf("Backup etcd on node %q", server.Hostname),
		Executor:    updateEtcdBackup,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	}
}

func (r phaseBuilder) etcdShutdownNode(server storage.Server, parent update.Phase, isLeader bool) update.Phase {
	return update.Phase{
		ID:          parent.ChildLiteral(server.Hostname),
		Description: fmt.Sprintf("Shutdown etcd on node %q", server.Hostname),
		Executor:    updateEtcdShutdown,
		Data: &storage.OperationPhaseData{
			Server: &server,
			Data:   strconv.FormatBool(isLeader),
		},
	}
}

func (r phaseBuilder) etcdUpgrade(server storage.Server, parent update.Phase) update.Phase {
	return update.Phase{
		ID:          parent.ChildLiteral(server.Hostname),
		Description: fmt.Sprintf("Upgrade etcd on node %q", server.Hostname),
		Executor:    updateEtcdMaster,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	}
}

func (r phaseBuilder) etcdRestart(server storage.Server, parent update.Phase) update.Phase {
	return update.Phase{
		ID:          parent.ChildLiteral(server.Hostname),
		Description: fmt.Sprintf("Restart etcd on node %q", server.Hostname),
		Executor:    updateEtcdRestart,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	}
}

func (r phaseBuilder) node(server storage.Server, parent update.ParentPhase, format string) update.Phase {
	return update.Phase{
		ID:          parent.ChildLiteral(server.Hostname),
		Description: fmt.Sprintf(format, server.Hostname),
	}
}

// dockerDevicePhase builds a phase that takes care of repurposing device used
// by Docker devicemapper driver for overlay data.
func dockerDevicePhase(node storage.UpdateServer) update.Phase {
	// This phase will remove devicemapper environment (pv, vg, lv) from
	// the devicemapper device.
	devicemapper := update.Phase{
		ID:       "devicemapper",
		Executor: dockerDevicemapper,
		Description: fmt.Sprintf("Remove devicemapper environment from %v",
			node.GetDockerDevice()),
		Data: &storage.OperationPhaseData{
			Server: &node.Server,
		},
	}
	// This phase will re-format the device to xfs or ext4.
	format := update.Phase{
		ID:          "format",
		Executor:    dockerFormat,
		Description: fmt.Sprintf("Format %v", node.GetDockerDevice()),
		Data: &storage.OperationPhaseData{
			Server: &node.Server,
		},
	}
	// This phase will mount the device under Docker data directory.
	mount := update.Phase{
		ID:       "mount",
		Executor: dockerMount,
		Description: fmt.Sprintf("Create mount for %v",
			node.GetDockerDevice()),
		Data: &storage.OperationPhaseData{
			Server: &node.Server,
		},
	}
	// This phase will start the Planet unit and wait till it's up.
	planet := update.Phase{
		ID:          "planet",
		Executor:    planetStart,
		Description: "Start the new Planet container",
		Data: &storage.OperationPhaseData{
			Server: &node.Server,
			Update: &storage.UpdateOperationData{
				Servers: []storage.UpdateServer{node},
			},
		},
	}
	return update.Phase{
		ID: "docker",
		Description: fmt.Sprintf("Repurpose devicemapper device %v for overlay data",
			node.GetDockerDevice()),
		Phases: update.Phases{
			devicemapper,
			format,
			mount,
			planet,
		}.AsPhases(),
	}
}

// commonNode returns a list of operations required for any node role to upgrade its system software
func (r phaseBuilder) commonNode(server storage.UpdateServer, executor *storage.Server, waitsForEndpoints waitsForEndpoints, changesetID string) []update.Phase {
	phases := []update.Phase{
		{
			ID:          "drain",
			Executor:    drainNode,
			Description: fmt.Sprintf("Drain node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		},
		{
			ID:          "system-upgrade",
			Executor:    updateSystem,
			Description: fmt.Sprintf("Update system software on node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				ExecServer: &server.Server,
				Update: &storage.UpdateOperationData{
					Servers:     []storage.UpdateServer{server},
					ChangesetID: changesetID,
				},
			},
		},
	}
	if server.ShouldMigrateDockerDevice() {
		phases = append(phases, dockerDevicePhase(server))
	}
	if r.supportsTaints {
		phases = append(phases, update.Phase{
			ID:          "taint",
			Executor:    taintNode,
			Description: fmt.Sprintf("Taint node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		})
	}
	phases = append(phases, update.Phase{
		ID:          "uncordon",
		Executor:    uncordonNode,
		Description: fmt.Sprintf("Uncordon node %q", server.Hostname),
		Data: &storage.OperationPhaseData{
			Server:     &server.Server,
			ExecServer: executor,
		},
	})
	if waitsForEndpoints {
		phases = append(phases, update.Phase{
			ID:          "endpoints",
			Executor:    endpoints,
			Description: fmt.Sprintf("Wait for DNS/cluster endpoints on %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		})
	}
	if r.supportsTaints {
		phases = append(phases, update.Phase{
			ID:          "untaint",
			Executor:    untaintNode,
			Description: fmt.Sprintf("Remove taint from node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		})
	}
	return phases
}

func (r phaseBuilder) cleanup() update.Phase {
	root := update.RootPhase(update.Phase{
		ID:          "gc",
		Description: "Run cleanup tasks",
	})

	for i := range r.servers {
		node := r.node(r.servers[i].Server, &root, "Clean up node %q")
		node.Executor = cleanupNode
		node.Data = &storage.OperationPhaseData{
			Server: &r.servers[i].Server,
		}
		root.AddParallel(node)
	}
	return root
}

func (r phaseBuilder) newPlan(root update.Phase) *storage.OperationPlan {
	plan := r.planTemplate
	plan.Phases = root.Phases
	update.ResolvePlan(&plan)
	return &plan
}

type phaseBuilder struct {
	operator packageRotator
	// planTemplate specifies the plan to bootstrap the resulting operation plan
	planTemplate storage.OperationPlan
	// operation is the operation to generate the plan for
	operation ops.SiteOperation
	// FIXME: have master, nodes split instead
	// servers is a list of servers from cluster state
	// servers []storage.Server
	// servers lists cluster server augmented with update requirements
	servers []storage.UpdateServer
	// leader refers to the master server running the update operation
	// leadMaster storage.UpdateServer
	leadMaster storage.UpdateServer
	// installedTeleport specifies the version of the currently installed teleport
	installedTeleport loc.Locator
	// updateTeleport specifies the version of teleport to update to
	updateTeleport loc.Locator
	// installedRuntimeApp is the runtime of the installed app
	installedRuntimeApp app.Application
	// installedApp is the installed app
	installedApp app.Application
	// updateRuntimeApp is the update runtime app
	updateRuntimeApp app.Application
	// updateApp is the update app
	updateApp app.Application
	// appUpdates lists the application updates
	appUpdates []loc.Locator
	// links is a list of configured remote Ops Center links
	links []storage.OpsCenterLink
	// trustedClusters is a list of configured trusted clusters
	trustedClusters []teleservices.TrustedCluster
	// packages is a reference to the cluster package service
	packages pack.PackageService
	// apps is a reference to the cluster application service
	apps app.Applications
	// roles is the existing cluster roles
	roles []teleservices.Role
	// installedDocker specifies the Docker configuration of the installed
	// cluster
	installedDocker storage.DockerConfig

	// supportsTaints specifies whether taints are supported by the cluster
	supportsTaints bool

	// FIXME: intermediate steps + final step
	// This will be done dynamically
	// // updateCoreDNS indicates whether we need to run coreDNS phase
	// updateCoreDNS bool
	// // installedTeleport identifies installed teleport package
	// installedTeleport loc.Locator
	// // updateTeleport specifies the version of teleport to update to
	// updateTeleport loc.Locator
	// // runtimeUpdates lists the runtime application updates
	// runtimeUpdates []loc.Locator
	// // changesetID specifies the ID to assign the final system update step
	// changesetID string
}

// supportsTaints determines whether the cluster supports node taints
func supportsTaints(version semver.Version) (supports bool) {
	return defaults.BaseTaintsVersion.Compare(version) <= 0
}

// setLeaderElection creates a phase that will change the leader election state in the cluster
// enable - the list of servers to enable election on
// disable - the list of servers to disable election on
// server - The server the phase should be executed on, and used to name the phase
// key - is the identifier of the phase (combined with server.Hostname)
// msg - is a format string used to describe the phase
func setLeaderElection(enable, disable []storage.Server, server storage.Server, key, msg string) update.Phase {
	return update.Phase{
		ID:          fmt.Sprintf("%s-%s", key, server.Hostname),
		Executor:    electionStatus,
		Description: fmt.Sprintf(msg, server.Hostname),
		Data: &storage.OperationPhaseData{
			Server: &server,
			ElectionChange: &storage.ElectionChange{
				EnableServers:  enable,
				DisableServers: disable,
			},
		},
	}
}

func serversToStorage(updates ...storage.UpdateServer) (result []storage.Server) {
	for _, update := range updates {
		result = append(result, update.Server)
	}
	return result
}

var disable = serversToStorage
var enable = serversToStorage

type waitsForEndpoints bool

const etcdPhaseName = "etcd"
