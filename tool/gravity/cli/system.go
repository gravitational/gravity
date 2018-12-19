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
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	appservice "github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/devicemapper"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/docker/docker/pkg/archive"
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

	packages, err := findPackages(env.Packages, runtimePackage)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, req := range packagesToUpgrade(packages...) {
		log.Debugf("Checking for update for %v.", req.filter)
		update, err := findPackageUpdate(env.Packages, remotePackages, req)
		if err != nil {
			if trace.IsNotFound(err) {
				log.Info(err)
				continue
			}
			return trace.Wrap(err)
		}
		env.Printf("Pulling update %v\n.", update)
		pullReq := appservice.PackagePullRequest{
			SrcPack:  remotePackages,
			DstPack:  env.Packages,
			Package:  update.To,
			Progress: env.Reporter,
		}
		if _, err = appservice.PullPackage(pullReq); err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// systemRollback rolls back system to the specified changesetID or the last update if changesetID is not specified
func systemRollback(env *localenv.LocalEnvironment, changesetID, serviceName string, withStatus bool) (err error) {
	changeset, err := getChangesetByID(env, changesetID)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("rolling back %v\n", changeset)
	if serviceName != "" {
		args := []string{"system", "rollback", "--changeset-id", changeset.ID, "--debug"}
		if withStatus {
			args = append(args, "--with-status")
		}
		return trace.Wrap(installOneshotService(env.Silent, serviceName, args))
	}

	changes := changeset.ReversedChanges()
	rollback, err := env.Backend.CreatePackageChangeset(storage.PackageChangeset{Changes: changes})
	if err != nil {
		return trace.Wrap(err)
	}

	err = applyUpdates(env, changes)
	if err != nil {
		log.Error(trace.DebugReport(err))
		fmt.Printf("error deploying packages: %v\n", err)
		return trace.Wrap(err)
	}

	if !withStatus {
		env.Printf("system rolled back: %v\n", rollback)
		return nil
	}

	err = getLocalNodeStatus(env)
	if err != nil {
		return trace.Wrap(err)
	}

	env.Printf("system rolled back: %v\n", rollback)
	return nil
}

// systemHistory prints upgrade history
func systemHistory(env *localenv.LocalEnvironment) error {
	changesets, err := env.Backend.GetPackageChangesets()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(changesets) == 0 {
		fmt.Printf("there are no updates recorded\n")
		return nil
	}
	for _, changeset := range changesets {
		fmt.Printf("* %v\n", changeset)
	}
	return nil
}

// systemUpdate searches and applies package updates if any
func systemUpdate(env *localenv.LocalEnvironment, changesetID string, serviceName string, withStatus bool,
	runtimePackage loc.Locator) error {
	if serviceName != "" {
		args := []string{"system", "update", "--changeset-id", changesetID, "--debug"}
		if withStatus {
			args = append(args, "--with-status")
		}
		return trace.Wrap(installOneshotService(env.Silent, serviceName, args))
	}

	packages, err := findPackages(env.Packages, runtimePackage)
	if err != nil {
		return trace.Wrap(err)
	}

	var changes []storage.PackageUpdate
	for _, req := range packagesToUpgrade(packages...) {
		update, err := findPackageUpdate(env.Packages, env.Packages, req)
		if err != nil {
			if trace.IsNotFound(err) {
				log.Info(err)
				continue
			}
			return trace.Wrap(err)
		}
		update.Labels = req.labels
		log.Debugf("Found %v.", update)
		changes = append(changes, *update)
	}
	if len(changes) == 0 {
		env.Println("System is already up to date.")
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
		env.Printf("System successfully updated: %v.\n", changeset)
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

	env.Printf("System successfully updated: %v.\n", changeset)
	return nil
}

func systemReinstall(env *localenv.LocalEnvironment, newPackage loc.Locator, serviceName string, labels map[string]string) error {
	if serviceName == "" {
		return trace.Wrap(systemBlockingReinstall(env, newPackage, newPackage, labels))
	}

	args := []string{"system", "reinstall", newPackage.String()}
	if len(labels) != 0 {
		kvs := configure.KeyVal(labels)
		args = append(args, "--labels", kvs.String())
	}
	err := installOneshotService(env.Silent, serviceName, args)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func systemBlockingReinstall(env *localenv.LocalEnvironment, oldPackage, newPackage loc.Locator, labels map[string]string) error {
	updates, err := systemReinstallPackage(env, oldPackage, newPackage, labels)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(updates) == 0 {
		return nil
	}
	return applyLabelUpdates(env.Packages, updates)
}

func systemReinstallPackage(env *localenv.LocalEnvironment, oldPackage, newPackage loc.Locator, labels map[string]string) ([]packageLabelUpdate, error) {
	log.Debugf("Reinstalling package %v -> %v.", oldPackage, newPackage)
	switch {
	case newPackage.Name == constants.GravityPackage:
		return updateGravityPackage(env.Packages, newPackage)
	case isPlanetPackage(newPackage, labels):
		configPackage, err := findLatestPlanetConfigPackage(env.Packages, newPackage)
		log.Debugf("Latest runtime configuration package: %v (%v).", configPackage, err)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		installedPackage, err := pack.FindInstalledPackage(env.Packages, oldPackage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updates, err := updatePlanetPackage(env, *installedPackage, newPackage, *configPackage)
		return updates, trace.Wrap(err)
	case newPackage.Name == constants.TeleportPackage:
		updates, err := reinstallSystemService(env, newPackage, newPackage, nil)
		return updates, trace.Wrap(err)
	case isSecretsPackage(newPackage):
		updates, err := reinstallSecretsPackage(env, newPackage)
		return updates, trace.Wrap(err)
	case isPlanetConfigPackage(newPackage):
		// See updatePlanetPackage for update to configuration package
		return nil, nil
	}
	return nil, trace.BadParameter("unsupported package: %v", newPackage)
}

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
			Type:            constants.OneshotService,
			StartCommand:    strings.Join(cmd, " "),
			RemainAfterExit: true,
		},
	})
	return trace.Wrap(err)
}

