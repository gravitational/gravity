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

	"github.com/coreos/go-semver/semver"
	"github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/sirupsen/logrus"
)

// Config is the builder configuration
type Config struct {
	// Env is the local build environment
	Env *localenv.LocalEnvironment
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
	// Syncer is used to synchronize local package cache
	Syncer Syncer
	// FieldLogger is used for logging
	logrus.FieldLogger
}

// CheckAndSetDefaults validates builder config and fills in defaults
func (c *Config) CheckAndSetDefaults() error {
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
	// if output tarball name is not specified, it defaults to the name of the
	// directory with manifest
	if c.OutPath == "" {
		c.OutPath = fmt.Sprintf("%v.tar", filepath.Base(c.manifestDir))
	}
	_, err = os.Stat(c.OutPath)
	if err == nil && !c.Overwrite {
		return trace.BadParameter("tarball %v already exists, please remove "+
			"it first or provide '-f' flag to overwrite it", c.OutPath)
	}
	if c.VendorReq.Parallel == 0 {
		c.VendorReq.Parallel = runtime.NumCPU()
	}
	if c.Generator == nil {
		c.Generator = &generator{}
	}
	if c.Syncer == nil {
		c.Syncer, err = newSyncer()
		if err != nil {
			return trace.Wrap(err)
		}
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
	manifestBytes, err := ioutil.ReadFile(config.ManifestPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := schema.ParseManifestYAMLNoValidate(manifestBytes)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse the application manifest, "+
			"please check that it's in correct YAML format: %v", err)
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

// SyncPackageCache ensures that all system dependencies are present in
// the local cache directory
func (b *Builder) SyncPackageCache() error {
	apps, err := b.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	// see if all required packages/apps are already present in the local cache
	err = app.VerifyDependencies(&app.Application{
		Manifest: b.Manifest,
		Package:  b.Manifest.Locator(),
	}, apps, b.Env.Packages)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		b.Info("Package cache is up-to-date.")
		return nil
	}
	b.Info("Synchronizing package cache.")
	return b.Syncer.Sync(b)
}

// Vendor vendors the application images in the provided directory and
// returns the compressed data stream with the application data
func (b *Builder) Vendor(ctx context.Context, dir string, progress utils.Progress) (io.ReadCloser, error) {
	err := utils.CopyDirContents(b.manifestDir, filepath.Join(dir, defaults.ResourcesDir))
	if err != nil {
		return nil, trace.Wrap(err)
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
	vendorReq.ManifestPath = filepath.Join(dir, defaults.ResourcesDir, b.manifestFilename)
	vendorReq.ProgressReporter = progress
	err = vendorer.VendorDir(ctx, dir, vendorReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return archive.Tar(dir, archive.Uncompressed)
}

// CreateApplication creates a Telekube application from the provided
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

// checkVersion makes sure that the tele version is compatible with the selected
// runtime version
//
// Compatibility is defined as "tele version must be >= runtime version"
func (b *Builder) checkVersion() error {
	teleVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return trace.Wrap(err, "failed to determine tele version")
	}
	runtimeLoc := b.Manifest.Base()
	if runtimeLoc == nil {
		return trace.BadParameter("failed to determine runtime version, make "+
			"sure your application type is %q", schema.KindBundle)
	}
	runtimePackage, err := b.Packages.ReadPackageEnvelope(*runtimeLoc)
	if err != nil {
		return trace.Wrap(err)
	}
	runtimeVersion, err := semver.NewVersion(runtimePackage.Locator.Version)
	if err != nil {
		return trace.Wrap(err, "invalid runtime version: %v",
			runtimePackage.Locator.Version)
	}
	if teleVersion.LessThan(*runtimeVersion) {
		// truncate meta information
		withoutMeta := fmt.Sprintf("%v.%v.%v",
			runtimeVersion.Major, runtimeVersion.Minor, runtimeVersion.Patch)
		return trace.BadParameter(
			`The version of the tele binary (%v) is not compatible with the selected runtime (%v).

Please upgrade your Telekube tools to match the runtime version (curl https://get.gravitational.io/telekube/install/%v | bash), or specify an appropriate runtime version in your application manifest. You can view available runtimes using "tele ls --runtimes" and pick the one that matches your tele version.`, teleVersion, runtimeVersion, withoutMeta)
	}

	b.Debugf("version check passed, tele version: %v, runtime version: %v",
		teleVersion, runtimeVersion)
	return nil
}

// Close cleans up build environment
func (b *Builder) Close() error {
	if b.Backend != nil {
		err := b.Backend.Close()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if b.Dir != "" {
		err := os.RemoveAll(b.Dir)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
