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

package cluster

import (
	"github.com/gravitational/gravity/lib/app"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"

	teleservices "github.com/gravitational/teleport/lib/services"
	"gopkg.in/check.v1"
)

type PlanSuite struct{}

var _ = check.Suite(&PlanSuite{})

func (s *PlanSuite) TestPlanWithRuntimeUpdate(c *check.C) {
	// setup
	runtimeLoc1 := loc.MustParseLocator("gravitational.io/runtime:1.0.0")
	appLoc1 := loc.MustParseLocator("gravitational.io/app:1.0.0")
	runtimeLoc2 := loc.MustParseLocator("gravitational.io/runtime:2.0.0")
	appLoc2 := loc.MustParseLocator("gravitational.io/app:2.0.0")

	params := newTestPlan(c, params{
		installedRuntime:         runtimeLoc1,
		installedApp:             appLoc1,
		updateRuntime:            runtimeLoc2,
		updateApp:                appLoc2,
		installedRuntimeManifest: installedRuntimeManifest,
		installedAppManifest:     installedAppManifest,
		updateRuntimeManifest:    updateRuntimeManifest,
		updateAppManifest:        updateAppManifest,
		updateCoreDNS:            true,
		links: []storage.OpsCenterLink{
			{
				Hostname:   "ops.example.com",
				Type:       storage.OpsCenterRemoteAccessLink,
				RemoteAddr: "ops.example.com:3024",
				APIURL:     "https://ops.example.com:32009",
				Enabled:    true,
			},
		},
	})
	plan := params.plan

	leadMaster := params.servers[0]
	builder := phaseBuilder{planConfig: params}
	init := *builder.init(leadMaster.Server)
	checks := *builder.checks().Require(init)
	preUpdate := *builder.preUpdate().Require(init)
	bootstrap := *builder.bootstrap().Require(init)
	coreDNS := *builder.corednsPhase(leadMaster.Server)
	masters := *builder.masters(leadMaster, params.servers[1:2], false).Require(checks, bootstrap, preUpdate, coreDNS)
	nodes := *builder.nodes(leadMaster, params.servers[2:], false).Require(masters)
	etcd := *builder.etcdPlan(leadMaster.Server, plan.Servers[1:2], plan.Servers[2:], "1.0.0", "2.0.0")
	migration := builder.migration(leadMaster.Server).Require(etcd)
	c.Assert(migration, check.NotNil)
	config := *builder.config(servers(params.servers[:2]...)).Require(masters)

	runtimeLocs := []loc.Locator{
		loc.MustParseLocator("gravitational.io/runtime-dep-2:2.0.0"),
		loc.MustParseLocator("gravitational.io/rbac-app:2.0.0"),
		runtimeLoc2,
	}
	runtime := *builder.runtime(runtimeLocs).Require(masters)

	appLocs := []loc.Locator{loc.MustParseLocator("gravitational.io/app-dep-2:2.0.0"), appLoc2}
	app := *builder.app(appLocs).Require(runtime)
	cleanup := *builder.cleanup().Require(app)

	plan.Phases = update.Phases{
		init,
		checks,
		preUpdate,
		coreDNS,
		bootstrap,
		masters,
		nodes,
		etcd,
		*migration,
		config,
		runtime,
		app,
		cleanup,
	}.AsPhases()
	update.ResolvePlan(&plan)

	// exercise
	obtainedPlan, err := newOperationPlan(params)
	c.Assert(err, check.IsNil)
	// Reset the capacity so the plans can be compared
	obtainedPlan.Phases = resetCap(obtainedPlan.Phases)
	update.ResolvePlan(obtainedPlan)

	// verify
	c.Assert(*obtainedPlan, compare.DeepEquals, plan)
}

func (s *PlanSuite) TestPlanWithoutRuntimeUpdate(c *check.C) {
	// setup
	runtimeLoc1 := loc.MustParseLocator("gravitational.io/runtime:1.0.0")
	appLoc1 := loc.MustParseLocator("gravitational.io/app:1.0.0")
	appLoc2 := loc.MustParseLocator("gravitational.io/app:2.0.0")

	params := newTestPlan(c, params{
		installedRuntime:         runtimeLoc1,
		installedApp:             appLoc1,
		updateRuntime:            runtimeLoc1, // same runtime on purpose
		updateApp:                appLoc2,
		installedRuntimeManifest: installedRuntimeManifest,
		installedAppManifest:     installedAppManifest,
		updateRuntimeManifest:    installedRuntimeManifest, // same manifest on purpose
		updateAppManifest:        updateAppManifest,
	})
	plan := params.plan

	leadMaster := params.servers[0]
	builder := phaseBuilder{planConfig: params}
	init := *builder.init(leadMaster.Server)
	checks := *builder.checks().Require(init)
	preUpdate := *builder.preUpdate().Require(init)
	appLocs := []loc.Locator{loc.MustParseLocator("gravitational.io/app-dep-2:2.0.0"), appLoc2}
	app := *builder.app(appLocs).Require(preUpdate)
	cleanup := *builder.cleanup().Require(app)

	plan.Phases = update.Phases{init, checks, preUpdate, app, cleanup}.AsPhases()
	update.ResolvePlan(&plan)

	// exercise
	obtainedPlan, err := newOperationPlan(params)
	c.Assert(err, check.IsNil)
	// Reset the capacity so the plans can be compared
	obtainedPlan.Phases = resetCap(obtainedPlan.Phases)
	update.ResolvePlan(obtainedPlan)

	// verify
	c.Assert(*obtainedPlan, compare.DeepEquals, plan)
}

