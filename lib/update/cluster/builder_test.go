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
	"fmt"
	"path"
	"sort"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/coreos/go-semver/semver"
	teleservices "github.com/gravitational/teleport/lib/services"
	"gopkg.in/check.v1"
)

type PlanSuite struct{}

var _ = check.Suite(&PlanSuite{})

func (s *PlanSuite) TestPlanWithRuntimeAppsUpdate(c *check.C) {
	// setup
	installedRuntimeApp := newApp("gravitational.io/runtime:1.0.0", installedRuntimeAppManifest)
	installedApp := newApp("gravitational.io/app:1.0.0", installedAppManifest)
	updateRuntimeApp := newApp("gravitational.io/runtime:2.0.0", updateRuntimeAppManifest)
	updateApp := newApp("gravitational.io/app:2.0.0", updateAppManifest)
	gravityPackage := mustLocator(updateRuntimeApp.Manifest.Dependencies.ByName(
		constants.GravityPackage))
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
			Runtime: runtimePackage,
		},
		{
			Server:  servers[1],
			Runtime: runtimePackage,
		},
		{
			Server:  servers[2],
			Runtime: runtimePackage,
		},
	}
	runtimeUpdates := []loc.Locator{
		loc.MustParseLocator("gravitational.io/rbac-app:2.0.0"),
		loc.MustParseLocator("gravitational.io/runtime-dep-2:2.0.0"),
		updateRuntimeApp.Package,
	}
	params := params{
		servers:             servers,
		installedRuntimeApp: installedRuntimeApp,
		installedApp:        installedApp,
		updateRuntimeApp:    updateRuntimeApp,
		updateApp:           updateApp,
		links: []storage.OpsCenterLink{
			{
				Hostname:   "ops.example.com",
				Type:       storage.OpsCenterRemoteAccessLink,
				RemoteAddr: "ops.example.com:3024",
				APIURL:     "https://ops.example.com:32009",
				Enabled:    true,
			},
		},
		dnsConfig: storage.DefaultDNSConfig,
		// Use an alternative (other than first) master node as leader
		leadMaster: servers[1],
		appUpdates: []loc.Locator{
			loc.MustParseLocator("gravitational.io/app-dep-2:2.0.0"),
			updateApp.Package,
		},
		targetStep: newTargetUpdateStep(updateStep{
			runtimeUpdates: runtimeUpdates,
			etcd: &etcdVersion{
				installed: "1.0.0",
				update:    "2.0.0",
			},
			servers:     updates,
			gravity:     gravityPackage,
			changesetID: "id",
		}),
	}
	builder := newBuilder(c, params)

	// exercise
	obtainedPlan, err := builder.newPlan()
	c.Assert(err, check.IsNil)

	// verify
	leadMaster := updates[1]
	rearrangedServers := []storage.UpdateServer{updates[1], updates[0], updates[2]}
	c.Assert(*obtainedPlan, check.DeepEquals, storage.OperationPlan{
		OperationID:        builder.operation.ID,
		OperationType:      builder.operation.Type,
		AccountID:          builder.operation.AccountID,
		ClusterName:        builder.operation.SiteDomain,
		Servers:            servers,
		DNSConfig:          storage.DefaultDNSConfig,
		GravityPackage:     loc.MustParseLocator("gravitational.io/gravity:3.0.0"),
		OfflineCoordinator: &params.leadMaster,
		Phases: []storage.OperationPhase{
			params.init(rearrangedServers),
			params.checks("/init"),
			params.preUpdate("/init", "/checks"),
			params.bootstrap(rearrangedServers, gravityPackage, "/checks", "/pre-update"),
			params.coreDNS("/bootstrap"),
			params.masters(leadMaster, updates[0:1], gravityPackage, "id", "/coredns"),
			params.nodes(updates[2:], leadMaster.Server, gravityPackage, "id", "/masters"),
			params.etcd(leadMaster.Server, updates[0:1], *params.targetStep.etcd),
			params.config("/etcd"),
			params.runtime(runtimeUpdates, "/config"),
			params.migration("/runtime"),
			params.app("/migration"),
			params.cleanup(),
		},
	})
}

