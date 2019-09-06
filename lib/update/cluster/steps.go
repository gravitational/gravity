package cluster

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	libupdate "github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/cluster/internal/intermediate"
	libbuilder "github.com/gravitational/gravity/lib/update/internal/builder"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

func (r *phaseBuilder) initSteps(ctx context.Context) error {
	steps, err := r.buildIntermediateSteps(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	r.steps = steps
	installedRuntimeFunc := getRuntimePackageFromManifest(r.installedApp.Manifest)
	updateRuntimeFunc := getRuntimePackageFromManifest(r.updateApp.Manifest)
	installedTeleport := r.installedTeleport
	installedRuntimeApp := r.installedRuntimeApp
	if len(steps) != 0 {
		installedTeleport = steps[len(steps)-1].teleport
		installedRuntimeFunc = getRuntimePackageStatic(steps[len(steps)-1].runtime)
		installedRuntimeApp = steps[len(steps)-1].runtimeApp
	}
	serverUpdates, err := r.configUpdates(
		installedTeleport,
		installedRuntimeFunc, updateRuntimeFunc,
	)
	if err != nil {
		return trace.Wrap(err)
	}
	runtimeAppUpdates, err := runtimeUpdates(r.installedRuntimeApp, r.updateRuntimeApp, r.updateApp)
	if err != nil {
		return trace.Wrap(err)
	}
	etcd, err := shouldUpdateEtcd(installedRuntimeApp, r.updateRuntimeApp, r.packages)
	if err != nil {
		return trace.Wrap(err)
	}
	r.targetStep = newTargetUpdateStep(updateStep{
		servers:        serverUpdates,
		runtimeUpdates: runtimeAppUpdates,
		etcd:           etcd,
	})
	return nil
}

func (r phaseBuilder) buildIntermediateSteps(ctx context.Context) (updates []intermediateUpdateStep, err error) {
	result := make(map[string]intermediateUpdateStep)
	err = pack.ForeachPackage(r.packages, func(env pack.PackageEnvelope) error {
		labels := pack.Labels(env.RuntimeLabels)
		if !labels.Has(pack.PurposeRuntimeUpgrade) {
			return nil
		}
		version := labels[pack.PurposeRuntimeUpgrade]
		step, ok := result[version]
		if !ok {
			v, err := semver.NewVersion(version)
			if err != nil {
				return trace.Wrap(err, "invalid semver: %q", version)
			}
			result[version] = newIntermediateUpdateStep(*v)
		}
		step.fromPackage(env, r.apps)
		result[version] = step
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updates = make([]intermediateUpdateStep, 0, len(result))
	prevRuntimeApp := r.installedRuntimeApp
	prevTeleport := r.installedTeleport
	prevRuntimeFunc := getRuntimePackageFromManifest(r.installedApp.Manifest)
	for version, update := range result {
		update.etcd, err = shouldUpdateEtcd(prevRuntimeApp, update.runtimeApp, r.packages)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result[version] = update
		path, err := intermediate.GravityPathForVersion(version)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = r.exportGravityBinary(ctx, update.gravity, path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		operator := intermediate.NewPackageRotatorForPath(path, r.operation.ID)
		serverUpdates, err := r.intermediateConfigUpdates(
			r.installedApp.Manifest,
			prevRuntimeFunc, update.runtime,
			prevTeleport, &update.teleport,
			r.installedDocker, operator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		update.servers = serverUpdates
		update.runtimeUpdates, err = runtimeUpdates(prevRuntimeApp, update.runtimeApp, r.installedApp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updates = append(updates, update)
		prevRuntimeApp = update.runtimeApp
		prevTeleport = update.teleport
		prevRuntimeFunc = getRuntimePackageStatic(update.runtime)
	}
	sort.Sort(updatesByVersion(updates))
	return updates, nil
}

// intermediateConfigUpdates computes the configuration updates for a specific update step
func (r phaseBuilder) intermediateConfigUpdates(
	installed schema.Manifest,
	installedRuntimeFunc runtimePackageGetterFunc, updateRuntime loc.Locator,
	installedTeleport loc.Locator, updateTeleport *loc.Locator,
	installedDocker storage.DockerConfig,
	operator intermediate.PackageRotator,
) (updates []storage.UpdateServer, err error) {
	for _, server := range r.planTemplate.Servers {
		installedRuntime, err := installedRuntimeFunc(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		secretsUpdate, err := operator.RotateSecrets(ops.RotateSecretsRequest{
			Key:    fsm.ClusterKey(r.planTemplate),
			Server: server,
			DryRun: true,
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
			Docker: storage.DockerUpdate{
				Installed: r.installedDocker,
				Update:    installedDocker,
			},
		}
		configUpdate, err := operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
			Key:            r.operation.Key(),
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
				Key:    r.operation.Key(),
				Server: server,
				DryRun: true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Teleport.Update = &storage.TeleportUpdate{
				Package:           *updateTeleport,
				NodeConfigPackage: nodeConfig.Locator,
			}
		}
		updates = append(updates, updateServer)
	}
	return updates, nil
}

func (r updatesByVersion) Len() int           { return len(r) }
func (r updatesByVersion) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r updatesByVersion) Less(i, j int) bool { return r[i].version.Compare(r[j].version) < 0 }

type updatesByVersion []intermediateUpdateStep

func newIntermediateUpdateStep(version semver.Version) intermediateUpdateStep {
	return intermediateUpdateStep{
		updateStep: newUpdateStep(updateStep{}),
		version:    version,
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
		}
	}
	return nil
}

func (r intermediateUpdateStep) addTo(builder phaseBuilder, root *libbuilder.Phase) {
	root.AddSequential(
		builder.bootstrapVersioned(r.version, r.gravity, r.servers),
	)
	r.updateStep.addTo(builder, root)
}

// intermediateUpdateStep describes an intermediate update step
type intermediateUpdateStep struct {
	updateStep
	// version defines the runtime application version as semver
	version semver.Version
	// gravity specifies the package with the gravity binary
	gravity loc.Locator
}

func newTargetUpdateStep(step updateStep) targetUpdateStep {
	return targetUpdateStep{updateStep: step}
}

func (r targetUpdateStep) addTo(builder phaseBuilder, root *libbuilder.Phase, depends ...*libbuilder.Phase) {
	root.AddParallel(builder.bootstrap().Require(depends...))
	root.AddSequential(builder.coredns())
	r.updateStep.addTo(builder, root)
}

// targetUpdateStep describes the target (final) kubernetes runtime update step
type targetUpdateStep struct {
	updateStep
}

func (r updateStep) addTo(builder phaseBuilder, root *libbuilder.Phase) {
	masters, nodes := libupdate.SplitServers(r.servers)
	masters = reorderServers(masters, builder.leadMaster)
	mastersPhase := builder.masters(masters[0], masters[1:], r.changesetID)
	nodesPhase := builder.nodes(masters[0], nodes, r.changesetID)
	root.AddSequential(mastersPhase)
	if nodesPhase.HasSubphases() {
		root.AddSequential(nodesPhase)
	}
	if r.etcd == nil {
		return
	}
	// This step does not depend on previous on purpose - when the etcd block is executed,
	// remote agents might not be able to sync the plan before the shutdown of etcd
	// instances has begun
	root.AddParallel(builder.etcdPlan(
		serversToStorage(masters[1:]...),
		serversToStorage(nodes...),
		*r.etcd),
	)
	// The "config" phase pulls new teleport master config packages used
	// by gravity-sites on master nodes: it needs to run *after* system
	// upgrade phase to make sure that old gravity-sites start up fine
	// in case new configuration is incompatible, but *before* runtime
	// phase so new gravity-sites can find it after they start
	root.AddSequential(
		builder.config(serversToStorage(masters...)),
		builder.runtime(r.runtimeUpdates),
	)
}

func newUpdateStep(step updateStep) updateStep {
	step.changesetID = uuid.New()
	return step
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
}

type etcdVersion struct {
	installed, update string
}

// configUpdates computes the configuration updates for the specified list of servers
func (r phaseBuilder) configUpdates(
	installedTeleport loc.Locator,
	installedRuntimeFunc, updateRuntimeFunc runtimePackageGetterFunc,
) (updates []storage.UpdateServer, err error) {
	for _, server := range r.planTemplate.Servers {
		secretsUpdate, err := r.operator.RotateSecrets(ops.RotateSecretsRequest{
			Key:    r.operation.ClusterKey(),
			Server: server,
			DryRun: true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		installedRuntime, err := installedRuntimeFunc(server)
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
			Docker: storage.DockerUpdate{
				Installed: r.installedDocker,
				Update: checks.DockerConfigFromSchemaValue(
					r.updateApp.Manifest.SystemDocker()),
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
			configUpdate, err := r.operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
				Key:            r.operation.Key(),
				Server:         server,
				Manifest:       r.updateApp.Manifest,
				RuntimePackage: *updateRuntime,
				DryRun:         true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Runtime.Update = &storage.RuntimeUpdate{
				Package:       *updateRuntime,
				ConfigPackage: configUpdate.Locator,
			}
		}
		if needsTeleportUpdate {
			_, nodeConfig, err := r.operator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
				Key:    r.operation.Key(),
				Server: server,
				DryRun: true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Teleport.Update = &storage.TeleportUpdate{
				Package:           r.updateTeleport,
				NodeConfigPackage: nodeConfig.Locator,
			}
		}
		updates = append(updates, updateServer)
	}
	return updates, nil
}

func getRuntimePackageFromManifest(m schema.Manifest) runtimePackageGetterFunc {
	return func(server storage.Server) (*loc.Locator, error) {
		loc, err := schema.GetRuntimePackage(m, server.Role,
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

func (r phaseBuilder) exportGravityBinary(ctx context.Context, loc loc.Locator, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), defaults.SharedDirMask); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to create directory for export at %v", filepath.Dir(path))
	}
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	uid, err := strconv.Atoi(r.serviceUser.UID)
	if err != nil {
		return trace.Wrap(err)
	}
	gid, err := strconv.Atoi(r.serviceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}
	return utils.CopyWithRetries(ctx, path,
		func() (io.ReadCloser, error) {
			_, rc, err := r.packages.ReadPackage(loc)
			return rc, trace.Wrap(err)
		},
		utils.PermOption(defaults.SharedExecutableMask),
		utils.OwnerOption(uid, gid),
	)
}