func (s *PlanSuite) TestUpdatesEtcdFromManifestWithoutLabels(c *check.C) {
	services := opsservice.SetupTestServices(c)
	files := []*archive.Item{
		archive.ItemFromString("orbit.manifest.json", `{"version": "0.0.1"}`),
	}
	runtimePackage := loc.MustParseLocator("example.com/runtime:1.0.0")
	apptest.CreateDummyPackageWithContents(
		runtimePackage,
		files,
		services.Packages, c)
	files = []*archive.Item{
		archive.ItemFromString("orbit.manifest.json", `{
	"version": "0.0.1",
	"labels": [
		{
			"name": "version-etcd",
			"value": "v3.3.3"
		}
	]
}`),
	}
	updateRuntimePackage := loc.MustParseLocator("example.com/runtime:1.0.1")
	apptest.CreateDummyPackageWithContents(
		updateRuntimePackage,
		files,
		services.Packages, c)
	p := planConfig{
		packageService: services.Packages,
		installedRuntime: app.Application{Manifest: schema.Manifest{
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: runtimePackage},
				},
			},
		}},
		updateRuntime: app.Application{Manifest: schema.Manifest{
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: updateRuntimePackage},
				},
			},
		}},
	}
	update, installedVersion, updateVersion, err := shouldUpdateEtcd(p)
	c.Assert(err, check.IsNil)
	c.Assert(update, check.Equals, true)
	c.Assert(installedVersion, check.Equals, "")
	c.Assert(updateVersion, check.Equals, "3.3.3")
}

func (s *PlanSuite) TestCorrectlyDeterminesWhetherToUpdateEtcd(c *check.C) {
	services := opsservice.SetupTestServices(c)
	files := []*archive.Item{
		archive.ItemFromString("orbit.manifest.json", `{
	"version": "0.0.1",
	"labels": [
		{
			"name": "version-etcd",
			"value": "v3.3.2"
		}
	]
}`),
	}
	runtimePackage := loc.MustParseLocator("example.com/runtime:1.0.0")
	apptest.CreateDummyPackageWithContents(
		runtimePackage,
		files,
		services.Packages, c)
	files = []*archive.Item{
		archive.ItemFromString("orbit.manifest.json", `{
	"version": "0.0.1",
	"labels": [
		{
			"name": "version-etcd",
			"value": "v3.3.3"
		}
	]
}`),
	}
	updateRuntimePackage := loc.MustParseLocator("example.com/runtime:1.0.1")
	apptest.CreateDummyPackageWithContents(
		updateRuntimePackage,
		files,
		services.Packages, c)
	p := planConfig{
		packageService: services.Packages,
		installedRuntime: app.Application{Manifest: schema.Manifest{
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: runtimePackage},
				},
			},
		}},
		updateRuntime: app.Application{Manifest: schema.Manifest{
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: updateRuntimePackage},
				},
			},
		}},
	}
	update, installedVersion, updateVersion, err := shouldUpdateEtcd(p)
	c.Assert(err, check.IsNil)
	c.Assert(update, check.Equals, true)
	c.Assert(installedVersion, check.Equals, "3.3.2")
	c.Assert(updateVersion, check.Equals, "3.3.3")
}