func (s *PlanSuite) TestPlanWithoutRuntimeAppUpdate(c *check.C) {
	// setup
	installedRuntimeApp := newApp("gravitational.io/runtime:1.0.0", installedRuntimeAppManifest)
	installedApp := newApp("gravitational.io/app:1.0.0", installedAppManifest)
	updateApp := newApp("gravitational.io/app:2.0.0", updateAppManifest)
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
	params := params{
		servers:             servers,
		installedRuntimeApp: installedRuntimeApp,
		installedApp:        installedApp,
		updateRuntimeApp:    installedRuntimeApp, // same runtime on purpose
		updateApp:           updateApp,
		dnsConfig:           storage.DefaultDNSConfig,
		leadMaster:          servers[0],
		appUpdates: []loc.Locator{
			loc.MustParseLocator("gravitational.io/app-dep-2:2.0.0"),
			updateApp.Package,
		},
	}
	builder := newBuilder(c, params)

	// exercise
	obtainedPlan, err := builder.newPlan()
	c.Assert(err, check.IsNil)

	// verify
	c.Assert(*obtainedPlan, check.DeepEquals, storage.OperationPlan{
		OperationID:        builder.operation.ID,
		OperationType:      builder.operation.Type,
		AccountID:          builder.operation.AccountID,
		ClusterName:        builder.operation.SiteDomain,
		Servers:            servers,
		DNSConfig:          storage.DefaultDNSConfig,
		GravityPackage:     gravityInstalledLoc,
		OfflineCoordinator: &params.leadMaster,
		Phases: []storage.OperationPhase{
			params.checks(),
			params.preUpdate("/checks"),
			params.app("/pre-update"),
			params.cleanup(),
		},
	})
}

func (s *PlanSuite) TestPlanWithIntermediateRuntimeUpdate(c *check.C) {
	// setup
	installedRuntimeApp := newApp("gravitational.io/runtime:1.0.0", installedRuntimeAppManifest)
	installedApp := newApp("gravitational.io/app:1.0.0", installedAppManifest)
	intermediateRuntimeApp := newApp("gravitational.io/runtime:2.0.0", intermediateRuntimeAppManifest)
	updateRuntimeApp := newApp("gravitational.io/runtime:3.0.0", updateRuntimeAppManifest)
	updateApp := newApp("gravitational.io/app:3.0.0", updateAppManifest)
	intermediateGravityPackage := mustLocator(intermediateRuntimeApp.Manifest.Dependencies.ByName(
		constants.GravityPackage))
	gravityPackage := mustLocator(updateRuntimeApp.Manifest.Dependencies.ByName(
		constants.GravityPackage))
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
	intermediateUpdates := []storage.UpdateServer{
		{
			Server:  servers[0],
			Runtime: intermediateRuntimePackage,
		},
		{
			Server:  servers[1],
			Runtime: intermediateRuntimePackage,
		},
		{
			Server:  servers[2],
			Runtime: intermediateRuntimePackage,
		},
	}
	updates := []storage.UpdateServer{
		{
			Server:  servers[0],
			Runtime: runtimePackage,
		},
		{
			Server:  servers[1],
			Runtime: runtimePackage,
		},
		{
			Server:  servers[2],
			Runtime: runtimePackage,
		},
	}
	intermediateRuntimeUpdates := []loc.Locator{intermediateRuntimeApp.Package}
	runtimeUpdates := []loc.Locator{
		loc.MustParseLocator("gravitational.io/rbac-app:2.0.0"),
		loc.MustParseLocator("gravitational.io/runtime-dep-2:2.0.0"),
		updateRuntimeApp.Package,
	}
	params := params{
		servers:             servers,
		installedRuntimeApp: installedRuntimeApp,
		installedApp:        installedApp,
		updateRuntimeApp:    updateRuntimeApp,
		updateApp:           updateApp,
		links: []storage.OpsCenterLink{
			{
				Hostname:   "ops.example.com",
				Type:       storage.OpsCenterRemoteAccessLink,
				RemoteAddr: "ops.example.com:3024",
				APIURL:     "https://ops.example.com:32009",
				Enabled:    true,
			},
		},
		dnsConfig: storage.DefaultDNSConfig,
		// Use an alternative (other than first) master node as leader
		leadMaster: servers[1],
		appUpdates: []loc.Locator{
			loc.MustParseLocator("gravitational.io/app-dep-2:2.0.0"),
			updateApp.Package,
		},
		steps: []intermediateUpdateStep{
			{
				updateStep: updateStep{
					changesetID: "id2",
					servers:     intermediateUpdates,
					etcd: &etcdVersion{
						installed: "1.0.0",
						update:    "2.0.0",
					},
					runtimeUpdates: intermediateRuntimeUpdates,
					gravity:        intermediateGravityPackage,
				},
				runtimeAppVersion: *semver.New("1.0.0"),
			},
		},
		targetStep: targetUpdateStep{updateStep: updateStep{
			changesetID:    "id",
			runtimeUpdates: runtimeUpdates,
			etcd: &etcdVersion{
				installed: "2.0.0",
				update:    "3.0.0",
			},
			gravity: gravityPackage,
			servers: updates,
		}},
	}
	builder := newBuilder(c, params)

	// exercise
	obtainedPlan, err := builder.newPlan()
	c.Assert(err, check.IsNil)

	// verify
	intermediateLeadMaster := intermediateUpdates[1]
	rearrangedIntermediateServers := []storage.UpdateServer{intermediateUpdates[1], intermediateUpdates[0], intermediateUpdates[2]}
	intermediateOtherMasters := intermediateUpdates[0:1]
	intermediateNodes := intermediateUpdates[2:]
	leadMaster := updates[1]
	rearrangedServers := []storage.UpdateServer{updates[1], updates[0], updates[2]}
	otherMasters := updates[0:1]
	nodes := updates[2:]

	c.Assert(*obtainedPlan, check.DeepEquals, storage.OperationPlan{
		OperationID:        builder.operation.ID,
		OperationType:      builder.operation.Type,
		AccountID:          builder.operation.AccountID,
		ClusterName:        builder.operation.SiteDomain,
		Servers:            servers,
		DNSConfig:          storage.DefaultDNSConfig,
		GravityPackage:     loc.MustParseLocator("gravitational.io/gravity:3.0.0"),
		OfflineCoordinator: &params.leadMaster,
		Phases: []storage.OperationPhase{
			params.init(rearrangedIntermediateServers),
			params.checks("/init"),
			params.preUpdate("/init", "/checks"),
			params.sub("/1.0.0", []string{"/checks", "/pre-update"},
				params.bootstrapVersioned(rearrangedIntermediateServers, "1.0.0", intermediateGravityPackage),
				params.masters(intermediateLeadMaster, intermediateOtherMasters, intermediateGravityPackage, "id2", "/bootstrap"),
				params.nodes(intermediateNodes, intermediateLeadMaster.Server, intermediateGravityPackage, "id2", "/masters"),
				params.etcd(intermediateLeadMaster.Server,
					intermediateOtherMasters,
					*params.steps[0].etcd),
				params.config("/etcd"),
				params.runtime(intermediateRuntimeUpdates, "/config"),
			),
			params.sub("/target", []string{"/1.0.0"},
				params.bootstrap(rearrangedServers, gravityPackage),
				params.coreDNS("/bootstrap"),
				params.masters(leadMaster, otherMasters, gravityPackage, "id", "/coredns"),
				params.nodes(nodes, leadMaster.Server, gravityPackage, "id", "/masters"),
				params.etcd(leadMaster.Server, otherMasters, *params.targetStep.etcd),
				params.config("/etcd"),
				params.runtime(runtimeUpdates, "/config"),
			),
			params.migration("/target"),
			params.app("/migration"),
			params.cleanup(),
		},
	})
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
	b := phaseBuilder{
		packages: services.Packages,
		installedRuntimeApp: app.Application{Manifest: schema.Manifest{
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: runtimePackage},
				},
			},
		}},
		updateRuntimeApp: app.Application{Manifest: schema.Manifest{
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: updateRuntimePackage},
				},
			},
		}},
	}
	version, err := shouldUpdateEtcd(b.installedRuntimeApp, b.updateRuntimeApp, services.Packages)
	c.Assert(err, check.IsNil)
	c.Assert(version, check.DeepEquals, &etcdVersion{
		update: "3.3.3",
	})
}

