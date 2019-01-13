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

package update

import (
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

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

	plan, params := newTestPlan(c, params{
		installedRuntime:         runtimeLoc1,
		installedApp:             appLoc1,
		updateRuntime:            runtimeLoc2,
		updateApp:                appLoc2,
		installedRuntimeManifest: installedRuntimeManifest,
		installedAppManifest:     installedAppManifest,
		updateRuntimeManifest:    updateRuntimeManifest,
		updateAppManifest:        updateAppManifest,
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

	runtimeLoc := loc.MustParseLocator("gravitational.io/planet:2.0.0")
	var servers runtimeServers
	for _, server := range params.servers {
		servers = append(servers, runtimeServer{server, runtimeLoc})
	}

	builder := phaseBuilder{}
	init := *builder.init(appLoc1, appLoc2)
	checks := *builder.checks(appLoc1, appLoc2).Require(init)
	preUpdate := *builder.preUpdate(appLoc2).Require(init)
	bootstrap := *builder.bootstrap(params.servers, appLoc1, appLoc2).Require(init)
	leadMaster := runtimeServer{params.servers[0], runtimeLoc}
	masters := *builder.masters(leadMaster, servers[1:2], false).Require(checks, bootstrap, preUpdate)
	nodes := *builder.nodes(leadMaster.Server, servers[2:], false).Require(masters)
	etcd := *builder.etcdPlan(leadMaster.Server, params.servers[1:2], params.servers[2:], "1.0.0", "2.0.0")
	migration := builder.migration(params)
	c.Assert(migration, check.NotNil)

	runtimeLocs := []loc.Locator{
		loc.MustParseLocator("gravitational.io/runtime-dep-2:2.0.0"),
		loc.MustParseLocator("gravitational.io/rbac-app:2.0.0"),
		runtimeLoc2,
	}
	runtime := *builder.runtime(runtimeLocs, true).Require(masters)

	appLocs := []loc.Locator{loc.MustParseLocator("gravitational.io/app-dep-2:2.0.0"), appLoc2}
	app := *builder.app(appLocs).Require(masters).RequireLiteral(runtime.ChildLiteral(constants.BootstrapConfigPackage))
	cleanup := *builder.cleanup(params.servers).Require(app)

	plan.Phases = phases{
		init,
		checks,
		preUpdate,
		bootstrap,
		masters,
		nodes,
		etcd,
		*migration,
		runtime,
		app,
		cleanup,
	}.asPhases()
	resolve(&plan)

	// exercise
	obtainedPlan, err := newOperationPlanFromParams(params)
	c.Assert(err, check.IsNil)
	// Reset the capacity so the plans can be compared
	obtainedPlan.Phases = resetCap(obtainedPlan.Phases)
	resolve(obtainedPlan)

	// verify
	compare.DeepCompare(c, *obtainedPlan, plan)
}

func (s *PlanSuite) TestPlanWithoutRuntimeUpdate(c *check.C) {
	// setup
	runtimeLoc1 := loc.MustParseLocator("gravitational.io/runtime:1.0.0")
	appLoc1 := loc.MustParseLocator("gravitational.io/app:1.0.0")
	appLoc2 := loc.MustParseLocator("gravitational.io/app:2.0.0")

	plan, params := newTestPlan(c, params{
		installedRuntime:         runtimeLoc1,
		installedApp:             appLoc1,
		updateRuntime:            runtimeLoc1, // same runtime on purpose
		updateApp:                appLoc2,
		installedRuntimeManifest: installedRuntimeManifest,
		installedAppManifest:     installedAppManifest,
		updateRuntimeManifest:    installedRuntimeManifest, // same manifest on purpose
		updateAppManifest:        updateAppManifest,
	})

	builder := phaseBuilder{}
	init := *builder.init(appLoc1, appLoc2)
	checks := *builder.checks(appLoc1, appLoc2).Require(init)
	preUpdate := *builder.preUpdate(appLoc2).Require(init)
	appLocs := []loc.Locator{loc.MustParseLocator("gravitational.io/app-dep-2:2.0.0"), appLoc2}
	app := *builder.app(appLocs)
	cleanup := *builder.cleanup(params.servers).Require(app)

	plan.Phases = phases{init, checks, preUpdate, app, cleanup}.asPhases()
	resolve(&plan)

	// exercise
	obtainedPlan, err := newOperationPlanFromParams(params)
	c.Assert(err, check.IsNil)
	// Reset the capacity so the plans can be compared
	obtainedPlan.Phases = resetCap(obtainedPlan.Phases)
	resolve(obtainedPlan)

	// verify
	compare.DeepCompare(c, *obtainedPlan, plan)
}

func newTestPlan(c *check.C, p params) (storage.OperationPlan, newPlanParams) {
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
	params := newPlanParams{
		operation: storage.SiteOperation{
			ID:         "123",
			Type:       ops.OperationUpdate,
			SiteDomain: "test",
		},
		servers: servers,
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
		shouldUpdateEtcd: shouldUpdateEtcdTeest,
	}

	gravityPackage, err := params.updateRuntime.Manifest.Dependencies.ByName(
		constants.GravityPackage)
	c.Assert(err, check.IsNil)

	return storage.OperationPlan{
		OperationID:    params.operation.ID,
		OperationType:  params.operation.Type,
		ClusterName:    params.operation.SiteDomain,
		Servers:        params.servers,
		GravityPackage: *gravityPackage,
	}, params
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
	links                    []storage.OpsCenterLink
	trustedClusters          []teleservices.TrustedCluster
}

func resetCap(phases []storage.OperationPhase) []storage.OperationPhase {
	return phases[:len(phases):len(phases)]
}

func shouldUpdateEtcdTeest(p newPlanParams) (bool, string, string, error) {
	return true, "1.0.0", "2.0.0", nil
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
