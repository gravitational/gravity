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
	"io/ioutil"
	"os"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/hub"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

// Syncer synchronizes the local package cache from a (remote) repository
type Syncer interface {
	// Sync synchronizes the dependencies between a remote repository and the local cache directory.
	// app specifies the application to synchronize dependencies for.
	// intermediateRuntimes optionally provides the list of additional runtime application dependencies
	// to sync.
	Sync(ctx context.Context, builder *Builder, app libapp.Application, intermediateRuntimes []semver.Version) error
}

// S3Syncer synchronizes local package cache with an S3 bucket
type S3Syncer struct {
	// hub provides access to runtimes stored in S3 bucket
	hub hub.Hub
}

// NewS3Syncer returns a syncer that syncs packages with an S3 bucket
func NewS3Syncer() (*S3Syncer, error) {
	hub, err := hub.New(hub.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &S3Syncer{
		hub: hub,
	}, nil
}

// intermediateRuntimes provides a list of additional runtime dependencies to sync.
// Sync synchronizes the runtime application of the specified app.
// intermediateRuntimes optionally specifies additional runtime application versions to sync.
//
// This implementation does not attempt to sync the application's direct dependencies since
// they are never available in the oss hub and instead should already be in the local package cache.
func (s *S3Syncer) Sync(ctx context.Context, builder *Builder, app libapp.Application, intermediateRuntimes []semver.Version) error {
	if app.Manifest.Base() == nil {
		return trace.BadParameter("runtime version unspecified in manifest")
	}
	runtimeApp := func(version string) loc.Locator {
		return loc.Locator{
			Repository: defaults.SystemAccountOrg,
			Name:       "telekube",
			Version:    version,
		}
	}
	cacheApps, err := builder.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, runtimeVersion := range intermediateRuntimes {
		err := libapp.VerifyDependencies(app, cacheApps, builder.Env.Packages)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err == nil {
			builder.WithField("runtime-app", runtimeVersion).Info("Local package cache is up-to-date.")
			builder.NextStep("Local package cache is up-to-date for %v", runtimeVersion)
			continue
		}
		runtimeApp := runtimeApp(runtimeVersion.String())
		if err := s.sync(ctx, builder, runtimeApp, cacheApps); err != nil {
			return trace.Wrap(err, "failed to sync packages for runtime version %v", runtimeVersion)
		}
	}
	if err := s.sync(ctx, builder, runtimeApp(app.Manifest.Base().Version), cacheApps); err != nil {
		return trace.Wrap(err, "failed to sync packages for runtime version %v", app.Manifest.Base().Version)
	}
	return nil
}

func (s *S3Syncer) sync(ctx context.Context, builder *Builder, app loc.Locator, cacheApps libapp.Applications) error {
	unpackedDir, err := ioutil.TempDir("", "runtime-unpacked")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(unpackedDir)

	err = s.download(unpackedDir, app)
	if err != nil {
		return trace.Wrap(err)
	}
	tarballEnv, err := localenv.NewTarballEnvironment(localenv.TarballEnvironmentArgs{
		StateDir: unpackedDir,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer tarballEnv.Close()

	// sync packages between unpacked tarball directory and package cache
	tarballApp, err := tarballEnv.Apps.GetApp(app)
	if err != nil {
		return trace.Wrap(err)
	}
	puller := libapp.Puller{
		FieldLogger: builder.FieldLogger,
		SrcPack:     tarballEnv.Packages,
		SrcApp:      tarballEnv.Apps,
		DstPack:     builder.Env.Packages,
		DstApp:      cacheApps,
		Parallel:    builder.VendorReq.Parallel,
		Upsert:      true,
	}
	return puller.PullApp(ctx, tarballApp.Package)
}

func (s *S3Syncer) download(path string, loc loc.Locator) error {
	tarball, err := s.hub.Get(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	defer tarball.Close()
	// unpack the downloaded tarball
	err = archive.Extract(tarball, path)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewPackSyncer creates a new syncer from provided pack and apps services
func NewPackSyncer(pack pack.PackageService, apps libapp.Applications) *PackSyncer {
	return &PackSyncer{
		pack: pack,
		apps: apps,
	}
}

// Sync pulls dependencies from the package/app service not available locally.
// app specifies the application to sync dependencies for.
// intermediateRuntimes optionally specifies additional runtime application versions to sync.
func (s *PackSyncer) Sync(ctx context.Context, builder *Builder, app libapp.Application, intermediateRuntimes []semver.Version) error {
	cacheApps, err := builder.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, runtimeVersion := range intermediateRuntimes {
		builder.NextStep("Syncing packages for %v", runtimeVersion)
		// TODO(dmitri): current distribution hub will not return the manifest correctly
		// for a recent version of the application due to compatibility issues.
		// Query the package and parse the manifest manually to be able to properly query
		// the runtime package
		app, err := getApp(s.pack, runtimeApp(runtimeVersion))
		if err != nil {
			return trace.Wrap(err)
		}
		err = libapp.VerifyDependencies(*app, cacheApps, builder.Env.Packages)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err == nil {
			builder.WithField("runtime-app", runtimeVersion).Info("Local package cache is up-to-date.")
			builder.NextStep("Local package cache is up-to-date for %v", runtimeVersion)
			continue
		}
		if err := s.sync(ctx, builder, *app, cacheApps); err != nil {
			return trace.Wrap(err, "failed to sync packages for runtime version %v", runtimeVersion)
		}
	}
	// Synchronize direct dependencies of the application
	if err := s.syncDeps(ctx, builder, app, cacheApps); err != nil {
		return trace.Wrap(err, "failed to sync dependencies for application %v", app)
	}
	return nil
}

func (s *PackSyncer) sync(ctx context.Context, builder *Builder, app libapp.Application, cacheApps libapp.Applications) error {
	puller := libapp.Puller{
		FieldLogger: builder.FieldLogger,
		SrcPack:     s.pack,
		SrcApp:      s.apps,
		DstPack:     builder.Env.Packages,
		DstApp:      cacheApps,
		Parallel:    builder.VendorReq.Parallel,
		OnConflict:  libapp.GetDependencyConflictHandler(false),
	}
	err := puller.PullAppDeps(ctx, app)
	if err != nil {
		if utils.IsNetworkError(err) || trace.IsEOF(err) {
			return trace.ConnectionProblem(err, "failed to download "+
				"application dependencies from %v - please make sure the "+
				"repository is reachable: %v", builder.Repository, err)
		}
		return trace.Wrap(err)
	}
	return puller.PullAppPackage(ctx, app.Package)
}

func (s *PackSyncer) syncDeps(ctx context.Context, builder *Builder, app libapp.Application, cacheApps libapp.Applications) error {
	puller := libapp.Puller{
		FieldLogger: builder.FieldLogger,
		SrcPack:     s.pack,
		SrcApp:      s.apps,
		DstPack:     builder.Env.Packages,
		DstApp:      cacheApps,
		Parallel:    builder.VendorReq.Parallel,
		OnConflict:  libapp.GetDependencyConflictHandler(false),
	}
	err := puller.PullAppDeps(ctx, app)
	if err != nil {
		if utils.IsNetworkError(err) || trace.IsEOF(err) {
			return trace.ConnectionProblem(err, "failed to download "+
				"application dependencies from %v - please make sure the "+
				"repository is reachable: %v", builder.Repository, err)
		}
		return trace.Wrap(err)
	}
	return nil
}

// PackSyncer synchronizes local package cache with remote package/application services
type PackSyncer struct {
	pack pack.PackageService
	apps libapp.Applications
}

func getApp(pack pack.PackageService, loc loc.Locator) (*libapp.Application, error) {
	env, err := pack.ReadPackageEnvelope(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := schema.ParseManifestYAMLNoValidate(env.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &libapp.Application{
		Package:  loc,
		Manifest: *manifest,
	}, nil
}
