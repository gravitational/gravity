package localpack

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/pack/suite"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"

	log "github.com/sirupsen/logrus"
	"github.com/mailgun/timetools"
	. "gopkg.in/check.v1"
)

func TestLocal(t *testing.T) { TestingT(t) }

type LocalSuite struct {
	server  *PackageServer
	objects blob.Objects
	backend storage.Backend
	suite   suite.PackageSuite
	dir     string
	clock   *timetools.FreezedTime
}

var _ = Suite(&LocalSuite{
	clock: &timetools.FreezedTime{
		CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
	},
})

func (s *LocalSuite) SetUpTest(c *C) {
	log.SetOutput(os.Stderr)
	s.dir = c.MkDir()

	var err error
	s.backend, err = keyval.NewBolt(keyval.BoltConfig{
		Path: filepath.Join(s.dir, "storage.db"),
	})
	c.Assert(err, IsNil)

	objects, err := fs.New(s.dir)
	c.Assert(err, IsNil)

	s.suite.S, err = New(Config{
		Backend:     s.backend,
		UnpackedDir: filepath.Join(s.dir, defaults.UnpackedDir),
		Clock:       s.clock,
		Objects:     objects,
	})
	c.Assert(err, IsNil)

	s.suite.O = objects
	s.suite.C = s.clock
}

func (s *LocalSuite) TearDownTest(c *C) {
	c.Assert(s.backend.Close(), IsNil)
}

func (s *LocalSuite) TestRepositoriesCRUD(c *C) {
	s.suite.RepositoriesCRUD(c)
}

func (s *LocalSuite) TestPackagesCRUD(c *C) {
	s.suite.PackagesCRUD(c)
}

func (s *LocalSuite) TestUpsertPackages(c *C) {
	s.suite.UpsertPackages(c)
}

func (s *LocalSuite) TestDeleteRepository(c *C) {
	s.suite.DeleteRepository(c)
}
