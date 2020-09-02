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

package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/environ"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/update/system"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/coreos/go-semver/semver"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/gravitational/configure"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// systemPullUpdates pulls new packages from remote Ops Center
func systemPullUpdates(env *localenv.LocalEnvironment, opsCenterURL string, runtimePackage loc.Locator) error {
	targetURL, err := env.SelectOpsCenter(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	remotePackages, err := env.PackageService(targetURL)
	if err != nil {
		return trace.Wrap(err)
	}

	reqs, err := findPackages(env.Packages, runtimePackage)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, req := range reqs {
		log := log.WithField("package", req.installedPackage)
		log.Info("Checking for update.")
		update, err := findPackageUpdate(env.Packages, remotePackages, req)
		if err != nil {
			if trace.IsNotFound(err) {
				log.Info("No update found.")
				continue
			}
			return trace.Wrap(err)
		}
		log.WithField("update", update).Info("Pulling update.")
		env.Printf("Pulling update %v\n.", update)
		err = pullUpdate(context.TODO(), env.Packages, remotePackages, env.Reporter, *update)
		if err != nil {
			return trace.Wrap(err)
		}
		if update.ConfigPackage != nil {
			err = pullUpdate(context.TODO(), env.Packages, remotePackages, env.Reporter,
				*update.ConfigPackage)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// systemUpdate searches and applies package updates if any
func systemUpdate(env *localenv.LocalEnvironment, changesetID string, serviceName string, withStatus bool,
	runtimePackage loc.Locator) error {
	if serviceName != "" {
		args := []string{"system", "update",
			"--changeset-id", changesetID,
			"--runtime-package", runtimePackage.String(),
			"--debug"}
		if withStatus {
			args = append(args, "--with-status")
		}
		return trace.Wrap(service.ReinstallOneshotSimple(serviceName, args...))
	}

	reqs, err := findPackages(env.Packages, runtimePackage)
	if err != nil {
		return trace.Wrap(err)
	}

	var changes []storage.PackageUpdate
	for _, req := range reqs {
		log := log.WithField("package", req)
		log.Info("Checking for update.")
		update, err := findPackageUpdate(env.Packages, env.Packages, req)
		if err != nil {
			if trace.IsNotFound(err) {
				log.Info("No update found.")
				continue
			}
			return trace.Wrap(err)
		}
		update.Labels = req.labels
		log.WithField("package", update).Info("Found update.")
		changes = append(changes, *update)
	}
	if len(changes) == 0 {
		env.Println("System is already up to date")
		return nil
	}

	changeset, err := env.Backend.CreatePackageChangeset(storage.PackageChangeset{ID: changesetID, Changes: changes})
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	err = applyUpdates(env, changes)
	if err != nil {
		return trace.Wrap(err)
	}

	if !withStatus {
		env.Printf("System successfully updated: %v\n", changeset)
		return nil
	}

	err = ensureServiceRunning(runtimePackage)
	if err != nil {
		return trace.Wrap(err)
	}

	err = getLocalNodeStatus(env)
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("System successfully updated: %v\n", changeset)
	return nil
}

// systemRollback rolls back system to the specified changesetID or the last update if changesetID is not specified
func systemRollback(env *localenv.LocalEnvironment, changesetID, serviceName string, withStatus bool) (err error) {
	changeset, err := getChangesetByID(env, changesetID)
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("Rolling back %v\n", changeset)
	if serviceName != "" {
		args := []string{"system", "rollback", "--changeset-id", changeset.ID, "--debug"}
		if withStatus {
			args = append(args, "--with-status")
		}
		return trace.Wrap(service.ReinstallOneshotSimple(serviceName, args...))
	}

	changes := changeset.ReversedChanges()
	rollback, err := env.Backend.CreatePackageChangeset(storage.PackageChangeset{Changes: changes})
	if err != nil {
		return trace.Wrap(err)
	}

	err = applyUpdates(env, changes)
	if err != nil {
		return trace.Wrap(err)
	}

	if !withStatus {
		env.Printf("System rolled back: %v\n", rollback)
		return nil
	}

	err = getLocalNodeStatus(env)
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("System rolled back: %v\n", rollback)
	return nil
}

// systemHistory prints upgrade history
func systemHistory(env *localenv.LocalEnvironment) error {
	changesets, err := env.Backend.GetPackageChangesets()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(changesets) == 0 {
		env.Println("There are no updates recorded")
		return nil
	}
	for _, changeset := range changesets {
		env.Printf("* %v\n", changeset)
	}
	return nil
}

func systemReinstall(env *localenv.LocalEnvironment, newPackage loc.Locator, serviceName string, labels map[string]string) error {
	if serviceName == "" {
		updater, err := system.NewPackageUpdater(env.Packages)
		if err != nil {
			return trace.Wrap(err)
		}
		update := storage.PackageUpdate{
			From:   newPackage,
			To:     newPackage,
			Labels: labels,
		}
		return trace.Wrap(updater.Reinstall(update))
	}

	args := []string{"system", "reinstall", newPackage.String()}
	if len(labels) != 0 {
		kvs := configure.KeyVal(labels)
		args = append(args, "--labels", kvs.String())
	}
	err := service.ReinstallOneshotSimple(serviceName, args...)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func systemBlockingReinstall(env *localenv.LocalEnvironment, update storage.PackageUpdate) error {
	labelUpdates, err := systemReinstallPackage(env, update)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(labelUpdates) == 0 {
		return nil
	}
	return applyLabelUpdates(env.Packages, labelUpdates)
}

func systemReinstallPackage(env *localenv.LocalEnvironment, update storage.PackageUpdate) ([]pack.LabelUpdate, error) {
	log.WithField("update", update).Info("Reinstalling package.")
	switch {
	case update.To.Name == constants.GravityPackage:
		return updateGravityPackage(env.Packages, update.To)
	case pack.IsPlanetPackage(update.To, update.Labels):
		updates, err := updatePlanetPackage(env, update)
		return updates, trace.Wrap(err)
	case update.To.Name == constants.TeleportPackage:
		updates, err := updateTeleportPackage(env, update)
		return updates, trace.Wrap(err)
	case pack.IsSecretsPackage(update.To, update.Labels):
		updates, err := reinstallSecretsPackage(env, update.To)
		return updates, trace.Wrap(err)
	}
	return nil, trace.BadParameter("unsupported package: %v", update.To)
}

// reinstallOneshotService stops and reinstalls the service specified by
// serviceName as a oneshot service.
func reinstallOneshotService(env *localenv.LocalEnvironment, serviceName string, cmd []string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	err = services.StopService(serviceName)
	if err != nil {
		log.Warnf("Error stopping service %v: %v.", serviceName, err)
	}

	err = services.InstallService(systemservice.NewServiceRequest{
		Name:    serviceName,
		NoBlock: true,
		ServiceSpec: systemservice.ServiceSpec{
			User:            constants.RootUIDString,
			Type:            service.OneshotService,
			StartCommand:    strings.Join(cmd, " "),
			RemainAfterExit: true,
		},
	})
	return trace.Wrap(err)
}

func applyUpdates(env *localenv.LocalEnvironment, updates []storage.PackageUpdate) error {
	var errors []error
	for _, u := range updates {
		env.Printf("Applying %v\n", u)
		err := systemBlockingReinstall(env, u)
		if err != nil {
			log.WithFields(logrus.Fields{
				"from": u.From,
				"to":   u.To,
			}).Warnf("Failed to reinstall: %v.", trace.DebugReport(err))
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func pullUpdate(ctx context.Context, localPackages, remotePackages pack.PackageService, reporter pack.ProgressReporter, update storage.PackageUpdate) error {
	puller := libapp.Puller{
		SrcPack:  remotePackages,
		DstPack:  localPackages,
		Progress: reporter,
	}
	err := puller.PullPackage(ctx, update.To)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	return nil
}

// findPackages returns a list of additional packages to pull during update.
func findPackages(packages pack.PackageService, runtimePackageUpdate loc.Locator) (reqs []packageRequest, err error) {
	secretsPackage, err := pack.FindSecretsPackage(packages)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find installed secrets package")
	}

	installedSecretsPackage, err := pack.FindInstalledPackage(packages, *secretsPackage)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find installed secrets package")
	}

	existingRuntime, existingRuntimeConfig, err := pack.FindRuntimePackageWithConfig(packages)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find runtime package")
	}
	log.WithFields(logrus.Fields{
		"runtime": existingRuntime,
		"config":  existingRuntimeConfig,
	}).Info("Found existing runtime and configuration packages.")

	runtimeConfigUpdate, err := maybeConvertLegacyPlanetConfigPackage(existingRuntimeConfig.Locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateGravityPackage, err := newPackageRequest(packages, gravityPackageFilter)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find installed gravity binary package")
	}

	existingTeleport, existingTeleportConfig, err := pack.FindTeleportPackageWithConfig(packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.WithFields(logrus.Fields{
		logrus.ErrorKey: err,
		"package":       existingTeleport,
		"config":        existingTeleportConfig,
	}).Info("Found existing teleport and configuration packages.")

	runtimeConfigLabels := map[string]string{
		pack.PurposeLabel:     pack.PurposePlanetConfig,
		pack.ConfigLabel:      existingRuntime.Locator.ZeroVersion().String(),
		pack.AdvertiseIPLabel: existingRuntimeConfig.RuntimeLabels[pack.AdvertiseIPLabel],
	}
	teleportConfigLabels := map[string]string{
		pack.PurposeLabel:     pack.PurposeTeleportNodeConfig,
		pack.ConfigLabel:      existingTeleport.Locator.ZeroVersion().String(),
		pack.AdvertiseIPLabel: existingTeleportConfig.RuntimeLabels[pack.AdvertiseIPLabel],
	}
	reqs = append(reqs,
		*updateGravityPackage,
		packageRequest{
			installedPackage: *installedSecretsPackage,
			labels:           pack.RuntimeSecretsPackageLabels,
		},
		packageRequest{
			installedPackage: existingRuntime.Locator,
			updatePackage:    &runtimePackageUpdate,
			labels:           pack.RuntimePackageLabels,
			configPackage: &packageRequest{
				installedPackage: existingRuntimeConfig.Locator,
				updateFilter:     runtimeConfigUpdate,
				labels:           runtimeConfigLabels,
				less:             configPackageLess,
			},
		},
		packageRequest{
			installedPackage: existingTeleport.Locator,
			configPackage: &packageRequest{
				installedPackage: existingTeleportConfig.Locator,
				labels:           teleportConfigLabels,
			},
		},
	)
	log.WithField("requests", packageRequests(reqs)).Debug("Find package updates.")
	return reqs, nil
}

func getChangesetByID(env *localenv.LocalEnvironment, changesetID string) (*storage.PackageChangeset, error) {
	if changesetID != "" {
		changeset, err := env.Backend.GetPackageChangeset(changesetID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return changeset, nil
	}

	env.Println("No changeset-id specified, using last changeset")
	changesets, err := env.Backend.GetPackageChangesets()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(changesets) == 0 {
		return nil, trace.NotFound("no updates found")
	}
	changeset := &changesets[0]
	return changeset, nil
}

func updateGravityPackage(packages *localpack.PackageServer, newPackage loc.Locator) (labelUpdates []pack.LabelUpdate, err error) {
	for _, targetPath := range state.GravityBinPaths {
		labelUpdates, err = reinstallBinaryPackage(packages, newPackage, targetPath)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, trace.Wrap(err, "failed to install gravity binary in any of %v",
			state.GravityBinPaths)
	}

	planetPath, err := getRuntimePackagePath(packages)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find planet package")
	}

	err = copyGravityToPlanet(newPackage, packages, planetPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to copy gravity inside planet")
	}
	return labelUpdates, nil
}

func getAnyRuntimePackagePath(packages *localpack.PackageServer) (packagePath string, err error) {
	runtimePackage, err := pack.FindAnyRuntimePackage(packages)
	if err != nil {
		return "", trace.Wrap(err)
	}
	packagePath, err = packages.UnpackedPath(*runtimePackage)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return packagePath, nil
}

func getRuntimePackagePath(packages *localpack.PackageServer) (packagePath string, err error) {
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

func updatePlanetPackage(env *localenv.LocalEnvironment, update storage.PackageUpdate) (labelUpdates []pack.LabelUpdate, err error) {
	err = env.Packages.Unpack(update.To, "")
	if err != nil {
		return nil, trace.Wrap(err, "failed to unpack package %v", update.To)
	}

	planetPath, err := env.Packages.UnpackedPath(update.To)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Look up installed packages
	gravityPackage, err := pack.FindInstalledPackage(env.Packages, gravityPackageFilter)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find installed package for gravity")
	}

	err = copyGravityToPlanet(*gravityPackage, env.Packages, planetPath)
	if err != nil {
		return nil, trace.Wrap(err, "failed to copy gravity inside planet")
	}

	err = updateKubectl(planetPath)
	if err != nil {
		log.Warningf("kubectl will not work on host: %v", trace.DebugReport(err))
	}

	labelUpdates, err = reinstallSystemService(env, update)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	configLabelUpdates := updateRuntimeConfigPackageLabels(env.Packages, update)
	labelUpdates = append(labelUpdates, configLabelUpdates...)

	return labelUpdates, nil
}

func updateTeleportPackage(env *localenv.LocalEnvironment, update storage.PackageUpdate) (labelUpdates []pack.LabelUpdate, err error) {
	updates, err := reinstallSystemService(env, update)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if update.ConfigPackage == nil {
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

func updateKubectl(planetPath string) (err error) {
	// update kubectl symlink
	kubectlPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.KubectlScript)
	var out []byte
	for _, path := range []string{defaults.KubectlBin, defaults.KubectlBinAlternate} {
		out, err = exec.Command("ln", "-sfT", kubectlPath, path).CombinedOutput()
		if err == nil {
			log.Infof("Updated kubectl symlink: %v -> %v.", path, kubectlPath)
			break
		}
		log.Warnf("Failed to update kubectl symlink: %s (%v).", out, err)
	}

	// update helm symlink
	helmPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.HelmScript)
	for _, path := range []string{defaults.HelmBin, defaults.HelmBinAlternate} {
		out, err = exec.Command("ln", "-sfT", helmPath, path).CombinedOutput()
		if err == nil {
			log.Infof("Updated helm symlink: %v -> %v.", path, helmPath)
			break
		}
		log.Warnf("Failed to update helm symlink: %s (%v).", out, err)
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

func copyGravityToPlanet(newPackage loc.Locator, packages pack.PackageService, planetPath string) error {
	targetPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.GravityBin)

	_, reader, err := packages.ReadPackage(newPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	return trace.Wrap(utils.CopyReaderWithPerms(targetPath, reader, defaults.SharedExecutableMask))
}

func reinstallSecretsPackage(env *localenv.LocalEnvironment, newPackage loc.Locator) (labelUpdates []pack.LabelUpdate, err error) {
	prevPackage, err := pack.FindInstalledPackage(env.Packages, newPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	targetPath, err := localenv.InGravity(defaults.SecretsDir)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine secrets directory")
	}

	opts, err := getChownOptionsForDir(targetPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = pack.Unpack(env.Packages, newPackage, targetPath, &archive.TarOptions{
		ChownOpts: opts,
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to unpack package %v", newPackage)
	}

	labelUpdates = append(labelUpdates,
		pack.LabelUpdate{Locator: *prevPackage, Remove: []string{pack.InstalledLabel}},
		pack.LabelUpdate{Locator: newPackage, Add: pack.InstalledLabels},
	)

	env.Printf("Secrets package %v installed in %v\n", newPackage, targetPath)
	return labelUpdates, nil
}

func updateRuntimeConfigPackageLabels(
	packages pack.PackageService,
	update storage.PackageUpdate,
) (labelUpdates []pack.LabelUpdate) {
	if update.ConfigPackage == nil {
		return nil
	}
	return append(labelUpdates,
		pack.LabelUpdate{
			Locator: update.ConfigPackage.From,
			Remove:  []string{pack.InstalledLabel},
		},
		pack.LabelUpdate{
			Locator: update.ConfigPackage.To,
			Add: utils.CombineLabels(
				pack.ConfigLabels(update.To, pack.PurposePlanetConfig),
				pack.InstalledLabels,
			),
		})
}

func getChownOptionsForDir(dir string) (*idtools.Identity, error) {
	var uid, gid int
	// preserve owner/group when unpacking, otherwise use current process user
	fi, err := os.Stat(dir)
	if err == nil {
		stat := fi.Sys().(*syscall.Stat_t)
		uid = int(stat.Uid)
		gid = int(stat.Gid)
		log.Debugf("assuming UID:GID from existing directory %v:%v", uid, gid)
		return &idtools.Identity{
			UID: uid,
			GID: gid,
		}, nil
	}
	u, err := user.Current()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query current user")
	}

	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return nil, trace.BadParameter("UID is not a number: %q", u.Uid)
	}

	gid, err = strconv.Atoi(u.Gid)
	if err != nil {
		return nil, trace.BadParameter("GID is not a number: %q", u.Gid)
	}

	log.Debugf("assuming UID:GID from current user %v", u)
	return &idtools.Identity{
		UID: uid,
		GID: gid,
	}, nil
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
		errors = append(errors, trace.Wrap(err, "error applying %v", update))
	}
	return trace.NewAggregate(errors...)
}

func reinstallSystemService(env *localenv.LocalEnvironment, update storage.PackageUpdate) (labelUpdates []pack.LabelUpdate, err error) {
	services, err := systemservice.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	packageUpdates, err := uninstallPackage(env, services, update.From)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	labelUpdates = append(labelUpdates, packageUpdates...)

	err = unpack(env.Packages, update.To)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	manifest, err := env.Packages.GetPackageManifest(update.To)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if manifest.Service == nil {
		return nil, trace.NotFound("%v needs service section in manifest to be installed",
			update.To)
	}

	var configPackage loc.Locator
	if update.ConfigPackage == nil {
		existingConfig, err := pack.FindConfigPackage(env.Packages, update.From)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		configPackage = *existingConfig
	} else {
		configPackage = update.ConfigPackage.To
	}

	err = unpack(env.Packages, configPackage)
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

	log.WithField("package", update.To).Info("Installing new package.")
	if err = services.InstallPackageService(*manifest.Service); err != nil {
		return nil, trace.Wrap(err, "error installing %v service", manifest.Service.Package)
	}

	labelUpdates = append(labelUpdates,
		pack.LabelUpdate{
			Locator: update.To,
			Add:     utils.CombineLabels(update.Labels, pack.InstalledLabels),
		})

	env.Printf("%v successfully installed\n", update.To)
	return labelUpdates, nil
}

func uninstallPackage(
	printer utils.Printer,
	services systemservice.ServiceManager,
	servicePackage loc.Locator,
) (updates []pack.LabelUpdate, err error) {
	installed, err := services.IsPackageServiceInstalled(servicePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if installed {
		printer.Printf("%v is installed as a service, uninstalling\n", servicePackage)
		err = services.UninstallPackageService(servicePackage)
		if err != nil {
			return nil, utils.NewUninstallServiceError(servicePackage)
		}
	}
	updates = append(updates, pack.LabelUpdate{
		Locator: servicePackage,
		Remove:  []string{pack.InstalledLabel},
	})
	return updates, nil
}

// systemServiceInstall installs a package as a system service
func systemServiceInstall(env *localenv.LocalEnvironment, req *systemservice.NewPackageServiceRequest) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	req.GravityPath, err = exec.LookPath(constants.GravityBin)
	if err != nil {
		return trace.Wrap(err, "failed to find %v binary in PATH",
			constants.GravityBin)
	}

	// Unpack the service package to make sure the package directory
	// is not partial
	err = unpack(env.Packages, req.Package)
	if err != nil {
		return trace.Wrap(err)
	}

	manifest, err := env.Packages.GetPackageManifest(req.Package)
	if err != nil {
		return trace.Wrap(err)
	}

	err = unpack(env.Packages, req.ConfigPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	if manifest.Service != nil {
		out := systemservice.MergeInto(*req, *manifest.Service)
		log.Infof("merging service %#v into %#v result: %#v",
			*req, *manifest.Service, out,
		)
		*req = out
	}

	if err = services.InstallPackageService(*req); err != nil {
		return trace.Wrap(err, "error installing service")
	}

	env.Println("service installed")
	return nil
}

// systemServiceUninstall uninstalls a package as a system service
func systemServiceUninstall(env *localenv.LocalEnvironment, pkg loc.Locator, serviceName string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	switch {
	case serviceName != "":
		err = services.UninstallService(systemservice.UninstallServiceRequest{
			Name: serviceName,
		})
	case !pkg.IsEmpty():
		if pkg.Version == loc.ZeroVersion {
			statuses, err := services.ListPackageServices(systemservice.DefaultListServiceOptions)
			if err != nil {
				return trace.Wrap(err)
			}
			for _, status := range statuses {
				if status.Package.Name == pkg.Name {
					pkg = status.Package
					break
				}
			}
		}
		err = services.UninstallPackageService(pkg)
	default:
		err = trace.BadParameter("need a package name or a service name")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	env.Println("service uninstalled")
	return nil
}

// systemServiceList lists all packages
func systemServiceList(env *localenv.LocalEnvironment) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	statuses, err := services.ListPackageServices(systemservice.DefaultListServiceOptions)
	if err != nil {
		return trace.Wrap(err)
	}
	common.PrintHeader("services")
	for _, s := range statuses {
		fmt.Printf("* %v %v\n", s.Package, s.Status)
	}
	return nil
}

// systemServiceStart starts or restarts the package service specified with packagePattern
func systemServiceStart(env *localenv.LocalEnvironment, packagePattern string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	if loc, err := loc.ParseLocator(packagePattern); err == nil {
		return services.StartPackageService(*loc, noBlock)
	}
	loc, err := queryPackageServiceByPattern(services, packagePattern)
	if err != nil {
		return trace.Wrap(err)
	}
	return services.StartPackageService(*loc, noBlock)
}

// systemServiceStop stops the running package service specified with packagePattern
func systemServiceStop(env *localenv.LocalEnvironment, packagePattern string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	if loc, err := loc.ParseLocator(packagePattern); err == nil {
		return services.StopPackageService(*loc)
	}
	loc, err := queryPackageServiceByPattern(services, packagePattern)
	if err != nil {
		return trace.Wrap(err)
	}
	return services.StopPackageService(*loc)
}

// systemServiceJournal queries the system journal of the package service specified with packagePattern
func systemServiceJournal(env *localenv.LocalEnvironment, packagePattern string, args []string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	if loc, err := loc.ParseLocator(packagePattern); err == nil {
		return execJournalctl(*loc, args...)
	}
	loc, err := queryPackageServiceByPattern(services, packagePattern)
	if err != nil {
		return trace.Wrap(err)
	}
	return execJournalctl(*loc, args...)
}

// systemServiceStatus prints status of this service
func systemServiceStatus(env *localenv.LocalEnvironment, packagePattern string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	if loc, err := loc.ParseLocator(packagePattern); err == nil {
		return outputServiceStatus(services, *loc, env)
	}
	loc, err := queryPackageServiceByPattern(services, packagePattern)
	if err != nil {
		return trace.Wrap(err)
	}
	return outputServiceStatus(services, *loc, env)
}

func queryPackageServiceByPattern(services systemservice.ServiceManager, packagePattern string) (*loc.Locator, error) {
	statuses, err := services.ListPackageServices(systemservice.ListServiceOptions{
		All:     true,
		Type:    systemservice.UnitTypeService,
		Pattern: packageServicePattern(packagePattern),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(statuses) == 0 {
		return nil, trace.NotFound("service given with %q was not found", packagePattern)
	}
	if len(statuses) != 1 {
		return nil, trace.BadParameter("invalid service pattern %q specified", packagePattern)
	}
	return &statuses[0].Package, nil
}

// systemUninstall uninstalls all gravity components
func systemUninstall(env *localenv.LocalEnvironment, confirmed, uninstallService bool) error {
	if !confirmed {
		env.Println("This action will delete gravity and all the application data. Are you sure?")
		re, err := confirm()
		if err != nil {
			return trace.Wrap(err)
		}
		if !re {
			env.Println("Action cancelled by user.")
			return nil
		}
	}

	// close the backend before attempting to unmount as the open file might
	// prevent the umount from succeeding
	env.Backend.Close()

	logger := log.WithField(trace.Component, "system:uninstall")
	if uninstallService {
		logger.Info("Uninstall the system services.")
		if err := environ.UninstallServices(env, logger); err != nil {
			logger.WithError(err).Warn("Failed to uninstall agent services.")
		}
	}
	logger.Info("Uninstall the system.")
	if err := environ.UninstallSystem(env, logger); err != nil {
		logger.WithError(err).Warn("Failed to uninstall system.")
	}
	env.PrintStep("Gravity has been successfully uninstalled")
	return nil
}

func execJournalctl(loc loc.Locator, args ...string) error {
	const cmd = defaults.JournalctlBin
	args = append([]string{cmd, "--unit", systemservice.PackageServiceName(loc)}, args...)
	if err := syscall.Exec(cmd, args, os.Environ()); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to execve(%v, %v)", cmd, args)
	}
	return nil
}

func outputServiceStatus(services systemservice.ServiceManager, loc loc.Locator, printer utils.Printer) error {
	status, err := services.StatusPackageService(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	printer.Print(status)
	return nil
}

func packageServicePattern(pattern string) string {
	if strings.Index(pattern, "*") != -1 {
		return pattern
	}
	return fmt.Sprintf("*%v*", pattern)
}

// findPackageUpdate searches for remote update for the local package specified with req
func findPackageUpdate(localPackages, remotePackages pack.PackageService, req packageRequest) (*storage.PackageUpdate, error) {
	if req.configPackage == nil {
		update, err := findPackageUpdateHelper(remotePackages, req)
		return update, trace.Wrap(err)
	}

	packageUpdate, err := findPackageUpdateHelper(remotePackages, req)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	configUpdate, err := findPackageUpdateHelper(remotePackages, *req.configPackage)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if packageUpdate == nil && configUpdate == nil {
		return nil, trace.NotFound("%v/%v is already at the latest version",
			req.installedPackage, req.configPackage.installedPackage)
	}

	update := storage.PackageUpdate{
		From:   req.installedPackage,
		To:     req.installedPackage,
		Labels: req.labels,
	}
	if packageUpdate != nil {
		update = *packageUpdate
	}
	if configUpdate != nil {
		update.ConfigPackage = configUpdate
	}
	return &update, nil
}

func findPackageUpdateHelper(packages pack.PackageService, req packageRequest) (update *storage.PackageUpdate, err error) {
	if req.less == nil {
		req.less = pack.Less
	}
	latestPackage := req.updatePackage
	if latestPackage == nil {
		filter := req.updateSearchFilter()
		latestPackage, err = pack.FindLatestPackageCustom(pack.FindLatestPackageRequest{
			Packages:   packages,
			Repository: filter.Repository,
			Match:      req.match,
			Less:       req.less,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	currentVersion, err := req.installedPackage.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	latestVersion, err := latestPackage.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if req.less(currentVersion, latestVersion) {
		return &storage.PackageUpdate{
			From:   req.installedPackage,
			To:     *latestPackage,
			Labels: req.labels,
		}, nil
	}
	return nil, trace.NotFound("%v is already at the latest version", req.installedPackage)
}

func ensureServiceRunning(servicePackage loc.Locator) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	return services.StartPackageService(servicePackage, noBlock)
}

func getLocalNodeStatus(env *localenv.LocalEnvironment) error {
	err := utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		command := exec.Command("gravity", "planet", "status", "--", "--local")
		err := utils.Exec(command, env)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

// unpack reads the package from the package service and unpacks its contents
// to the default package unpack directory
func unpack(p *localpack.PackageServer, loc loc.Locator) error {
	path, err := p.UnpackedPath(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	return p.Unpack(loc, path)
}

func maybeConvertLegacyPlanetConfigPackage(configPackage loc.Locator) (*loc.Locator, error) {
	if configPackage.Name != constants.PlanetConfigPackage {
		// Nothing to do
		return nil, nil
	}

	ver, err := configPackage.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Format the new package name as <planet-config-prefix>-<prerelease>
	name := fmt.Sprintf("%v-%v", constants.PlanetConfigPackage, ver.PreRelease)
	convertedConfigPackage, err := loc.NewLocator(configPackage.Repository, name, configPackage.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return convertedConfigPackage, nil
}

func configPackageLess(a, b *semver.Version) bool {
	if pack.Less(a, b) {
		return true
	}
	return a.Metadata < b.Metadata
}

func newPackageRequest(packages pack.PackageService, filter loc.Locator) (*packageRequest, error) {
	installed, err := pack.FindInstalledPackage(packages, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &packageRequest{installedPackage: *installed}, nil
}

// String formats this list of requests as readable text
func (r packageRequests) String() string {
	var buf bytes.Buffer
	for _, req := range r {
		buf.WriteString(req.String())
		buf.WriteString(",")
	}
	return buf.String()
}

type packageRequests []packageRequest

// String formats this request as readable text
func (r packageRequest) String() string {
	maybe := func(loc *loc.Locator) string {
		if loc != nil {
			return loc.String()
		}
		return "<none>"
	}
	if r.configPackage == nil {
		return fmt.Sprintf("packageRequest(installed=%v, updatePackage=%v, labels=%v)",
			r.installedPackage, maybe(r.updatePackage), r.labels)
	}
	return fmt.Sprintf("packageRequest(installed=%v, updatePackage=%v, labels=%v, config=%v)",
		r.installedPackage, maybe(r.updatePackage), r.labels, r.configPackage)
}

func (r packageRequest) match(env pack.PackageEnvelope) bool {
	filter := r.updateSearchFilter()
	matched := env.Locator.Repository == filter.Repository &&
		env.Locator.Name == filter.Name
	if len(r.labels) != 0 {
		matched = matched && env.HasLabels(r.labels)
	}
	return matched
}

func (r packageRequest) updateSearchFilter() loc.Locator {
	if r.updateFilter != nil {
		return *r.updateFilter
	}
	return r.installedPackage
}

type packageRequest struct {
	installedPackage loc.Locator
	// updatePackage specifies the locator of the update package if known
	updatePackage *loc.Locator
	// updateFilter specifies an alternative locator for the upstream package
	// if it was renamed
	updateFilter *loc.Locator
	// labels defines labels to assign to the update package
	labels map[string]string
	// less specifies optional version comparator to use when searching
	// for an update
	less          pack.LessFunc
	configPackage *packageRequest
}

var (
	gravityPackageFilter = loc.MustCreateLocator(
		defaults.SystemAccountOrg, constants.GravityPackage, loc.ZeroVersion)
	teleportPackageFilter = loc.MustCreateLocator(
		defaults.SystemAccountOrg, constants.TeleportPackage, loc.ZeroVersion)
)

// printStateDir outputs directory where all gravity data is stored on the node
func printStateDir() error {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Println(stateDir)
	return nil
}

const noBlock = true
