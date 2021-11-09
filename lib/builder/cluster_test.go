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

package builder

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	dockerarchive "github.com/docker/docker/pkg/archive"
	libapp "github.com/gravitational/gravity/lib/app"
	app "github.com/gravitational/gravity/lib/app/service"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"

	check "gopkg.in/check.v1"
)

type InstallerBuilderSuite struct{}

var _ = check.Suite(&InstallerBuilderSuite{})

func (s *InstallerBuilderSuite) TestBuildInstallerWithDefaultPlanetPackage(c *check.C) {
	if !checkDockerAvailable() {
		c.Skip("test requires docker")
	}

	var (
		manifestBytes = []byte(`
apiVersion: cluster.gravitational.io/v2
kind: Cluster
metadata:
  name: app
  resourceVersion: "0.0.1"
dependencies:
  apps:
    - gravitational.io/app-dependency:0.0.1
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
  runtime:
    version: 0.0.1
`)

		dependencyManifestBytes = []byte(`
apiVersion: bundle.gravitational.io/v2
kind: SystemApplication
metadata:
  name: app-dependency
  resourceVersion: "0.0.1"
`)
	)

	// setup
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	buildEnv := newEnviron(c)
	defer buildEnv.Close()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)

	writeFile(manifestPath, manifestBytes, c)
	b, err := NewClusterBuilder(Config{
		Progress:         utils.DiscardProgress,
		SkipVersionCheck: true,
		Repository:       "repository",
		NewSyncer:        newPackSyncer(remoteEnv.Packages, remoteEnv.Apps, "repository"),
		env:              buildEnv,
	})
	c.Assert(err, check.IsNil)

	createRuntimeApplication(remoteEnv, c)
	createApp(dependencyManifestBytes, remoteEnv.Apps, c)
	createApp(manifestBytes, remoteEnv.Apps, c)

	// verify
	err = b.Build(context.TODO(), ClusterRequest{
		OutputPath: filepath.Join(buildEnv.StateDir, "app.tar"),
		SourcePath: filepath.Dir(manifestPath),
	})
	c.Assert(err, check.IsNil)
}

func (s *InstallerBuilderSuite) TestBuildInstallerWithPackagesInCache(c *check.C) {
	var manifestBytes = []byte(`
apiVersion: cluster.gravitational.io/v2
kind: Cluster
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
  runtime:
    version: 0.0.2-dev.1
`)

	// setup
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	buildEnv := newEnviron(c)
	defer buildEnv.Close()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)

	writeFile(manifestPath, manifestBytes, c)
	b, err := NewClusterBuilder(Config{
		Progress:         utils.DiscardProgress,
		SkipVersionCheck: true,
		Repository:       "repository",
		NewSyncer:        newPackSyncer(remoteEnv.Packages, remoteEnv.Apps, "repository"),
		env:              buildEnv,
	})
	c.Assert(err, check.IsNil)

	// Simulate a development workflow with packages/applications
	// explicitly cached (but also unavailable in the remote hub)
	createRuntimeApplicationWithVersion(buildEnv, "0.0.2-dev.1", c)
	createApp(manifestBytes, buildEnv.Apps, c)

	// verify
	err = b.Build(context.TODO(), ClusterRequest{
		OutputPath: filepath.Join(buildEnv.StateDir, "app.tar"),
		SourcePath: filepath.Dir(manifestPath),
	})
	c.Assert(err, check.IsNil)
}