func (s *PlanSuite) TestDeterminesWhetherToUpdateEtcd(c *check.C) {
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
	b := phaseBuilder{
		packages: services.Packages,
		installedRuntimeApp: app.Application{Manifest: schema.Manifest{
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: runtimePackage},
				},
			},
		}},
		updateRuntimeApp: app.Application{Manifest: schema.Manifest{
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: updateRuntimePackage},
				},
			},
		}},
	}
	version, err := shouldUpdateEtcd(b.installedRuntimeApp, b.updateRuntimeApp, b.packages)
	c.Assert(err, check.IsNil)
	c.Assert(version, check.DeepEquals, &etcdVersion{
		installed: "3.3.2",
		update:    "3.3.3",
	})
}

func newBuilder(c *check.C, params params) phaseBuilder {
	builder := phaseBuilder{
		operator:            testOperator,
		operation:           operation,
		installedRuntimeApp: params.installedRuntimeApp,
		installedApp:        params.installedApp,
		updateRuntimeApp:    params.updateRuntimeApp,
		updateApp:           params.updateApp,
		links:               params.links,
		trustedClusters:     params.trustedClusters,
		leadMaster:          params.leadMaster,
		appUpdates:          params.appUpdates,
		steps:               params.steps,
		targetStep:          params.targetStep,
	}
	gravityPackage, err := builder.updateRuntimeApp.Manifest.Dependencies.ByName(
		constants.GravityPackage)
	c.Assert(err, check.IsNil)
	builder.planTemplate = storage.OperationPlan{
		OperationID:        operation.ID,
		OperationType:      operation.Type,
		AccountID:          operation.AccountID,
		ClusterName:        operation.SiteDomain,
		Servers:            params.servers,
		GravityPackage:     *gravityPackage,
		DNSConfig:          params.dnsConfig,
		OfflineCoordinator: &params.leadMaster,
	}
	return builder
}

func (r *params) sub(id string, requires []string, phases ...storage.OperationPhase) storage.OperationPhase {
	parentize(id, phases)
	return storage.OperationPhase{
		ID:       id,
		Phases:   phases,
		Requires: requires,
	}
}

