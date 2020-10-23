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
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	libupdate "github.com/gravitational/gravity/lib/update"
	libphase "github.com/gravitational/gravity/lib/update/cluster/phases"
	"github.com/gravitational/gravity/lib/update/cluster/versions"
	"github.com/gravitational/gravity/lib/update/internal/builder"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

func (r *phaseBuilder) initSteps(ctx context.Context) (err error) {
	var installedOrUpgradedEtcdVersion *semver.Version

	if !r.isDirectUpgrade() {
		r.steps, installedOrUpgradedEtcdVersion, err = r.buildIntermediateSteps(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	installedRuntimeFunc := getRuntimePackageFromManifest(r.installedApp.Manifest)
	updateRuntimeFunc := getRuntimePackageFromManifest(r.updateApp.Manifest)
	installedTeleport := r.installedTeleport
	installedRuntimeApp := r.installedRuntimeApp
	if len(r.steps) != 0 {
		installedTeleport = r.steps[len(r.steps)-1].teleport
		installedRuntimeFunc = getRuntimePackageStatic(r.steps[len(r.steps)-1].runtime)
		installedRuntimeApp = r.steps[len(r.steps)-1].runtimeApp
	}
	serverUpdates, err := r.configUpdates(
		installedTeleport,
		installedRuntimeFunc, updateRuntimeFunc,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	runtimeAppUpdates, err := runtimeUpdates(installedRuntimeApp, r.updateRuntimeApp, r.updateApp)
	if err != nil {
		return trace.Wrap(err)
	}

	if installedOrUpgradedEtcdVersion == nil {
		installedOrUpgradedEtcdVersion, err = getEtcdVersionFromManifest(r.installedApp.Manifest, r.packages)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	if installedOrUpgradedEtcdVersion == nil {
		installedOrUpgradedEtcdVersion = &r.currentEtcdVersion
	}
	updateEtcdVersion, err := getEtcdVersionFromManifest(r.updateApp.Manifest, r.packages)
	if err != nil {
		return trace.Wrap(err)
	}
	etcd := shouldUpdateEtcd(*installedOrUpgradedEtcdVersion, *updateEtcdVersion)
	// check if OpenEBS integration has been enabled in the new application
	openEBSEnabled := !r.installedApp.Manifest.OpenEBSEnabled() && r.updateApp.Manifest.OpenEBSEnabled()
	r.targetStep = newTargetUpdateStep(updateStep{
		servers:        serverUpdates,
		runtimeUpdates: runtimeAppUpdates,
		etcd:           etcd,
		gravity:        r.planTemplate.GravityPackage,
		supportsTaints: supportsTaints(r.updateRuntimeAppVersion),
		openEBSEnabled: openEBSEnabled,
	})
	return nil
}

func (r phaseBuilder) hasRuntimeUpdates() bool {
	return len(r.steps) != 0 || len(r.targetStep.runtimeUpdates) != 0
}

func (r phaseBuilder) buildIntermediateSteps(context.Context) (updates []intermediateUpdateStep, lastUpgradeEtcdVersion *semver.Version, err error) {
	result, err := r.collectIntermediateSteps()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	updates = make([]intermediateUpdateStep, 0, len(result))
	prevRuntimeApp := r.installedRuntimeApp
	prevTeleport := r.installedTeleport
	prevRuntimeFunc := getRuntimePackageFromManifest(r.installedApp.Manifest)
	for version, update := range result {
		if err := update.validate(); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		installedEtcdVersion, err := getEtcdVersionFromManifest(prevRuntimeApp.Manifest, r.packages)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		updateEtcdVersion, err := getEtcdVersionFromManifest(update.runtimeApp.Manifest, r.packages)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		lastUpgradeEtcdVersion = updateEtcdVersion
		update.etcd = shouldUpdateEtcd(*installedEtcdVersion, *updateEtcdVersion)
		result[version] = update
		serverUpdates, err := r.intermediateConfigUpdates(
			r.installedApp.Manifest,
			prevRuntimeFunc, update.runtime,
			prevTeleport, &update.teleport,
			r.operator)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		log.WithField("updates", serverUpdates).Info("Intermediate server upgrade step.")
		update.servers = serverUpdates
		update.runtimeUpdates, err = runtimeUpdates(prevRuntimeApp, update.runtimeApp, r.installedApp)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		updates = append(updates, update)
		prevRuntimeApp = update.runtimeApp
		prevTeleport = update.teleport
		prevRuntimeFunc = getRuntimePackageStatic(update.runtime)
	}
	sort.Sort(updatesByVersion(updates))
	return updates, lastUpgradeEtcdVersion, nil
}

func (r phaseBuilder) collectIntermediateSteps() (result map[string]intermediateUpdateStep, err error) {
	result = make(map[string]intermediateUpdateStep)
	err = pack.ForeachPackage(r.packages, func(env pack.PackageEnvelope) error {
		labels := pack.Labels(env.RuntimeLabels)
		if !labels.HasAny(pack.PurposeRuntimeUpgrade) {
			return nil
		}
		version := labels[pack.PurposeRuntimeUpgrade]
		step, ok := result[version]
		if !ok {
			v, err := semver.NewVersion(version)
			if err != nil {
				return trace.Wrap(err, "invalid semver: %q", version)
			}
			if r.shouldSkipIntermediateUpdate(*v) {
				return nil
			}
			step = newIntermediateUpdateStep(*v)
		}
		step.fromPackage(env, r.apps)
		result[version] = step
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// intermediateConfigUpdates computes the configuration updates for a specific update step
func (r phaseBuilder) intermediateConfigUpdates(
	installed schema.Manifest,
	installedRuntimeFunc runtimePackageGetterFunc, updateRuntime loc.Locator,
	installedTeleport loc.Locator, updateTeleport *loc.Locator,
	operator libphase.PackageRotator,
) (updates []storage.UpdateServer, err error) {
	for _, server := range r.planTemplate.Servers {
		installedRuntime, err := installedRuntimeFunc(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		secretsUpdate, err := operator.RotateSecrets(ops.RotateSecretsRequest{
			Key:            fsm.ClusterKey(r.planTemplate),
			Server:         server,
			RuntimePackage: updateRuntime,
			DryRun:         true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateServer := storage.UpdateServer{
			Server: server,
			Runtime: storage.RuntimePackage{
				Installed:      *installedRuntime,
				SecretsPackage: &secretsUpdate.Locator,
			},
			Teleport: storage.TeleportPackage{
				Installed: installedTeleport,
			},
		}
		configUpdate, err := operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
			Key:            (ops.SiteOperation)(r.operation).Key(),
			Server:         server,
			Manifest:       installed,
			RuntimePackage: updateRuntime,
			DryRun:         true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateServer.Runtime.Update = &storage.RuntimeUpdate{
			Package:       updateRuntime,
			ConfigPackage: configUpdate.Locator,
		}
		if updateTeleport != nil {
			_, nodeConfig, err := operator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
				Key:             (ops.SiteOperation)(r.operation).Key(),
				Server:          server,
				TeleportPackage: *updateTeleport,
				DryRun:          true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Teleport.Update = &storage.TeleportUpdate{
				Package: *updateTeleport,
			}
			if nodeConfig != nil {
				updateServer.Teleport.Update.NodeConfigPackage = &nodeConfig.Locator
			}
		}
		updates = append(updates, updateServer)
	}
	return updates, nil
}

// configUpdates computes the configuration updates for the specified list of servers
func (r phaseBuilder) configUpdates(
	installedTeleport loc.Locator,
	installedRuntimeFunc, updateRuntimeFunc runtimePackageGetterFunc,
) (updates []storage.UpdateServer, err error) {
	for _, server := range r.planTemplate.Servers {
		installedRuntime, err := installedRuntimeFunc(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateServer := storage.UpdateServer{
			Server: server,
			Runtime: storage.RuntimePackage{
				Installed: *installedRuntime,
			},
			Teleport: storage.TeleportPackage{
				Installed: installedTeleport,
			},
		}
		needsPlanetUpdate, needsTeleportUpdate, err := systemNeedsUpdate(
			server.Role, server.ClusterRole,
			r.installedApp.Manifest, r.updateApp.Manifest,
			installedTeleport, r.updateTeleport)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if needsPlanetUpdate {
			updateRuntime, err := updateRuntimeFunc(server)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			secretsUpdate, err := r.operator.RotateSecrets(ops.RotateSecretsRequest{
				Key:            (ops.SiteOperation)(r.operation).ClusterKey(),
				Server:         server,
				RuntimePackage: *updateRuntime,
				DryRun:         true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			configUpdate, err := r.operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
				Key:            (ops.SiteOperation)(r.operation).Key(),
				Server:         server,
				Manifest:       r.updateApp.Manifest,
				RuntimePackage: *updateRuntime,
				DryRun:         true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Runtime.SecretsPackage = &secretsUpdate.Locator
			updateServer.Runtime.Update = &storage.RuntimeUpdate{
				Package:       *updateRuntime,
				ConfigPackage: configUpdate.Locator,
			}
		}
		if needsTeleportUpdate {
			_, nodeConfig, err := r.operator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
				Key:             (ops.SiteOperation)(r.operation).Key(),
				Server:          server,
				TeleportPackage: r.updateTeleport,
				DryRun:          true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Teleport.Update = &storage.TeleportUpdate{
				Package: r.updateTeleport,
			}
			if nodeConfig != nil {
				updateServer.Teleport.Update.NodeConfigPackage = &nodeConfig.Locator
			}
		}
		updates = append(updates, updateServer)
	}
	return updates, nil
}

func newIntermediateUpdateStep(version semver.Version) intermediateUpdateStep {
	return intermediateUpdateStep{
		updateStep:        newUpdateStep(updateStep{}),
		runtimeAppVersion: version,
	}
}

func (r *intermediateUpdateStep) fromPackage(env pack.PackageEnvelope, apps libapp.Applications) error {
	switch env.Locator.Name {
	case constants.PlanetPackage:
		r.runtime = env.Locator
	case constants.TeleportPackage:
		r.teleport = env.Locator
	case constants.GravityPackage:
		r.gravity = env.Locator
	default:
		if env.Type == "" {
			break
		}
		app, err := apps.GetApp(env.Locator)
		if err != nil {
			return trace.Wrap(err)
		}
		r.apps = append(r.apps, *app)
		if app.Package.Name == defaults.Runtime {
			r.runtimeApp = *app
			runtimeAppVersion, err := app.Package.SemVer()
			if err != nil {
				return trace.Wrap(err)
			}
			r.supportsTaints = supportsTaints(*runtimeAppVersion)
		}
	}
	return nil
}

func (r intermediateUpdateStep) build(leadMaster storage.Server, installedApp, updateApp loc.Locator) *builder.Phase {
	servers := reorderServers(r.servers, leadMaster)
	masters, nodes := libupdate.SplitServers(servers)
	root := newRoot(r.runtimeAppVersion.String())
	root.AddSequential(r.bootstrapPhase(servers, installedApp, updateApp))
	r.updateStep.addTo(root, masters, nodes)
	return root
}

func (r intermediateUpdateStep) initPhase(leadMaster storage.Server, installedApp, updateApp loc.Locator) *builder.Phase {
	return initPhase(leadMaster, r.servers, installedApp, updateApp)
}

func (r intermediateUpdateStep) bootstrapPhase(servers []storage.UpdateServer, installedApp, updateApp loc.Locator) *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "bootstrap",
		Description: "Bootstrap update operation on nodes",
	})
	leadMaster := servers[0]
	root.AddParallelRaw(storage.OperationPhase{
		ID:          leadMaster.Hostname,
		Executor:    updateBootstrapLeader,
		Description: fmt.Sprintf("Bootstrap node %q", leadMaster.Hostname),
		Data: &storage.OperationPhaseData{
			ExecServer:       &leadMaster.Server,
			Package:          &updateApp,
			InstalledPackage: &installedApp,
			Update: &storage.UpdateOperationData{
				Servers:           servers,
				RuntimeAppVersion: r.runtimeAppVersion.String(),
				GravityPackage:    &r.gravity,
			},
		},
	})
	for _, server := range servers[1:] {
		server := server
		root.AddParallelRaw(storage.OperationPhase{
			ID:          server.Hostname,
			Executor:    updateBootstrap,
			Description: fmt.Sprintf("Bootstrap node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				ExecServer:       &server.Server,
				Package:          &updateApp,
				InstalledPackage: &installedApp,
				Update: &storage.UpdateOperationData{
					Servers:           []storage.UpdateServer{server},
					RuntimeAppVersion: r.runtimeAppVersion.String(),
					GravityPackage:    &r.gravity,
				},
			},
		})
	}
	return root
}

func (r intermediateUpdateStep) validate() error {
	var errors []error
	if r.runtime.IsEmpty() {
		errors = append(errors, trace.NotFound("planet package for version %v not found", r.runtimeAppVersion))
	}
	if r.teleport.IsEmpty() {
		errors = append(errors, trace.NotFound("teleport package for version %v not found", r.runtimeAppVersion))
	}
	if r.gravity.IsEmpty() {
		errors = append(errors, trace.NotFound("gravity package for version %v not found", r.runtimeAppVersion))
	}
	if r.runtimeApp.Package.IsEmpty() {
		errors = append(errors, trace.NotFound("runtime application package for version %v not found", r.runtimeAppVersion))
	}
	return trace.NewAggregate(errors...)
}

func (r intermediateUpdateStep) String() string {
	var buf bytes.Buffer
	fmt.Fprint(&buf, "intermediateUpdateStep(")
	fmt.Fprintf(&buf, "runtimeAppVersion=%v,", r.runtimeAppVersion)
	fmt.Fprintf(&buf, "gravity=%v,", r.gravity)
	fmt.Fprintf(&buf, "updateStep=%v,", r.updateStep)
	fmt.Fprint(&buf, ")")
	return buf.String()
}

// intermediateUpdateStep describes an intermediate update step
type intermediateUpdateStep struct {
	updateStep
	// runtimeAppVersion defines the runtime application runtimeAppVersion
	runtimeAppVersion semver.Version
}

func newTargetUpdateStep(step updateStep) targetUpdateStep {
	return targetUpdateStep{updateStep: newUpdateStep(step)}
}

func (r targetUpdateStep) build(leadMaster storage.Server, installedApp, updateApp loc.Locator) *builder.Phase {
	root := newRoot("target")
	r.buildInline(root, leadMaster, installedApp, updateApp)
	return root
}

func (r targetUpdateStep) buildInline(
	root *builder.Phase,
	leadMaster storage.Server,
	installedApp, updateApp loc.Locator,
	depends ...*builder.Phase,
) {
	servers := reorderServers(r.servers, leadMaster)
	masters, nodes := libupdate.SplitServers(servers)
	root.AddParallel(r.bootstrapPhase(servers, installedApp, updateApp).Require(depends...))
	if shouldUpdateSELinux(servers) {
		root.AddSequential(r.bootstrapSELinuxPhase(servers, installedApp, updateApp))
	}
	root.AddSequential(corednsPhase(leadMaster))
	r.updateStep.addTo(root, masters, nodes)
}

func (r targetUpdateStep) initPhase(leadMaster storage.Server, installedApp, updateApp loc.Locator) *builder.Phase {
	return initPhase(leadMaster, r.servers, installedApp, updateApp)
}

func (r targetUpdateStep) bootstrapPhase(servers []storage.UpdateServer, installedApp, updateApp loc.Locator) *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "bootstrap",
		Description: "Bootstrap update operation on nodes",
	})
	leadMaster := servers[0]
	root.AddParallelRaw(storage.OperationPhase{
		ID:          leadMaster.Hostname,
		Executor:    updateBootstrapLeader,
		Description: fmt.Sprintf("Bootstrap node %q", leadMaster.Hostname),
		Data: &storage.OperationPhaseData{
			ExecServer:       &leadMaster.Server,
			Package:          &updateApp,
			InstalledPackage: &installedApp,
			Update: &storage.UpdateOperationData{
				Servers:        servers,
				GravityPackage: &r.gravity,
			},
		},
	})
	for _, server := range servers[1:] {
		server := server
		root.AddParallelRaw(storage.OperationPhase{
			ID:          server.Hostname,
			Executor:    updateBootstrap,
			Description: fmt.Sprintf("Bootstrap node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				ExecServer:       &server.Server,
				Package:          &updateApp,
				InstalledPackage: &installedApp,
				Update: &storage.UpdateOperationData{
					Servers:        []storage.UpdateServer{server},
					GravityPackage: &r.gravity,
				},
			},
		})
	}
	return root
}

func (r targetUpdateStep) bootstrapSELinuxPhase(servers []storage.UpdateServer, installedApp, updateApp loc.Locator) *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "selinux-bootstrap",
		Description: "Configure SELinux on nodes",
	})

	for i, server := range servers {
		if !server.SELinux {
			continue
		}
		server := server
		root.AddParallelRaw(storage.OperationPhase{
			ID:          server.Hostname,
			Executor:    updateBootstrapSELinux,
			Description: fmt.Sprintf("Configure SELinux on node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				ExecServer:       &servers[i].Server,
				Package:          &updateApp,
				InstalledPackage: &installedApp,
				Update: &storage.UpdateOperationData{
					Servers: []storage.UpdateServer{server},
				},
			},
		})
	}
	return root
}

// targetUpdateStep describes the target (final) kubernetes runtime update step
type targetUpdateStep struct {
	updateStep
}

func (r updateStep) addTo(root *builder.Phase, masters, nodes []storage.UpdateServer) {
	leadMaster := masters[0]
	mastersPhase := r.mastersPhase(leadMaster, masters[1:])
	nodesPhase := r.nodesPhase(leadMaster, nodes)
	root.AddSequential(mastersPhase)
	if nodesPhase.HasSubphases() {
		root.AddSequential(nodesPhase)
	}
	if r.etcd != nil {
		// This step does not depend on previous on purpose - when the etcd block is executed,
		// remote agents might not be able to sync the plan before the shutdown of etcd
		// instances has begun
		root.AddParallel(r.etcdPhase(
			leadMaster.Server,
			serversToStorage(masters[1:]...),
		))
	}
	phases := []*builder.Phase{
		// The "config" phase pulls new teleport master config packages used
		// by gravity-sites on master nodes: it needs to run *after* system
		// upgrade phase to make sure that old gravity-sites start up fine
		// in case new configuration is incompatible, but *before* runtime
		// phase so new gravity-sites can find it after they start
		r.configPhase(serversToStorage(masters...)),
	}
	if r.openEBSEnabled {
		phases = append(phases, r.openEBS(leadMaster.Server))
	}
	phases = append(phases, r.runtimePhase())
	root.AddSequential(phases...)
}

// openEBS returns phase that creates OpenEBS configuration in the cluster.
func (r updateStep) openEBS(leadMaster storage.Server) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          "openebs",
		Executor:    openebs,
		Description: "Create OpenEBS configuration",
		Data: &storage.OperationPhaseData{
			ExecServer: &leadMaster,
		},
	})
}

func (r updateStep) runtimePhase() *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "runtime",
		Description: "Update application runtime",
	})
	for i, loc := range r.runtimeUpdates {
		root.AddSequentialRaw(storage.OperationPhase{
			ID:       loc.Name,
			Executor: updateApp,
			Description: fmt.Sprintf(
				"Update system application %q to %v", loc.Name, loc.Version),
			Data: &storage.OperationPhaseData{
				Package: &r.runtimeUpdates[i],
			},
		})
	}
	return root
}

