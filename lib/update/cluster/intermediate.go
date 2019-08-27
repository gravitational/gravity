package cluster

import (
	"sort"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

func (r phaseBuilder) collectIntermediateUpdates() (updates []intermediateUpdate, err error) {
	result := make(map[string]*intermediateUpdate)
	err = pack.ForeachPackage(r.packages, func(env pack.PackageEnvelope) error {
		labels := pack.Labels(env.RuntimeLabels)
		if !labels.HasPurpose(pack.PurposeRuntimeUpgrade) {
			return nil
		}
		version := labels[pack.PurposeRuntimeUpgrade]
		if result[version] == nil {
			v, err := semver.NewVersion(version)
			if err != nil {
				return trace.Wrap(err, "invalid semver: %q", version)
			}
			result[version] = newIntermediateUpdate(*v)
		}
		result[version].fromPackage(env, r.apps)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updates = make([]intermediateUpdate, 0, len(result))
	prevRuntime := r.installedRuntime
	for version, update := range result {
		result[version].etcd, err = shouldUpdateEtcd(prevRuntime, update.runtimeApp, packages)
		if err != nil {
			return trace.Wrap(err)
		}
		updates = append(updates, *update)
		prevRuntime = update.runtimeApp
	}
	sort.Sort(updatesByVersion(updates))
	for u := range updates {
		sort.Slice(updates.apps, func(i, j int) bool {
			// Push RBAC package update to front
			return updates[u].apps[i].Name == constants.BootstrapConfigPackage
		})
	}
	return updates, nil
}

// configIntermediateUpdates computes the configuration updates for a specific upgrade step
func (r phaseBuilder) configIntermediateUpdates(
	installedRuntime, updateRuntime loc.Locator,
	installedTeleport, updateTeleport loc.Locator,
	operator packageRotator,
) (updates []storage.UpdateServer, err error) {
	for _, server := range r.planTemplate.Servers {
		secretsUpdate, err := operator.RotateSecrets(ops.RotateSecretsRequest{
			AccountID:   r.planTemplate.AccountID,
			ClusterName: r.planTemplate.SiteDomain,
			Server:      server,
			DryRun:      true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateServer := storage.UpdateServer{
			Server: server,
			Runtime: storage.RuntimePackage{
				Installed:      installedRuntime,
				SecretsPackage: &secretsUpdate.Locator,
			},
			Teleport: storage.TeleportPackage{
				Installed: installedTeleport,
			},
			Docker: storage.DockerUpdate{
				// Docker configuration is only updated at the final
				// update step
				Installed: &installedDocker,
				Update:    installedDocker,
			},
		}
		// update planet
		configUpdate, err := operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
			Key:    operation,
			Server: server,
			// FIXME: use installed application's manifest
			// Manifest:       update,
			RuntimePackage: updateRuntime,
			DryRun:         true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateServer.Runtime.Update = &storage.RuntimeUpdate{
			Package:       *updateRuntime,
			ConfigPackage: configUpdate.Locator,
		}
		// update teleport
		_, nodeConfig, err := operator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
			Key:    fsm.OperationKey(r.planTemplate),
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
		updates = append(updates, updateServer)
	}
	return updates, nil
}

func (r updatesByVersion) Len() int           { return len(r) }
func (r updatesByVersion) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r updatesByVersion) Less(i, j int) bool { return r[i].version.Compare(r[j].version) < 0 }

type updatesByVersion []intermediateUpdate

func newIntermediateUpdate(version semver.Version) *intermediateUpdate {
	return &intermediateUpdate{
		changesetID: uuid.New(),
		version:     version,
	}
}

func (r *intermediateUpdate) fromPackage(env pack.PackageEnvelope, apps libapp.Applications) error {
	switch env.Locator.Name {
	case constants.PlanetPackage:
		r.runtime = env.Locator
	case constants.TeleportPackage:
		r.teleport = &env.Locator
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
		if app.Name == defaults.Runtime {
			r.runtimeApp = *app
		}
	}
	return nil
}

// intermediateUpdate groups package dependencies for a specific
// intermediate version of the runtime
type intermediateUpdate struct {
	// version defines the runtime application version as semver
	version semver.Version
	// changesetID defines the ID for the system update operation
	changesetID string
	// gravity specifies the package with the gravity binary
	gravity loc.Locator
	// runtime specifies the planet package
	runtime loc.Locator
	// teleport specifies the package with teleport
	teleport *loc.Locator
	// runtimeApp specifies the runtime application package
	runtimeApp libapp.Application
	// apps lists system application packages.
	// apps is sorted with RBAC application to be in front
	apps []libapp.Application
	// etcd describes the etcd update
	etcd *etcdVersion
	// servers lists the server updates for this step
	servers []storage.UpdateServer
}

type etcdVersion struct {
	installed, update string
}
