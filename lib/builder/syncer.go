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
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/hub"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"

	"github.com/gravitational/trace"
)

// Syncer defines a method for synchronizing the local package cache
type Syncer interface {
	// Sync makes sure that local cache has all required dependencies for the
	// selected runtime
	Sync(*Builder) error
}

type syncer struct {
	// hub provides access to runtimes stored in S3 bucket
	hub hub.Hub
}

// newSyncer returns a new syncer instance
func newSyncer() (*syncer, error) {
	hub, err := hub.New(hub.Config{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &syncer{
		hub: hub,
	}, nil
}

// Sync makes sure that local cache has all required dependencies for the
// selected runtime
func (s *syncer) Sync(builder *Builder) error {
	// download runtime tarball of the required version
	runtimeLocator := builder.Manifest.Base()
	if runtimeLocator == nil {
		return trace.NotFound("failed to determine application runtime")
	}
	tarball, err := s.hub.Get(loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       defaults.TelekubePackage,
		Version:    runtimeLocator.Version,
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