// installOneshotService installs a systemd service named serviceName of type=oneshot
// using args as arguments to the gravity command on host.
// args should only list secondary arguments w/o specifying the binary.
// The operation is non-blocking - e.g. it does not block waiting for service to complete.
func installOneshotService(printer localenv.Printer, serviceName string, args []string) error {
	gravityPath, err := exec.LookPath(constants.GravityBin)
	if err != nil {
		return trace.Wrap(err, "failed to find %v binary in PATH",
			constants.GravityBin)
	}

	args = append([]string{gravityPath}, args...)
	err = installOneshotServiceFromSpec(printer, serviceName, args,
		systemservice.ServiceSpec{
			// Dump the gravity binary version as a start command
			StartCommand: fmt.Sprintf("%v version", gravityPath),
		})
	return trace.Wrap(err)
}

// installOneshotServiceFromSpec installs a systemd service named serviceName of type=oneshot
// using args as the ExecStartPre command and spec as the service specification.
// The operation is non-blocking - e.g. it does not block waiting for service to complete.
// The spec will have fields responsible for making a oneshot service automatically populated.
func installOneshotServiceFromSpec(printer localenv.Printer, serviceName string, args []string, spec systemservice.ServiceSpec) error {
	printer.Printf("launching oneshot system service %v\n", serviceName)

	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	if len(args) != 0 {
		command := strings.Join(args, " ")
		// We do actual job as a command executed before the service entrypoint
		// to distinguish between completed job (status active) and in-progress job
		// (status activating)
		spec.StartPreCommand = command
	}

	if spec.User == "" {
		spec.User = constants.RootUIDString
	}
	spec.Type = constants.OneshotService
	spec.RemainAfterExit = true

	err = services.InstallService(systemservice.NewServiceRequest{
		Name:        serviceName,
		NoBlock:     true,
		ServiceSpec: spec,
	})
	return trace.Wrap(err)
}

