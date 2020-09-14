/*
Copyright 2019 Gravitational, Inc.

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
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	libapp "github.com/gravitational/gravity/lib/app"
	app "github.com/gravitational/gravity/lib/app/service"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/sirupsen/logrus"
	check "gopkg.in/check.v1"
)

func TestBuilder(t *testing.T) { check.TestingT(t) }

type BuilderSuite struct{}

var _ = check.Suite(&BuilderSuite{})

func (s *BuilderSuite) TestVersionsCompatibility(c *check.C) {
	testCases := []struct {
		teleVer    semver.Version
		runtimeVer semver.Version
		compatible bool
		comment    string
	}{
		{
			teleVer:    *semver.New("5.5.0"),
			runtimeVer: *semver.New("5.5.0"),
			compatible: true,
			comment:    "tele and runtime versions are the same",
		},
		{
			teleVer:    *semver.New("5.5.1"),
			runtimeVer: *semver.New("5.5.0"),
			compatible: true,
			comment:    "tele version is newer than runtime version",
		},
		{
			teleVer:    *semver.New("5.5.0"),
			runtimeVer: *semver.New("5.5.1"),
			compatible: false,
			comment:    "runtime version is newer than tele version",
		},
		{
			teleVer:    *semver.New("5.5.0"),
			runtimeVer: *semver.New("5.4.0"),
			compatible: false,
			comment:    "tele and runtime versions are different releases",
		},
	}
	for _, t := range testCases {
		c.Assert(versionsCompatible(t.teleVer, t.runtimeVer), check.Equals,
			t.compatible, check.Commentf(t.comment))
	}
}

func (s *BuilderSuite) TestSelectRuntimeVersion(c *check.C) {
	b := &Builder{
		Config: Config{
			FieldLogger: logrus.WithField(trace.Component, "test"),
			Progress:    utils.DiscardProgress,
		},
	}

	b.Manifest = schema.MustParseManifestYAML([]byte(manifestWithBase))
	ver, err := b.SelectRuntime()
	c.Assert(err, check.IsNil)
	c.Assert(ver, check.DeepEquals, semver.New("5.5.0"))

	b.Manifest = schema.MustParseManifestYAML([]byte(manifestWithoutBase))
	version.Init("5.4.2")
	ver, err = b.SelectRuntime()
	c.Assert(err, check.IsNil)
	c.Assert(ver, check.DeepEquals, semver.New("5.4.2"))

	b.Manifest = schema.MustParseManifestYAML([]byte(manifestInvalidBase))
	ver, err = b.SelectRuntime()
	c.Assert(err, check.FitsTypeOf, trace.BadParameter(""))
	c.Assert(err, check.ErrorMatches, "unsupported base image .*")
}

func (s *BuilderSuite) TestBuildInstallerWithDefaultPlanetPackage(c *check.C) {
	// setup
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	stateDir := c.MkDir()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)
	manifestBytes := []byte(`
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: app
  resourceVersion: "0.0.1"
installer:
  flavors:
    items:
      - name: "one"
        nodes:
          - profile: master
            count: 1
      - name: "three"
        nodes:
          - profile: master
            count: 1
          - profile: node
            count: 2
nodeProfiles:
  - name: master
    labels:
      node-role.kubernetes.io/master: "true"
  - name: node
    labels:
      node-role.kubernetes.io/node: "true"
systemOptions:
  #dependencies:
  #  runtimePackage: gravitational.io/planet:0.0.1
  runtime:
    version: 0.0.1
`)
	writeFile(manifestPath, manifestBytes, c)
	b, err := New(Config{
		FieldLogger:      logrus.WithField(trace.Component, "test"),
		Progress:         utils.DiscardProgress,
		StateDir:         stateDir,
		OutPath:          filepath.Join(stateDir, "app.tar"),
		ManifestPath:     manifestPath,
		SkipVersionCheck: true,
		GetRepository: func(*Builder) (r string, err error) {
			return "repository", nil
		},
		NewSyncer: func(*Builder) (Syncer, error) {
			return NewPackSyncer(remoteEnv.Packages, remoteEnv.Apps, "repository"), nil
		},
	})
	c.Assert(err, check.IsNil)

	createRuntimeApplication(remoteEnv, c)
	createApp(manifestBytes, remoteEnv.Apps, c)

	// verify
	err = Build(context.TODO(), b)
	c.Assert(err, check.IsNil)
}

func (s *BuilderSuite) TestBuildInstallerWithDefaultPlanetPackageFromHub(c *check.C) {
	// setup
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	stateDir := c.MkDir()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)
	manifestBytes := []byte(`
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: app
  resourceVersion: "0.0.1"
installer:
  flavors:
    items:
      - name: "one"
        nodes:
          - profile: master
            count: 1
      - name: "three"
        nodes:
          - profile: master
            count: 1
          - profile: node
            count: 2
nodeProfiles:
  - name: master
    labels:
      node-role.kubernetes.io/master: "true"
  - name: node
    labels:
      node-role.kubernetes.io/node: "true"
systemOptions:
  #dependencies:
  #  runtimePackage: gravitational.io/planet:0.0.1
  runtime:
    version: 0.0.1
`)
	writeFile(manifestPath, manifestBytes, c)
	b, err := New(Config{
		FieldLogger:      logrus.WithField(trace.Component, "test"),
		Progress:         utils.DiscardProgress,
		StateDir:         stateDir,
		OutPath:          filepath.Join(stateDir, "app.tar"),
		ManifestPath:     manifestPath,
		SkipVersionCheck: true,
		GetRepository: func(*Builder) (r string, err error) {
			return "repository", nil
		},
		NewSyncer: func(*Builder) (Syncer, error) {
			return NewPackSyncer(remoteEnv.Packages, newHubApps(remoteEnv.Apps), "repository"), nil
		},
	})
	c.Assert(err, check.IsNil)

	createRuntimeApplication(remoteEnv, c)
	createApp(manifestBytes, remoteEnv.Apps, c)

	// verify
	err = Build(context.TODO(), b)
	c.Assert(err, check.IsNil)
}

/*
// TODO(dmitri): enable this test
func (s *BuilderSuite) TestBuildInstallerWithCustomPlanetPackageFromHub(c *check.C) {
	// setup
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	stateDir := c.MkDir()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)
	manifestBytes := []byte(`
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: app
  resourceVersion: "0.0.1"
installer:
  flavors:
    items:
      - name: "one"
        nodes:
          - profile: master
            count: 1
      - name: "three"
        nodes:
          - profile: master
            count: 1
          - profile: node
            count: 2
nodeProfiles:
  - name: master
    labels:
      node-role.kubernetes.io/master: "true"
  - name: node
    labels:
      node-role.kubernetes.io/node: "true"
systemOptions:
  baseImage: quay.io/gravitational.io/planet:0.0.2
  runtime:
    version: 0.0.1
`)
	writeFile(manifestPath, manifestBytes, c)
	b, err := New(Config{
		FieldLogger:      logrus.WithField(trace.Component, "test"),
		Progress:         utils.DiscardProgress,
		StateDir:         stateDir,
		OutPath:          filepath.Join(stateDir, "app.tar"),
		ManifestPath:     manifestPath,
		SkipVersionCheck: true,
		VendorReq: app.VendorRequest{
			PackageName:      "app",
			PackageVersion:   "0.0.1",
			VendorRuntime:    true,
			ManifestPath:     manifestPath,
			ResourcePatterns: []string{defaults.VendorPattern},
		},
		GetRepository: func(*Builder) (r string, err error) {
			return "repository", nil
		},
		NewSyncer: func(*Builder) (Syncer, error) {
			return NewPackSyncer(remoteEnv.Packages, newHubApps(remoteEnv.Apps), "repository"), nil
		},
	})
	c.Assert(err, check.IsNil)

	createRuntimeApplication(remoteEnv, c)
	createApp(manifestBytes, remoteEnv.Apps, c)

	// verify
	err = Build(context.TODO(), b)
	c.Assert(err, check.IsNil)
}
*/

