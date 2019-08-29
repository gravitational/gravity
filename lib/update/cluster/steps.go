package cluster

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"path/filepath"
	"sort"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	libupdate "github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

func (r phaseBuilder) collectSteps() (steps []updateStepper, err error) {
	result := make(map[string]*intermediateUpdateStep)
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
			result[version] = newIntermediateUpdateStep(*v)
		}
		result[version].fromPackage(env, r.apps)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updates := make([]*intermediateUpdateStep, 0, len(result))
	prevRuntimeApp := r.installedRuntimeApp
	prevTeleport := r.installedTeleport
	for version, update := range result {
		result[version].etcd, err = shouldUpdateEtcd(prevRuntimeApp, update.runtimeApp, r.packages)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// FIXME: factor me out
		operator, err := newPackageRotatorForVersion(version, r.operation.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		serverUpdates, err := r.configUpdates(
			r.installedApp.Manifest, r.updateApp.Manifest,
			prevTeleport, update.teleport,
			r.installedDocker,
			operator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		update.servers = serverUpdates
		update.runtimeUpdates, err = runtimeUpdates(prevRuntimeApp, update.runtimeApp, r.installedApp)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// FIXME: end-of-factor me out
		updates = append(updates, update)
		err = exportGravityBinary(context.Background(), r.packages, update.gravity, operator.path)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		prevRuntimeApp = update.runtimeApp
		if update.teleport != nil {
			prevTeleport = *update.teleport
		}
	}
	sort.Sort(updatesByVersion(updates))
	// Add target update step
	serverUpdates, err := r.configUpdates(
		r.installedApp.Manifest, r.updateApp.Manifest,
		prevTeleport, &r.updateTeleport,
		checks.DockerConfigFromSchemaValue(r.updateApp.Manifest.SystemDocker()),
		r.operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runtimeUpdates, err := runtimeUpdates(r.installedRuntimeApp, r.updateRuntimeApp, r.updateApp)
	steps = make([]updateStepper, 0, len(updates)+1)
	for _, update := range updates {
		steps = append(steps, *update)
	}
	steps = append(steps, r.newTargetUpdateStep(serverUpdates, runtimeUpdates))
	return steps, nil
}

// configUpdates computes the configuration updates for a specific update step
func (r phaseBuilder) configUpdates(
	installed, update schema.Manifest,
	installedTeleport loc.Locator, updateTeleport *loc.Locator,
	updateDocker storage.DockerConfig,
	operator packageRotator,
) (updates []storage.UpdateServer, err error) {
	for _, server := range r.planTemplate.Servers {
		installedRuntime, err := getRuntimePackage(installed, server.Role,
			schema.ServiceRole(server.ClusterRole))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateRuntime, err := update.RuntimePackageForProfile(server.Role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		secretsUpdate, err := operator.RotateSecrets(ops.RotateSecretsRequest{
			AccountID:   r.planTemplate.AccountID,
			ClusterName: r.planTemplate.ClusterName,
			Server:      server,
			DryRun:      true,
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
				// Docker configuration is only updated at the final
				// update step
				Installed: r.installedDocker,
				Update:    updateDocker,
			},
		}
		configUpdate, err := operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
			Key:            r.operation.Key(),
			Server:         server,
			Manifest:       r.installedApp.Manifest,
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
		if updateTeleport != nil {
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
		}
		updates = append(updates, updateServer)
	}
	return updates, nil
}

func (r updatesByVersion) Len() int           { return len(r) }
func (r updatesByVersion) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r updatesByVersion) Less(i, j int) bool { return r[i].version.Compare(r[j].version) < 0 }

type updatesByVersion []*intermediateUpdateStep

func newIntermediateUpdateStep(version semver.Version) *intermediateUpdateStep {
	return &intermediateUpdateStep{
		updateStep: newUpdateStep(),
		version:    version,
	}
}

func (r *intermediateUpdateStep) fromPackage(env pack.PackageEnvelope, apps libapp.Applications) error {
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
		if app.Package.Name == defaults.Runtime {
			r.runtimeApp = *app
		}
	}
	return nil
}

func (r intermediateUpdateStep) addTo(builder phaseBuilder, depends ...libupdate.PhaseIder) libupdate.Phase {
	root := (&libupdate.Phase{
		ID: r.version.String(),
	}).Require(depends...)
	root.AddSequential(
		*builder.bootstrapVersioned(r.version),
	)
	r.updateStep.addTo(root, builder)
	return *root
}

// intermediateUpdateStep describes an intermediate update step
type intermediateUpdateStep struct {
	updateStep
	// version defines the runtime application version as semver
	version semver.Version
	// gravity specifies the package with the gravity binary
	gravity loc.Locator
}

func (r targetUpdateStep) addTo(builder phaseBuilder, depends ...libupdate.PhaseIder) libupdate.Phase {
	root := (&libupdate.Phase{
		ID: "target",
	}).Require(depends...)
	root.AddSequential(
		*builder.bootstrap(),
		*builder.corednsPhase(),
	)
	r.updateStep.addTo(root, builder)
	return *root
}

func (r phaseBuilder) newTargetUpdateStep(servers []storage.UpdateServer, runtimeUpdates []loc.Locator) targetUpdateStep {
	step := newUpdateStep()
	step.servers = servers
	step.runtimeUpdates = runtimeUpdates
	return targetUpdateStep{
		updateStep: step,
	}
}

// targetUpdateStep describes the target (final) kubernetes runtime update step
type targetUpdateStep struct {
	updateStep
}

func (r updateStep) addTo(root *libupdate.Phase, builder phaseBuilder) {
	masters, nodes := libupdate.SplitServers(r.servers)
	masters = reorderServers(masters, builder.leadMaster)
	mastersPhase := *builder.masters(masters[0], masters[1:], r.changesetID)
	nodesPhase := *builder.nodes(masters[0], nodes, r.changesetID)
	root.AddSequential(mastersPhase)
	if len(nodesPhase.Phases) > 0 {
		root.AddSequential(nodesPhase)
	}
	if r.etcd == nil {
		return
	}
	// This step does not depend on previous on purpose - when the etcd block is executed,
	// remote agents might not be able to sync the plan before the shutdown of etcd
	// instances has begun
	root.Add(*builder.etcdPlan(
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
		*builder.config(serversToStorage(masters...)),
		*builder.runtime(r.runtimeUpdates),
	)
}

func newUpdateStep() updateStep {
	return updateStep{
		changesetID: uuid.New(),
	}
}

// updateStep groups package dependencies and other update-relevant metadata
// for a specific version of the runtime
type updateStep struct {
	// changesetID defines the ID for the system update operation
	changesetID string
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
	// runtimeUpdates lists updates to runtime applications
	runtimeUpdates []loc.Locator
}

type etcdVersion struct {
	installed, update string
}

type updateStepper interface {
	addTo(builder phaseBuilder, requires ...libupdate.PhaseIder) (root libupdate.Phase)
}

func (r gravityPackageRotator) RotateSecrets(req ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error) {
	args := []string{
		"update", "rotate-secrets",
		"--server-addr", req.Server.AdvertiseIP,
		"--id", r.operationID,
	}
	// FIXME: Locator->Package
	if req.Locator != nil {
		args = append(args, "--package", req.Locator.String())
	}
	return r.exec(args...)
}

func (r gravityPackageRotator) RotatePlanetConfig(req ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error) {
	args := []string{
		"update", "rotate-planet-config",
		"--runtime-package", req.RuntimePackage.String(),
		"--server-addr", req.Server.AdvertiseIP,
		"--id", r.operationID,
	}
	// FIXME: Locator->Package
	if req.Locator != nil {
		args = append(args, "--package", req.Locator.String())
	}
	return r.exec(args...)
}

func (r gravityPackageRotator) RotateTeleportConfig(req ops.RotateTeleportConfigRequest) (*ops.RotatePackageResponse, *ops.RotatePackageResponse, error) {
	args := []string{
		"update", "rotate-teleport-config",
		"--server-addr", req.Server.AdvertiseIP,
		"--id", r.operationID,
	}
	// FIXME: Node->NodePackage
	if req.Node != nil {
		args = append(args, "--package", req.Node.String())
	}
	resp, err := r.exec(args...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return resp, nil, nil
}

func (r gravityPackageRotator) exec(args ...string) (resp *ops.RotatePackageResponse, err error) {
	cmd := exec.Command(r.path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out = bytes.TrimSpace(out)
	resp = &ops.RotatePackageResponse{}
	if len(out) == 0 {
		return resp, nil
	}
	loc, err := loc.ParseLocator(string(out))
	if err != nil {
		return nil, trace.Wrap(err, "failed to interpret %q as package locator", out)
	}
	resp.Locator = *loc
	return resp, nil
}

func newPackageRotatorForVersion(version, operationID string) (*gravityPackageRotator, error) {
	path, err := gravityPathForVersion(version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &gravityPackageRotator{
		path:        path,
		operationID: operationID,
	}, nil
}

// gravityPackageRotator configures packages using a gravity binary
type gravityPackageRotator struct {
	// path specifies the path to the gravity binary
	path string
	// operationID specifies the ID of the active update operation
	operationID string
}

func gravityPathForVersion(version string) (path string, err error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(state.GravityUpdateDir(stateDir), version, constants.GravityBin), nil
}

func exportGravityBinary(ctx context.Context, packages pack.PackageService, loc loc.Locator, path string) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	return utils.CopyWithRetries(ctx, path, func() (io.ReadCloser, error) {
		_, rc, err := packages.ReadPackage(loc)
		return rc, trace.Wrap(err)
	}, defaults.SharedExecutableMask)
}