func parentize(parentID string, phases []storage.OperationPhase) {
	for i, phase := range phases {
		phases[i].ID = path.Join(parentID, phase.ID)
		for j, req := range phase.Requires {
			phases[i].Requires[j] = path.Join(parentID, req)
		}
		if len(phase.Phases) != 0 {
			parentize(parentID, phase.Phases)
		}
	}
}

func (r *params) init(servers []storage.UpdateServer) storage.OperationPhase {
	root := storage.OperationPhase{
		ID:          "/init",
		Description: "Initialize update operation",
	}
	leadMaster := servers[0]
	root.Phases = append(root.Phases, storage.OperationPhase{
		ID:          fmt.Sprintf("/init/%v", leadMaster.Hostname),
		Executor:    updateInitLeader,
		Description: fmt.Sprintf("Initialize node %q", leadMaster.Hostname),
		Data: &storage.OperationPhaseData{
			ExecServer:       &leadMaster.Server,
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
			Update: &storage.UpdateOperationData{
				Servers: []storage.UpdateServer{leadMaster},
			},
		},
	})
	for _, server := range servers[1:] {
		root.Phases = append(root.Phases, r.initServer(server))
	}
	return root
}

func (r *params) initServer(server storage.UpdateServer) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          fmt.Sprintf("/init/%v", server.Hostname),
		Executor:    updateInit,
		Description: fmt.Sprintf("Initialize node %q", server.Hostname),
		Data: &storage.OperationPhaseData{
			ExecServer: &server.Server,
			Update: &storage.UpdateOperationData{
				Servers: []storage.UpdateServer{server},
			},
		},
	}
}

func (r *params) checks(requires ...string) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/checks",
		Executor:    updateChecks,
		Description: "Run preflight checks",
		Requires:    requires,
		Data: &storage.OperationPhaseData{
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
		},
	}
}

func (r *params) preUpdate(requires ...string) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/pre-update",
		Executor:    preUpdate,
		Description: "Run pre-update application hook",
		Requires:    requires,
		Data: &storage.OperationPhaseData{
			Package: &r.updateApp.Package,
		},
	}
}

func (r *params) coreDNS(requires ...string) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/coredns",
		Description: "Provision CoreDNS resources",
		Executor:    coredns,
		Requires:    requires,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster,
		},
	}
}

func (r *params) masters(leadMaster storage.UpdateServer, otherMasters []storage.UpdateServer, gravityPackage loc.Locator, changesetID string, requires ...string) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/masters",
		Description: "Update master nodes",
		Requires:    requires,
		Phases: []storage.OperationPhase{
			r.leaderMasterPhase("/masters", leadMaster, otherMasters, gravityPackage, changesetID),
			r.otherMasterPhase(otherMasters[0], "/masters", leadMaster.Server, gravityPackage, changesetID),
		},
	}
}

func (r *params) leaderMasterPhase(parent string, leadMaster storage.UpdateServer, otherMasters []storage.UpdateServer, gravityPackage loc.Locator, changesetID string) storage.OperationPhase {
	p := func(format string) string {
		return fmt.Sprintf(path.Join(parent, format), leadMaster.Hostname)
	}
	t := func(format string) string {
		return fmt.Sprintf(format, leadMaster.Hostname)
	}
	result := storage.OperationPhase{
		ID:          p("%v"),
		Description: t("Update system software on master node %q"),
		Phases: []storage.OperationPhase{
			{
				ID:          p("%v/kubelet-permissions"),
				Description: t("Add permissions to kubelet on %q"),
				Executor:    kubeletPermissions,
				Data: &storage.OperationPhaseData{
					Server: &leadMaster.Server,
				},
			},
			{
				ID:          p("%v/stepdown"),
				Executor:    electionStatus,
				Description: t("Step down %q as Kubernetes leader"),
				Data: &storage.OperationPhaseData{
					Server: &leadMaster.Server,
					ElectionChange: &storage.ElectionChange{
						DisableServers: []storage.Server{leadMaster.Server},
					},
				},
				Requires: []string{p("%v/kubelet-permissions")},
			},
			{
				ID:          p("%v/drain"),
				Executor:    drainNode,
				Description: t("Drain node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &leadMaster.Server,
					ExecServer: &leadMaster.Server,
				},
				Requires: []string{p("%v/stepdown")},
			},
			{
				ID:          p("%v/system-upgrade"),
				Executor:    updateSystem,
				Description: t("Update system software on node %q"),
				Data: &storage.OperationPhaseData{
					ExecServer: &leadMaster.Server,
					Update: &storage.UpdateOperationData{
						Servers:        []storage.UpdateServer{leadMaster},
						GravityPackage: &gravityPackage,
						ChangesetID:    changesetID,
					},
				},
				Requires: []string{p("%v/drain")},
			},
			{
				ID:          p("%v/elect"),
				Executor:    electionStatus,
				Description: t("Make node %q Kubernetes leader"),
				Data: &storage.OperationPhaseData{
					Server: &leadMaster.Server,
					ElectionChange: &storage.ElectionChange{
						EnableServers:  []storage.Server{leadMaster.Server},
						DisableServers: serversToStorage(otherMasters...),
					},
				},
				Requires: []string{p("%v/system-upgrade")},
			},
			{
				ID:          p("%v/health"),
				Executor:    nodeHealth,
				Description: t("Health check node %q"),
				Data: &storage.OperationPhaseData{
					Server: &leadMaster.Server,
				},
				Requires: []string{p("%v/elect")},
			},
			{
				ID:          p("%v/uncordon"),
				Executor:    uncordonNode,
				Description: t("Uncordon node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &leadMaster.Server,
					ExecServer: &leadMaster.Server,
				},
				Requires: []string{p("%v/health")},
			},
		},
	}
	return result
}

