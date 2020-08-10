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

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

// Syncer synchronizes the local package cache from a (remote) repository
type Syncer interface {
	// Sync makes sure that local cache has all required dependencies for the
	// selected runtime
	Sync(ctx context.Context, builder *Builder, runtimeVersion semver.Version) error
}

// s3Syncer synchronizes local package cache with S3 bucket
type S3Syncer struct {
	// hub provides access to runtimes stored in S3 bucket
	hub hub.Hub
}

// NewS3Syncer returns a syncer that syncs packages from an S3 bucket
func NewS3Syncer() (*S3Syncer, error) {
	hub, err := hub.New(hub.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &S3Syncer{
		hub: hub,
	}, nil
}

// Sync makes sure that local cache has all required dependencies for the
// selected runtime
func (s *S3Syncer) Sync(ctx context.Context, builder *Builder, runtimeVersion semver.Version) error {
	var application = loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       "telekube",
		Version:    runtimeVersion.String(),
	}

	unpackedDir, err := ioutil.TempDir("", "runtime-unpacked")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(unpackedDir)

	err = s.download(unpackedDir, application)
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

	cacheApps, err := builder.Env.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	// sync packages between unpacked tarball directory and package cache
	app, err := tarballEnv.Apps.GetApp(RuntimeApp(runtimeVersion))
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
	err = puller.PullAppDeps(ctx, *app)
	if err != nil {
		return trace.Wrap(err)
	}
	return puller.PullAppPackage(ctx, app.Package)
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
