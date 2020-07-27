package cli

import (
	"path/filepath"
	"testing"

	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage/keyval"

	"gopkg.in/check.v1"
)

func TestCLI(t *testing.T) { check.TestingT(t) }

type S struct{}

var _ = check.Suite(&S{})

func (s *S) TestGetsUpdateLatestPackage(c *check.C) {
	var args localenv.TarballEnvironmentArgs
	var emptyPackagePattern string

	// exercise
	loc, err := getUpdatePackage(args, emptyPackagePattern, clusterApp)
	c.Assert(err, check.IsNil)

	// verify
	c.Assert(*loc, check.DeepEquals, clusterApp.WithLiteralVersion("0.0.0+latest"))
}

func (s *S) TestGetsUpdatePackageByPattern(c *check.C) {
	var args localenv.TarballEnvironmentArgs
	updatePackagePattern := "app:2.0.2"

	// exercise
	loc, err := getUpdatePackage(args, updatePackagePattern, clusterApp)
	c.Assert(err, check.IsNil)

	// verify
	c.Assert(*loc, check.DeepEquals, clusterApp.WithLiteralVersion("2.0.2"))
}

func (s *S) TestGetsUpdatePackageFromTarballEnviron(c *check.C) {
	stateDir := c.MkDir()
	createTarballEnviron(stateDir, c)
	args := localenv.TarballEnvironmentArgs{
		StateDir: stateDir,
	}
	var emptyPackagePattern string

	// exercise
	loc, err := getUpdatePackage(args, emptyPackagePattern, clusterApp)
	c.Assert(err, check.IsNil)

	// verify
	c.Assert(*loc, check.DeepEquals, clusterApp.WithLiteralVersion("2.0.1"))
}

func createTarballEnviron(stateDir string, c *check.C) {
	backend, err := keyval.NewBolt(keyval.BoltConfig{Path: filepath.Join(stateDir, defaults.GravityDBFile)})
	c.Assert(err, check.IsNil)
	objects, err := fs.New(fs.Config{Path: filepath.Join(stateDir, defaults.PackagesDir)})
	c.Assert(err, check.IsNil)
	services := opsservice.SetupTestServicesInDirectory(stateDir, backend, objects, c)
	apptest.CreateRuntimeApplication(services.Apps, c)
	apptest.CreateDummyApplication(clusterAppUpdate, c, services.Apps)
	backend.Close()
}

var (
	clusterApp = loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       "app",
		Version:    "2.0.0",
	}

	clusterAppUpdate = loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       "app",
		Version:    "2.0.1",
	}
)