func (r *params) otherMasterPhase(server storage.UpdateServer, parent string, leadMaster storage.Server, gravityPackage loc.Locator, changesetID string) storage.OperationPhase {
	p := func(format string) string {
		return fmt.Sprintf(path.Join(parent, format), server.Hostname)
	}
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          p("%v"),
		Description: t("Update system software on master node %q"),
		Requires:    []string{fmt.Sprintf("%v/%v", parent, leadMaster.Hostname)},
		Phases: []storage.OperationPhase{
			{
				ID:          p("%v/drain"),
				Executor:    drainNode,
				Description: t("Drain node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
			},
			{
				ID:          p("%v/system-upgrade"),
				Executor:    updateSystem,
				Description: t("Update system software on node %q"),
				Data: &storage.OperationPhaseData{
					ExecServer: &server.Server,
					Update: &storage.UpdateOperationData{
						Servers:        []storage.UpdateServer{server},
						GravityPackage: &gravityPackage,
						ChangesetID:    changesetID,
					},
				},
				Requires: []string{p("%v/drain")},
			},
			{
				ID:          p("%v/elect"),
				Executor:    electionStatus,
				Description: t("Enable leader election on node %q"),
				Data: &storage.OperationPhaseData{
					Server: &server.Server,
					ElectionChange: &storage.ElectionChange{
						EnableServers: []storage.Server{server.Server},
					},
				},
				Requires: []string{p("%v/system-upgrade")},
			},
			{
				ID:          p("%v/health"),
				Executor:    nodeHealth,
				Description: t("Health check node %q"),
				Data: &storage.OperationPhaseData{
					Server: &server.Server,
				},
				Requires: []string{p("%v/elect")},
			},
			{
				ID:          p("%v/uncordon"),
				Executor:    uncordonNode,
				Description: t("Uncordon node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
				Requires: []string{p("%v/health")},
			},
			{
				ID:          p("%v/endpoints"),
				Executor:    endpoints,
				Description: t("Wait for DNS/cluster endpoints on %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
				Requires: []string{p("%v/uncordon")},
			},
		},
	}
}

func (r *params) nodes(updates []storage.UpdateServer, leadMaster storage.Server, gravityPackage loc.Locator, changesetID string, requires ...string) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/nodes",
		Description: "Update regular nodes",
		Requires:    requires,
		Phases: []storage.OperationPhase{
			r.nodePhase(updates[0], leadMaster, gravityPackage, "/nodes", changesetID),
		},
	}
}

func (r *params) nodePhase(server storage.UpdateServer, leadMaster storage.Server, gravityPackage loc.Locator, parent, id string) storage.OperationPhase {
	p := func(format string) string {
		return fmt.Sprintf(path.Join(parent, format), server.Hostname)
	}
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          p("%v"),
		Description: t("Update system software on node %q"),
		Phases: []storage.OperationPhase{
			{
				ID:          p("%v/drain"),
				Executor:    drainNode,
				Description: t("Drain node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
			},
			{
				ID:          p("%v/system-upgrade"),
				Executor:    updateSystem,
				Description: t("Update system software on node %q"),
				Data: &storage.OperationPhaseData{
					ExecServer: &server.Server,
					Update: &storage.UpdateOperationData{
						Servers:        []storage.UpdateServer{server},
						GravityPackage: &gravityPackage,
						ChangesetID:    id,
					},
				},
				Requires: []string{p("%v/drain")},
			},
			{
				ID:          p("%v/health"),
				Executor:    nodeHealth,
				Description: t("Health check node %q"),
				Data: &storage.OperationPhaseData{
					Server: &server.Server,
				},
				Requires: []string{p("%v/system-upgrade")},
			},
			{
				ID:          p("%v/uncordon"),
				Executor:    uncordonNode,
				Description: t("Uncordon node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
				Requires: []string{p("%v/health")},
			},
			{
				ID:          p("%v/endpoints"),
				Executor:    endpoints,
				Description: t("Wait for DNS/cluster endpoints on %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
				Requires: []string{p("%v/uncordon")},
			},
		},
	}
}

func (r *params) bootstrap(servers []storage.UpdateServer, gravityPackage loc.Locator, requires ...string) storage.OperationPhase {
	root := storage.OperationPhase{
		ID:          "/bootstrap",
		Description: "Bootstrap update operation on nodes",
		Requires:    requires,
	}
	root.Phases = append(root.Phases, r.bootstrapLeaderNode(servers, gravityPackage))
	for _, server := range servers[1:] {
		server := server
		root.Phases = append(root.Phases, r.bootstrapNode(server, gravityPackage))
	}
	return root
}

func (r *params) bootstrapLeaderNode(servers []storage.UpdateServer, gravityPackage loc.Locator) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, servers[0].Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/bootstrap/%v"),
		Description: t("Bootstrap node %q"),
		Executor:    updateBootstrapLeader,
		Data: &storage.OperationPhaseData{
			ExecServer:       &servers[0].Server,
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
			Update: &storage.UpdateOperationData{
				Servers:        servers,
				GravityPackage: &gravityPackage,
			},
		},
	}
}

