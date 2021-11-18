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
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/app"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

type PullerSuite struct {
	srcPack pack.PackageService
	dstPack pack.PackageService
	srcApp  app.Applications
	dstApp  app.Applications
}

var _ = Suite(&PullerSuite{})

func (s *PullerSuite) SetUpTest(c *C) {
	_, s.srcPack, s.srcApp = setupServices(c)
	_, s.dstPack, s.dstApp = setupServices(c)
	err := s.srcPack.UpsertRepository("example.com", time.Time{})
	c.Assert(err, IsNil)
	err = s.dstPack.UpsertRepository("example.com", time.Time{})
	c.Assert(err, IsNil)
}

func (s *PullerSuite) TestPullPackage(c *C) {
	loc := loc.MustParseLocator("example.com/package:0.0.1")
	logger := log.WithField("test", "PullPackage")

	_, err := s.srcPack.CreatePackage(loc, bytes.NewBuffer([]byte("data")))
	c.Assert(err, IsNil)

	puller := app.Puller{
		FieldLogger: logger,
		SrcPack:     s.srcPack,
		DstPack:     s.dstPack,
	}
	err = puller.PullPackage(context.TODO(), loc)
	c.Assert(err, IsNil)

	env, err := s.dstPack.ReadPackageEnvelope(loc)
	c.Assert(err, IsNil)
	c.Assert(env.Locator, Equals, loc)

	puller = app.Puller{
		FieldLogger: logger,
		SrcPack:     s.srcPack,
		DstPack:     s.dstPack,
	}
	err = puller.PullPackage(context.TODO(), loc)
	c.Assert(trace.IsAlreadyExists(err), Equals, true)
}

func (s *PullerSuite) TestPullApp(c *C) {
	s.pullApp(c, 0)
}

func (s *PullerSuite) TestPullAppInParallel(c *C) {
	s.pullApp(c, 2)
}

func (s *PullerSuite) pullApp(c *C, parallel int) {
	packageBytes := bytes.NewReader([]byte(nil))
	for _, loc := range []loc.Locator{loc.MustParseLocator("example.com/existing:0.0.1")} {
		_, err := s.dstPack.CreatePackage(loc, packageBytes)
		c.Assert(err, IsNil)
	}
	for _, loc := range []loc.Locator{
		loc.MustParseLocator("example.com/new:0.0.1"),
		loc.MustParseLocator("example.com/new:0.0.2"),
		loc.MustParseLocator("example.com/existing:0.0.1"),
	} {
		_, err := s.srcPack.CreatePackage(loc, packageBytes)
		c.Assert(err, IsNil)
	}

	runtimePackage := loc.MustParseLocator("gravitational.io/planet:0.0.1")
	apptest.CreatePackage(s.srcPack, runtimePackage, nil, c)
	apptest.CreateRuntimeApplication(s.srcApp, c)

	locator := loc.MustParseLocator("example.com/app:0.0.2")
	const dependencies = `
dependencies:
  packages:
  - example.com/new:0.0.1
  - example.com/new:0.0.2
  - example.com/existing:0.0.1
`
	apptest.CreateDummyApplicationWithDependencies(s.srcApp, locator, dependencies, c)

	puller := app.Puller{
		SrcPack:  s.srcPack,
		DstPack:  s.dstPack,
		SrcApp:   s.srcApp,
		DstApp:   s.dstApp,
		Upsert:   true,
		Parallel: parallel,
	}
	err := puller.PullApp(context.TODO(), locator)
	c.Assert(err, IsNil)

	packages, err := s.dstPack.GetPackages("example.com")
	c.Assert(err, IsNil)
	c.Assert(packagesByName(locators(packages)), compare.SortedSliceEquals, packagesByName([]loc.Locator{
		loc.MustParseLocator("example.com/app:0.0.2"),
		loc.MustParseLocator("example.com/existing:0.0.1"),
		loc.MustParseLocator("example.com/new:0.0.1"),
		loc.MustParseLocator("example.com/new:0.0.2"),
	}))

	local, err := s.dstApp.GetApp(locator)
	c.Assert(err, IsNil)
	c.Assert(local.Package, Equals, locator)

	puller = app.Puller{
		SrcPack:  s.srcPack,
		DstPack:  s.dstPack,
		SrcApp:   s.srcApp,
		DstApp:   s.dstApp,
		Parallel: parallel,
	}
	err = puller.PullApp(context.TODO(), locator)
	c.Assert(trace.IsAlreadyExists(err), Equals, true)
}

func setupServices(c *C) (storage.Backend, pack.PackageService, *applications) {
	dir := c.MkDir()

	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path: filepath.Join(dir, "bolt.db"),
	})
	c.Assert(err, IsNil)

	objects, err := fs.New(dir)
	c.Assert(err, IsNil)

	packService, err := localpack.New(localpack.Config{
		Backend:     backend,
		UnpackedDir: filepath.Join(dir, defaults.UnpackedDir),
		Objects:     objects,
	})
	c.Assert(err, IsNil)

	charts, err := helm.NewRepository(helm.Config{
		Packages: packService,
		Backend:  backend,
	})
	c.Assert(err, IsNil)

	appService, err := New(Config{
		Backend:  backend,
		StateDir: filepath.Join(dir, defaults.ImportDir),
		Packages: packService,
		Charts:   charts,
	})
	c.Assert(err, IsNil)

	return backend, packService, appService
}

func locators(envelopes []pack.PackageEnvelope) []loc.Locator {
	out := make([]loc.Locator, 0, len(envelopes))
	for _, env := range envelopes {
		out = append(out, env.Locator)
	}
	return out
}

type packagesByName []loc.Locator

func (r packagesByName) Len() int           { return len(r) }
func (r packagesByName) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r packagesByName) Less(i, j int) bool { return r[i].String() < r[j].String() }
