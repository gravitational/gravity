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

package builder

import (
	"context"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service"
	blobfs "github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/localenv/credentials"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/layerpack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/utils"
	"k8s.io/helm/pkg/chartutil"

	"github.com/coreos/go-semver/semver"
	"github.com/docker/docker/pkg/archive"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/sirupsen/logrus"
)

// Config is the builder configuration
type Config struct {
	// Context is the build context
	Context context.Context
	// StateDir is the configured builder state directory
	StateDir string
	// Insecure disables client verification of the server TLS certificate chain
	Insecure bool
	// ManifestPath holds the path to the application manifest
	ManifestPath string
	// OutPath holds the path to the installer tarball to be output
	OutPath string
	// Overwrite indicates whether or not to overwrite an existing installer file
	Overwrite bool
	// Repository represents the source package repository
	Repository string
	// SkipVersionCheck allows to skip tele/runtime compatibility check
	SkipVersionCheck bool
	// VendorReq combines vendoring options
	VendorReq service.VendorRequest
	// Generator is used to generate installer
	Generator Generator
	// Syncer specifies the package cache syncer for the builder
	Syncer Syncer
	// Credentials is the credentials set on the CLI
	Credentials *credentials.Credentials
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Progress allows builder to report build progress
	utils.Progress
	// UpgradeVia lists intermediate runtime versions to embed
	UpgradeVia []string

	// manifestDir is the fully-qualified directory path where manifest file resides
	manifestDir string
	// manifestFilename is the name of the manifest file
	manifestFilename string
}

// CheckAndSetDefaults validates builder config and fills in defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Context == nil {
		c.Context = context.Background()
	}
	fi, err := os.Stat(c.ManifestPath)
	if err != nil {
		return trace.Wrap(err)
	}
	if fi.IsDir() {
		c.manifestDir = c.ManifestPath
	} else {
		manifestAbsPath, err := filepath.Abs(c.ManifestPath)
		if err != nil {
			return trace.Wrap(err)
		}
		c.manifestDir = filepath.Dir(manifestAbsPath)
		c.manifestFilename = filepath.Base(manifestAbsPath)
		if c.manifestFilename != defaults.ManifestFileName {
			return trace.BadParameter("manifest filename should be %q",
				defaults.ManifestFileName)
		}
	}
	if c.VendorReq.Parallel == 0 {
		c.VendorReq.Parallel = runtime.NumCPU()
	}
	if c.Generator == nil {
		c.Generator = &generator{}
	}
	if c.FieldLogger == nil {
		c.FieldLogger = logrus.WithField(trace.Component, "builder")
	}
	return nil
}

