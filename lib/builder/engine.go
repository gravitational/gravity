/*
Copyright 2018-2020 Gravitational, Inc.

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
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gravitational/gravity/lib/app"
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

	"github.com/coreos/go-semver/semver"
	"github.com/docker/docker/pkg/archive"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "builder")

// Config is the builder configuration
type Config struct {
	// StateDir is the configured builder state directory
	StateDir string
	// Insecure disables client verification of the server TLS certificate chain
	Insecure bool
	// Repository represents the source package repository
	Repository string
	// SkipVersionCheck allows to skip tele/runtime compatibility check
	SkipVersionCheck bool
	// Parallel is the builder's parallelism level
	Parallel int
	// Generator is used to generate installer
	Generator Generator
	// NewSyncer is used to initialize package cache syncer for the builder
	NewSyncer NewSyncerFunc
	// GetRepository is a function that returns package source repository
	GetRepository GetRepositoryFunc
	// CredentialsService provides access to user credentials
	CredentialsService credentials.Service
	// Credentials is the credentials set on the CLI
	Credentials *credentials.Credentials
	// Level is the level at which the progress should be reported
	Level utils.ProgressLevel
	// Progress allows builder to report build progress
	utils.Progress
}

// CheckAndSetDefaults validates builder config and fills in defaults
func (c *Config) CheckAndSetDefaults() error {
	if c.Parallel == 0 {
		c.Parallel = runtime.NumCPU()
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
	var err error
	if c.CredentialsService == nil {
		c.CredentialsService, err = credentials.New(credentials.Config{
			LocalKeyStoreDir: c.StateDir,
			Credentials:      c.Credentials,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if c.Progress == nil {
		c.Progress = utils.NewProgressWithConfig(context.TODO(), "Build",
			utils.ProgressConfig{
				Level:       c.Level,
				StepPrinter: utils.TimestampedStepPrinter,
			})
	}
	return nil
}

// newEngine creates a new builder engine.
func newEngine(config Config) (*Engine, error) {
	if err := checkBuildEnv(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	b := &Engine{
		Config: config,
	}
	if err := b.initServices(); err != nil {
		b.Close()
		return nil, trace.Wrap(err)
	}
	return b, nil
}

// Engine is the builder engine that provides common functionality for building
// cluster and application images.
type Engine struct {
	// Config is the builder Engine configuration.
	Config
	// Env is the local build environment.
	Env *localenv.LocalEnvironment
	// Dir is the directory where build-related data is stored.
	Dir string
	// Backend is the local backend.
	Backend storage.Backend
	// Packages is the layered package service with the local cache
	// directory serving as a 'read' layer and the temporary directory
	// as a 'read-write' layer.
	Packages pack.PackageService
	// Apps is the application service based on the layered package service.
	Apps app.Applications
}

// SelectRuntime picks an appropriate base image version for the cluster
// image that's being built
func (b *Engine) SelectRuntime(manifest *schema.Manifest) (*semver.Version, error) {
	runtime := manifest.Base()
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
		log.WithField("version", runtime.Version).Info("Using pinned runtime version.")
		b.PrintSubStep("Will use base image version %s set in manifest", runtime.Version)
		return semver.NewVersion(runtime.Version)
	}
	// Otherwise, default to the version of this tele binary to ensure
	// compatibility.
	teleVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.WithField("version", teleVersion).Info("Selected runtime version based on tele version.")
	b.PrintSubStep("Will use base image version %s", teleVersion)
	return teleVersion, nil
}

// SyncPackageCache ensures that all system dependencies are present in
// the local cache directory
func (b *Engine) SyncPackageCache(manifest *schema.Manifest, runtimeVersion *semver.Version) error {
	apps, err := b.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	// see if all required packages/apps are already present in the local cache
	manifest.SetBase(loc.Runtime.WithVersion(runtimeVersion))
	err = app.VerifyDependencies(&app.Application{
		Manifest:    *manifest,
		Package:     manifest.Locator(),
		ExcludeApps: app.AppsToExclude(*manifest),
	}, apps, b.Env.Packages)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		log.Info("Local package cache is up-to-date.")
		b.NextStep("Local package cache is up-to-date")
		return nil
	}
	repository, err := b.GetRepository(b)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Synchronizing package cache with %v.", repository)
	b.NextStep("Downloading dependencies from %v", repository)
	syncer, err := b.NewSyncer(b)
	if err != nil {
		return trace.Wrap(err)
	}
	return syncer.Sync(b, manifest, runtimeVersion)
}

// VendorRequest combines vendoring parameters.
type VendorRequest struct {
	// SourceDir is the cluster or application image source directory.
	SourceDir string
	// VendorDir is the directory to perform vendoring in.
	VendorDir string
	// Manifest is the image manifest.
	Manifest *schema.Manifest
	// Vendor is parameters of the vendorer.
	Vendor service.VendorRequest
}

// Vendor vendors the application images in the provided directory and
// returns the compressed data stream with the application data
func (b *Engine) Vendor(ctx context.Context, req VendorRequest) (io.ReadCloser, error) {
	err := utils.CopyDirContents(req.SourceDir, filepath.Join(req.VendorDir, defaults.ResourcesDir))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifestPath := filepath.Join(req.VendorDir, defaults.ResourcesDir, "app.yaml")
	data, err := yaml.Marshal(req.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = ioutil.WriteFile(manifestPath, data, defaults.SharedReadMask)
	if err != nil {
		return nil, trace.Wrap(err)
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
	vendorReq := req.Vendor
	vendorReq.ManifestPath = manifestPath
	vendorReq.ProgressReporter = b.Progress
	err = vendorer.VendorDir(ctx, req.VendorDir, vendorReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return archive.Tar(req.VendorDir, archive.Uncompressed)
}

// CreateApplication creates a Gravity application from the provided
// data in the local database
func (b *Engine) CreateApplication(data io.ReadCloser, excludeApps []loc.Locator) (*app.Application, error) {
	progressC := make(chan *app.ProgressEntry)
	errorC := make(chan error, 1)
	err := b.Packages.UpsertRepository(defaults.SystemAccountOrg, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	op, err := b.Apps.CreateImportOperation(&app.ImportRequest{
		Source:      data,
		ProgressC:   progressC,
		ErrorC:      errorC,
		ExcludeApps: excludeApps,
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
func (b *Engine) GenerateInstaller(manifest *schema.Manifest, application app.Application) (io.ReadCloser, error) {
	return b.Generator.Generate(b, manifest, application)
}

// WriteInstaller writes the provided installer tarball data to disk
func (b *Engine) WriteInstaller(data io.ReadCloser, outPath string) error {
	f, err := os.Create(outPath)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = io.Copy(f, data)
	return trace.Wrap(err)
}

// initServices initializes the builder backend, package and apps services
func (b *Engine) initServices() (err error) {
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
	objects, err := blobfs.New(filepath.Join(b.Dir, defaults.PackagesDir))
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
func (b *Engine) makeBuildEnv() (*localenv.LocalEnvironment, error) {
	// if state directory was specified explicitly, it overrides
	// both cache directory and config directory as it's used as
	// a special case only for building from local packages
	if b.StateDir != "" {
		log.Infof("Using package cache from %v.", b.StateDir)
		return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
			StateDir:         b.StateDir,
			LocalKeyStoreDir: b.StateDir,
			Insecure:         b.Insecure,
			Credentials:      b.Credentials,
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
	log.Infof("Using package cache from %v.", cacheDir)
	return localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
		StateDir:    cacheDir,
		Insecure:    b.Insecure,
		Credentials: b.Credentials,
	})
}

// checkVersion makes sure that the tele version is compatible with the selected
// runtime version.
func (b *Engine) checkVersion(runtimeVersion *semver.Version) error {
	if b.SkipVersionCheck {
		return nil
	}
	if err := checkVersion(runtimeVersion); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close cleans up build environment
func (b *Engine) Close() error {
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

// GetRepositoryFunc defines function that returns package source repository
type GetRepositoryFunc func(*Engine) (string, error)

// getRepository returns package source repository for the provided builder
func getRepository(b *Engine) (string, error) {
	return fmt.Sprintf("s3://%v", defaults.HubBucket), nil
}
