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
	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
)

// Syncer synchronizes the local package cache from a (remote) repository
type Syncer interface {
	// Sync makes sure that local cache has all required dependencies for the
	// selected runtime
	Sync(context.Context, *Engine, *schema.Manifest, *semver.Version) error
}

// NewSyncerFunc defines function that creates syncer for a builder
type NewSyncerFunc func(*Engine) (Syncer, error)

// NewSyncer returns a new syncer instance for the provided builder
//
// Satisfies NewSyncerFunc type.
func NewSyncer(b *Engine) (Syncer, error) {
	return newS3Syncer()
}

// s3Syncer synchronizes local package cache with S3 bucket
type s3Syncer struct {
	// hub provides access to runtimes stored in S3 bucket
	hub hub.Hub
}

// newS3Syncer returns a syncer that syncs packages with S3 bucket
func newS3Syncer() (*s3Syncer, error) {
	hub, err := hub.New(hub.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &s3Syncer{
		hub: hub,
	}, nil
}

// Sync makes sure that local cache has all required dependencies for the
// selected runtime
func (s *s3Syncer) Sync(ctx context.Context, engine *Engine, manifest *schema.Manifest, runtimeVersion *semver.Version) error {
	tarball, err := s.hub.Get(loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       defaults.TelekubePackage,
		Version:    runtimeVersion.String(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer tarball.Close()
	// unpack the downloaded tarball
	unpackedDir, err := ioutil.TempDir("", "runtime-unpacked")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(unpackedDir)
	err = archive.Extract(tarball, unpackedDir)
	if err != nil {
		return trace.Wrap(err)
	}
	// sync packages between unpacked tarball directory and package cache
	env, err := localenv.New(unpackedDir)
	if err != nil {
		return trace.Wrap(err)
	}
	cacheApps, err := engine.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	tarballApps, err := env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	puller := libapp.Puller{
		FieldLogger: log,
		SrcPack:     env.Packages,
		SrcApp:      tarballApps,
		DstPack:     engine.Env.Packages,
		DstApp:      cacheApps,
		Parallel:    engine.Parallel,
		OnConflict:  libapp.GetDependencyConflictHandler(false),
	}
	return puller.PullAppDeps(ctx, libapp.Application{
		Package:  manifest.Locator(),
		Manifest: *manifest,
	})
}

// PackSyncer synchronizes local package cache with pack/apps services
type PackSyncer struct {
	pack pack.PackageService
	apps libapp.Applications
	repo string
}

// NewPackSyncer creates a new syncer from provided pack and apps services
func NewPackSyncer(pack pack.PackageService, apps libapp.Applications, repo string) *PackSyncer {
	return &PackSyncer{
		pack: pack,
		apps: apps,
		repo: repo,
	}
}

// Sync pulls dependencies from the package/app service not available locally
func (s *PackSyncer) Sync(ctx context.Context, engine *Engine, manifest *schema.Manifest, runtimeVersion *semver.Version) error {
	cacheApps, err := engine.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	puller := libapp.Puller{
		SrcPack:     s.pack,
		SrcApp:      s.apps,
		DstPack:     engine.Env.Packages,
		DstApp:      cacheApps,
		Parallel:    engine.Parallel,
		FieldLogger: log,
		OnConflict:  libapp.GetDependencyConflictHandler(false),
	}
	err = puller.PullAppDeps(ctx, libapp.Application{
		Package:  manifest.Locator(),
		Manifest: *manifest,
	})
	if err != nil {
		if utils.IsNetworkError(err) || trace.IsEOF(err) {
			return trace.ConnectionProblem(err, "failed to download "+
				"application dependencies from %v - please make sure the "+
				"repository is reachable: %v", s.repo, err)
		}
		return trace.Wrap(err)
	}
	return nil
}