// New creates a new builder instance from the provided config
func New(config Config) (*Builder, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fi, err := os.Stat(config.ManifestPath)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	var manifest *schema.Manifest
	if fi.IsDir() {
		manifest, err = generateManifestFromChart(config.ManifestPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		manifestBytes, err := ioutil.ReadFile(config.ManifestPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		manifest, err = schema.ParseManifestYAMLNoValidate(manifestBytes)
		if err != nil {
			logrus.WithError(err).Warn("Failed to parse the application manifest.")
			return nil, trace.BadParameter("could not parse the application manifest:\n%v",
				trace.Unwrap(err)) // show original parsing error
		}
	}
	runtimeVersions, err := parseVersions(config.UpgradeVia)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b := &Builder{
		Config:     config,
		Manifest:   *manifest,
		UpgradeVia: runtimeVersions,
	}
	err = b.initServices()
	if err != nil {
		b.Close()
		return nil, trace.Wrap(err)
	}
	return b, nil
}

// Builder implements the installer builder
type Builder struct {
	// Config is the builder configuration
	Config
	// Env is the local build environment
	Env *localenv.LocalEnvironment
	// Manifest is the parsed manifest of the application being built
	Manifest schema.Manifest
	// Dir is the directory where build-related data is stored
	Dir string
	// Backend is the local backend
	Backend storage.Backend
	// Packages is the layered package service with the local cache
	// directory serving as a 'read' layer and the temporary directory
	// as a 'read-write' layer
	Packages pack.PackageService
	// Apps is the application service based on the layered package service
	Apps libapp.Applications
	// UpgradeVia lists intermediate runtime versions to embed in the resulting installer
	UpgradeVia []semver.Version
}

// Locator returns locator of the application that's being built
func (b *Builder) Locator() loc.Locator {
	version := b.Manifest.Metadata.ResourceVersion
	if b.VendorReq.PackageVersion != "" {
		version = b.VendorReq.PackageVersion
	}
	return loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       b.Manifest.Metadata.Name,
		Version:    version,
	}
}

// SelectRuntime picks an appropriate runtime for the application that's
// being built
func (b *Builder) SelectRuntime() (*semver.Version, error) {
	runtime := b.Manifest.Base()
	if runtime == nil {
		return nil, trace.NotFound("failed to determine application runtime")
	}
	switch runtime.Name {
	case constants.BaseImageName, defaults.Runtime:
	default:
		return nil, trace.BadParameter("unsupported base image %q, only %q is "+
			"supported as a base image", runtime.Name, constants.BaseImageName)
	}
	// If runtime version is explicitly set in the manifest, use it.
	if runtime.Version != loc.LatestVersion {
		b.Infof("Using pinned runtime version: %s.", runtime.Version)
		b.PrintSubStep("Will use base image version %s set in manifest", runtime.Version)
		return semver.NewVersion(runtime.Version)
	}
	// Otherwise, default to the version of this tele binary to ensure
	// compatibility.
	teleVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b.Infof("Selected runtime version based on tele version: %s.", teleVersion)
	b.PrintSubStep("Will use base image version %s", teleVersion)
	return teleVersion, nil
}

// SyncPackageCache ensures that all system dependencies are present in
// the local cache directory for the specified list of runtime versions
func (b *Builder) SyncPackageCache(ctx context.Context, runtimeVersion semver.Version, intermediateVersions ...semver.Version) error {
	apps, err := b.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, runtimeVersion := range append([]semver.Version{runtimeVersion}, intermediateVersions...) {
		b.NextStep("Syncing packages for %v", runtimeVersion)
		if err := b.syncPackageCache(ctx, runtimeVersion, b.Syncer, apps); err != nil {
			return trace.Wrap(err, "failed to sync packages for runtime version %v", runtimeVersion)
		}
	}
	return nil
}

func (b *Builder) syncPackageCache(ctx context.Context, runtimeVersion semver.Version, syncer Syncer, apps libapp.Applications) error {
	// see if all required packages/apps are already present in the local cache
	runtimeApp, err := apps.GetApp(RuntimeApp(runtimeVersion))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if runtimeApp != nil {
		err = libapp.VerifyDependencies(*runtimeApp, apps, b.Env.Packages)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err == nil {
			b.Info("Local package cache is up-to-date.")
			b.NextStep("Local package cache is up-to-date")
			return nil
		}
	}
	b.Infof("Synchronizing package cache with %v.", b.Repository)
	b.NextStep("Downloading dependencies from %v", b.Repository)
	return syncer.Sync(ctx, b, runtimeVersion)
}

// Vendor vendors the application images in the provided directory and
// returns the compressed data stream with the application data
func (b *Builder) Vendor(ctx context.Context, dir string) (io.ReadCloser, error) {
	err := utils.CopyDirContents(b.manifestDir, filepath.Join(dir, defaults.ResourcesDir))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifestPath := filepath.Join(dir, defaults.ResourcesDir, "app.yaml")
	// If manifest filename is empty, it means it was auto-generated
	// out of a Helm chart so write the generated manifest to the
	// vendor directory as well.
	if b.manifestFilename == "" {
		data, err := yaml.Marshal(b.Manifest)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = ioutil.WriteFile(manifestPath, data, defaults.SharedReadMask)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	dockerClient, err := docker.NewDefaultClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vendorer, err := service.NewVendorer(service.VendorerConfig{
		DockerClient: dockerClient,
		ImageService: docker.NewDefaultImageService(),
		RegistryURL:  defaults.DockerRegistry,
		Packages:     b.Packages,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vendorReq := b.VendorReq
	vendorReq.ManifestPath = manifestPath
	vendorReq.ProgressReporter = b.Progress
	err = vendorer.VendorDir(ctx, dir, vendorReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return archive.Tar(dir, archive.Uncompressed)
}

// CreateApplication creates a Gravity application from the provided
// data in the local database
func (b *Builder) CreateApplication(data io.ReadCloser) (*libapp.Application, error) {
	progressC := make(chan *libapp.ProgressEntry)
	errorC := make(chan error, 1)
	err := b.Packages.UpsertRepository(defaults.SystemAccountOrg, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	op, err := b.Apps.CreateImportOperation(&libapp.ImportRequest{
		Source:    data,
		ProgressC: progressC,
		ErrorC:    errorC,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// wait for the import to complete
	for range progressC {
	}
	err = <-errorC
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b.Apps.GetImportedApplication(*op)
}

// GenerateInstaller generates an installer tarball for the specified
// application and returns its data as a stream
func (b *Builder) GenerateInstaller(application libapp.Application) (io.ReadCloser, error) {
	dependencies, err := b.collectUpgradeDependencies()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req, err := b.Generator.NewInstallerRequest(b, application)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.AdditionalDependencies = *dependencies
	return b.Apps.GetAppInstaller(*req)
}

// WriteInstaller writes the provided installer tarball data to disk
func (b *Builder) WriteInstaller(data io.ReadCloser) error {
	f, err := os.Create(b.OutPath)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = io.Copy(f, data)
	return trace.Wrap(err)
}

// Close cleans up build environment
func (b *Builder) Close() error {
	var errors []error
	if b.Env != nil {
		errors = append(errors, b.Env.Close())
	}
	if b.Backend != nil {
		errors = append(errors, b.Backend.Close())
	}
	if b.Dir != "" {
		errors = append(errors, os.RemoveAll(b.Dir))
	}
	if b.Progress != nil {
		b.Progress.Stop()
	}
	return trace.NewAggregate(errors...)
}

// initServices initializes the builder backend, package and apps services
func (b *Builder) initServices() (err error) {
	b.Env, err = b.makeBuildEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	b.Dir, err = ioutil.TempDir("", "build")
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(b.Dir)
		}
	}()
	b.Backend, err = keyval.NewBolt(keyval.BoltConfig{
		Path: filepath.Join(b.Dir, defaults.GravityDBFile),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	objects, err := blobfs.New(blobfs.Config{Path: filepath.Join(b.Dir, defaults.PackagesDir)})
	if err != nil {
		return trace.Wrap(err)
	}
	packages, err := localpack.New(localpack.Config{
		Backend:     b.Backend,
		UnpackedDir: filepath.Join(b.Dir, defaults.PackagesDir, defaults.UnpackedDir),
		Objects:     objects,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	b.Packages = layerpack.New(b.Env.Packages, packages)
	b.Apps, err = b.Env.AppServiceLocal(localenv.AppConfig{
		Packages: b.Packages,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// makeBuildEnv creates a new local build environment instance
func (b *Builder) makeBuildEnv() (*localenv.LocalEnvironment, error) {
	// if state directory was specified explicitly, it overrides
	// both cache directory and config directory as it's used as
	// a special case only for building from local packages
	if b.StateDir != "" {
		b.Infof("Using package cache from %v.", b.StateDir)
		return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
			StateDir:         b.StateDir,
			LocalKeyStoreDir: b.StateDir,
			Insecure:         b.Insecure,
			Credentials:      b.Credentials,
		})
	}
	// otherwise use default locations for cache / key store
	cacheDir, err := ensureCacheDir(b.Repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b.Infof("Using package cache from %v.", cacheDir)
	return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
		StateDir:    cacheDir,
		Insecure:    b.Insecure,
		Credentials: b.Credentials,
	})
}

// checkVersion makes sure that the tele version is compatible with the selected
// runtime version.
func (b *Builder) checkVersion(runtimeVersion *semver.Version) error {
	if b.SkipVersionCheck {
		return nil
	}
	teleVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return trace.Wrap(err, "failed to determine tele version")
	}
	if !versionsCompatible(*teleVersion, *runtimeVersion) {
		return trace.BadParameter(
			`Version of this tele binary (%[1]v) is not compatible with the base image version specified in the manifest (%[2]v).

There are a few ways to resolve the issue:

 * Download tele binary of the same version as the specified base image (%[2]v) and use it to build the image.

 * Specify base image version compatible with this tele in the manifest file: "baseImage: gravity:%[1]v".

 * Do not specify "baseImage" in the manifest file, in which case tele will automatically pick compatible version.
`, teleVersion, runtimeVersion)
	}
	b.Debugf("Version check passed; tele version: %v, runtime version: %v.",
		teleVersion, runtimeVersion)
	return nil
}

// collectUpgradeDependencies computes and returns a set of package dependencies for each
// configured intermediate runtime version.
// result contains combined dependencies marked with a label per runtime version.
func (b *Builder) collectUpgradeDependencies() (result *libapp.Dependencies, err error) {
	apps, err := b.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result = &libapp.Dependencies{}
	for _, runtimeVersion := range b.UpgradeVia {
		app, err := apps.GetApp(loc.Runtime.WithVersion(runtimeVersion))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req := libapp.GetDependenciesRequest{
			App:  *app,
			Apps: apps,
			Pack: b.Env.Packages,
		}
		dependencies, err := libapp.GetDependencies(req)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		dependencies.Apps = append(dependencies.Apps, *app)
		addUpgradeVersionLabel(dependencies, runtimeVersion.String())
		result.Packages = append(result.Packages, filterUpgradePackageDependencies(dependencies.Packages)...)
		result.Apps = append(result.Apps, dependencies.Apps...)
	}
	return result, nil
}

// RuntimeApp returns the locator of the runtime application with the specified version
func RuntimeApp(version semver.Version) loc.Locator {
	return loc.Runtime.WithVersion(version)
}

// versionsCompatible returns true if the provided tele and runtime versions
// are compatible. Tele version is said to be compatible to the given runtime
// version if the installer built with the specified combination will work as
// expected.
//
// Compatibility is defined as follows:
//   1. Major and minor semver components of both versions are equal.
//   2. Runtime version is not greater than tele version.
func versionsCompatible(teleVer, runtimeVer semver.Version) bool {
	return teleVer.Major == runtimeVer.Major &&
		teleVer.Minor == runtimeVer.Minor &&
		!teleVer.LessThan(runtimeVer)
}

// ensureCacheDir makes sure a local cache directory for the provided Ops Center
// exists
func ensureCacheDir(opsURL string) (string, error) {
	u, err := url.Parse(opsURL)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// cache directory is ~/.gravity/cache/<opscenter>/
	dir, err := utils.EnsureLocalPath("", defaults.LocalCacheDir, u.Host)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return dir, nil
}

func generateManifestFromChart(manifestPath string) (*schema.Manifest, error) {
	// If this is a Helm chart directory, extract the chart metadata
	// and generate a basic application manifest.
	fi, err := os.Stat(filepath.Join(manifestPath, constants.HelmChartFile))
	if err != nil || fi.IsDir() {
		if err != nil {
			logrus.Warn(err)
		}
		return nil, trace.BadParameter("expected a chart directory: %v", manifestPath)
	}
	chart, err := chartutil.Load(manifestPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return generateManifest(chart)
}

// filterUpgradePackageDependencies returns the list of package dependencies
// to include as additional dependencies when building an installer.
// packages lists all package dependencies for a specific intermediate version.
// The resulting list will only include the packages the upgrade will need
// for each intermediate hop which includes the gravity binary, teleport and planet container packages.
// All other packages are not necessary for an intermediate upgrade hop and will be omitted.
func filterUpgradePackageDependencies(packages []pack.PackageEnvelope) (result []pack.PackageEnvelope) {
	result = packages[:0]
	for _, pkg := range packages {
		if pkg.Locator.Repository != defaults.SystemAccountOrg {
			continue
		}
		switch pkg.Locator.Name {
		case constants.TeleportPackage,
			constants.GravityPackage,
			constants.PlanetPackage:
		default:
			continue
		}
		result = append(result, pkg)
	}
	return result
}

func addUpgradeVersionLabel(dependencies *libapp.Dependencies, version string) {
	for i := range dependencies.Packages {
		dependencies.Packages[i].RuntimeLabels = utils.CombineLabels(
			dependencies.Packages[i].RuntimeLabels,
			pack.RuntimeUpgradeLabels(version),
		)
	}
	for i := range dependencies.Apps {
		dependencies.Apps[i].PackageEnvelope.RuntimeLabels = utils.CombineLabels(
			dependencies.Apps[i].PackageEnvelope.RuntimeLabels,
			pack.RuntimeUpgradeLabels(version),
		)
	}
}

func parseVersions(versions []string) (result []semver.Version, err error) {
	result = make([]semver.Version, 0, len(versions))
	for _, version := range versions {
		runtimeVersion, err := semver.NewVersion(version)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, *runtimeVersion)
	}
	return result, nil
}
