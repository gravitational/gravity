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
	"github.com/gravitational/gravity/lib/update/cluster/internal/intermediate"
	libphase "github.com/gravitational/gravity/lib/update/cluster/phases"
	"github.com/gravitational/gravity/lib/update/internal/builder"

	"github.com/coreos/go-semver/semver"
	teleservices "github.com/gravitational/teleport/lib/services"
)

func (r phaseBuilder) init() *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          "init",
		Executor:    updateInit,
		Description: "Initialize update operation",
		Data: &storage.OperationPhaseData{
			Package:          &r.updateApp.Package,
			ExecServer:       &r.leadMaster,
			InstalledPackage: &r.installedApp.Package,
		},
	})
}

func (r phaseBuilder) checks() *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          "checks",
		Executor:    updateChecks,
		Description: "Run preflight checks",
		Data: &storage.OperationPhaseData{
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
		},
	})
}

func (r phaseBuilder) bootstrapVersioned(version semver.Version, gravityPackage loc.Locator, servers []storage.UpdateServer) *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "bootstrap",
		Description: "Bootstrap update operation on nodes",
	})
	for i, server := range servers {
		root.AddParallelRaw(storage.OperationPhase{
			ID:          server.Hostname,
			Executor:    updateBootstrap,
			Description: fmt.Sprintf("Bootstrap node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				ExecServer:       &servers[i].Server,
				Package:          &r.updateApp.Package,
				InstalledPackage: &r.installedApp.Package,
				Update: &storage.UpdateOperationData{
					Servers:        []storage.UpdateServer{server},
					Version:        version.String(),
					GravityPackage: &gravityPackage,
				},
			},
		})
	}
	return root
}

func (r phaseBuilder) bootstrap() *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "bootstrap",
		Description: "Bootstrap update operation on nodes",
	})
	for i, server := range r.targetStep.servers {
		root.AddParallelRaw(storage.OperationPhase{
			ID:          server.Hostname,
			Executor:    updateBootstrap,
			Description: fmt.Sprintf("Bootstrap node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				ExecServer:       &r.targetStep.servers[i].Server,
				Package:          &r.updateApp.Package,
				InstalledPackage: &r.installedApp.Package,
				Update: &storage.UpdateOperationData{
					Servers: []storage.UpdateServer{server},
				},
			},
		})
	}
	return root
}

func (r phaseBuilder) preUpdate() *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          "pre-update",
		Description: "Run pre-update application hook",
		Executor:    preUpdate,
		Data: &storage.OperationPhaseData{
			Package: &r.updateApp.Package,
		},
	})
}

func (r phaseBuilder) corednsVersioned(version semver.Version) *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          "coredns",
		Description: "Provision CoreDNS resources",
		Executor:    coredns,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster,
			CoreDNS: &storage.CoreDNSOperationData{
				Version: version.String(),
			},
		},
	})
}

func (r phaseBuilder) coredns() *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          "coredns",
		Description: "Provision CoreDNS resources",
		Executor:    coredns,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster,
		},
	})
}

func (r phaseBuilder) app() *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "app",
		Description: "Update installed application",
	})
	for i, loc := range r.appUpdates {
		root.AddParallelRaw(storage.OperationPhase{
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
func (r phaseBuilder) migration() *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "migration",
		Description: "Perform system database migration",
	})
	var subphases []storage.OperationPhase
	// do we need to migrate links to trusted clusters?
	if len(r.links) != 0 && len(r.trustedClusters) == 0 {
		subphases = append(subphases, storage.OperationPhase{
			ID:          "links",
			Description: "Migrate remote Ops Center links to trusted clusters",
			Executor:    migrateLinks,
		})
	}
	// Update / reset the labels during upgrade
	subphases = append(subphases, storage.OperationPhase{
		ID:          "labels",
		Description: "Update node labels",
		Executor:    updateLabels,
	})
	// migrate roles
	if libphase.NeedMigrateRoles(r.roles) {
		subphases = append(subphases, storage.OperationPhase{
			ID:          "roles",
			Description: "Migrate cluster roles to a new format",
			Executor:    migrateRoles,
			Data: &storage.OperationPhaseData{
				ExecServer: &r.leadMaster,
			},
		})
	}
	// no migrations needed
	if len(subphases) == 0 {
		return nil
	}
	root.AddParallelRaw(subphases...)
	return root
}

// config returns phase that pulls system configuration on provided nodes
func (r phaseBuilder) config(nodes []storage.Server) *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "config",
		Description: "Update system configuration on nodes",
	})
	for i, node := range nodes {
		root.AddParallelRaw(storage.OperationPhase{
			ID:       node.Hostname,
			Executor: config,
			Description: fmt.Sprintf("Update system configuration on node %q",
				node.Hostname),
			Data: &storage.OperationPhaseData{
				Server: &nodes[i],
			},
		})
	}
	return root
}