// configPhase returns phase that pulls system configuration on provided nodes
func (r updateStep) configPhase(nodes []storage.Server) *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
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

func (r updateStep) etcdPhase(leadMaster storage.Server, otherMasters []storage.Server) *builder.Phase {
	description := fmt.Sprintf("Upgrade etcd %v to %v", r.etcd.installed, r.etcd.update)
	if r.etcd.installed == "" {
		description = fmt.Sprintf("Upgrade etcd to %v", r.etcd.update)
	}
	root := builder.NewPhase(storage.OperationPhase{
		ID:          etcdPhaseName,
		Description: description,
	})

	// Backup etcd on each master server
	// Do each master, just in case
	backupEtcd := builder.NewPhase(storage.OperationPhase{
		ID:          "backup",
		Description: "Backup etcd data",
	})
	backupEtcd.AddParallel(r.etcdBackupNodePhase(leadMaster))

	for _, server := range otherMasters {
		p := r.etcdBackupNodePhase(server)
		backupEtcd.AddParallel(p)
	}

	root.AddSequential(backupEtcd)

	// Shutdown etcd
	// Move data directory to backup location
	shutdownEtcd := builder.NewPhase(storage.OperationPhase{
		ID:          "shutdown",
		Description: "Shutdown etcd cluster",
	})
	shutdownEtcd.AddWithDependency(
		builder.DependencyForServer(backupEtcd, leadMaster),
		r.etcdShutdownNodePhase(leadMaster, true))

	for _, server := range otherMasters {
		p := r.etcdShutdownNodePhase(server, false)
		shutdownEtcd.AddWithDependency(builder.DependencyForServer(backupEtcd, server), p)
	}

	root.AddParallel(shutdownEtcd)

	// Upgrade servers
	// Replace configuration and data directories, for new version of etcd
	// relaunch etcd on temporary port
	upgradeServers := builder.NewPhase(storage.OperationPhase{
		ID:          "upgrade",
		Description: "Upgrade etcd servers",
	})
	upgradeServers.AddWithDependency(
		builder.DependencyForServer(shutdownEtcd, leadMaster),
		r.etcdUpgradePhase(leadMaster))

	for _, server := range otherMasters {
		p := r.etcdUpgradePhase(server)
		upgradeServers.AddWithDependency(builder.DependencyForServer(shutdownEtcd, server), p)
	}
	root.AddParallel(upgradeServers)

	// Copy member directory to the new version
	migrateData := builder.NewPhase(storage.OperationPhase{
		ID:          "migrate",
		Description: "Migrate etcd data to new version",
	})
	migrateData.AddWithDependency(
		builder.DependencyForServer(upgradeServers, leadMaster),
		r.etcdMigratePhase(leadMaster))
	for _, server := range otherMasters {
		p := r.etcdMigratePhase(server)
		migrateData.AddWithDependency(builder.DependencyForServer(upgradeServers, server), p)
	}
	root.AddParallel(migrateData)

	// restart master servers
	// Rolling restart of master servers. ETCD outage ends here
	restartMasters := builder.NewPhase(storage.OperationPhase{
		ID:          "restart",
		Description: "Restart etcd servers",
	})
	restartMasters.AddWithDependency(
		builder.DependencyForServer(migrateData, leadMaster),
		r.etcdRestartPhase(leadMaster))

	for _, server := range otherMasters {
		p := r.etcdRestartPhase(server)
		restartMasters.AddWithDependency(builder.DependencyForServer(migrateData, server), p)
	}

	// also restart gravity-site, so that elections get unbroken
	restartMasters.AddParallelRaw(storage.OperationPhase{
		ID:          constants.GravityServiceName,
		Description: fmt.Sprint("Restart ", constants.GravityServiceName, " service"),
		Executor:    updateEtcdRestartGravity,
		Data: &storage.OperationPhaseData{
			Server: &leadMaster,
		},
	})
	root.AddParallel(restartMasters)

	return root
}