func (r *params) bootstrapNode(server storage.UpdateServer, gravityPackage loc.Locator) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/bootstrap/%v"),
		Description: t("Bootstrap node %q"),
		Executor:    updateBootstrap,
		Data: &storage.OperationPhaseData{
			ExecServer:       &server.Server,
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
			Update: &storage.UpdateOperationData{
				Servers:        []storage.UpdateServer{server},
				GravityPackage: &gravityPackage,
			},
		},
	}
}

func (r *params) bootstrapVersioned(servers []storage.UpdateServer, version string, gravityPackage loc.Locator, requires ...string) storage.OperationPhase {
	root := storage.OperationPhase{
		ID:          "/bootstrap",
		Description: "Bootstrap update operation on nodes",
		Requires:    requires,
	}
	root.Phases = append(root.Phases, r.bootstrapLeaderNodeVersioned(servers, version, gravityPackage))
	for _, server := range servers[1:] {
		server := server
		root.Phases = append(root.Phases, r.bootstrapNodeVersioned(server, version, gravityPackage))
	}
	return root
}

func (r *params) bootstrapLeaderNodeVersioned(servers []storage.UpdateServer, version string, gravityPackage loc.Locator) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, servers[0].Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/bootstrap/%v"),
		Description: t("Bootstrap node %q"),
		Executor:    updateBootstrapLeader,
		Data: &storage.OperationPhaseData{
			ExecServer:       &servers[0].Server,
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
			Update: &storage.UpdateOperationData{
				Servers:           servers,
				RuntimeAppVersion: version,
				GravityPackage:    &gravityPackage,
			},
		},
	}
}

func (r *params) bootstrapNodeVersioned(server storage.UpdateServer, version string, gravityPackage loc.Locator) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("bootstrap/%v"),
		Description: t("Bootstrap node %q"),
		Executor:    updateBootstrap,
		Data: &storage.OperationPhaseData{
			ExecServer:       &server.Server,
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
			Update: &storage.UpdateOperationData{
				Servers:           []storage.UpdateServer{server},
				RuntimeAppVersion: version,
				GravityPackage:    &gravityPackage,
			},
		},
	}
}

func (r params) etcd(leadMaster storage.Server, otherMasters []storage.UpdateServer, etcd etcdVersion) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/etcd",
		Description: fmt.Sprintf("Upgrade etcd %v to %v", etcd.installed, etcd.update),
		Phases: []storage.OperationPhase{
			{
				ID:          "/etcd/backup",
				Description: "Backup etcd data",
				Phases: []storage.OperationPhase{
					r.etcdBackupNode(leadMaster),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdBackupNode(otherMasters[0].Server),
				},
			},
			{
				ID:          "/etcd/shutdown",
				Description: "Shutdown etcd cluster",
				Phases: []storage.OperationPhase{
					r.etcdShutdownNode(leadMaster, true),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdShutdownNode(otherMasters[0].Server, false),
				},
			},
			{
				ID:          "/etcd/upgrade",
				Description: "Upgrade etcd servers",
				Phases: []storage.OperationPhase{
					r.etcdUpgradeNode(leadMaster),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdUpgradeNode(otherMasters[0].Server),
				},
			},
			{
				ID:          "/etcd/migrate",
				Description: "Migrate etcd data to new version",
				Phases: []storage.OperationPhase{
					r.etcdMigrateNode(leadMaster, etcd),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdMigrateNode(otherMasters[0].Server, etcd),
				},
			},
			{
				ID:          "/etcd/restart",
				Description: "Restart etcd servers",
				Phases: []storage.OperationPhase{
					r.etcdRestartLeaderNode(leadMaster),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdRestartNode(otherMasters[0].Server),
					r.etcdRestartGravity(leadMaster),
				},
			},
		},
	}
}

func (r params) etcdBackupNode(server storage.Server) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/backup/%v"),
		Description: t("Backup etcd on node %q"),
		Executor:    updateEtcdBackup,
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	}
}

