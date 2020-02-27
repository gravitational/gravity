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
	"os"
	"os/exec"
	"strings"
	"syscall"

	appservice "github.com/gravitational/gravity/lib/app/service"
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
		err = pullUpdate(env.Packages, remotePackages, env.Reporter, *update)
		if err != nil {
			return trace.Wrap(err)
		}
		if update.ConfigPackage != nil {
			err = pullUpdate(env.Packages, remotePackages, env.Reporter,
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

func systemReinstall(env *localenv.LocalEnvironment, newPackage loc.Locator, serviceName string, labels map[string]string, clusterRole string) error {
	if serviceName == "" {
		updater := system.PackageUpdater{
			Packages:    env.Packages,
			ClusterRole: clusterRole,
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
	if clusterRole != "" {
		args = append(args, "--cluster-role", clusterRole)
	}
	err := service.ReinstallOneshotSimple(serviceName, args...)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func systemBlockingReinstall(env *localenv.LocalEnvironment, update storage.PackageUpdate, clusterRole string) error {
	updater := system.PackageUpdater{
		Packages:    env.Packages,
		ClusterRole: clusterRole,
	}
	return updater.Reinstall(update)
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

func applyUpdates(env *localenv.LocalEnvironment, updates []storage.PackageUpdate) error {
	var errors []error
	for _, u := range updates {
		env.Printf("Applying %v\n", u)
		err := systemBlockingReinstall(env, u, "")
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

func pullUpdate(localPackages, remotePackages pack.PackageService, reporter pack.ProgressReporter, update storage.PackageUpdate) error {
	pullReq := appservice.PackagePullRequest{
		SrcPack:  remotePackages,
		DstPack:  localPackages,
		Package:  update.To,
		Progress: reporter,
	}
	_, err := appservice.PullPackage(pullReq)
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

// systemServiceStart starts the package service specified with packagePattern
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
func systemUninstall(env *localenv.LocalEnvironment, confirmed bool) error {
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
	if err := environ.UninstallServices(env, logger); err != nil {
		log.WithError(err).Warn("Failed to uninstall agent services.")
	}
	if err := environ.UninstallSystem(env, logger); err != nil {
		log.WithError(err).Warn("Failed to uninstall system.")
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
	return trace.Wrap(pack.Unpack(p, loc, path, nil))
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