func (r updateStep) etcdBackupNodePhase(server storage.Server) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf("Backup etcd on node %q", server.Hostname),
		Executor:    updateEtcdBackup,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	})
}

func (r updateStep) etcdShutdownNodePhase(server storage.Server, isLeader bool) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf("Shutdown etcd on node %q", server.Hostname),
		Executor:    updateEtcdShutdown,
		Data: &storage.OperationPhaseData{
			Server: &server,
			Data:   strconv.FormatBool(isLeader),
		},
	})
}

func (r updateStep) etcdUpgradePhase(server storage.Server) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf("Upgrade etcd on node %q", server.Hostname),
		Executor:    updateEtcdMaster,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	})
}

func (r updateStep) etcdMigratePhase(server storage.Server) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID: server.Hostname,
		Description: fmt.Sprintf("Migrate etcd data to version %v on node %q",
			r.etcd.update, server.Hostname),
		Executor: updateEtcdMigrate,
		Data: &storage.OperationPhaseData{
			Server: &server,
			Update: &storage.UpdateOperationData{
				Etcd: &storage.EtcdUpgrade{
					From: r.etcd.installed,
					To:   r.etcd.update,
				},
			},
		},
	})
}

func (r updateStep) etcdRestartPhase(server storage.Server) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf("Restart etcd on node %q", server.Hostname),
		Executor:    updateEtcdRestart,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	})
}