func (r params) etcdShutdownNode(server storage.Server, isLeader bool) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/shutdown/%v"),
		Description: t("Shutdown etcd on node %q"),
		Executor:    updateEtcdShutdown,
		Requires:    []string{t("/etcd/backup/%v")},
		Data: &storage.OperationPhaseData{
			Server: &server,
			Data:   strconv.FormatBool(isLeader),
		},
	}
}

func (r params) etcdShutdownWorkerNode(server storage.Server) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/shutdown/%v"),
		Description: t("Shutdown etcd on node %q"),
		Executor:    updateEtcdShutdown,
		Data: &storage.OperationPhaseData{
			Server: &server,
			Data:   "false",
		},
	}
}

func (r params) etcdUpgradeNode(server storage.Server) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/upgrade/%v"),
		Description: t("Upgrade etcd on node %q"),
		Executor:    updateEtcdMaster,
		Requires:    []string{t("/etcd/shutdown/%v")},
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	}
}

func (r params) etcdMigrateNode(server storage.Server, etcd etcdVersion) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID: t("/etcd/migrate/%v"),
		Description: fmt.Sprintf("Migrate etcd data to version %v on node %q",
			etcd.update, server.Hostname),
		Executor: updateEtcdMigrate,
		Requires: []string{t("/etcd/upgrade/%v")},
		Data: &storage.OperationPhaseData{
			Server: &server,
			Update: &storage.UpdateOperationData{
				Etcd: &storage.EtcdUpgrade{
					From: etcd.installed,
					To:   etcd.update,
				},
			},
		},
	}
}

func (r params) etcdRestartLeaderNode(leadMaster storage.Server) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, leadMaster.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/restart/%v"),
		Description: t("Restart etcd on node %q"),
		Executor:    updateEtcdRestart,
		Requires:    []string{t("/etcd/migrate/%v")},
		Data: &storage.OperationPhaseData{
			Server: &leadMaster,
		},
	}
}

func (r params) etcdRestartNode(server storage.Server) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/restart/%v"),
		Description: t("Restart etcd on node %q"),
		Executor:    updateEtcdRestart,
		Requires:    []string{t("/etcd/migrate/%v")},
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	}
}

func (r params) etcdRestartWorkerNode(server storage.Server) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/restart/%v"),
		Description: t("Restart etcd on node %q"),
		Executor:    updateEtcdRestart,
		Requires:    []string{t("/etcd/upgrade/%v")},
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	}
}

func (r params) etcdRestartGravity(leadMaster storage.Server) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          fmt.Sprint("/etcd/restart/", constants.GravityServiceName),
		Description: fmt.Sprint("Restart ", constants.GravityServiceName, " service"),
		Executor:    updateEtcdRestartGravity,
		Data: &storage.OperationPhaseData{
			Server: &leadMaster,
		},
	}
}

func (r *params) migration(requires ...string) storage.OperationPhase {
	phase := storage.OperationPhase{
		ID:          "/migration",
		Description: "Perform system database migration",
		Requires:    requires,
	}
	if len(r.links) != 0 && len(r.trustedClusters) == 0 {
		phase.Phases = append(phase.Phases, storage.OperationPhase{
			ID:          "/migration/links",
			Description: "Migrate remote Gravity Hub links to trusted clusters",
			Executor:    migrateLinks,
		})
	}
	phase.Phases = append(phase.Phases, storage.OperationPhase{
		ID:          "/migration/labels",
		Description: "Update node labels",
		Executor:    updateLabels,
	})
	// FIXME: add roles migration step
	return phase
}

func (r params) config(requires ...string) storage.OperationPhase {
	masters, _ := fsm.SplitServers(r.servers)
	masters = reorderStorageServers(masters, r.leadMaster)
	return storage.OperationPhase{
		ID:          "/config",
		Description: "Update system configuration on nodes",
		Requires:    requires,
		Phases: []storage.OperationPhase{
			r.configNode(masters[0]),
			r.configNode(masters[1]),
		},
	}
}

func (r params) configNode(server storage.Server) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/config/%v"),
		Executor:    config,
		Description: t("Update system configuration on node %q"),
		Data: &storage.OperationPhaseData{
			Server: &server,
		},
	}
}

func (r params) runtime(updates []loc.Locator, requires ...string) storage.OperationPhase {
	phase := storage.OperationPhase{
		ID:          "/runtime",
		Description: "Update application runtime",
		Requires:    requires,
	}
	var deps []string
	for _, update := range updates {
		app := runtimeUpdate(update, deps...)
		phase.Phases = append(phase.Phases, app)
		deps = []string{app.ID}
	}
	return phase
}

func runtimeUpdate(loc loc.Locator, requires ...string) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          fmt.Sprintf("/runtime/%v", loc.Name),
		Executor:    updateApp,
		Description: fmt.Sprintf("Update system application %q to %v", loc.Name, loc.Version),
		Data: &storage.OperationPhaseData{
			Package: &loc,
		},
		Requires: requires,
	}
}

