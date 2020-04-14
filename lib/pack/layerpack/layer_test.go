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

package layerpack

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/pack/suite"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"

	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestLayer(t *testing.T) { TestingT(t) }

type LayerSuite struct {
	server       *Layer
	innerBackend storage.Backend
	outerBackend storage.Backend
	suite        suite.PackageSuite
	clock        *timetools.FreezedTime
}

var _ = Suite(&LayerSuite{
	clock: &timetools.FreezedTime{
		CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
	},
})

func (s *LayerSuite) SetUpTest(c *C) {
	log.SetOutput(os.Stderr)
	innerDir, outerDir := c.MkDir(), c.MkDir()

	var err error
	s.innerBackend, err = keyval.NewBolt(keyval.BoltConfig{Path: filepath.Join(innerDir, "storage.db")})
	c.Assert(err, IsNil)
	s.outerBackend, err = keyval.NewBolt(keyval.BoltConfig{Path: filepath.Join(outerDir, "storage.db")})
	c.Assert(err, IsNil)

	innerObjects, err := fs.New(fs.Config{Path: innerDir})
	c.Assert(err, IsNil)
	outerObjects, err := fs.New(fs.Config{Path: outerDir})
	c.Assert(err, IsNil)

	inner, err := localpack.New(localpack.Config{
		Backend:     s.innerBackend,
		UnpackedDir: filepath.Join(innerDir, defaults.UnpackedDir),
		Clock:       s.clock,
		Objects:     innerObjects,
	})
	c.Assert(err, IsNil)

	outer, err := localpack.New(localpack.Config{
		Backend:     s.outerBackend,
		UnpackedDir: filepath.Join(outerDir, defaults.UnpackedDir),
		Clock:       s.clock,
		Objects:     outerObjects,
	})
	c.Assert(err, IsNil)

	s.server = New(inner, outer)

	s.suite.S = s.server
	c.Assert(err, IsNil)

	s.suite.O = outerObjects
	s.suite.C = s.clock
}

func (s *LayerSuite) TearDownTest(c *C) {
	s.innerBackend.Close()
	s.outerBackend.Close()
}

func (s *LayerSuite) TestRepositoriesCRUD(c *C) {
	s.suite.RepositoriesCRUD(c)
}

func (s *LayerSuite) TestPackagesCRUD(c *C) {
	s.suite.PackagesCRUD(c)
}

func (s *LayerSuite) TestUpsertPackages(c *C) {
	s.suite.UpsertPackages(c)
}

func (s *LayerSuite) TestDeleteRepository(c *C) {
	s.suite.DeleteRepository(c)
}

func (s *LayerSuite) TestLayers(c *C) {
	// create one package in the inner layer
	c.Assert(s.server.inner.UpsertRepository("inner.example.com", time.Time{}), IsNil)

	innerData := []byte("hello, world!")
	innerLoc := loc.MustParseLocator("inner.example.com/inner-1:0.0.1")

	innerPack, err := s.server.inner.CreatePackage(innerLoc, bytes.NewBuffer(innerData))
	c.Assert(err, IsNil)
	c.Assert(innerPack, NotNil)

	// read will succeed, as inner server has the package
	opack1, readclose, err := s.server.ReadPackage(innerLoc)
	c.Assert(err, IsNil)
	c.Assert(opack1, DeepEquals, innerPack)
	out, err := ioutil.ReadAll(readclose)
	c.Assert(err, IsNil)
	c.Assert(string(out), DeepEquals, string(innerData))

	c.Assert(s.server.UpsertRepository("outer.example.com", time.Time{}), IsNil)

	outerData := []byte("hello, outer world!")
	outerLoc := loc.MustParseLocator("outer.example.com/outer-1:0.0.1")

	outerPack, err := s.server.CreatePackage(outerLoc, bytes.NewBuffer(outerData))
	c.Assert(err, IsNil)
	c.Assert(outerPack, NotNil)

	// make sure repository and package were created in the outer package
	_, _, err = s.server.inner.ReadPackage(outerLoc)
	c.Assert(err, NotNil)

	// read will succeed, as inner server has the package
	opack2, readclose, err := s.server.outer.ReadPackage(outerLoc)
	c.Assert(err, IsNil)
	c.Assert(opack2, DeepEquals, outerPack)
	out, err = ioutil.ReadAll(readclose)
	c.Assert(err, IsNil)
	c.Assert(string(out), DeepEquals, string(outerData))

	// list repositories will feature both inner and outer repositories
	repos, err := s.server.GetRepositories()
	c.Assert(err, IsNil)
	suite.CompareAsSets(c, []string{"inner.example.com", "outer.example.com"}, repos)

	// inner/outer repositories should be deduplicated
	s.server.UpsertRepository("inner.example.com", time.Time{})
	repos, err = s.server.GetRepositories()
	c.Assert(err, IsNil)
	suite.CompareAsSets(c, []string{"inner.example.com", "outer.example.com"}, repos)
}