func newUpdateStep(step updateStep) updateStep {
	if step.changesetID == "" {
		step.changesetID = uuid.New()
	}
	return step
}

// mastersPhase returns a new phase for upgrading master servers.
// otherMasters lists the rest of the master nodes (without the leader)
func (r updateStep) mastersPhase(leadMaster storage.UpdateServer, otherMasters []storage.UpdateServer) *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "masters",
		Description: "Update master nodes",
	})
	node := nodePhase(leadMaster.Server, "Update system software on master node %q")
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
		node.AddSequential(setLeaderElection(
			electionChanges{
				id:          "stepdown",
				description: fmt.Sprintf("Step down %q as Kubernetes leader", leadMaster.Hostname),
				disable:     serversToStorage(leadMaster),
			},
			leadMaster.Server,
		))
		// election - force failover to first upgraded master
		electionChanges := electionChanges{
			description: fmt.Sprintf("Make node %q Kubernetes leader", leadMaster.Hostname),
			enable:      serversToStorage(leadMaster),
			disable:     serversToStorage(otherMasters...),
		}
		node.AddSequential(r.commonNode(leadMaster, &leadMaster.Server,
			waitsForEndpoints(false), electionChanges)...)
	} else {
		node.AddSequential(r.commonNode(leadMaster, &leadMaster.Server,
			waitsForEndpoints(true), electionChanges{})...)
	}

	root.AddSequential(node)

	for i, server := range otherMasters {
		node = nodePhase(server.Server, "Update system software on master node %q")
		electionChanges := electionChanges{
			description: fmt.Sprintf("Enable leader election on node %q", server.Hostname),
			enable:      serversToStorage(server),
		}
		node.AddSequential(r.commonNode(otherMasters[i], &leadMaster.Server, waitsForEndpoints(true), electionChanges)...)
		root.AddSequential(node)
	}
	return root
}

