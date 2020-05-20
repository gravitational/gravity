/*
Copyright 2020 Gravitational, Inc.

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

package service

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/layerpack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/storage/keyval"

	"github.com/gravitational/trace"
)

// LayeredApps contains layered package and application services.
type LayeredApps struct {
	// Package is a layered package service.
	Packages pack.PackageService
	// Apps is app service based on the layered package service.
	Apps app.Applications
	// dir is the directory with the new write layer data.
	dir string
}

// Cleanup removes the write layer directory.
func (l LayeredApps) Cleanup() error {
	if l.dir != "" {
		return os.RemoveAll(l.dir)
	}
	return nil
}

// NewLayeredApps returns layered package and application services where
// the provided package service serves as a read-only layer.
func NewLayeredApps(readPackages pack.PackageService) (*LayeredApps, error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path:  filepath.Join(dir, defaults.GravityDBFile),
		Multi: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	objects, err := fs.New(filepath.Join(dir, defaults.PackagesDir))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	packages, err := localpack.New(localpack.Config{
		Backend:     backend,
		UnpackedDir: filepath.Join(dir, defaults.PackagesDir, defaults.UnpackedDir),
		Objects:     objects,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	layeredPackages := layerpack.New(readPackages, packages)
	layeredApps, err := New(Config{
		StateDir:    filepath.Join(dir, defaults.ImportDir),
		Backend:     backend,
		Packages:    layeredPackages,
		UnpackedDir: filepath.Join(dir, defaults.PackagesDir, defaults.UnpackedDir),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &LayeredApps{
		Packages: layeredPackages,
		Apps:     layeredApps,
		dir:      dir,
	}, nil
}