func (r params) app(requires ...string) storage.OperationPhase {
	phase := storage.OperationPhase{
		ID:          "/app",
		Description: "Update installed application",
		Requires:    requires,
	}
	for _, update := range r.appUpdates {
		phase.Phases = append(phase.Phases, appUpdate(update))
	}
	return phase
}

func appUpdate(loc loc.Locator, requires ...string) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          fmt.Sprintf("/app/%v", loc.Name),
		Executor:    updateApp,
		Description: fmt.Sprintf("Update application %q to %v", loc.Name, loc.Version),
		Data: &storage.OperationPhaseData{
			Package: &loc,
		},
		Requires: requires,
	}
}

func (r params) cleanup() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/gc",
		Description: "Run cleanup tasks",
		Requires:    []string{"/app"},
		Phases: []storage.OperationPhase{
			{
				ID:          "/gc/node-1",
				Executor:    cleanupNode,
				Description: `Clean up node "node-1"`,
				Data: &storage.OperationPhaseData{
					Server: &r.servers[0],
				},
			},
			{
				ID:          "/gc/node-2",
				Executor:    cleanupNode,
				Description: `Clean up node "node-2"`,
				Data: &storage.OperationPhaseData{
					Server: &r.servers[1],
				},
			},
			{
				ID:          "/gc/node-3",
				Executor:    cleanupNode,
				Description: `Clean up node "node-3"`,
				Data: &storage.OperationPhaseData{
					Server: &r.servers[2],
				},
			},
		},
	}
}

type params struct {
	installedRuntimeApp      app.Application
	installedApp             app.Application
	updateRuntimeApp         app.Application
	updateApp                app.Application
	installedRuntimeManifest string
	installedAppManifest     string
	updateRuntimeManifest    string
	updateAppManifest        string
	updateCoreDNS            bool
	links                    []storage.OpsCenterLink
	trustedClusters          []teleservices.TrustedCluster
	leadMaster               storage.Server
	dnsConfig                storage.DNSConfig
	appUpdates               []loc.Locator
	servers                  []storage.Server
	steps                    []intermediateUpdateStep
	targetStep               targetUpdateStep
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
	secretsPackage:        loc.MustParseLocator("gravitational.io/secrets:0.0.1"),
	runtimeConfigPackage:  loc.MustParseLocator("gravitational.io/planet-config:0.0.1"),
	teleportMasterPackage: loc.MustParseLocator("gravitational.io/teleport-master-config:0.0.1"),
	teleportNodePackage:   loc.MustParseLocator("gravitational.io/teleport-node-config:0.0.1"),
}

type testRotator struct {
	secretsPackage        loc.Locator
	runtimeConfigPackage  loc.Locator
	teleportMasterPackage loc.Locator
	teleportNodePackage   loc.Locator
}

func newApp(appLoc string, manifestBytes string) app.Application {
	return app.Application{
		Package:  loc.MustParseLocator(appLoc),
		Manifest: schema.MustParseManifestYAML([]byte(manifestBytes)),
		PackageEnvelope: pack.PackageEnvelope{
			Manifest: []byte(manifestBytes),
		},
	}
}

func mustLocator(loc *loc.Locator, err error) loc.Locator {
	if err != nil {
		panic(err)
	}
	return *loc
}

func reorderStorageServers(servers []storage.Server, server storage.Server) (result []storage.Server) {
	sort.Slice(servers, func(i, j int) bool {
		// Push server to the front
		return servers[i].AdvertiseIP == server.AdvertiseIP
	})
	return servers
}

var runtimePackage = storage.RuntimePackage{
	Update: &storage.RuntimeUpdate{
		Package: loc.MustParseLocator("gravitational.io/planet:2.0.0"),
	},
}
var intermediateRuntimePackage = storage.RuntimePackage{
	Update: &storage.RuntimeUpdate{
		Package: loc.MustParseLocator("gravitational.io/planet:1.2.0"),
	},
}
var gravityInstalledLoc = loc.MustParseLocator("gravitational.io/gravity:1.0.0")

var operation = storage.SiteOperation{
	AccountID:  "000",
	SiteDomain: "test",
	ID:         "123",
	Type:       ops.OperationUpdate,
}

const installedRuntimeAppManifest = `apiVersion: bundle.gravitational.io/v2
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

const intermediateRuntimeAppManifest = `apiVersion: bundle.gravitational.io/v2
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

const updateRuntimeAppManifest = `apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: runtime
  resourceVersion: 3.0.0
dependencies:
  packages:
    - gravitational.io/gravity:3.0.0
  apps:
    - gravitational.io/runtime-dep-1:1.0.0
    - gravitational.io/runtime-dep-2:3.0.0
    - gravitational.io/rbac-app:3.0.0
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

const intermediateAppManifest = `apiVersion: bundle.gravitational.io/v2
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

const updateAppManifest = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: app
  resourceVersion: 3.0.0
dependencies:
  apps:
    - gravitational.io/app-dep-1:1.0.0
    - gravitational.io/app-dep-2:3.0.0
nodeProfiles:
  - name: node
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:3.0.0
`