func (r phaseBuilder) runtime(updates []loc.Locator) *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "runtime",
		Description: "Update application runtime",
	})
	for i, loc := range updates {
		root.AddSequentialRaw(storage.OperationPhase{
			ID:       loc.Name,
			Executor: updateApp,
			Description: fmt.Sprintf(
				"Update system application %q to %v", loc.Name, loc.Version),
			Data: &storage.OperationPhaseData{
				Package: &updates[i],
			},
		})
	}
	return root
}

// masters returns a new phase for upgrading master servers.
// otherMasters lists the rest of the master nodes (without the leader)
func (r phaseBuilder) masters(leadMaster storage.UpdateServer, otherMasters []storage.UpdateServer, changesetID string) *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "masters",
		Description: "Update master nodes",
	})
	return r.mastersInternal(leadMaster, otherMasters, root, changesetID)
}

func (r phaseBuilder) mastersInternal(leadMaster storage.UpdateServer, otherMasters []storage.UpdateServer, root *builder.Phase, changesetID string) *builder.Phase {
	node := r.node(leadMaster.Server, "Update system software on master node %q")
	if len(otherMasters) != 0 {
		node.AddSequentialRaw(storage.OperationPhase{
			ID:          "kubelet-permissions",
			Executor:    kubeletPermissions,
			Description: fmt.Sprintf("Add permissions to kubelet on %q", leadMaster.Hostname),
			Data: &storage.OperationPhaseData{
				Server: &leadMaster.Server,
			},
		})

		// election - stepdown first node we will upgrade
		node.AddSequentialRaw(setLeaderElection(enable(), disable(leadMaster), leadMaster.Server,
			"stepdown", "Step down %q as Kubernetes leader"))
	}

	node.AddSequential(r.commonNode(leadMaster, &leadMaster.Server, waitsForEndpoints(len(otherMasters) == 0), changesetID)...)
	root.AddSequential(node)

	if len(otherMasters) != 0 {
		// election - force election to first upgraded node
		root.AddSequentialRaw(setLeaderElection(enable(leadMaster), disable(otherMasters...), leadMaster.Server,
			"elect", "Make node %q Kubernetes leader"))
	}

	for i, server := range otherMasters {
		node = r.node(server.Server, "Update system software on master node %q")
		node.AddSequential(r.commonNode(otherMasters[i], &leadMaster.Server, waitsForEndpoints(true), changesetID)...)
		// election - enable election on the upgraded node
		node.AddSequentialRaw(setLeaderElection(enable(server), disable(), server.Server, "enable", "Enable leader election on node %q"))
		root.AddSequential(node)
	}
	return root
}

func (r phaseBuilder) nodes(leadMaster storage.UpdateServer, nodes []storage.UpdateServer, changesetID string) *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "nodes",
		Description: "Update regular nodes",
	})
	return r.nodesInternal(leadMaster, nodes, root, changesetID)
}

func (r phaseBuilder) nodesInternal(leadMaster storage.UpdateServer, nodes []storage.UpdateServer, root *builder.Phase, changesetID string) *builder.Phase {
	for i, server := range nodes {
		node := r.node(server.Server, "Update system software on node %q")
		node.AddSequential(r.commonNode(nodes[i], &leadMaster.Server, waitsForEndpoints(true), changesetID)...)
		root.AddParallel(node)
	}
	return root
}

