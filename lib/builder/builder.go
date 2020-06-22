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
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service"
	blobfs "github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
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
	// manifestDir is the fully-qualified directory path where manifest file resides
	manifestDir string
	// manifestFilename is the name of the manifest file
	manifestFilename string
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
	// NewSyncer is used to initialize package cache syncer for the builder
	NewSyncer NewSyncerFunc
	// GetRepository is a function that returns package source repository
	GetRepository GetRepositoryFunc
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Progress allows builder to report build progress
	utils.Progress
	// Silent suppresses all std output when set to true
	Silent bool
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
	if c.NewSyncer == nil {
		c.NewSyncer = NewSyncer
	}
	if c.GetRepository == nil {
		c.GetRepository = getRepository
	}
	if c.FieldLogger == nil {
		c.FieldLogger = logrus.WithField(trace.Component, "builder")
	}
	if c.Progress == nil {
		c.Progress = utils.NewProgress(c.Context, "Build", 6, false)
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
		// If this is a Helm chart directory, extract the chart metadata
		// and generate a basic application manifest.
		fi, err := os.Stat(filepath.Join(config.ManifestPath, constants.HelmChartFile))
		if err != nil || fi.IsDir() {
			if err != nil {
				logrus.Warn(err)
			}
			return nil, trace.BadParameter("not a chart directory")
		}
		chart, err := chartutil.Load(config.ManifestPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		manifest, err = generateManifest(chart)
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
			logrus.Errorf(trace.DebugReport(err))
			return nil, trace.BadParameter("could not parse the application manifest:\n%v",
				trace.Unwrap(err)) // show original parsing error
		}
	}
	b := &Builder{
		Config:   config,
		Manifest: *manifest,
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
	Apps app.Applications
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
// the local cache directory
func (b *Builder) SyncPackageCache(runtimeVersion *semver.Version) error {
	apps, err := b.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	// see if all required packages/apps are already present in the local cache
	b.Manifest.SetBase(loc.Runtime.WithVersion(runtimeVersion))
	err = app.VerifyDependencies(&app.Application{
		Manifest: b.Manifest,
		Package:  b.Manifest.Locator(),
	}, apps, b.Env.Packages)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		b.Info("Local package cache is up-to-date.")
		b.NextStep("Local package cache is up-to-date")
		return nil
	}
	repository, err := b.GetRepository(b)
	if err != nil {
		return trace.Wrap(err)
	}
	b.Infof("Synchronizing package cache with %v.", repository)
	b.NextStep("Downloading dependencies from %v", repository)
	syncer, err := b.NewSyncer(b)
	if err != nil {
		return trace.Wrap(err)
	}
	return syncer.Sync(b, runtimeVersion)
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
	vendorer, err := service.NewVendorer(service.VendorerConfig{
		DockerURL:   constants.DockerEngineURL,
		RegistryURL: constants.DockerRegistry,
		Packages:    b.Packages,
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
func (b *Builder) CreateApplication(data io.ReadCloser) (*app.Application, error) {
	progressC := make(chan *app.ProgressEntry)
	errorC := make(chan error, 1)
	err := b.Packages.UpsertRepository(defaults.SystemAccountOrg, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	op, err := b.Apps.CreateImportOperation(&app.ImportRequest{
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
func (b *Builder) GenerateInstaller(application app.Application) (io.ReadCloser, error) {
	return b.Generator.Generate(b, application)
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
		})
	}
	// otherwise use default locations for cache / key store
	repository, err := b.GetRepository(b)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cacheDir, err := ensureCacheDir(repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	b.Infof("Using package cache from %v.", cacheDir)
	return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
		StateDir: cacheDir,
		Insecure: b.Insecure,
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

// versionsCompatible returns true if the provided tele and runtime versions
// are compatible.
//
// Compatibility is defined as follows:
//   1. Major and minor semver components of both versions are equal.
//   2. Runtime version is not greater than tele version.
func versionsCompatible(teleVer, runtimeVer semver.Version) bool {
	return teleVer.Major == runtimeVer.Major &&
		teleVer.Minor == runtimeVer.Minor &&
		!teleVer.LessThan(runtimeVer)
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

// GetRepositoryFunc defines function that returns package source repository
type GetRepositoryFunc func(*Builder) (string, error)

// getRepository returns package source repository for the provided builder
func getRepository(b *Builder) (string, error) {
	return fmt.Sprintf("s3://%v", defaults.HubBucket), nil
}