func (r updateStep) nodesPhase(leadMaster storage.UpdateServer, nodes []storage.UpdateServer) *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "nodes",
		Description: "Update regular nodes",
	})
	for i, server := range nodes {
		node := nodePhase(server.Server, "Update system software on node %q")
		node.AddSequential(r.commonNode(nodes[i], &leadMaster.Server, waitsForEndpoints(true), electionChanges{})...)
		root.AddParallel(node)
	}
	return root
}

// commonNode returns a list of operations required for any node role to upgrade its system software
func (r updateStep) commonNode(
	server storage.UpdateServer,
	executor *storage.Server,
	waitsForEndpoints waitsForEndpoints,
	electionChanges electionChanges,
) (result []*builder.Phase) {
	phases := []*builder.Phase{
		builder.NewPhase(storage.OperationPhase{
			ID:          "drain",
			Executor:    drainNode,
			Description: fmt.Sprintf("Drain node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		}),
		builder.NewPhase(storage.OperationPhase{
			ID:          "system-upgrade",
			Executor:    updateSystem,
			Description: fmt.Sprintf("Update system software on node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				ExecServer: &server.Server,
				Update: &storage.UpdateOperationData{
					Servers:        []storage.UpdateServer{server},
					ChangesetID:    r.changesetID,
					GravityPackage: &r.gravity,
				},
			},
		}),
	}
	if electionChanges.shouldAddPhase() {
		phases = append(phases,
			setLeaderElection(
				electionChanges,
				server.Server,
			),
		)
	}
	phases = append(phases, builder.NewPhase(storage.OperationPhase{
		ID:          "health",
		Executor:    nodeHealth,
		Description: fmt.Sprintf("Health check node %q", server.Hostname),
		Data: &storage.OperationPhaseData{
			Server: &server.Server,
		},
	}))
	if r.supportsTaints {
		phases = append(phases, builder.NewPhase(storage.OperationPhase{
			ID:          "taint",
			Executor:    taintNode,
			Description: fmt.Sprintf("Taint node %q", server.Hostname),
			Data: &storage.OperationPhaseData{
				Server:     &server.Server,
				ExecServer: executor,
			},
		}))
	}
	phases = append(phases, builder.NewPhase(storage.OperationPhase{
		ID:          "uncordon",
		Executor:    uncordonNode,
		Description: fmt.Sprintf("Uncordon node %q", server.Hostname),
		Data: &storage.OperationPhaseData{
			Server:     &server.Server,
			ExecServer: executor,
		},
	}))
	if waitsForEndpoints {
		phases = append(phases, builder.NewPhase(storage.OperationPhase{
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
		phases = append(phases, builder.NewPhase(storage.OperationPhase{
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

func (r updateStep) String() string {
	var buf bytes.Buffer
	fmt.Fprint(&buf, "updateStep(")
	fmt.Fprintf(&buf, "changesetID=%v,", r.changesetID)
	fmt.Fprintf(&buf, "runtime=%v,", r.runtime)
	fmt.Fprintf(&buf, "teleport=%v,", r.teleport)
	fmt.Fprintf(&buf, "runtimeApp=%v,", r.runtimeApp)
	fmt.Fprintf(&buf, "apps=%v,", r.apps)
	fmt.Fprintf(&buf, "runtimeUpdates=%v,", r.runtimeUpdates)
	if r.etcd != nil {
		fmt.Fprintf(&buf, "etcd=%v,", *r.etcd)
	}
	fmt.Fprint(&buf, ")")
	return buf.String()
}

// updateStep groups package dependencies and other update-relevant metadata
// for a specific version of the runtime
type updateStep struct {
	// changesetID defines the ID for the system update operation
	changesetID string
	// runtime specifies the planet package
	runtime loc.Locator
	// teleport specifies the package with teleport
	teleport loc.Locator
	// gravity specifies the package with the gravity binary
	gravity loc.Locator
	// runtimeApp specifies the runtime application package
	runtimeApp libapp.Application
	// apps lists system application packages.
	// apps is sorted with RBAC application to be in front
	apps []libapp.Application
	// etcd describes the etcd update
	etcd *etcdVersion
	// servers lists the server updates for this step
	servers []storage.UpdateServer
	// runtimeUpdates lists updates to runtime applications in proper
	// order (i.e. with RBAC application in front)
	runtimeUpdates []loc.Locator
	// supportsTaints specifies whether this runtime version supports node taints
	supportsTaints bool
	// openEBSEnabled specifies whether openEBS application is enabled and should be updated
	openEBSEnabled bool
}

type etcdVersion struct {
	installed, update string
}

func getRuntimePackageFromManifest(m schema.Manifest) runtimePackageGetterFunc {
	return func(server storage.Server) (*loc.Locator, error) {
		loc, err := schema.GetRuntimePackageForProfile(m, server.Role,
			schema.ServiceRole(server.ClusterRole))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return loc, nil
	}
}

func getRuntimePackageStatic(runtimePackage loc.Locator) runtimePackageGetterFunc {
	return func(storage.Server) (*loc.Locator, error) {
		return &runtimePackage, nil
	}
}

// runtimePackageGetterFunc returns the runtime package for the specified server
type runtimePackageGetterFunc func(storage.Server) (*loc.Locator, error)

func (r phaseBuilder) isDirectUpgrade() bool {
	return versions.RuntimeUpgradePath{
		From: &r.installedRuntimeAppVersion,
		To:   &r.updateRuntimeAppVersion,
	}.SupportsDirectUpgrade()
}

func (r phaseBuilder) shouldSkipIntermediateUpdate(v semver.Version) bool {
	// Skip the update if it's older than the installed cluster's
	// runtime version
	return v.Compare(r.installedRuntimeAppVersion) <= 0
}

func corednsPhase(leadMaster storage.Server) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          "coredns",
		Description: "Provision CoreDNS resources",
		Executor:    coredns,
		Data: &storage.OperationPhaseData{
			Server: &leadMaster,
		},
	})
}

func initPhase(leadMaster storage.Server, servers []storage.UpdateServer, installedApp, updateApp loc.Locator) *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "init",
		Description: "Initialize update operation",
	})
	servers = reorderServers(servers, leadMaster)
	root.AddParallelRaw(storage.OperationPhase{
		ID:          leadMaster.Hostname,
		Description: fmt.Sprintf("Initialize node %q", leadMaster.Hostname),
		Executor:    updateInitLeader,
		Data: &storage.OperationPhaseData{
			ExecServer:       &leadMaster,
			Package:          &updateApp,
			InstalledPackage: &installedApp,
			Update: &storage.UpdateOperationData{
				Servers: []storage.UpdateServer{servers[0]},
			},
		},
	})
	for _, server := range servers[1:] {
		server := server
		root.AddParallelRaw(storage.OperationPhase{
			ID:          server.Hostname,
			Description: fmt.Sprintf("Initialize node %q", server.Hostname),
			Executor:    updateInit,
			Data: &storage.OperationPhaseData{
				ExecServer: &server.Server,
				Update: &storage.UpdateOperationData{
					Servers: []storage.UpdateServer{server},
				},
			},
		})
	}
	return root
}

func nodePhase(server storage.Server, format string) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          server.Hostname,
		Description: fmt.Sprintf(format, server.Hostname),
	})
}

func newRoot(id string) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID: id,
	})
}

// supportsTaints determines whether the specified runtime version supports node taints
func supportsTaints(runtimeAppVersion semver.Version) (supports bool) {
	return defaults.BaseTaintsVersion.Compare(runtimeAppVersion) <= 0
}

func (r updatesByVersion) Len() int      { return len(r) }
func (r updatesByVersion) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r updatesByVersion) Less(i, j int) bool {
	return r[i].runtimeAppVersion.Compare(r[j].runtimeAppVersion) < 0
}

type updatesByVersion []intermediateUpdateStep

func shouldUpdateSELinux(servers []storage.UpdateServer) bool {
	for _, server := range servers {
		if server.SELinux {
			return true
		}
	}
	return false
}
