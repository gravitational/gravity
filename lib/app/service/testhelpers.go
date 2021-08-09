/*
Copyright 2021 Gravitational, Inc.

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
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"

	"github.com/mailgun/timetools"
	"gopkg.in/check.v1"
)

// TestServices groups services relevant in package/application tests
type TestServices struct {
	Backend  storage.Backend
	Packages pack.PackageService
	Apps     *Applications
}

// NewTestServices creates a new set of test services
func NewTestServices(dir string, c *check.C) TestServices {
	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path: filepath.Join(dir, "bolt.db"),
	})
	c.Assert(err, check.IsNil)

	objects, err := fs.New(dir)
	c.Assert(err, check.IsNil)

	packages, err := localpack.New(localpack.Config{
		Backend:     backend,
		UnpackedDir: filepath.Join(dir, defaults.UnpackedDir),
		Objects:     objects,
		Clock: &timetools.FreezedTime{
			CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
		},
		DownloadURL: "https://ops.example.com",
	})
	c.Assert(err, check.IsNil)

	charts, err := helm.NewRepository(helm.Config{
		Packages: packages,
		Backend:  backend,
	})
	c.Assert(err, check.IsNil)

	apps, err := New(Config{
		Backend:  backend,
		StateDir: filepath.Join(dir, defaults.ImportDir),
		Packages: packages,
		Charts:   charts,
	})
	c.Assert(err, check.IsNil)

	return TestServices{
		Backend:  backend,
		Packages: packages,
		Apps:     apps,
	}
}
