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

package pack

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
	"github.com/opencontainers/selinux/go-selinux"
	log "github.com/sirupsen/logrus"
)

// PackagePath generates a path to the package composed of base directory,
// repository name, package name and version
func PackagePath(baseDir string, loc loc.Locator) string {
	// the path can not be too long because it leads to problems like this:
	// https: //github.com/golang/go/issues/6895
	return filepath.Join(baseDir, loc.Repository, loc.Name, loc.Version)
}

// IsUnpacked checks if the package has been unpacked at the provided directory
// (currently just by checking if the dir exists)
func IsUnpacked(targetDir string) (bool, error) {
	info, err := os.Stat(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, trace.Wrap(err)
	}
	if !info.IsDir() {
		return false, trace.Errorf(
			"expected %v to be a directory, got %v", targetDir, info)
	}
	return true, nil
}

// Unpack reads the package from the package service and unpacks its contents
// to base directory targetDir
func Unpack(p PackageService, loc loc.Locator, targetDir string, opts *dockerarchive.TarOptions) error {
	var err error
	// if target dir is not provided, unpack to the default location
	if targetDir == "" {
		stateDir, err := state.GetStateDir()
		if err != nil {
			return trace.Wrap(err)
		}
		baseDir := filepath.Join(stateDir, defaults.LocalDir,
			defaults.PackagesDir, defaults.UnpackedDir)
		targetDir = PackagePath(baseDir, loc)
		log.Infof("Unpacking %v into the default directory %v.",
			loc, targetDir)
	}
	if err := os.MkdirAll(targetDir, defaults.SharedDirMask); err != nil {
		return trace.Wrap(err)
	}
	_, reader, err := p.ReadPackage(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	if opts == nil {
		opts = archive.DefaultOptions()
	}

	if err := dockerarchive.Untar(reader, targetDir, opts); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UnpackIfNotUnpacked unpacks the specified package only if it's not yet unpacked
func UnpackIfNotUnpacked(p PackageService, loc loc.Locator, targetDir string, opts *dockerarchive.TarOptions) error {
	isUnpacked, err := IsUnpacked(targetDir)
	if err != nil {
		return trace.Wrap(err)
	}

	if isUnpacked {
		return nil
	}

	err = Unpack(p, loc, targetDir, opts)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetConfigPackage creates the config package without saving it into package service
func GetConfigPackage(p PackageService, loc loc.Locator, confLoc loc.Locator, args []string) (io.Reader, error) {
	_, reader, err := p.ReadPackage(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	decompressed, err := dockerarchive.DecompressStream(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tarball := tar.NewReader(decompressed)

	manifest, err := ReadManifest(tarball)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger := log.WithFields(log.Fields{
		"package":  loc.String(),
		"manifest": fmt.Sprintf("%#v", manifest),
		"args":     args,
	})
	logger.Info("Generate configuration package.")
	if manifest.Config == nil {
		return nil, trace.BadParameter("manifest does not have configuration parameters")
	}

	if err := manifest.Config.ParseArgs(args); err != nil {
		logger.WithError(err).Warn("Failed to parse arguments.")
		return nil, trace.Wrap(err)
	}

	// now create a new package with configuration inside
	buf := &bytes.Buffer{}
	if err := WriteConfigPackage(manifest, buf); err != nil {
		return nil, trace.Wrap(err)
	}

	return buf, nil
}

// GetPackageManifest will retrieve the manifest file for the specified package
func GetPackageManifest(p PackageService, loc loc.Locator) (*Manifest, error) {
	_, reader, err := p.ReadPackage(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	decompressed, err := dockerarchive.DecompressStream(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer decompressed.Close()
	tarball := tar.NewReader(decompressed)

	manifest, err := ReadManifest(tarball)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return manifest, nil
}

// ConfigurePackage reads the given package, and configures it using arguments passed,
// the resulting package is created within the scope of the same package service
func ConfigurePackage(p PackageService, loc loc.Locator, confLoc loc.Locator, args []string, labels map[string]string) error {
	reader, err := GetConfigPackage(p, loc, confLoc, args)
	if err != nil {
		return trace.Wrap(err)
	}
	allLabels := map[string]string{
		ConfigLabel: loc.ZeroVersion().String(),
	}
	for k, v := range labels {
		allLabels[k] = v
	}
	_, err = p.CreatePackage(confLoc, reader, WithLabels(allLabels))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ExecutePackageCommand executes command specified in the package and returns
// results of CombinedOutput call on the package binary
func ExecutePackageCommand(p PackageService, cmd string, loc loc.Locator, confLoc *loc.Locator, execArgs []string, storageDir string) ([]byte, error) {
	log.Infof("exec with config %v %v", loc, confLoc)

	unpackedPath := PackagePath(storageDir, loc)
	if err := Unpack(p, loc, unpackedPath, nil); err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := OpenManifest(unpackedPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := Unpack(p, loc, unpackedPath, nil); err != nil {
		return nil, trace.Wrap(err)
	}

	manifestCmdSpec, err := manifest.Command(cmd)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	env := []string{fmt.Sprintf("PATH=%v", os.Getenv("PATH"))}
	// read package with configuration if it's provided
	if confLoc != nil && confLoc.Name != "" {
		_, reader, err := p.ReadPackage(*confLoc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		defer reader.Close()

		vars, err := ReadConfigPackage(reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for k, v := range vars {
			env = append(env, fmt.Sprintf("%v=%v", k, v))
		}
	}

	args := append(manifestCmdSpec.Args, execArgs...)
	command := exec.Command(args[0], args[1:]...)
	command.Dir = unpackedPath
	command.Env = env

	log.Infof("ExecutePackageCommand(%v %v %v, unpacked=%v)",
		manifestCmdSpec.Args[0], cmd, execArgs, unpackedPath)

	out, err := command.CombinedOutput()
	if err != nil {
		return out, trace.Wrap(err)
	}
	return out, nil
}

// FindPackage finds package matching the predicate fn
func FindPackage(packages PackageService, fn func(e PackageEnvelope) bool) (*PackageEnvelope, error) {
	var env *PackageEnvelope
	err := ForeachPackage(packages, func(e PackageEnvelope) error {
		if fn(e) {
			env = &e
		}
		return nil
	})
	if env == nil {
		return nil, trace.NotFound("package not found")
	}
	return env, trace.Wrap(err)
}

// ForeachPackage executes function fn for each package in
// each repository
func ForeachPackage(packages PackageService, fn func(e PackageEnvelope) error) error {
	repos, err := packages.GetRepositories()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, repo := range repos {
		if err := ForeachPackageInRepo(packages, repo, fn); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ForeachPackageInRepo executes fn for each package found in repository
func ForeachPackageInRepo(packages PackageService, repo string, fn func(e PackageEnvelope) error) error {
	packs, err := packages.GetPackages(repo)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, pkg := range packs {
		if err := fn(pkg); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// FindInstalledPackage finds package currently installed on the host
func FindInstalledPackage(packages PackageService, filter loc.Locator) (*loc.Locator, error) {
	env, err := findInstalledPackage(packages, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &env.Locator, nil
}

// FindConfigPackage returns configuration package for given package
func FindConfigPackage(packages PackageService, filter loc.Locator) (*loc.Locator, error) {
	config, err := findConfigPackage(packages, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &config.Locator, nil
}

// FindInstalledConfigPackage returns an installed configuration package for given package
func FindInstalledConfigPackage(packages PackageService, filter loc.Locator) (*loc.Locator, error) {
	config, err := findInstalledConfigPackage(packages, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &config.Locator, nil
}

// FindInstalledPackageWithConfig finds installed package and associated configuration package
func FindInstalledPackageWithConfig(packages PackageService, filter loc.Locator) (installed, config *loc.Locator, err error) {
	installed, err = FindInstalledPackage(packages, filter)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	config, err = FindConfigPackage(packages, *installed)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return installed, config, nil
}

// ProcessMetadata processes some special metadata conventions, e.g. 'latest' metadata label
func ProcessMetadata(packages PackageService, loc *loc.Locator) (*loc.Locator, error) {
	ver, err := loc.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if ver.Metadata == LatestLabel {
		return FindLatestPackage(packages, *loc)
	}
	return loc, nil
}

// FindLatestPackageWithLabels returns the latest package matching the provided
// labels
func FindLatestPackageWithLabels(packages PackageService, repository string, labels map[string]string) (*loc.Locator, error) {
	loc, err := FindLatestPackageCustom(FindLatestPackageRequest{
		Packages:   packages,
		Repository: repository,
		Match: func(e PackageEnvelope) bool {
			return e.HasLabels(labels)
		},
	})
	if err != nil && trace.IsNotFound(err) {
		return nil, trace.NotFound("latest package in repository %q with labels %v not found", repository, labels)
	}
	return loc, trace.Wrap(err)
}

// FindLatestPackage returns package the latest package matching the provided
// locator
func FindLatestPackage(packages PackageService, filter loc.Locator) (*loc.Locator, error) {
	loc, err := FindLatestPackageCustom(FindLatestPackageRequest{
		Packages:   packages,
		Repository: filter.Repository,
		Match:      PackageMatch(filter),
	})
	if err != nil && trace.IsNotFound(err) {
		return nil, trace.NotFound("latest package with filter %v not found", filter)
	}
	return loc, trace.Wrap(err)
}

// FindLatestPackageByName returns latest package with the specified name (across all repositories)
func FindLatestPackageByName(packages PackageService, name string) (*loc.Locator, error) {
	loc, err := FindLatestPackageCustom(FindLatestPackageRequest{
		Packages: packages,
		Match: func(e PackageEnvelope) bool {
			return e.Locator.Name == name
		}},
	)
	if err != nil && trace.IsNotFound(err) {
		return nil, trace.NotFound("latest package with name %q not found", name)
	}
	return loc, trace.Wrap(err)
}

// FindLatestPackageCustom searches for the latest version of the package given with req
func FindLatestPackageCustom(req FindLatestPackageRequest) (pkg *loc.Locator, err error) {
	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	var max *loc.Locator
	predicate := func(e PackageEnvelope) error {
		if !req.Match(e) {
			return nil
		}
		if max == nil {
			max = &e.Locator
			return nil
		}
		a, err := max.SemVer()
		if err != nil {
			return nil
		}
		b, err := e.Locator.SemVer()
		if err != nil {
			return nil
		}
		if req.Less(a, b) {
			max = &e.Locator
		}
		return nil
	}
	if req.Repository != "" {
		err = ForeachPackageInRepo(req.Packages, req.Repository, predicate)
	} else {
		err = ForeachPackage(req.Packages, predicate)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if max == nil {
		return nil, trace.NotFound("latest package not found")
	}
	return max, nil
}

func (r *FindLatestPackageRequest) checkAndSetDefaults() error {
	if r.Packages == nil {
		return trace.BadParameter("package service is required")
	}
	if r.Match == nil {
		return trace.BadParameter("package matcher is required")
	}
	if r.Less == nil {
		r.Less = Less
	}
	return nil
}

// FindLatestPackageRequest defines the request to search for the latest version of
// a package
type FindLatestPackageRequest struct {
	// Packages specifies the package service to use with the request
	Packages PackageService
	// Repository specifies the optional repository for search.
	// If unspecifed, all repositories are searched
	Repository string
	// Match specifies the package matcher
	Match MatchFunc
	// Less specifies the optional version comparator.
	// If unspecified, default comparator will be used
	Less LessFunc
}

// FindNewerPackages returns packages with versions greater than in the provided package
func FindNewerPackages(packages PackageService, filter loc.Locator) ([]loc.Locator, error) {
	var result []loc.Locator
	err := ForeachPackageInRepo(packages, filter.Repository, func(e PackageEnvelope) error {
		if e.Locator.Name != filter.Name {
			return nil
		}
		vera, err := filter.SemVer()
		if err != nil {
			return trace.Wrap(err)
		}
		verb, err := e.Locator.SemVer()
		if err != nil {
			return trace.Wrap(err)
		}
		if verb.Compare(*vera) > 0 {
			result = append(result, e.Locator)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result, nil
}

// FindPackageUpdate determines if an update is available for package specified with pkg
// and returns a descriptor object to go from existing version to a new one.
// If no update can be found, returns a nil descriptor and an instance of trace.NotFound as error
func FindPackageUpdate(packages PackageService, pkg loc.Locator) (*storage.PackageUpdate, error) {
	latestPackage, err := FindLatestPackage(packages, pkg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	currentVersion, err := pkg.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	latestVersion, err := latestPackage.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if latestVersion.Compare(*currentVersion) > 0 {
		return &storage.PackageUpdate{From: pkg, To: *latestPackage}, nil
	}
	return nil, trace.NotFound("%v is already at the latest version", pkg)
}

// CheckUpdatePackage makes sure that "to" package is acceptable when updating from "from" package
func CheckUpdatePackage(from, to loc.Locator) error {
	// repository and package name must match
	if from.Repository != to.Repository || from.Name != to.Name {
		return trace.BadParameter(
			"you are attempting to upgrade to %v %v, but different application is installed: %v %v",
			to.Name, to.Version, from.Name, from.Version)
	}
	// do not allow downgrades
	fromVer, err := from.SemVer()
	if err != nil {
		return trace.Wrap(err)
	}
	toVer, err := to.SemVer()
	if err != nil {
		return trace.Wrap(err)
	}
	if !fromVer.LessThan(*toVer) {
		return trace.BadParameter(
			"update version (%v) must be greater than the currently installed version (%v)", toVer, fromVer)
	}
	return nil
}

// ConfigLabels returns the label set to assign a configuration role for the specified package loc
func ConfigLabels(loc loc.Locator, purpose string) map[string]string {
	return map[string]string{
		ConfigLabel:  loc.ZeroVersion().String(),
		PurposeLabel: purpose,
	}
}

// Labels is a set of labels
type Labels map[string]string

// HasPurpose returns true if these labels contain the purpose label for any of the given values
func (r Labels) HasPurpose(values ...string) bool {
	for _, value := range values {
		purpose, ok := r[PurposeLabel]
		if ok && purpose == value {
			return true
		}
	}
	return false
}

// FindAnyRuntimePackageWithConfig searches for the runtime package and the corresponding
// configuration package in the specified package service.
// It looks up both legacy packages and packages marked as runtime
func FindAnyRuntimePackageWithConfig(packages PackageService) (runtimePackage *loc.Locator, runtimeConfig *loc.Locator, err error) {
	runtimePackage, err = FindAnyRuntimePackage(packages)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	runtimeConfig, err = FindConfigPackage(packages, *runtimePackage)
	if trace.IsNotFound(err) {
		runtimeConfig, err = FindLegacyRuntimeConfigPackage(packages)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	return runtimePackage, runtimeConfig, nil
}

// FindAnyRuntimePackage searches for the runtime package in the specified package service.
// It looks up both legacy packages and packages marked as runtime
func FindAnyRuntimePackage(packages PackageService) (runtimePackage *loc.Locator, err error) {
	runtimePackage, err = FindRuntimePackage(packages)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		runtimePackage, err = FindLegacyRuntimePackage(packages)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return runtimePackage, nil
}

// FindRuntimePackageWithConfig locates the planet package using the purpose label.
// Returns a pair - planet package with the corresponding configuration package.
func FindRuntimePackageWithConfig(packages PackageService) (runtimePackage, runtimeConfig *PackageEnvelope, err error) {
	runtimePackage, err = findRuntimePackage(packages)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	runtimeConfig, err = findInstalledConfigPackage(packages, runtimePackage.Locator)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return runtimePackage, runtimeConfig, nil
}

// FindTeleportPackageWithConfig locates the teleport package using the purpose label.
// Returns a pair - teleport package with the corresponding configuration package.
func FindTeleportPackageWithConfig(packages PackageService) (teleportPackage, teleportConfig *PackageEnvelope, err error) {
	teleportPackage, err = findInstalledPackage(packages, teleportFilter)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	labels := map[string]string{
		PurposeLabel:   PurposeTeleportNodeConfig,
		InstalledLabel: InstalledLabel,
	}
	teleportConfig, err = FindPackage(packages, func(e PackageEnvelope) bool {
		return e.HasLabels(labels)
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, nil, trace.NotFound("no configuration package for teleport found")
		}
		return nil, nil, trace.Wrap(err)
	}
	return teleportPackage, teleportConfig, nil
}

// FindRuntimePackage locates the installed runtime package
func FindRuntimePackage(packages PackageService) (runtimePackage *loc.Locator, err error) {
	env, err := findRuntimePackage(packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &env.Locator, nil
}

// FindLegacyRuntimePackage locates the planet package using the obsolete master/node
// flavors.
func FindLegacyRuntimePackage(packages PackageService) (runtimePackage *loc.Locator, err error) {
	err = ForeachPackage(packages, func(env PackageEnvelope) error {
		if loc.IsLegacyRuntimePackage(env.Locator) &&
			env.HasLabel(InstalledLabel, InstalledLabel) {
			runtimePackage = &env.Locator
			return utils.Abort(nil)
		}
		return nil
	})
	if err != nil && !utils.IsAbortError(err) {
		return nil, trace.Wrap(err)
	}
	if runtimePackage == nil {
		return nil, trace.NotFound("no runtime package found")
	}

	return runtimePackage, nil
}

// FindLegacyRuntimeConfigPackage locates the configuration package for the legacy
// runtime package in the specified package service
func FindLegacyRuntimeConfigPackage(packages PackageService) (configPackage *loc.Locator, err error) {
	err = ForeachPackage(packages, func(env PackageEnvelope) error {
		if env.Locator.Name == constants.PlanetConfigPackage {
			configPackage = &env.Locator
			return utils.Abort(nil)
		}
		return nil
	})
	if err != nil && !utils.IsAbortError(err) {
		return nil, trace.Wrap(err)
	}
	if configPackage == nil {
		return nil, trace.NotFound("no runtime configuration package found")
	}
	return configPackage, nil
}

// FindSecretsPackage returns the first secrets package from the given package service
func FindSecretsPackage(packages PackageService) (*loc.Locator, error) {
	env, err := FindPackage(packages, func(env PackageEnvelope) bool {
		return IsSecretsPackage(env.Locator, env.RuntimeLabels)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &env.Locator, nil
}

// ExportExecutable downloads the specified package from the package service
// into the provided path as an executable.
func ExportExecutable(packages PackageService, locator loc.Locator, path, label string) error {
	_, reader, err := packages.ReadPackage(locator)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	err = utils.CopyReaderWithPerms(path, reader, defaults.SharedExecutableMask)
	if err != nil {
		return trace.Wrap(err)
	}
	if selinux.GetEnabled() && label != "" {
		if err := selinux.SetFileLabel(path, label); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// IsSecretsPackage returns true if the specified package is a runtime secrets package
func IsSecretsPackage(loc loc.Locator, labels map[string]string) bool {
	if Labels(labels).HasPurpose(PurposePlanetSecrets) {
		return true
	}
	return strings.Contains(loc.Name, "secrets") && loc.Repository != defaults.SystemAccountOrg
}

// IsPlanetPackage returns true if the specified package is a runtime package
func IsPlanetPackage(packageLoc loc.Locator, labels map[string]string) bool {
	if Labels(labels).HasPurpose(PurposeRuntime) {
		return true
	}
	return (packageLoc.Name == loc.LegacyPlanetMaster.Name ||
		packageLoc.Name == loc.LegacyPlanetNode.Name)
}

// IsPlanetConfigPackage returns true if the specified package is a runtime configuration package
func IsPlanetConfigPackage(loc loc.Locator, labels map[string]string) bool {
	if Labels(labels).HasPurpose(PurposePlanetConfig) {
		return true
	}
	return strings.Contains(loc.Name, constants.PlanetConfigPackage) &&
		loc.Repository != defaults.SystemAccountOrg
}

// IsMetadataPackage determines if the specified package is a metadata package.
// A metadata package is a package that identifies a remote package but does not
// carry any data
func IsMetadataPackage(envelope PackageEnvelope) bool {
	return envelope.RuntimeLabels[PurposeLabel] == PurposeMetadata
}

// LessFunc defines a version comparator
type LessFunc func(a, b *semver.Version) bool

// MatchFunc defines a predicate to match a package.
// Matcher returns true to indicate that the given package
// matches a specific condition
type MatchFunc func(PackageEnvelope) bool

// Less is the standard version comparator that
// returns whether a < b.
// If versions are equal, it compares their metadata
func Less(a, b *semver.Version) bool {
	if a.Compare(*b) < 0 {
		return true
	}
	return a.Metadata < b.Metadata
}

// PackageMatch returns a matcher for the specified package filter
func PackageMatch(filter loc.Locator) MatchFunc {
	return func(env PackageEnvelope) bool {
		return env.Locator.Repository == filter.Repository &&
			env.Locator.Name == filter.Name
	}
}

// String formats this update as readable text
func (r LabelUpdate) String() string {
	return fmt.Sprintf("update package labels %v (+%v -%v)",
		r.Locator, r.Add, r.Remove)
}

// LabelUpdate defines an intent to update package's labels
type LabelUpdate struct {
	// Locator identifies the package
	loc.Locator
	// Remove is the list of labels to remove
	Remove []string
	// Add is the map of labels to add
	Add Labels
}

func findInstalledPackage(packages PackageService, filter loc.Locator) (env *PackageEnvelope, err error) {
	env, err = FindPackage(packages, func(e PackageEnvelope) bool {
		if e.Locator.Repository != filter.Repository {
			return false
		}
		if e.Locator.Name != filter.Name {
			return false
		}
		return e.HasLabel(InstalledLabel, InstalledLabel)
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no installed package for %v found",
				filter)
		}
		return nil, trace.Wrap(err)
	}
	return env, nil
}

func findInstalledConfigPackage(packages PackageService, filter loc.Locator) (config *PackageEnvelope, err error) {
	config, err = FindPackage(packages, func(e PackageEnvelope) bool {
		return e.HasLabels(Labels{
			ConfigLabel:    filter.ZeroVersion().String(),
			InstalledLabel: InstalledLabel,
		})
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no configuration package for %v found",
				filter)
		}
		return nil, trace.Wrap(err)
	}
	return config, nil
}

func findConfigPackage(packages PackageService, filter loc.Locator) (config *PackageEnvelope, err error) {
	config, err = FindPackage(packages, func(e PackageEnvelope) bool {
		return e.HasLabel(ConfigLabel, filter.ZeroVersion().String())
	})
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no configuration package for %v found",
				filter)
		}
		return nil, trace.Wrap(err)
	}
	return config, nil
}

func findRuntimePackage(packages PackageService) (runtimePackage *PackageEnvelope, err error) {
	labels := map[string]string{
		PurposeLabel:   PurposeRuntime,
		InstalledLabel: InstalledLabel,
	}
	err = ForeachPackage(packages, func(env PackageEnvelope) error {
		if env.HasLabels(labels) {
			runtimePackage = &env
			return utils.Abort(nil)
		}
		return nil
	})
	if err != nil && !utils.IsAbortError(err) {
		return nil, trace.Wrap(err)
	}
	if runtimePackage == nil {
		return nil, trace.NotFound("no runtime package found")
	}

	return runtimePackage, nil
}

var teleportFilter = loc.Locator{
	Repository: defaults.SystemAccountOrg,
	Name:       "teleport",
}