func (s *InstallerBuilderSuite) TestBuildInstallerWithMixedVersionPackages(c *check.C) {
	var manifestBytes = []byte(`
apiVersion: cluster.gravitational.io/v2
kind: Cluster
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
  runtime:
    version: 0.0.0+latest # use meta version as a placeholder
`)

	// setup
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	buildEnv := newEnviron(c)
	defer buildEnv.Close()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)

	version.Init("0.0.1")
	writeFile(manifestPath, manifestBytes, c)
	outputPath := filepath.Join(buildEnv.StateDir, "app.tar")
	b, err := NewClusterBuilder(Config{
		Progress:         utils.DiscardProgress,
		SkipVersionCheck: true,
		Repository:       "repository",
		NewSyncer:        newPackSyncer(remoteEnv.Packages, remoteEnv.Apps, "repository"),
		env:              buildEnv,
	})
	c.Assert(err, check.IsNil)

	// Simulate a workflow with packages/applications of mixed versions
	// and at least one version available above the requested runtime version
	createRuntimeApplicationWithVersion(buildEnv, "0.0.1", c)
	createRuntimeApplicationWithVersion(buildEnv, "0.0.2", c)
	createApp(manifestBytes, buildEnv.Apps, c)

	// verify
	err = b.Build(context.TODO(), ClusterRequest{
		OutputPath: outputPath,
		SourcePath: filepath.Dir(manifestPath),
	})
	c.Assert(err, check.IsNil)

	unpackDir := c.MkDir()
	tarballEnv := unpackTarball(outputPath, unpackDir, c)
	defer tarballEnv.Close()

	verifyPackages(tarballEnv.Packages, []string{
		"gravitational.io/planet:0.0.1",
		"gravitational.io/app:0.0.1",
		"gravitational.io/gravity:0.0.1",
		"gravitational.io/kubernetes:0.0.1",
	}, c)
}