func (r phaseBuilder) etcdPlan(
	otherMasters []storage.Server,
	workers []storage.Server,
	etcd etcdVersion,
) *builder.Phase {
	description := fmt.Sprintf("Upgrade etcd %v to %v", etcd.installed, etcd.update)
	if etcd.installed == "" {
		description = fmt.Sprintf("Upgrade etcd to %v", etcd.update)
	}
	root := builder.New(storage.OperationPhase{
		ID:          etcdPhaseName,
		Description: description,
	})

	// Backup etcd on each master server
	// Do each master, just in case
	backupEtcd := builder.New(storage.OperationPhase{
		ID:          "backup",
		Description: "Backup etcd data",
	})
	backupEtcd.AddParallel(r.etcdBackupNode(r.leadMaster))

	for _, server := range otherMasters {
		p := r.etcdBackupNode(server)
		backupEtcd.AddParallel(p)
	}

	root.AddSequential(backupEtcd)

	// Shutdown etcd
	// Move data directory to backup location
	shutdownEtcd := builder.New(storage.OperationPhase{
		ID:          "shutdown",
		Description: "Shutdown etcd cluster",
	})
	shutdownEtcd.AddWithDependency(
		builder.DependencyForServer(backupEtcd, r.leadMaster),
		r.etcdShutdownNode(r.leadMaster, true))

	for _, server := range otherMasters {
		p := r.etcdShutdownNode(server, false)
		shutdownEtcd.AddWithDependency(builder.DependencyForServer(backupEtcd, server), p)
	}
	for _, server := range workers {
		p := r.etcdShutdownNode(server, false)
		shutdownEtcd.AddParallel(p)
	}

	root.AddParallel(shutdownEtcd)

	// Upgrade servers
	// Replace configuration and data directories, for new version of etcd
	// relaunch etcd on temporary port
	upgradeServers := builder.New(storage.OperationPhase{
		ID:          "upgrade",
		Description: "Upgrade etcd servers",
	})
	upgradeServers.AddWithDependency(
		builder.DependencyForServer(shutdownEtcd, r.leadMaster),
		r.etcdUpgrade(r.leadMaster))

	for _, server := range otherMasters {
		p := r.etcdUpgrade(server)
		upgradeServers.AddWithDependency(builder.DependencyForServer(shutdownEtcd, server), p)
	}
	for _, server := range workers {
		p := r.etcdUpgrade(server)
		upgradeServers.AddWithDependency(builder.DependencyForServer(shutdownEtcd, server), p)
	}
	root.AddParallel(upgradeServers)

	// Restore kubernetes data
	// migrate to etcd3 store
	// clear kubernetes data from etcd2 store
	restoreData := builder.New(storage.OperationPhase{
		ID:          "restore",
		Description: "Restore etcd data from backup",
		Executor:    updateEtcdRestore,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster,
		},
	})
	root.AddSequential(restoreData)

	// restart master servers
	// Rolling restart of master servers to listen on normal ports. ETCD outage ends here
	restartMasters := builder.New(storage.OperationPhase{
		ID:          "restart",
		Description: "Restart etcd servers",
	})
	restartMasters.AddWithDependency(restoreData, r.etcdRestart(r.leadMaster))

	for _, server := range otherMasters {
		p := r.etcdRestart(server)
		restartMasters.AddWithDependency(builder.DependencyForServer(upgradeServers, server), p)
	}
	for _, server := range workers {
		p := r.etcdRestart(server)
		restartMasters.AddWithDependency(builder.DependencyForServer(upgradeServers, server), p)
	}

	// also restart gravity-site, so that elections get unbroken
	restartMasters.AddParallelRaw(storage.OperationPhase{
		ID:          constants.GravityServiceName,
		Description: fmt.Sprint("Restart ", constants.GravityServiceName, " service"),
		Executor:    updateEtcdRestartGravity,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster,
		},
	})
	root.AddParallel(restartMasters)

	return root
}

func (r phaseBuilder) etcdBackupNode(server storage.Server) *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf("Backup etcd on node %q", server.Hostname),
		Executor:    updateEtcdBackup,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	})
}

func (r phaseBuilder) etcdShutdownNode(server storage.Server, isLeader bool) *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf("Shutdown etcd on node %q", server.Hostname),
		Executor:    updateEtcdShutdown,
		Data: &storage.OperationPhaseData{
			Server: &server,
			Data:   strconv.FormatBool(isLeader),
		},
	})
}

func (r phaseBuilder) etcdUpgrade(server storage.Server) *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf("Upgrade etcd on node %q", server.Hostname),
		Executor:    updateEtcdMaster,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	})
}

func (r phaseBuilder) etcdRestart(server storage.Server) *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf("Restart etcd on node %q", server.Hostname),
		Executor:    updateEtcdRestart,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	})
}

// dockerDevicePhase builds a phase that takes care of repurposing device used
// by Docker devicemapper driver for overlay data.
func dockerDevicePhase(node storage.UpdateServer) *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID: "docker",
		Description: fmt.Sprintf("Repurpose devicemapper device %v for overlay data",
			node.GetDockerDevice()),
	})
	phases := []storage.OperationPhase{
		// Remove devicemapper environment (pv, vg, lv) from
		// the devicemapper device.
		{
			ID:       "devicemapper",
			Executor: dockerDevicemapper,
			Description: fmt.Sprintf("Remove devicemapper environment from %v",
				node.GetDockerDevice()),
			Data: &storage.OperationPhaseData{
				Server: &node.Server,
			},
		},
		// Re-format the device to xfs or ext4.
		{
			ID:          "format",
			Executor:    dockerFormat,
			Description: fmt.Sprintf("Format %v", node.GetDockerDevice()),
			Data: &storage.OperationPhaseData{
				Server: &node.Server,
			},
		},
		// Mount the device under Docker data directory.
		{
			ID:       "mount",
			Executor: dockerMount,
			Description: fmt.Sprintf("Create mount for %v",
				node.GetDockerDevice()),
			Data: &storage.OperationPhaseData{
				Server: &node.Server,
			},
		},
		// Start the Planet unit and wait till it's up.
		{
			ID:          "planet",
			Executor:    planetStart,
			Description: "Start the new Planet container",
			Data: &storage.OperationPhaseData{
				Server: &node.Server,
				Update: &storage.UpdateOperationData{
					Servers: []storage.UpdateServer{node},
				},
			},
		},
	}
	root.AddSequentialRaw(phases...)
	return root
}

