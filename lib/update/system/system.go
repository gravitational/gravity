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

package system

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"os/exec"
	"path/filepath"
	"strconv"

	archiveutils "github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/log"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	libstatus "github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	libselinux "github.com/gravitational/gravity/lib/system/selinux"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/gravitational/satellite/agent/proto/agentpb"
	teleconfig "github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/trace"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// New returns a new system updater
func New(config Config) (*System, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &System{
		Config: config,
	}, nil
}

// Update applies updates to the system packages specified with config
func (r *System) Update(ctx context.Context, withStatus bool) error {
	if r.ChangesetID == "" {
		return trace.BadParameter("ChangesetID is required")
	}

	if err := r.Config.PackageUpdates.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	var changes []storage.PackageUpdate
	for _, u := range r.updates() {
		logger := r.WithField("package", u)
		logger.Info("Checking for update.")
		update, err := needsPackageUpdate(r.Packages, u)
		if err != nil {
			if trace.IsNotFound(err) {
				logger.Info("No update found.")
				continue
			}
			return trace.Wrap(err)
		}
		logger.WithField("package", update).Info("Found update.")
		changes = append(changes, *update)
	}
	if len(changes) == 0 {
		r.Info("System is already up to date.")
		return nil
	}

	changeset, err := r.Backend.CreatePackageChangeset(storage.PackageChangeset{ID: r.ChangesetID, Changes: changes})
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	err = r.applyUpdates(ctx, changes)
	if err != nil {
		return trace.Wrap(err)
	}

	if !withStatus {
		r.WithField("changeset", changeset).Info("System successfully updated.")
		return nil
	}

	err = ensureServiceRunning(r.Runtime.To)
	if err != nil {
		return trace.Wrap(err)
	}

	err = waitNodeStatus(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	r.WithField("changeset", changeset).Info("System successfully updated.")
	return nil
}

// Rollback rolls back system to the specified changesetID or the last update if changesetID is not specified
func (r *System) Rollback(ctx context.Context, withStatus bool) (err error) {
	if r.ChangesetID == "" {
		return trace.BadParameter("ChangesetID is required")
	}

	changeset, err := r.getChangesetByID(r.ChangesetID)
	if err != nil {
		return trace.Wrap(err)
	}

	logger := r.WithField("changeset", changeset)
	logger.Info("Rolling back.")

	changes := changeset.ReversedChanges()
	rollback, err := r.Backend.CreatePackageChangeset(storage.PackageChangeset{Changes: changes})
	if err != nil {
		return trace.Wrap(err)
	}

	err = r.applyUpdates(ctx, changes)
	if err != nil {
		return trace.Wrap(err)
	}

	if !withStatus {
		r.WithField("rollback", rollback).Info("Rolled back.")
		return nil
	}

	err = waitNodeStatus(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	r.WithField("rollback", rollback).Info("Rolled back.")
	return nil
}

// UpdateTctlScript writes tctl script that invokes tctl binary from the
// specified package with appropriate configuration.
func (r *System) UpdateTctlScript(newPackage loc.Locator) error {
	return updateTctlScript(r.Packages, newPackage, r.Logger)
}

// System is a service to update system package on a node
type System struct {
	// Config specifies service configuration
	Config
}

func (r *Config) checkAndSetDefaults() error {
	if r.Backend == nil {
		return trace.BadParameter("Backend is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("Packages is required")
	}
	if r.Logger == nil {
		r.Logger = log.New(logrus.WithFields(logrus.Fields{
			trace.Component: "system-update",
		}))
	}
	return nil
}

// Config defines the configuration of the system updater
type Config struct {
	// Logger specifies the logger
	log.Logger
	// PackageUpdates describes the packages to update
	PackageUpdates
	// ChangesetID specifies the unique ID of this update operation
	ChangesetID string
	// Backend specifies the local host backend
	Backend storage.Backend
	// Packages specifies the local host package service
	Packages update.LocalPackageService
	// ClusterRole specifies cluster role of the node this system updater runs on
	ClusterRole string
	// SELinux controls SELinux support
	SELinux bool
}

func (r *PackageUpdates) checkAndSetDefaults() error {
	if len(r.Runtime.Labels) == 0 {
		r.Runtime.Labels = pack.RuntimePackageLabels
	}
	if r.Runtime.ConfigPackage != nil {
		if len(r.Runtime.ConfigPackage.Labels) == 0 {
			r.Runtime.ConfigPackage.Labels = pack.RuntimeConfigPackageLabels
		}
	}
	if r.RuntimeSecrets != nil && len(r.RuntimeSecrets.Labels) == 0 {
		r.RuntimeSecrets.Labels = pack.RuntimeSecretsPackageLabels
	}
	if r.Teleport != nil {
		if r.Teleport.ConfigPackage != nil && len(r.Teleport.ConfigPackage.Labels) == 0 {
			r.Teleport.ConfigPackage.Labels = pack.TeleportNodeConfigPackageLabels
		}
	}
	return nil
}

func (r *PackageUpdates) updates() (result []storage.PackageUpdate) {
	result = append(result, r.Runtime)
	if r.Gravity != nil {
		result = append(result, *r.Gravity)
	}
	if r.RuntimeSecrets != nil {
		result = append(result, *r.RuntimeSecrets)
	}
	if r.Teleport != nil {
		result = append(result, *r.Teleport)
	}
	return result
}

// PackageUpdates describes the packages to update
type PackageUpdates struct {
	// Gravity specifies the gravity package update
	Gravity *storage.PackageUpdate
	// Runtime specifies the runtime package update
	Runtime storage.PackageUpdate
	// RuntimeSecrets specifies the update for the runtime secrets package
	RuntimeSecrets *storage.PackageUpdate
	// Teleport specifies the teleport package update
	Teleport *storage.PackageUpdate
}

func (r *System) applyUpdates(ctx context.Context, updates []storage.PackageUpdate) error {
	var errors []error
	packageUpdater := &PackageUpdater{
		Logger:      r.WithField(trace.Component, "update:package"),
		Packages:    r.Packages,
		ClusterRole: r.ClusterRole,
		SELinux:     r.SELinux,
	}
	for _, u := range updates {
		r.WithField("update", u).Info("Applying.")
		err := packageUpdater.Reinstall(ctx, u)
		if err != nil {
			r.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"from":          u.From,
				"to":            u.To,
			}).Warn("Failed to reinstall.")
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (r *System) getChangesetByID(changesetID string) (*storage.PackageChangeset, error) {
	if changesetID != "" {
		changeset, err := r.Backend.GetPackageChangeset(changesetID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return changeset, nil
	}
	r.Info("No changeset-id specified, using last changeset.")
	changesets, err := r.Backend.GetPackageChangesets()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(changesets) == 0 {
		return nil, trace.NotFound("no updates found")
	}
	changeset := &changesets[0]
	return changeset, nil
}

// Reinstall reinstalls the package specified by update
func (r *PackageUpdater) Reinstall(ctx context.Context, update storage.PackageUpdate) error {
	if err := r.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	labelUpdates, err := r.reinstallPackage(ctx, update)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(labelUpdates) == 0 {
		return nil
	}
	return applyLabelUpdates(r.Packages, labelUpdates)
}

func (r *PackageUpdater) reinstallPackage(ctx context.Context, update storage.PackageUpdate) ([]pack.LabelUpdate, error) {
	r.WithField("update", update).Info("Reinstalling package.")
	switch {
	case update.To.Name == constants.GravityPackage:
		return r.updateGravityPackage(ctx, update.To)
	case pack.IsPlanetPackage(update.To, update.Labels):
		updates, err := r.updatePlanetPackage(ctx, update)
		return updates, trace.Wrap(err)
	case update.To.Name == constants.TeleportPackage:
		updates, err := r.updateTeleportPackage(update)
		return updates, trace.Wrap(err)
	case pack.IsSecretsPackage(update.To, update.Labels):
		updates, err := r.reinstallSecretsPackage(update.To)
		return updates, trace.Wrap(err)
	}
	return nil, trace.BadParameter("unsupported package: %v", update.To)
}

func (r *PackageUpdater) updateGravityPackage(ctx context.Context, newPackage loc.Locator) (labelUpdates []pack.LabelUpdate, err error) {
	for _, targetPath := range state.GravityBinPaths {
		labelUpdates, err = reinstallBinaryPackage(r.Packages, newPackage, targetPath)
		if err == nil {
			break
		}
		r.WithFields(logrus.Fields{
			logrus.ErrorKey: err,
			"path":          targetPath,
		}).Warn("Failed to install gravity binary.")
	}
	if err != nil {
		return nil, trace.Wrap(err, "failed to install gravity binary in any of %v",
			state.GravityBinPaths)
	}
	planetPath, err := getRuntimePackagePath(r.Packages)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find planet package")
	}
	targetPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.GravityBin)
	err = copyGravityToPlanet(newPackage, r.Packages, targetPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to copy gravity inside planet")
	}
	err = r.applySELinuxFileContexts(ctx, targetPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return labelUpdates, nil
}

func (r *PackageUpdater) updatePlanetPackage(ctx context.Context, update storage.PackageUpdate) (labelUpdates []pack.LabelUpdate, err error) {
	var gravityPackageFilter = loc.MustCreateLocator(
		defaults.SystemAccountOrg, constants.GravityPackage, loc.ZeroVersion)
	err = unpack(r.Packages, update.To, r.planetUnpackOptions())
	if err != nil {
		return nil, trace.Wrap(err, "failed to unpack package %v", update.To)
	}

	planetPath, err := r.Packages.UnpackedPath(update.To)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Look up installed packages
	gravityPackage, err := pack.FindInstalledPackage(r.Packages, gravityPackageFilter)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find installed package for gravity")
	}

	targetPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.GravityBin)
	err = copyGravityToPlanet(*gravityPackage, r.Packages, targetPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to copy gravity inside planet")
	}

	err = updateSymlinks(planetPath, r.Logger)
	if err != nil {
		r.WithError(err).Warn("kubectl will not work on host.")
	}

	labelUpdates, err = r.reinstallService(update, r.environForPlanetService())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = r.applySELinuxFileContexts(ctx, planetPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if update.ConfigPackage != nil {
		labelUpdates = append(labelUpdates, labelsForPackageUpdate(r.Packages, *update.ConfigPackage)...)
	}

	return labelUpdates, nil
}

func (r *PackageUpdater) planetUnpackOptions() *archive.TarOptions {
	return &archive.TarOptions{
		NoLchown: true,
		UIDMaps: []idtools.IDMap{
			{
				ContainerID: defaults.ServiceUID,
				HostID:      r.ServiceUser.UID,
				Size:        1,
			},
			{
				ContainerID: constants.RootUID,
				HostID:      constants.RootUID,
				Size:        1,
			},
		},
		GIDMaps: []idtools.IDMap{
			{
				ContainerID: defaults.ServiceGID,
				HostID:      r.ServiceUser.GID,
				Size:        1,
			},
			{
				ContainerID: constants.RootGID,
				HostID:      constants.RootGID,
				Size:        1,
			},
		},
	}
}

func (r *PackageUpdater) updateTeleportPackage(update storage.PackageUpdate) (labelUpdates []pack.LabelUpdate, err error) {
	err = unpack(r.Packages, update.To, nil)
	if err != nil {
		return nil, trace.Wrap(err, "failed to unpack package %v", update.To)
	}
	updates, err := r.reinstallService(update, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// tctl has to be run on auth server nodes only.
	if r.ClusterRole == defaults.RoleMaster {
		err = updateTctlScript(r.Packages, update.To, r.Logger)
		if err != nil {
			r.WithError(err).Error("Failed to update tctl script.")
		}
	}
	if update.ConfigPackage == nil {
		return updates, nil
	}
	if update.ConfigPackage.From.IsEqualTo(update.ConfigPackage.To) {
		// Short-circuit on idempotent configuration update
		return updates, nil
	}
	return append(updates,
		pack.LabelUpdate{
			Locator: update.ConfigPackage.From,
			Remove:  []string{pack.InstalledLabel},
		},
		pack.LabelUpdate{
			Locator: update.ConfigPackage.To,
			Add: map[string]string{
				pack.InstalledLabel: pack.InstalledLabel,
			},
		},
	), nil
}

func (r *PackageUpdater) reinstallSecretsPackage(newPackage loc.Locator) (labelUpdates []pack.LabelUpdate, err error) {
	prevPackage, err := pack.FindInstalledPackage(r.Packages, newPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	targetPath, err := localenv.InGravity(defaults.SecretsDir)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine secrets directory")
	}

	opts, err := archiveutils.GetChownOptionsForDir(targetPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = pack.Unpack(r.Packages, newPackage, targetPath, &archive.TarOptions{
		ChownOpts: opts,
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to unpack package %v", newPackage)
	}

	labelUpdates = append(labelUpdates,
		pack.LabelUpdate{Locator: *prevPackage, Remove: []string{pack.InstalledLabel}},
		pack.LabelUpdate{Locator: newPackage, Add: pack.InstalledLabels},
	)

	r.WithFields(logrus.Fields{
		"target-path": targetPath,
		"package":     newPackage,
	}).Info("Installed secrets package.", newPackage, targetPath)
	return labelUpdates, nil
}

func (r *PackageUpdater) reinstallService(update storage.PackageUpdate, environ map[string]string) (labelUpdates []pack.LabelUpdate, err error) {
	services, err := systemservice.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	packageUpdates, err := uninstallPackage(services, update.From, r.Logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	labelUpdates = append(labelUpdates, packageUpdates...)

	manifest, err := r.Packages.GetPackageManifest(update.To)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if manifest.Service == nil {
		return nil, trace.NotFound("%v needs service section in manifest to be installed",
			update.To)
	}

	var configPackage loc.Locator
	if update.ConfigPackage == nil {
		existingConfig, err := pack.FindInstalledConfigPackage(r.Packages, update.From)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		configPackage = *existingConfig
	} else {
		configPackage = update.ConfigPackage.To
	}

	err = unpack(r.Packages, configPackage, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gravityPath, err := exec.LookPath(constants.GravityBin)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find %v binary in PATH",
			constants.GravityBin)
	}

	manifest.Service.Package = update.To
	manifest.Service.ConfigPackage = configPackage
	manifest.Service.GravityPath = gravityPath
	if manifest.Service.Environment == nil {
		manifest.Service.Environment = make(map[string]string)
	}
	for key, value := range environ {
		manifest.Service.Environment[key] = value
	}

	r.WithField("package", update.To).Info("Installing new package.")
	if err = services.InstallPackageService(*manifest.Service); err != nil {
		return nil, trace.Wrap(err, "error installing %v service", manifest.Service.Package)
	}

	labelUpdates = append(labelUpdates,
		pack.LabelUpdate{
			Locator: update.To,
			Add:     utils.CombineLabels(update.Labels, pack.InstalledLabels),
		})

	r.WithField("service", update.To).Info("Successfully installed.")
	return labelUpdates, nil
}

func (r *PackageUpdater) checkAndSetDefaults() error {
	if r.Logger == nil {
		r.Logger = log.New(logrus.WithField(trace.Component, "packupdate"))
	}
	return nil
}

func (r *PackageUpdater) environForPlanetService() map[string]string {
	return map[string]string{
		defaults.PlanetSELinuxEnv: strconv.FormatBool(r.SELinux),
	}
}

func (r *PackageUpdater) applySELinuxFileContexts(ctx context.Context, path string) error {
	if !(selinux.GetEnabled() && r.SELinux) {
		r.Info("SELinux is disabled.")
		return nil
	}
	w := r.Logger.Writer()
	defer w.Close()
	r.WithField("path", path).Info("Restore file contexts.")
	return libselinux.ApplyFileContexts(ctx, w, path)
}

// WithSELinux sets SELinux support
func WithSELinux(on bool) PackageUpdaterOption {
	return func(p *PackageUpdater) {
		p.SELinux = on
	}
}

// PackageUpdaterOption is a functional configuration option for PackageUpdater
type PackageUpdaterOption func(*PackageUpdater)

// PackageUpdater manages the updates to a known subset of packages
type PackageUpdater struct {
	// Logger specifies the logger
	log.Logger
	// Packages specifies the package service to use
	Packages update.LocalPackageService
	// ClusterRole specifies cluster role of the node this system updater runs on
	ClusterRole string
	// ServiceUser specifies the container service user
	ServiceUser systeminfo.User
	// SELinux specifies whether SELinux support is on
	SELinux bool
}

func updateTctlScript(packages update.LocalPackageService, newPackage loc.Locator, logger log.Logger) error {
	scriptBytes, err := renderTctlScript(packages, newPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	var errors []error
	for _, path := range []string{defaults.TctlBin, defaults.TctlBinAlternate} {
		err = utils.CopyExecutable(path, bytes.NewReader(scriptBytes))
		if err == nil {
			logger.WithField("path", path).Info("Updated tctl script.")
			return nil
		}
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// renderTctlScript renders the contents of the script that invokes tctl binary
// with appropriate configuration.
func renderTctlScript(packages update.LocalPackageService, newPackage loc.Locator) ([]byte, error) {
	// First, make up configuration for tctl. It only requires the data_dir
	// to be set. The data directory points to the location where teleport
	// masters embedded into gravity-sites store their data on disk.
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configBytes, err := yaml.Marshal(teleconfig.FileConfig{
		Global: teleconfig.Global{
			DataDir: filepath.Join(stateDir, defaults.SiteDir, defaults.TeleportDir),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Now, render the script which invokes the tctl binary from the specified
	// unpacked teleport package and passes configuration to it via environment
	// variable as a base64-encoded string.
	teleportPath, err := packages.UnpackedPath(newPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var buffer bytes.Buffer
	err = tctlScript.Execute(&buffer, map[string]string{
		"path": filepath.Join(teleportPath, defaults.RootfsDir, defaults.TctlBin),
		"conf": base64.StdEncoding.EncodeToString(configBytes),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return buffer.Bytes(), nil
}

func getRuntimePackagePath(packages update.LocalPackageService) (packagePath string, err error) {
	runtimePackage, err := pack.FindRuntimePackage(packages)
	if err != nil {
		return "", trace.Wrap(err)
	}
	packagePath, err = packages.UnpackedPath(*runtimePackage)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return packagePath, nil
}

func updateSymlinks(planetPath string, logger log.Logger) (err error) {
	// update kubectl symlink
	kubectlPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.KubectlScript)
	var out []byte
	for _, path := range []string{defaults.KubectlBin, defaults.KubectlBinAlternate} {
		out, err = exec.Command("ln", "-sfT", kubectlPath, path).CombinedOutput()
		if err == nil {
			logger.Infof("Updated kubectl symlink: %v -> %v.", path, kubectlPath)
			break
		}
		logger.WithError(err).WithField("output", string(out)).Warn(
			"Failed to update kubectl symlink.")
	}

	// update helm symlink
	helmPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.HelmScript)
	for _, path := range []string{defaults.HelmBin, defaults.HelmBinAlternate} {
		out, err = exec.Command("ln", "-sfT", helmPath, path).CombinedOutput()
		if err == nil {
			logger.Infof("Updated helm symlink: %v -> %v.", path, helmPath)
			break
		}
		logger.WithError(err).WithField("output", string(out)).Warn(
			"Failed to update helm symlink.")
	}

	// update kube config environment variable
	kubeConfigPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.PlanetKubeConfigPath)
	environment, err := utils.ReadEnv(defaults.EnvironmentPath)
	if err != nil {
		return trace.Wrap(err)
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}

	// update kubeconfig symlink
	kubectlSymlink := filepath.Join(stateDir, constants.KubectlConfig)
	out, err = exec.Command("ln", "-sfT", kubeConfigPath, kubectlSymlink).CombinedOutput()
	if err != nil {
		return trace.Wrap(err, "failed to update %v symlink: %v",
			kubectlSymlink, string(out))
	}

	environment[constants.EnvKubeConfig] = kubeConfigPath
	err = utils.WriteEnv(defaults.EnvironmentPath, environment)
	if err != nil {
		return trace.Wrap(err, "failed to update %v environment variable in %v",
			constants.EnvKubeConfig, defaults.EnvironmentPath)
	}

	return nil
}

func copyGravityToPlanet(newPackage loc.Locator, packages pack.PackageService, targetPath string) error {
	_, rc, err := packages.ReadPackage(newPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	defer rc.Close()

	return trace.Wrap(utils.CopyReaderWithPerms(targetPath, rc, defaults.SharedExecutableMask))
}

func labelsForPackageUpdate(packages pack.PackageService, update storage.PackageUpdate) (labelUpdates []pack.LabelUpdate) {
	return append(labelUpdates,
		pack.LabelUpdate{
			Locator: update.From,
			Remove:  []string{pack.InstalledLabel},
		},
		pack.LabelUpdate{
			Locator: update.To,
			Add: utils.CombineLabels(
				update.Labels,
				pack.InstalledLabels,
			),
		})
}

func reinstallBinaryPackage(packages pack.PackageService, newPackage loc.Locator, targetPath string) ([]pack.LabelUpdate, error) {
	prevPackage, err := pack.FindInstalledPackage(packages, newPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, reader, err := packages.ReadPackage(newPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()

	err = utils.CopyReaderWithPerms(targetPath, reader, defaults.SharedExecutableMask)
	if err != nil {
		return nil, trace.Wrap(err, "failed to copy package %v to %v", newPackage, targetPath)
	}

	var updates []pack.LabelUpdate
	updates = append(updates,
		pack.LabelUpdate{Locator: *prevPackage, Remove: []string{pack.InstalledLabel}},
		pack.LabelUpdate{Locator: newPackage, Add: pack.InstalledLabels},
	)

	fmt.Printf("binary package %v installed in %v\n", newPackage, targetPath)
	return updates, nil
}

func applyLabelUpdates(packages pack.PackageService, labelUpdates []pack.LabelUpdate) error {
	var errors []error
	for _, update := range labelUpdates {
		err := packages.UpdatePackageLabels(update.Locator, update.Add, update.Remove)
		if err != nil && !trace.IsNotFound(err) {
			errors = append(errors, trace.Wrap(err, "error applying %v", update))
		}
	}
	return trace.NewAggregate(errors...)
}

func uninstallPackage(
	services systemservice.ServiceManager,
	servicePackage loc.Locator,
	logger log.Logger,
) (updates []pack.LabelUpdate, err error) {
	installed, err := services.IsPackageServiceInstalled(servicePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if installed {
		logger.WithField("service", servicePackage).Info("Package installed as a service, will uninstall.")
		err = services.UninstallPackageService(servicePackage)
		if err != nil {
			return nil, utils.NewUninstallServiceError(err, servicePackage)
		}
	}
	updates = append(updates, pack.LabelUpdate{
		Locator: servicePackage,
		Remove:  []string{pack.InstalledLabel},
	})
	return updates, nil
}

// needsPackageUpdate determines whether the package specified with u needs to be updated on local host.
// Returns a storage.PackageUpdate if either the package or its configuration package needs an update
func needsPackageUpdate(packages pack.PackageService, u storage.PackageUpdate) (update *storage.PackageUpdate, err error) {
	format := func(u storage.PackageUpdate) string {
		if u.ConfigPackage == nil {
			return u.To.String()
		}
		return fmt.Sprintf("%v (configuration %v)", u.To, u.ConfigPackage.To)
	}
	err = updateFromInstalled(packages, &u)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	packageUpdate := err == nil
	var configPackageUpdate bool
	if u.ConfigPackage != nil {
		err = updateFromInstalled(packages, u.ConfigPackage)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		configPackageUpdate = err == nil
	}
	if !packageUpdate && !configPackageUpdate {
		return nil, trace.NotFound("package %v is already the latest version", format(u))
	}
	return &u, nil
}

func updateFromInstalled(localPackages pack.PackageService, update *storage.PackageUpdate) (err error) {
	installed := &update.From
	if installed.IsEmpty() {
		installed, err = pack.FindInstalledPackage(localPackages, update.To)
		if err != nil {
			return trace.Wrap(err)
		}
		update.From = *installed
	}
	if installed.IsEqualTo(update.To) {
		return trace.NotFound("package %v is already the latest version", update.To)
	}
	return nil
}

func ensureServiceRunning(servicePackage loc.Locator) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	noBlock := true
	err = services.StartPackageService(servicePackage, noBlock)
	return trace.Wrap(err)
}

func waitNodeStatus(ctx context.Context) (err error) {
	b := utils.NewExponentialBackOff(defaults.NodeStatusTimeout)
	err = utils.RetryWithInterval(ctx, b, func() error {
		return trace.Wrap(getLocalNodeStatus(ctx))
	})
	return trace.Wrap(err)
}

func getLocalNodeStatus(ctx context.Context) (err error) {
	var status *libstatus.Agent
	b := utils.NewExponentialBackOff(defaults.NodeStatusTimeout)
	err = utils.RetryTransient(ctx, b, func() error {
		status, err = libstatus.FromLocalPlanetAgent(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if status.GetSystemStatus() != agentpb.SystemStatus_Running {
		return trace.BadParameter("node is degraded")
	}
	return nil
}

// unpack reads the package from the package service and unpacks its contents
// to the default package unpack directory
func unpack(packages update.LocalPackageService, loc loc.Locator, opts *archive.TarOptions) error {
	path, err := packages.UnpackedPath(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	return packages.Unpack(loc, path)
}

// tctlScript is the template of the script that invokes tctl binary with
// configuration supplied via an environment variable.
var tctlScript = template.Must(template.New("tctl").Parse(`#!/bin/bash
TELEPORT_CONFIG={{.conf}} {{.path}} "$@"`))