func (s *InstallerBuilderSuite) TestBuildInstallerWithDefaultPlanetPackageFromLegacyHub(c *check.C) {
	// setup
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	buildEnv := newEnviron(c)
	defer buildEnv.Close()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)
	manifestBytes := []byte(`
apiVersion: cluster.gravitational.io/v2
kind: Cluster
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
	b, err := NewClusterBuilder(Config{
		Progress:         utils.DiscardProgress,
		SkipVersionCheck: true,
		Repository:       "repository",
		NewSyncer:        newPackSyncer(remoteEnv.Packages, newHubApps(remoteEnv.Apps), "repository"),
		env:              buildEnv,
	})
	c.Assert(err, check.IsNil)

	createRuntimeApplication(remoteEnv, c)
	createApp(manifestBytes, remoteEnv.Apps, c)

	// verify
	err = b.Build(context.TODO(), ClusterRequest{
		OutputPath: filepath.Join(buildEnv.StateDir, "app.tar"),
		SourcePath: filepath.Dir(manifestPath),
	})
	c.Assert(err, check.IsNil)
}

type CustomImageBuilderSuite struct{}

var _ = check.Suite(&CustomImageBuilderSuite{})

func (s *CustomImageBuilderSuite) SetUpTest(c *check.C) {
	if !checkDockerAvailable() {
		c.Skip("test requires docker")
	}
	dockerDir := c.MkDir()
	createPlanetDockerImage(dockerDir, planetTag, c)
}

func (s *CustomImageBuilderSuite) TearDownTest(c *check.C) {
	removePlanetDockerImage(c)
}

func (s *CustomImageBuilderSuite) TestBuildInstallerWithCustomGlobalPlanetPackage(c *check.C) {
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	buildEnv := newEnviron(c)
	defer buildEnv.Close()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)
	manifestBytes := []byte(`
apiVersion: cluster.gravitational.io/v2
kind: Cluster
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
  baseImage: quay.io/gravitational/planet:0.0.2
  runtime:
    version: 0.0.1
`)
	writeFile(manifestPath, manifestBytes, c)
	outputPath := filepath.Join(buildEnv.StateDir, "app.tar")
	b, err := NewClusterBuilder(Config{
		Progress:         utils.DiscardProgress,
		SkipVersionCheck: true,
		Repository:       "repository",
		NewSyncer:        newPackSyncer(remoteEnv.Packages, newHubApps(remoteEnv.Apps), "repository"),
		env:              buildEnv,
	})
	c.Assert(err, check.IsNil)

	createRuntimeApplication(remoteEnv, c)
	createApp(manifestBytes, remoteEnv.Apps, c)

	// verify
	err = b.Build(context.TODO(), ClusterRequest{
		OutputPath: outputPath,
		SourcePath: filepath.Dir(manifestPath),
		Vendor: app.VendorRequest{
			PackageName:      "app",
			PackageVersion:   "0.0.1",
			VendorRuntime:    true,
			ManifestPath:     manifestPath,
			ResourcePatterns: []string{defaults.VendorPattern},
		},
	})
	c.Assert(err, check.IsNil)

	unpackDir := c.MkDir()
	tarballEnv := unpackTarball(outputPath, unpackDir, c)
	defer tarballEnv.Close()

	verifyPackages(tarballEnv.Packages, []string{
		"gravitational.io/planet:0.0.2",
		"gravitational.io/app:0.0.1",
		"gravitational.io/gravity:0.0.1",
		"gravitational.io/kubernetes:0.0.1",
	}, c)
}

func (s *CustomImageBuilderSuite) TestBuildInstallerWithCustomPerNodePlanetPackage(c *check.C) {
	remoteEnv := newEnviron(c)
	defer remoteEnv.Close()
	buildEnv := newEnviron(c)
	defer buildEnv.Close()
	appDir := c.MkDir()
	manifestPath := filepath.Join(appDir, defaults.ManifestFileName)
	manifestBytes := []byte(`
apiVersion: cluster.gravitational.io/v2
kind: Cluster
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
    systemOptions:
      baseImage: quay.io/gravitational/planet:0.0.2
    labels:
      node-role.kubernetes.io/node: "true"
systemOptions:
  runtime:
    version: 0.0.1
`)
	writeFile(manifestPath, manifestBytes, c)
	outputPath := filepath.Join(buildEnv.StateDir, "app.tar")
	b, err := NewClusterBuilder(Config{
		Progress:         utils.DiscardProgress,
		SkipVersionCheck: true,
		Repository:       "repository",
		NewSyncer:        newPackSyncer(remoteEnv.Packages, newHubApps(remoteEnv.Apps), "repository"),
		env:              buildEnv,
	})
	c.Assert(err, check.IsNil)

	createRuntimeApplication(remoteEnv, c)
	createApp(manifestBytes, remoteEnv.Apps, c)

	// verify
	err = b.Build(context.TODO(), ClusterRequest{
		OutputPath: outputPath,
		SourcePath: filepath.Dir(manifestPath),
		Vendor: app.VendorRequest{
			PackageName:      "app",
			PackageVersion:   "0.0.1",
			VendorRuntime:    true,
			ManifestPath:     manifestPath,
			ResourcePatterns: []string{defaults.VendorPattern},
		},
	})
	c.Assert(err, check.IsNil)

	unpackDir := c.MkDir()
	tarballEnv := unpackTarball(outputPath, unpackDir, c)
	defer tarballEnv.Close()

	verifyPackages(tarballEnv.Packages, []string{
		// Per-node custom planet configuration adds to the list of planet packages
		"gravitational.io/quay.io-gravitational-planet:0.0.2",
		"gravitational.io/planet:0.0.1",
		"gravitational.io/app:0.0.1",
		"gravitational.io/gravity:0.0.1",
		"gravitational.io/kubernetes:0.0.1",
	}, c)
}

func newPackSyncer(packages pack.PackageService, apps libapp.Applications, repository string) NewSyncerFunc {
	return func(*Engine) (Syncer, error) {
		s := NewPackSyncer(packages, apps, repository)
		return s, nil
	}
}

func newEnviron(c *check.C) *localenv.LocalEnvironment {
	env, err := localenv.New(c.MkDir())
	c.Assert(err, check.IsNil)
	c.Assert(env.Packages.UpsertRepository(defaults.SystemAccountOrg, time.Time{}), check.IsNil)
	return env
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

func createRuntimeApplication(env *localenv.LocalEnvironment, c *check.C) {
	createRuntimeApplicationWithVersion(env, "0.0.1", c)
}

func createRuntimeApplicationWithVersion(env *localenv.LocalEnvironment, version string, c *check.C) {
	runtimePackage := loc.Planet.WithLiteralVersion(version)
	gravityPackage := loc.Gravity.WithLiteralVersion(version)
	items := []*archive.Item{
		archive.ItemFromString("planet", "planet"),
	}
	apptest.CreatePackage(env.Packages, runtimePackage, items, c)
	items = []*archive.Item{
		archive.ItemFromString("gravity", "gravity"),
	}
	apptest.CreatePackage(env.Packages, gravityPackage, items, c)
	manifestBytes := fmt.Sprintf(`apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: kubernetes
  resourceVersion: %[1]v
dependencies:
  packages:
  - gravitational.io/planet:%[1]v
  - gravitational.io/gravity:%[1]v
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:%[1]v
`, version)
	runtimeAppLoc := loc.Runtime.WithLiteralVersion(version)
	items = []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
	}
	apptest.CreateApplicationFromData(env.Apps, runtimeAppLoc, items, c)
}

func verifyPackages(packages pack.PackageService, expected []string, c *check.C) {
	var obtained sort.StringSlice
	pack.ForeachPackage(packages, func(e pack.PackageEnvelope) error {
		obtained = append(obtained, e.Locator.String())
		return nil
	})
	c.Assert(obtained, compare.SortedSliceEquals, sort.StringSlice(expected))
}

func unpackTarball(path, unpackedDir string, c *check.C) *localenv.LocalEnvironment {
	f, err := os.Open(path)
	c.Assert(err, check.IsNil)
	defer f.Close()
	err = dockerarchive.Untar(f, unpackedDir, archive.DefaultOptions())
	c.Assert(err, check.IsNil)
	env, err := localenv.New(unpackedDir)
	c.Assert(err, check.IsNil)
	return env
}

func writeFile(path string, contents []byte, c *check.C) {
	err := ioutil.WriteFile(path, contents, defaults.SharedReadWriteMask)
	c.Assert(err, check.IsNil)
}

func checkDockerAvailable() bool {
	_, err := docker.NewClientFromEnv()
	return err == nil
}

func createPlanetDockerImage(dir, tag string, c *check.C) {
	dockerfileBytes := []byte(`
FROM scratch
COPY ./orbit.manifest.json /etc/planet/
`)
	planetManifestBytes := []byte(`{}`)

	writeFile(filepath.Join(dir, "Dockerfile"), dockerfileBytes, c)
	writeFile(filepath.Join(dir, "orbit.manifest.json"), planetManifestBytes, c)
	buildDockerImage(dir, tag, c)
}

func removePlanetDockerImage(c *check.C) {
	removeDockerImage(planetTag)
	removeDockerImage(planetPlainTag)
}

func buildDockerImage(dir, tag string, c *check.C) {
	out, err := exec.Command("docker", "build", "-t", tag, dir).CombinedOutput()
	c.Assert(err, check.IsNil, check.Commentf(string(out)))
}

func removeDockerImage(tag string) {
	exec.Command("docker", "rmi", tag).Run()
}

func newHubApps(apps libapp.Applications) libapp.Applications {
	return legacyHubApps{Applications: apps}
}

func (r legacyHubApps) GetApp(loc loc.Locator) (*libapp.Application, error) {
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

// legacyHubApps implements the libapp.Applications interface but replaces
// the GetApp API to mimic the behavior of the legacy enterprise hub - namely,
// that it does not understand the recent versions of the manifest and strips
// away SystemOptions which is used to detect the planet package
type legacyHubApps struct {
	libapp.Applications
}

const (
	planetTag      = "quay.io/gravitational/planet:0.0.2"
	planetPlainTag = "gravitational/planet:0.0.2"
)