// commonNode returns a list of operations required for any node role to upgrade its system software
func (r phaseBuilder) commonNode(server storage.UpdateServer, executor *storage.Server, waitsForEndpoints waitsForEndpoints, changesetID string) (result []*builder.Phase) {
	phases := []*builder.Phase{
		builder.New(storage.OperationPhase{
			ID:          "drain",
			Executor:    drainNode,
			Description: fmt.Sprintf("Drain node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		}),
		builder.New(storage.OperationPhase{
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
		}),
	}
	if server.ShouldMigrateDockerDevice() {
		phases = append(phases, dockerDevicePhase(server))
	}
	if r.supportsTaints {
		phases = append(phases, builder.New(storage.OperationPhase{
			ID:          "taint",
			Executor:    taintNode,
			Description: fmt.Sprintf("Taint node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		}))
	}
	phases = append(phases, builder.New(storage.OperationPhase{
		ID:          "uncordon",
		Executor:    uncordonNode,
		Description: fmt.Sprintf("Uncordon node %q", server.Hostname),
		Data: &storage.OperationPhaseData{
			Server:     &server.Server,
			ExecServer: executor,
		},
	}))
	if waitsForEndpoints {
		phases = append(phases, builder.New(storage.OperationPhase{
			ID:          "endpoints",
			Executor:    endpoints,
			Description: fmt.Sprintf("Wait for DNS/cluster endpoints on %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		}))
	}
	if r.supportsTaints {
		phases = append(phases, builder.New(storage.OperationPhase{
			ID:          "untaint",
			Executor:    untaintNode,
			Description: fmt.Sprintf("Remove taint from node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		}))
	}
	return phases
}

func (r phaseBuilder) cleanup() *builder.Phase {
	root := builder.New(storage.OperationPhase{
		ID:          "gc",
		Description: "Run cleanup tasks",
	})
	for i, server := range r.planTemplate.Servers {
		node := storage.OperationPhase{
			ID:          server.Hostname,
			Description: fmt.Sprintf("Clean up node %q", server.Hostname),
			Executor:    cleanupNode,
			Data: &storage.OperationPhaseData{
				Server: &r.planTemplate.Servers[i],
			},
		}
		root.AddParallelRaw(node)
	}
	return root
}

func (r phaseBuilder) newPlan(root *builder.Phase) *storage.OperationPlan {
	return builder.ResolveInline(root, r.planTemplate)
}

func (r phaseBuilder) node(server storage.Server, format string) *builder.Phase {
	return builder.New(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf(format, server.Hostname),
	})
}

type phaseBuilder struct {
	// operator specifies the cluster operator
	operator intermediate.PackageRotator
	// planTemplate specifies the plan to bootstrap the resulting operation plan
	planTemplate storage.OperationPlan
	// operation is the operation to generate the plan for
	operation ops.SiteOperation
	// leadMaster refers to the master server running the update operation
	leadMaster storage.Server
	// installedTeleport specifies the version of the currently installed teleport
	installedTeleport loc.Locator
	// updateTeleport specifies the version of teleport to update to
	updateTeleport loc.Locator
	// installedRuntimeApp is the installed runtime application
	installedRuntimeApp app.Application
	// installedRuntimeAppVersion specifies the version of the installed runtime application
	installedRuntimeAppVersion semver.Version
	// installedApp is the installed application
	installedApp app.Application
	// updateRuntimeApp is the update runtime application
	updateRuntimeApp app.Application
	// updateApp is the update application
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
	// serviceUser defines the service user on the cluster
	serviceUser storage.OSUser
	// steps lists additional intermediate runtime update steps
	steps []intermediateUpdateStep
	// targetStep defines the final runtime update step
	targetStep targetUpdateStep
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
func setLeaderElection(enable, disable []storage.Server, server storage.Server, key, msg string) storage.OperationPhase {
	return storage.OperationPhase{
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
