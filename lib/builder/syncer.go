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
	"fmt"
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/hub"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/sirupsen/logrus"
)

// Syncer defines methods for synchronizing the local package cache
type Syncer interface {
	// Sync makes sure that local cache has all required dependencies for the
	// selected runtime
	Sync(*Builder, *semver.Version) error
	// SelectRuntime picks an appropriate runtime for the application that's
	// being built
	SelectRuntime(*Builder) (*semver.Version, error)
	// GetRepository returns the syncer repository
	GetRepository() string
}

// NewSyncerFunc defines function that creates syncer for a builder
type NewSyncerFunc func(*Builder) (Syncer, error)

// NewSyncer returns a new syncer instance for the provided builder
//
// Satisfies NewSyncerFunc type.
func NewSyncer(b *Builder) (Syncer, error) {
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

// GetRepository returns the name of S3 hub
func (s *s3Syncer) GetRepository() string {
	return fmt.Sprintf("s3://%v", defaults.HubBucket)
}

// SelectRuntime picks an appropriate runtime for the application that's
// being built
func (s *s3Syncer) SelectRuntime(builder *Builder) (*semver.Version, error) {
	// determine version of this binary
	teleVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine tele version")
	}
	// determine the latest runtime compatible with this tele
	releases, err := s.hub.List(true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var latest *semver.Version
	for _, release := range releases {
		ver, err := semver.NewVersion(release.Version)
		if err != nil {
			logrus.Warnf("Failed to parse release version: %v %v.", release, err)
			continue
		}
		if ver.Major != teleVersion.Major || ver.Minor != teleVersion.Minor {
			continue
		}
		if latest == nil || latest.LessThan(*ver) {
			latest = ver
		}
	}
	if latest == nil {
		return nil, trace.NotFound("could not find compatible runtime for "+
			"this tele version %v", teleVersion)
	}
	return latest, nil
}

// Sync makes sure that local cache has all required dependencies for the
// selected runtime
func (s *s3Syncer) Sync(builder *Builder, runtimeVersion *semver.Version) error {
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
	cacheApps, err := builder.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	tarballApps, err := env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	return service.PullAppDeps(service.AppPullRequest{
		FieldLogger: builder.FieldLogger,
		SrcPack:     env.Packages,
		SrcApp:      tarballApps,
		DstPack:     builder.Env.Packages,
		DstApp:      cacheApps,
		Parallel:    builder.VendorReq.Parallel,
	}, builder.Manifest)
}

// packSyncer synchronizes local package cache with pack/apps services
type packSyncer struct {
	pack pack.PackageService
	apps app.Applications
	repo string
}

// NewPackSyncer creates a new syncer from provided pack and apps services
func NewPackSyncer(pack pack.PackageService, apps app.Applications, repo string) *packSyncer {
	return &packSyncer{
		pack: pack,
		apps: apps,
		repo: repo,
	}
}

// GetRepository returns the syncer's repository address
func (s *packSyncer) GetRepository() string {
	return s.repo
}

// SelectRuntime picks an appropriate runtime for the application that's
// being built
func (s *packSyncer) SelectRuntime(builder *Builder) (*semver.Version, error) {
	// determine version of this binary
	teleVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine tele version")
	}
	// determine the latest runtime compatible with this tele
	runtime, err := pack.FindLatestCompatiblePackage(s.pack, loc.Runtime, *teleVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return runtime.SemVer()
}

// Sync makes sure that local cache has all required dependencies for the
// selected runtime
func (s *packSyncer) Sync(builder *Builder, runtimeVersion *semver.Version) error {
	cacheApps, err := builder.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	err = service.PullAppDeps(service.AppPullRequest{
		SrcPack:     s.pack,
		SrcApp:      s.apps,
		DstPack:     builder.Env.Packages,
		DstApp:      cacheApps,
		Parallel:    builder.VendorReq.Parallel,
		FieldLogger: builder.FieldLogger,
	}, builder.Manifest)
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
