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
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	installphases "github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/ops"
	libphase "github.com/gravitational/gravity/lib/update/cluster/phases"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	// updateInit is the phase to initialize the update operation
	updateInit = "update_init"
	// updateChecks is the phase to run preflight checks
	updateChecks = "update_checks"
	// updateBootstrap is the phase to bootstrap cluster update operation
	updateBootstrap = "update_bootstrap"
	// updateBootstrapSELinux is the phase to configure SELinux on nodes
	updateBootstrapSELinux = "bootstrap_selinux"
	// updateSystem is the phase to update system software on nodes
	updateSystem = "update_system"
	// preUpdate is the phase to run pre-update application hook
	preUpdate = "pre_update"
	// coredns is a phase to create coredns related roles
	coredns = "coredns"
	// updateApp is the phase to update the application
	updateApp = "update_app"
	// electionStatus is the phase to control node leader elections
	electionStatus = "election_status"
	// taintNode is the phase to taint a node
	taintNode = "taint_node"
	// untaintNode is the phase to remove the node taint
	untaintNode = "untaint_node"
	// drainNode is the phase to drain a node
	drainNode = "drain_node"
	// uncordonNode is the phase to uncordon a node
	uncordonNode = "uncordon_node"
	// endpoints is the phase to wait for system service endpoints
	endpoints = "endpoints"
	// config is the phase that updates system configuration
	config = "config"
	// kubeletPermissions is the phase to add kubelet permissions
	kubeletPermissions = "kubelet_permissions"
	// migrateLinks is the phase to migrate links to trusted clusters
	migrateLinks = "links"
	// updateLabels is the phase to update node labels in the cluster
	updateLabels = "labels"
	// migrateRoles is the phase to migrate roles to a new format
	migrateRoles = "roles"
	// updateEtcdBackup is the phase to backup the etcd datastore before upgrade
	updateEtcdBackup = "etcd_backup"
	// updateEtcdShutdown is the phase to shutdown the etcd datastore for upgrade
	updateEtcdShutdown = "etcd_shutdown"
	// updateEtcdMaster is the phase to upgrade the leader (first) etcd server
	updateEtcdMaster = "etcd_upgrade"
	// updateEtcdRestore is the phase to restore the etcd data to the new etcd instance
	updateEtcdRestore = "etcd_restore"
	// updateEtcdRestart is the phase that restarts etcd service to listen on regular ports
	updateEtcdRestart = "etcd_restart"
	// updateEtcdRestartGravity is the phase that restarts gravity-site
	updateEtcdRestartGravity = "etcd_restart_gravity"
	// cleanupNode is the phase to clean up a node after the upgrade
	cleanupNode = "cleanup_node"
	// openebs is the phase that creates OpenEBS configuration
	openebs = "openebs"
)

// fsmSpec returns the function that returns an appropriate phase executor
func fsmSpec(c Config) fsm.FSMSpecFunc {
	return func(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
		if p.Phase.Executor == "" {
			return nil, trace.BadParameter("error in plan, executor for phase %q was not specified", p.Phase.ID)
		}
		if p.Plan.OperationType != ops.OperationUpdate {
			return nil, trace.BadParameter("unsupported operation %q", p.Plan.OperationType)
		}

		logger := &fsm.Logger{
			FieldLogger: log.WithFields(log.Fields{
				constants.FieldPhase: p.Phase.ID,
			}),
			Key:      fsm.OperationKey(p.Plan),
			Operator: c.Operator,
		}
		if p.Phase.Data != nil {
			logger.Server = p.Phase.Data.Server
		}

		switch p.Phase.Executor {
		case updateInit:
			return libphase.NewUpdatePhaseInit(p, c.Operator, c.Apps,
				c.Backend, c.LocalBackend, c.ClusterPackages, c.Users,
				c.Client, logger)
		case updateChecks:
			return libphase.NewUpdatePhaseChecks(p, c.Operator, c.Apps, c.Runner, logger)
		case updateBootstrap:
			return libphase.NewUpdatePhaseBootstrap(p, c.Operator,
				c.Backend, c.LocalBackend, c.HostLocalBackend,
				c.HostLocalPackages, c.ClusterPackages,
				remote, logger)
		case updateBootstrapSELinux:
			return libphase.NewUpdatePhaseSELinux(p, c.Operator, c.Apps, logger)
		case coredns:
			return libphase.NewPhaseCoreDNS(p, c.Operator, c.Client, logger)
		case updateSystem:
			return libphase.NewUpdatePhaseSystem(p, remote,
				c.LocalBackend, c.ClusterPackages, c.HostLocalPackages,
				logger)
		case preUpdate:
			return libphase.NewUpdatePhaseBeforeApp(p, c.Apps, c.Client, logger)
		case updateApp:
			return libphase.NewUpdatePhaseApp(p, c.Operator, c.Apps, c.Client, logger)
		case electionStatus:
			return libphase.NewPhaseElectionChange(p, c.Operator, remote, logger)
		case taintNode:
			return libphase.NewPhaseTaint(p, c.Client, logger)
		case untaintNode:
			return libphase.NewPhaseUntaint(p, c.Client, logger)
		case drainNode:
			return libphase.NewPhaseDrain(p, c.Client, logger)
		case uncordonNode:
			return libphase.NewPhaseUncordon(p, c.Client, logger)
		case endpoints:
			return libphase.NewPhaseEndpoints(p, c.Client, logger)
		case config:
			return libphase.NewUpdatePhaseConfig(p, c.Operator, c.ClusterPackages, c.HostLocalPackages, remote, logger)
		case kubeletPermissions:
			return libphase.NewPhaseKubeletPermissions(p, c.Client, logger)
		case migrateLinks:
			return libphase.NewPhaseMigrateLinks(p.Plan, c.Backend, logger)
		case updateLabels:
			return libphase.NewPhaseUpdateLabels(p.Plan, c.Client, logger)
		case migrateRoles:
			return libphase.NewPhaseMigrateRoles(p.Plan, c.Backend, logger)
		case updateEtcdBackup:
			return libphase.NewPhaseUpgradeEtcdBackup(logger)
		case updateEtcdShutdown:
			return libphase.NewPhaseUpgradeEtcdShutdown(p.Phase, c.Client, logger)
		case updateEtcdMaster:
			return libphase.NewPhaseUpgradeEtcd(p.Phase, logger)
		case updateEtcdRestore:
			return libphase.NewPhaseUpgradeEtcdRestore(p.Phase, logger)
		case updateEtcdRestart:
			return libphase.NewPhaseUpgradeEtcdRestart(p.Phase, logger)
		case updateEtcdRestartGravity:
			return libphase.NewPhaseUpgradeGravitySiteRestart(p.Phase, c.Client, logger)
		case cleanupNode:
			return libphase.NewGarbageCollectPhase(p, remote, logger)
		case openebs:
			return installphases.NewOpenEBS(p, c.Operator, c.Client)
		default:
			return nil, trace.BadParameter(
				"phase %q requires executor %q (potential mismatch between upgrade versions)",
				p.Phase.ID, p.Phase.Executor)
		}
	}
}