func newEnviron(c *check.C) *localenv.LocalEnvironment {
	stateDir := c.MkDir()
	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path: filepath.Join(stateDir, "bolt.db"),
	})
	c.Assert(err, check.IsNil)

	objects, err := fs.New(fs.Config{Path: stateDir})
	c.Assert(err, check.IsNil)

	pack, err := localpack.New(localpack.Config{
		Backend:     backend,
		UnpackedDir: filepath.Join(stateDir, defaults.UnpackedDir),
		Objects:     objects,
	})
	c.Assert(err, check.IsNil)

	apps, err := app.New(app.Config{
		StateDir: filepath.Join(stateDir, defaults.ImportDir),
		Backend:  backend,
		Packages: pack,
	})
	c.Assert(err, check.IsNil)

	return &localenv.LocalEnvironment{
		Backend:  backend,
		Objects:  objects,
		Packages: pack,
		Apps:     apps,
	}
}

func createRuntimeApplication(env *localenv.LocalEnvironment, c *check.C) {
	runtimePackage := loc.MustParseLocator("gravitational.io/planet:0.0.1")
	items := []*archive.Item{
		archive.ItemFromString("planet", "planet"),
	}
	apptest.CreatePackage(env.Packages, runtimePackage, items, c)
	gravityPackage := loc.MustParseLocator("gravitational.io/gravity:0.0.1")
	items = []*archive.Item{
		archive.ItemFromString("gravity", "gravity"),
	}
	apptest.CreatePackage(env.Packages, gravityPackage, items, c)
	manifestBytes := `apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: kubernetes
  resourceVersion: 0.0.1
dependencies:
  packages:
  - gravitational.io/planet:0.0.1
  - gravitational.io/gravity:0.0.1
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:0.0.1
`
	locator := loc.MustParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", defaults.SystemAccountOrg, defaults.Runtime))
	items = []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
	}
	apptest.CreateApplicationFromData(env.Apps, locator, items, c)
}