func newTestPlan(c *check.C, p params) planConfig {
	runtimeLoc := loc.MustParseLocator("gravitational.io/planet:2.0.0")
	servers := []storage.Server{
		{
			AdvertiseIP: "192.168.0.1",
			Hostname:    "node-1",
			Role:        "node",
			ClusterRole: string(schema.ServiceRoleMaster),
		},
		{
			AdvertiseIP: "192.168.0.2",
			Hostname:    "node-2",
			Role:        "node",
			ClusterRole: string(schema.ServiceRoleMaster),
		},
		{
			AdvertiseIP: "192.168.0.3",
			Hostname:    "node-3",
			Role:        "node",
			ClusterRole: string(schema.ServiceRoleNode),
		},
	}
	updates := []storage.UpdateServer{
		{
			Server:  servers[0],
			Runtime: &storage.RuntimeUpdate{Package: runtimeLoc},
		},
		{
			Server:  servers[1],
			Runtime: &storage.RuntimeUpdate{Package: runtimeLoc},
		},
		{
			Server:  servers[2],
			Runtime: &storage.RuntimeUpdate{Package: runtimeLoc},
		},
	}
	operation := storage.SiteOperation{
		AccountID:  "000",
		SiteDomain: "test",
		ID:         "123",
		Type:       ops.OperationUpdate,
	}
	params := planConfig{
		operator:  testOperator,
		operation: operation,
		servers:   updates,
		installedRuntime: app.Application{
			Package:  p.installedRuntime,
			Manifest: schema.MustParseManifestYAML([]byte(p.installedRuntimeManifest)),
			PackageEnvelope: pack.PackageEnvelope{
				Manifest: []byte(p.installedRuntimeManifest),
			},
		},
		installedApp: app.Application{
			Package:  p.installedApp,
			Manifest: schema.MustParseManifestYAML([]byte(p.installedAppManifest)),
			PackageEnvelope: pack.PackageEnvelope{
				Manifest: []byte(p.installedAppManifest),
			},
		},
		updateRuntime: app.Application{
			Package:  p.updateRuntime,
			Manifest: schema.MustParseManifestYAML([]byte(p.updateRuntimeManifest)),
			PackageEnvelope: pack.PackageEnvelope{
				Manifest: []byte(p.updateRuntimeManifest),
			},
		},
		updateApp: app.Application{
			Package:  p.updateApp,
			Manifest: schema.MustParseManifestYAML([]byte(p.updateAppManifest)),
			PackageEnvelope: pack.PackageEnvelope{
				Manifest: []byte(p.updateAppManifest),
			},
		},
		links:            p.links,
		trustedClusters:  p.trustedClusters,
		shouldUpdateEtcd: shouldUpdateEtcdTest,
		updateCoreDNS:    p.updateCoreDNS,
	}
	gravityPackage, err := params.updateRuntime.Manifest.Dependencies.ByName(
		constants.GravityPackage)
	c.Assert(err, check.IsNil)
	params.plan = storage.OperationPlan{
		OperationID:    operation.ID,
		OperationType:  operation.Type,
		ClusterName:    operation.SiteDomain,
		Servers:        servers,
		GravityPackage: *gravityPackage,
	}
	return params
}

type params struct {
	installedRuntime         loc.Locator
	installedApp             loc.Locator
	updateRuntime            loc.Locator
	updateApp                loc.Locator
	installedRuntimeManifest string
	installedAppManifest     string
	updateRuntimeManifest    string
	updateAppManifest        string
	updateCoreDNS            bool
	links                    []storage.OpsCenterLink
	trustedClusters          []teleservices.TrustedCluster
}

func resetCap(phases []storage.OperationPhase) []storage.OperationPhase {
	return phases[:len(phases):len(phases)]
}

func shouldUpdateEtcdTest(planConfig) (bool, string, string, error) {
	return true, "1.0.0", "2.0.0", nil
}

func (r testRotator) RotateSecrets(ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error) {
	return &ops.RotatePackageResponse{Locator: r.secretsPackage}, nil
}

func (r testRotator) RotatePlanetConfig(ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error) {
	return &ops.RotatePackageResponse{Locator: r.runtimeConfigPackage}, nil
}

func (r testRotator) RotateTeleportConfig(ops.RotateTeleportConfigRequest) (*ops.RotatePackageResponse, *ops.RotatePackageResponse, error) {
	return &ops.RotatePackageResponse{Locator: r.teleportMasterPackage},
		&ops.RotatePackageResponse{Locator: r.teleportNodePackage},
		nil
}

var testOperator = testRotator{
	secretsPackage:        loc.Locator{Repository: "gravitational.io", Name: "secrets", Version: "0.0.1"},
	runtimeConfigPackage:  loc.Locator{Repository: "gravitational.io", Name: "planet-config", Version: "0.0.1"},
	teleportMasterPackage: loc.Locator{Repository: "gravitational.io", Name: "teleport-master-config", Version: "0.0.1"},
	teleportNodePackage:   loc.Locator{Repository: "gravitational.io", Name: "teleport-node-config", Version: "0.0.1"},
}

type testRotator struct {
	secretsPackage        loc.Locator
	runtimeConfigPackage  loc.Locator
	teleportMasterPackage loc.Locator
	teleportNodePackage   loc.Locator
}

const installedRuntimeManifest = `apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: runtime
  resourceVersion: 1.0.0
dependencies:
  packages:
    - gravitational.io/gravity:1.0.0
  apps:
    - gravitational.io/runtime-dep-1:1.0.0
    - gravitational.io/runtime-dep-2:1.0.0
    - gravitational.io/rbac-app:1.0.0
`

const installedAppManifest = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: app
  resourceVersion: 1.0.0
dependencies:
  apps:
    - gravitational.io/app-dep-1:1.0.0
    - gravitational.io/app-dep-2:1.0.0
nodeProfiles:
  - name: node
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:1.0.0
`

const updateRuntimeManifest = `apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: runtime
  resourceVersion: 2.0.0
dependencies:
  packages:
    - gravitational.io/gravity:2.0.0
  apps:
    - gravitational.io/runtime-dep-1:1.0.0
    - gravitational.io/runtime-dep-2:2.0.0
    - gravitational.io/rbac-app:2.0.0
`

const updateAppManifest = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: app
  resourceVersion: 2.0.0
dependencies:
  apps:
    - gravitational.io/app-dep-1:1.0.0
    - gravitational.io/app-dep-2:2.0.0
nodeProfiles:
  - name: node
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:2.0.0
`