func applyUpdates(env *localenv.LocalEnvironment, updates []storage.PackageUpdate) error {
	var errors []error
	for _, u := range updates {
		env.Printf("Applying %v\n", u)
		err := systemBlockingReinstall(env, u.From, u.To, u.Labels)
		if err != nil {
			log.Warnf("Failed to reinstall %v -> %v: %v.",
				u.From, u.To, trace.DebugReport(err))
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// findPackages returns a list of additional packages to pull during update.
// These are packages that do not have a static name and need to be looked up
// dynamically
func findPackages(packages pack.PackageService, runtimePackageUpdate loc.Locator) (reqs []packageRequest, err error) {
	secrets, err := findSecretsPackage(packages)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find secrets package")
	}

	err = updateInstalledLabelIfNecessary(packages, secrets.Locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	planetPackage, planetConfig, err := pack.FindRuntimePackageWithConfig(packages)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find runtime package")
	}
	log.Debugf("Found existing runtime package %v with configuration package %v.",
		*planetPackage, *planetConfig)

	planetConfigUpdate, err := maybeConvertLegacyPlanetConfigPackage(*planetConfig)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find runtime configuration package")
	}

	reqs = append(reqs,
		newPackageRequest(secrets.Locator),
		packageRequest{
			filter:       *planetPackage,
			updateFilter: &runtimePackageUpdate,
			labels:       pack.RuntimePackageLabels,
		},
		packageRequest{
			filter: *planetConfig,
			// Look for updated package name in upstream packages
			updateFilter:          planetConfigUpdate,
			withoutInstalledLabel: true,
		},
	)
	log.Debugf("New package update requests: %v.", packageRequests(reqs))
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

	env.Println("No changeset-id specified, using last changeset.")
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

func updateGravityPackage(packages *localpack.PackageServer, newPackage loc.Locator) (labelUpdates []packageLabelUpdate, err error) {
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

func updatePlanetPackage(env *localenv.LocalEnvironment, installedPackage, newPackage, configPackage loc.Locator) (labelUpdates []packageLabelUpdate, err error) {
	err = env.Packages.Unpack(newPackage, "")
	if err != nil {
		return nil, trace.Wrap(err, "failed to unpack package %v", newPackage)
	}

	planetPath, err := env.Packages.UnpackedPath(newPackage)
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

	labelUpdates, err = reinstallSystemService(env, installedPackage, newPackage, &configPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	configLabelUpdates, err := reinstallPlanetConfigPackage(env, newPackage, configPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	labelUpdates = append(labelUpdates, configLabelUpdates...)

	return labelUpdates, nil
}

func updateKubectl(planetPath string) (err error) {
	// update kubectl symlink
	kubectlPath := filepath.Join(planetPath, constants.PlanetRootfs, defaults.KubectlScript)
	var out []byte
	for _, path := range []string{defaults.KubectlBin, defaults.KubectlBinAlternate} {
		out, err = exec.Command("ln", "-sfT", kubectlPath, path).CombinedOutput()
		if err == nil {
			break
		}
		log.Warnf("Failed to update kubectl symlink: %s (%v).", out, err)
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

func reinstallSecretsPackage(env *localenv.LocalEnvironment, newPackage loc.Locator) ([]packageLabelUpdate, error) {
	prevPackage, err := pack.FindInstalledPackage(env.Packages, newPackage)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// this is legacy use case when secrets did not have the installed label,
		// find the first secrets package
		p, err := findSecretsPackage(env.Packages)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		prevPackage = &p.Locator
	}

	targetPath, err := localenv.InGravity(defaults.SecretsDir)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine secrets dir")
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

	var updates []packageLabelUpdate
	updates = append(updates,
		packageLabelUpdate{locator: *prevPackage, remove: []string{pack.InstalledLabel}},
		packageLabelUpdate{locator: newPackage, add: map[string]string{pack.InstalledLabel: pack.InstalledLabel}},
	)

	env.Printf("secrets package %v installed in %v\n", newPackage, targetPath)
	return updates, nil
}

func reinstallPlanetConfigPackage(env *localenv.LocalEnvironment, planetPackage, configPackage loc.Locator) ([]packageLabelUpdate, error) {
	prevPackage, err := pack.FindConfigPackage(env.Packages, planetPackage)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	err = env.Packages.Unpack(configPackage, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var updates []packageLabelUpdate
	if prevPackage != nil {
		updates = append(updates,
			packageLabelUpdate{locator: *prevPackage, remove: []string{pack.ConfigLabel}})
	}
	updates = append(updates,
		packageLabelUpdate{
			locator: configPackage,
			add:     pack.ConfigLabels(planetPackage, pack.PurposePlanetConfig),
		})

	return updates, nil
}

func getChownOptionsForDir(dir string) (*archive.TarChownOptions, error) {
	var uid, gid int
	// preserve owner/group when unpacking, otherwise use current process user
	fi, err := os.Stat(dir)
	if err == nil {
		stat := fi.Sys().(*syscall.Stat_t)
		uid = int(stat.Uid)
		gid = int(stat.Gid)
		log.Debugf("assuming UID:GID from existing directory %v:%v", uid, gid)
		return &archive.TarChownOptions{
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
	return &archive.TarChownOptions{
		UID: uid,
		GID: gid,
	}, nil
}

func reinstallBinaryPackage(packages pack.PackageService, newPackage loc.Locator, targetPath string) ([]packageLabelUpdate, error) {
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

	var updates []packageLabelUpdate
	updates = append(updates,
		packageLabelUpdate{locator: *prevPackage, remove: []string{pack.InstalledLabel}},
		packageLabelUpdate{locator: newPackage, add: map[string]string{pack.InstalledLabel: pack.InstalledLabel}},
	)

	fmt.Printf("binary package %v installed in %v\n", newPackage, targetPath)
	return updates, nil
}

type packageLabelUpdate struct {
	locator loc.Locator
	remove  []string
	add     map[string]string
}

func applyLabelUpdates(packages pack.PackageService, labelUpdates []packageLabelUpdate) error {
	var errors []error
	for _, update := range labelUpdates {
		err := packages.UpdatePackageLabels(update.locator, update.add, update.remove)
		errors = append(errors, trace.Wrap(err, "error updating %v", update.locator))
	}
	return trace.NewAggregate(errors...)
}

func reinstallSystemService(env *localenv.LocalEnvironment, oldPackage, newPackage loc.Locator, configPackage *loc.Locator) ([]packageLabelUpdate, error) {
	prevPackage, prevConfigPackage, err := pack.FindInstalledPackageWithConfig(env.Packages, oldPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	services, err := systemservice.New()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var updates []packageLabelUpdate
	if prevPackage != nil {
		packageUpdates, err := uninstallPackage(env, services, *prevPackage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updates = append(updates, packageUpdates...)
	}

	err = unpack(env.Packages, newPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	manifest, err := env.Packages.GetPackageManifest(newPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if manifest.Service == nil {
		return nil, trace.NotFound("%v needs service section in manifest to be installed", newPackage)
	}

	if configPackage == nil {
		configPackage = prevConfigPackage
	}

	err = unpack(env.Packages, *configPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gravityPath, err := exec.LookPath(constants.GravityBin)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find %v binary in PATH",
			constants.GravityBin)
	}

	manifest.Service.Package = newPackage
	manifest.Service.ConfigPackage = *configPackage
	manifest.Service.GravityPath = gravityPath

	log.Debugf("Installing new package %v.", newPackage)
	if err = services.InstallPackageService(*manifest.Service); err != nil {
		return nil, trace.Wrap(err, "error installing %v service", manifest.Service.Package)
	}

	updates = append(updates, packageLabelUpdate{locator: newPackage, add: map[string]string{pack.InstalledLabel: pack.InstalledLabel}})

	env.Printf("%v successfully installed\n", newPackage)
	return updates, nil
}

func uninstallPackage(env *localenv.LocalEnvironment, services systemservice.ServiceManager, servicePackage loc.Locator) (updates []packageLabelUpdate, err error) {
	installed, err := services.IsPackageServiceInstalled(servicePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if installed {
		env.Printf("%v is installed as a service, uninstalling\n", servicePackage)
		err = services.UninstallPackageService(servicePackage)
		if err != nil {
			return nil, utils.NewUninstallServiceError(servicePackage)
		}
	}
	updates = append(updates, packageLabelUpdate{locator: servicePackage, remove: []string{pack.InstalledLabel}})
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
		err = services.UninstallService(serviceName)
	case !pkg.IsEmpty():
		if pkg.Version == loc.ZeroVersion {
			statuses, err := services.ListPackageServices()
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
	statuses, err := services.ListPackageServices()
	if err != nil {
		return trace.Wrap(err)
	}
	common.PrintHeader("services")
	for _, s := range statuses {
		fmt.Printf("* %v %v\n", s.Package, s.Status)
	}
	return nil
}

// systemServiceStatus prints status of this service
func systemServiceStatus(env *localenv.LocalEnvironment, pkg loc.Locator, serviceName string) error {
	services, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	var status string
	if serviceName != "" {
		status, err = services.StatusService(serviceName)
	} else if !pkg.IsEmpty() {
		status, err = services.StatusPackageService(pkg)
	} else {
		return trace.BadParameter("need either package name or service name")
	}
	if status != "" {
		fmt.Printf("%v", status)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// systemUninstall uninstalls all gravity components
func systemUninstall(env *localenv.LocalEnvironment, confirmed bool) error {
	dockerInfo, err := dockerInfo()
	if err != nil {
		log.Warnf("Failed to get docker info: %v.", trace.DebugReport(err))
	} else {
		log.Debugf("Detected docker: %#v.", dockerInfo)
	}

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

	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}

	services, err := svm.ListPackageServices()
	if err != nil {
		return trace.Wrap(err)
	}

	sort.Slice(services, func(i, j int) bool {
		// Move teleport package to the front of uninstall chain.
		// The reason for this is, if uninstalling the planet package would fail,
		// the node would continue sending heartbeats that would make it persist
		// in the list of nodes although it might have already been removed from
		// everywhere else during shrink.
		return services[i].Package.Name == constants.TeleportPackage
	})
	for _, service := range services {
		env.PrintStep("Uninstalling system service %v", service)
		if err := svm.UninstallPackageService(service.Package); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := svm.UninstallService(defaults.GravityRPCAgentServiceName); err != nil {
		log.WithError(err).Warn("Failed to uninstall agent sevice.")
	}

	// close the backend before attempting to unmount as the open file might
	// prevent the umount from succeeding
	env.Backend.Close()

	out := &bytes.Buffer{}
	log := logrus.NewEntry(logrus.StandardLogger())
	if dockerInfo != nil && dockerInfo.StorageDriver == constants.DockerStorageDriverDevicemapper {
		env.PrintStep("Detected devicemapper, cleaning up disks")
		if err = devicemapper.Unmount(out, log); err != nil {
			return trace.Wrap(err, "failed to unmount devicemapper: %s", out.Bytes())
		}
	}

	if err := removeInterfaces(env); err != nil {
		log.Warnf("Failed to clean up network interfaces: %v.", trace.DebugReport(err))
	}

	for _, targetPath := range state.GravityBinPaths {
		err = os.Remove(targetPath)
		if err == nil {
			env.PrintStep("Removed gravity binary %v", targetPath)
			break
		}
	}
	if err != nil {
		log.Warnf("Failed to delete gravity binary: %v.",
			trace.DebugReport(err))
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}

	env.PrintStep("Deleting all local data at %v", stateDir)
	if err = os.RemoveAll(stateDir); err != nil {
		// do not fail if the state directory cannot be removed, probably
		// this means it is a mount
		log.Warnf("Failed to remove %v: %v.", stateDir, err)
	}

	// remove all files and directories gravity might have created on the system
	for _, path := range append(state.StateLocatorPaths, defaults.ModulesPath, defaults.SysctlPath, defaults.GravityEphemeralDir) {
		// errors are expected since some of them may not exist
		if err := os.Remove(path); err == nil {
			env.PrintStep("Removed %v", path)
		}
	}

	env.PrintStep("Gravity has been successfully uninstalled")
	return nil
}

func dockerInfo() (*utils.DockerInfo, error) {
	out := &bytes.Buffer{}
	command := exec.Command("gravity", "enter", "--", "--notty", "/usr/bin/docker", "--", "info")
	err := utils.Exec(command, out)
	if err != nil {
		return nil, trace.Wrap(err, out.String())
	}
	return utils.ParseDockerInfo(out)
}

func removeInterfaces(env *localenv.LocalEnvironment) error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, iface := range ifaces {
		if utils.HasOneOfPrefixes(iface.Name, "docker", "flannel") {
			env.PrintStep("Removing network interface %q", iface.Name)
			out := &bytes.Buffer{}
			if err := utils.Exec(exec.Command("ip", "link", "del", iface.Name), out); err != nil {
				return trace.Wrap(err, out.String())
			}
		}
	}

	return nil
}

func findSecretsPackage(packages pack.PackageService) (*pack.PackageEnvelope, error) {
	return pack.FindPackage(packages, func(env pack.PackageEnvelope) bool {
		return isSecretsPackage(env.Locator)
	})
}

// findPackageUpdate searches for updates for the installed package specified with req
func findPackageUpdate(localPackages, remotePackages pack.PackageService, req packageRequest) (*storage.PackageUpdate, error) {
	if req.withoutInstalledLabel {
		return findPackageUpdateHelper(remotePackages, req.filter, req.updateFilter)
	}

	installedPackage, err := pack.FindInstalledPackage(localPackages, req.filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return findPackageUpdateHelper(remotePackages, *installedPackage, req.updateFilter)
}

func findPackageUpdateHelper(packages pack.PackageService, filter loc.Locator, updateFilter *loc.Locator) (*storage.PackageUpdate, error) {
	if updateFilter == nil {
		updateFilter = &filter
	}
	latestPackage, err := pack.FindLatestPackage(packages, *updateFilter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	currentVersion, err := filter.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	latestVersion, err := latestPackage.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if latestVersion.Compare(*currentVersion) > 0 {
		return &storage.PackageUpdate{From: filter, To: *latestPackage}, nil
	}
	return nil, trace.NotFound("%v is already at the latest version", filter)
}

func findLatestPlanetConfigPackage(localPackages pack.PackageService, planetPackage loc.Locator) (*loc.Locator, error) {
	configPackage, err := pack.FindConfigPackage(localPackages, planetPackage)
	log.Debugf("Runtime configuration package: %v (%v).", configPackage, err)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pack.FindLatestPackage(localPackages, *configPackage)
}

func isPlanetPackage(packageLoc loc.Locator, labels map[string]string) bool {
	if purpose := labels[pack.PurposeLabel]; purpose == pack.PurposeRuntime {
		return true
	}
	return (packageLoc.Name == loc.LegacyPlanetMaster.Name ||
		packageLoc.Name == loc.LegacyPlanetNode.Name)
}

func isSecretsPackage(loc loc.Locator) bool {
	return strings.Contains(loc.Name, "secrets") && loc.Repository != defaults.SystemAccountOrg
}

func isPlanetConfigPackage(loc loc.Locator) bool {
	return strings.Contains(loc.Name, constants.PlanetConfigPackage) &&
		loc.Repository != defaults.SystemAccountOrg
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

func updateInstalledLabelIfNecessary(packages pack.PackageService, locator loc.Locator) error {
	_, err := pack.FindInstalledPackage(packages, locator)
	if err != nil && trace.IsNotFound(err) {
		log.Debugf("No installed package found for %[1]v, migrating by applying installed label to %[1]v.", locator)
		err = packages.UpdatePackageLabels(locator, pack.InstalledLabels, nil)
	}
	if err != nil {
		return trace.Wrap(err)
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
	return trace.Wrap(pack.Unpack(p, loc, path, nil))
}

func newPackageRequest(filter loc.Locator) packageRequest {
	return packageRequest{filter: filter}
}

// packagesToUpgrade returns a list of packages to upgrade.
// Package are upgraded in the order listed.
func packagesToUpgrade(extraPackages ...packageRequest) (upgrades []packageRequest) {
	upgrades = append(upgrades, newPackageRequest(gravityPackageFilter))
	for _, extra := range extraPackages {
		upgrades = append(upgrades, extra)
	}
	upgrades = append(upgrades, newPackageRequest(teleportPackageFilter))
	return upgrades
}

func (r packageRequests) String() string {
	var buf bytes.Buffer
	for _, req := range r {
		buf.WriteString(req.String())
		buf.WriteString(",")
	}
	return buf.String()
}

type packageRequests []packageRequest

func (r packageRequest) String() string {
	maybe := func(loc *loc.Locator) string {
		if loc != nil {
			return loc.String()
		}
		return "<none>"
	}
	return fmt.Sprintf("packageRequest(filter=%v, updateFilter=%v, labels=%v, withoutInstalled=%v)",
		r.filter, maybe(r.updateFilter), r.labels, r.withoutInstalledLabel)
}

type packageRequest struct {
	filter loc.Locator
	// updateFilter specifies an alternative filter for the package
	// when looking for an update.
	// This is helpful when the package name has changed between releases
	updateFilter *loc.Locator
	// labels defines labels to assign to the updated package
	labels map[string]string
	// withoutInstalledLabel specifies if the search does not require the
	// source package to be labeled with installed label
	withoutInstalledLabel bool
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