func createApp(manifestBytes []byte, apps libapp.Applications, c *check.C) *libapp.Application {
	manifest := schema.MustParseManifestYAML(manifestBytes)
	loc := loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       manifest.Metadata.Name,
		Version:    manifest.Metadata.ResourceVersion,
	}
	files := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", string(manifestBytes)),
	}
	return apptest.CreateApplicationFromData(apps, loc, files, c)
}

func writeFile(path string, contents []byte, c *check.C) {
	err := ioutil.WriteFile(path, contents, defaults.SharedReadWriteMask)
	c.Assert(err, check.IsNil)
}

func newHubApps(apps libapp.Applications) libapp.Applications {
	return hubApps{Applications: apps}
}

func (r hubApps) GetApp(loc loc.Locator) (*libapp.Application, error) {
	app, err := r.Applications.GetApp(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest := app.Manifest
	// Enterprise Hub runs an old version of gravity which strips down manifest details
	// it does not understand
	manifest.SystemOptions = nil
	app.Manifest = manifest
	return app, nil
}

type hubApps struct {
	libapp.Applications
}

const (
	manifestWithBase = `apiVersion: cluster.gravitational.io/v2
kind: Cluster
baseImage: gravity:5.5.0
metadata:
  name: test
  resourceVersion: 1.0.0`

	manifestWithoutBase = `apiVersion: cluster.gravitational.io/v2
kind: Cluster
metadata:
  name: test
  resourceVersion: 1.0.0`

	manifestInvalidBase = `apiVersion: cluster.gravitational.io/v2
kind: Cluster
baseImage: example:1.2.3
metadata:
  name: test
  resourceVersion: 1.0.0`
)
