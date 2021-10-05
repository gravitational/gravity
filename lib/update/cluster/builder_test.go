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
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/app"
	appservice "github.com/gravitational/gravity/lib/app/service"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update/cluster/versions"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/google/go-cmp/cmp"
	"gopkg.in/check.v1"
)

type PlanSuite struct {
	services appservice.TestServices
}

func (s *PlanSuite) SetUpTest(c *check.C) {
	s.services = appservice.NewTestServices(c.MkDir(), c)
}

func (s *PlanSuite) TearDownTest(*check.C) {
	s.services.Close()
}

var _ = check.Suite(&PlanSuite{})

func (s *PlanSuite) TestPlanWithRuntimeAppsUpdate(c *check.C) {
	// setup
	servers := []storage.Server{
		newMaster("node-1"),
		newMaster("node-2"),
		newWorker("node-3"),
	}
	updateAppLoc, updateRuntimeAppLoc := newLoc("app:2.0.0"), newLoc("runtime:2.0.0")
	b := newClusterBuilder(s.services).
		withInstalledApp(newLoc("app:1.0.0"), newLoc("runtime:1.0.0")).
		withUpdateApp(updateAppLoc, updateRuntimeAppLoc).
		// Use an alternative leader node
		withServers(servers, servers[1]).
		withInstalledRuntimeDependencies(apps(
			"rbac-app:1.0.0",
			"runtime-app-1:1.0.0",
			"runtime-app-2:1.0.0")...).
		withInstalledDependencies(apps(
			"dep-app-1:1.0.0",
			"dep-app-2:1.0.0")...).
		withUpdateRuntimeDependencies(apps(
			"rbac-app:2.0.0",
			"runtime-app-1:1.0.0", // no change
			"runtime-app-2:2.0.0")...).
		withUpdateDependencies(apps(
			"dep-app-1:1.0.0", // no change
			"dep-app-2:2.0.0")...).
		withEtcdUpdate(etcdVersion{installed: "3.3.2", update: "3.3.3"})
	params := b.build(c)

	// exercise
	obtainedPlan, err := newOperationPlan(context.Background(), params.planConfig)
	c.Assert(err, check.IsNil)

	// verify
	updates := b.updateServers()
	expectedRuntimeAppUpdates := []loc.Locator{
		newLoc("rbac-app:2.0.0"),
		newLoc("runtime-app-2:2.0.0"),
		updateRuntimeAppLoc,
	}
	expectedAppUpdates := []loc.Locator{
		newLoc("dep-app-2:2.0.0"),
		updateAppLoc,
	}
	expectedPlan := storage.OperationPlan{
		OperationID:        params.operation.ID,
		OperationType:      params.operation.Type,
		AccountID:          params.operation.AccountID,
		ClusterName:        params.operation.SiteDomain,
		Servers:            servers,
		DNSConfig:          dnsConfig,
		GravityPackage:     params.gravityPackage,
		OfflineCoordinator: params.planConfig.leadMaster,
		Phases: []storage.OperationPhase{
			params.init(updates),
			params.checks("/init"),
			params.preUpdate("/init", "/checks"),
			params.bootstrap(updates, params.gravityPackage, "/checks", "/pre-update"),
			params.coreDNS("/bootstrap"),
			params.masters(updates, params.gravityPackage, "id", "/coredns"),
			params.nodes(updates, params.gravityPackage, "id", "/masters"),
			params.etcd(updates, params.etcdVersion),
			params.config(updates.masters(), "/etcd"),
			params.runtime(expectedRuntimeAppUpdates, "/config"),
			params.migration("/runtime"),
			params.app(expectedAppUpdates, "/migration"),
			params.cleanup(),
		},
	}
	if !cmp.Equal(*obtainedPlan, expectedPlan) {
		c.Error("Plans differ:", cmp.Diff(*obtainedPlan, expectedPlan))
	}
}

func (s *PlanSuite) TestPlanWithSingleMasterUpdate(c *check.C) {
	// setup
	servers := []storage.Server{
		newMaster("node-1"),
	}
	updateAppLoc, updateRuntimeAppLoc := newLoc("app:2.0.0"), newLoc("runtime:2.0.0")
	b := newClusterBuilder(s.services).
		withInstalledApp(newLoc("app:1.0.0"), newLoc("runtime:1.0.0")).
		withUpdateApp(updateAppLoc, updateRuntimeAppLoc).
		withServers(servers, servers[0]).
		withInstalledRuntimeDependencies(apps(
			"rbac-app:1.0.0",
			"runtime-app-1:1.0.0",
			"runtime-app-2:1.0.0")...).
		withInstalledDependencies(apps(
			"dep-app-1:1.0.0",
			"dep-app-2:1.0.0")...).
		withUpdateRuntimeDependencies(apps(
			"rbac-app:2.0.0",
			"runtime-app-1:1.0.0", // no change
			"runtime-app-2:2.0.0")...).
		withUpdateDependencies(apps(
			"dep-app-1:1.0.0", // no change
			"dep-app-2:2.0.0")...).
		withEtcdUpdate(etcdVersion{installed: "3.3.2", update: "3.3.3"})
	params := b.build(c)

	// exercise
	obtainedPlan, err := newOperationPlan(context.Background(), params.planConfig)
	c.Assert(err, check.IsNil)

	// verify
	updates := b.updateServers()
	expectedRuntimeAppUpdates := []loc.Locator{
		newLoc("rbac-app:2.0.0"),
		newLoc("runtime-app-2:2.0.0"),
		updateRuntimeAppLoc,
	}
	expectedAppUpdates := []loc.Locator{
		newLoc("dep-app-2:2.0.0"),
		updateAppLoc,
	}
	expectedPlan := storage.OperationPlan{
		OperationID:        params.operation.ID,
		OperationType:      params.operation.Type,
		AccountID:          params.operation.AccountID,
		ClusterName:        params.operation.SiteDomain,
		Servers:            servers,
		DNSConfig:          dnsConfig,
		GravityPackage:     params.gravityPackage,
		OfflineCoordinator: params.planConfig.leadMaster,
		Phases: []storage.OperationPhase{
			params.init(updates),
			params.checks("/init"),
			params.preUpdate("/init", "/checks"),
			params.bootstrap(updates, params.gravityPackage, "/checks", "/pre-update"),
			params.coreDNS("/bootstrap"),
			params.masters(updates, params.gravityPackage, "id", "/coredns"),
			params.etcd(updates, params.etcdVersion),
			params.config(updates.masters(), "/etcd"),
			params.runtime(expectedRuntimeAppUpdates, "/config"),
			params.migration("/runtime"),
			params.app(expectedAppUpdates, "/migration"),
			params.cleanup(),
		},
	}
	if !cmp.Equal(*obtainedPlan, expectedPlan) {
		c.Error("Plans differ:", cmp.Diff(*obtainedPlan, expectedPlan))
	}
}

func (s *PlanSuite) TestPlanWithoutRuntimeAppsUpdate(c *check.C) {
	// setup
	servers := []storage.Server{
		newMaster("node-1"),
		newMaster("node-2"),
		newWorker("node-3"),
	}
	params := newClusterBuilder(s.services).
		withInstalledApp(newLoc("app:1.0.0"), newLoc("runtime:1.0.0")).
		withServers(servers, servers[1]).
		withInstalledRuntimeDependencies(apps(
			"rbac-app:1.0.0",
			"runtime-app-1:1.0.0",
			"runtime-app-2:1.0.0")...).
		withInstalledDependencies(apps(
			"dep-app-1:1.0.0",
			"dep-app-2:1.0.0")...).
		withEmptyUpdate().
		build(c)

	// exercise
	obtainedPlan, err := newOperationPlan(context.Background(), params.planConfig)
	c.Assert(err, check.IsNil)

	// verify
	expectedPlan := storage.OperationPlan{
		OperationID:        params.operation.ID,
		OperationType:      params.operation.Type,
		AccountID:          params.operation.AccountID,
		ClusterName:        params.operation.SiteDomain,
		Servers:            servers,
		DNSConfig:          dnsConfig,
		GravityPackage:     params.gravityPackage,
		OfflineCoordinator: params.leadMaster,
		Phases: []storage.OperationPhase{
			params.checks(),
			params.preUpdate("/checks"),
			params.app(nil, "/pre-update"),
			params.cleanup(),
		},
	}
	if !cmp.Equal(*obtainedPlan, expectedPlan) {
		c.Error("Plans differ:", cmp.Diff(*obtainedPlan, expectedPlan))
	}
}

func (s *PlanSuite) TestPlanWithIntermediateRuntimeUpdate(c *check.C) {
	// setup
	servers := []storage.Server{
		newMaster("node-1"),
		newMaster("node-2"),
		newWorker("node-3"),
	}
	updateAppLoc, updateRuntimeAppLoc := newLoc("app:2.0.0"), newLoc("kubernetes:3.0.0")
	intermediate1 := intermediateConfigStep{
		runtimeAppLoc: newLoc("kubernetes:2.0.0"),
		runtimeLoc:    newLoc("planet:2.0.0"),
		etcdVersion:   "2.0.0",
		gravityLoc:    newLoc("gravity:2.0.0"),
		teleportLoc:   newLoc("teleport:2.0.0"),
	}
	intermediate2 := intermediateConfigStep{
		runtimeAppLoc: newLoc("kubernetes:2.1.0"),
		runtimeLoc:    newLoc("planet:2.2.0"),
		etcdVersion:   "2.5.0",
		gravityLoc:    newLoc("gravity:2.1.0"),
		teleportLoc:   newLoc("teleport:2.0.0"),
	}
	b := newClusterBuilder(s.services).
		withInstalledApp(newLoc("app:1.0.0"), newLoc("kubernetes:1.0.0")).
		withIntermediateStep(intermediate1).
		withIntermediateStep(intermediate2).
		withUpdateApp(updateAppLoc, updateRuntimeAppLoc).
		withServers(servers, servers[1]).
		withInstalledRuntimeDependencies(apps(
			"rbac-app:1.0.0",
			"runtime-app-1:1.0.0")...).
		withInstalledDependencies(apps("dep-app:1.0.0")...).
		withUpdateRuntimeDependencies(apps(
			"rbac-app:2.0.0",
			"runtime-app-1:2.0.0")...).
		withUpdateDependencies(apps("dep-app:2.0.0")...).
		withEtcdUpdate(etcdVersion{installed: "1.0.0", update: "3.0.0"}).
		withChangesetIDFrom(1)
	params := b.build(c)

	// exercise
	obtainedPlan, err := newOperationPlan(context.Background(), params.planConfig)
	c.Assert(err, check.IsNil)

	// verify
	updates := b.updateServers()
	intermediates1 := b.intermediateServers(0)
	intermediates2 := b.intermediateServers(1)
	expectedIntermediate1RuntimeAppUpdates := []loc.Locator{b.intermediates[0].runtimeAppLoc}
	expectedIntermediate2RuntimeAppUpdates := []loc.Locator{b.intermediates[1].runtimeAppLoc}
	expectedRuntimeAppUpdates := []loc.Locator{
		newLoc("rbac-app:2.0.0"),
		newLoc("runtime-app-1:2.0.0"),
		b.update.runtimeAppLoc,
	}
	expectedAppUpdates := []loc.Locator{
		newLoc("dep-app:2.0.0"),
		b.update.appLoc,
	}
	expectedPlan := storage.OperationPlan{
		OperationID:        params.operation.ID,
		OperationType:      params.operation.Type,
		AccountID:          params.operation.AccountID,
		ClusterName:        params.operation.SiteDomain,
		Servers:            servers,
		DNSConfig:          dnsConfig,
		GravityPackage:     newLoc("gravity:3.0.0"),
		OfflineCoordinator: params.planConfig.leadMaster,
		Phases: []storage.OperationPhase{
			params.init(intermediates1),
			params.checks("/init"),
			params.preUpdate("/init", "/checks"),
			params.sub("/2.0.0", []string{"/checks", "/pre-update"},
				params.bootstrapVersioned(intermediates1, "2.0.0", intermediate1.gravityLoc),
				params.masters(intermediates1, intermediate1.gravityLoc, "1", "/bootstrap"),
				params.nodes(intermediates1, intermediate1.gravityLoc, "1", "/masters"),
				params.etcd(intermediates1,
					etcdVersion{installed: "1.0.0", update: "2.0.0"}),
				params.config(updates.masters(), "/etcd"),
				params.runtime(expectedIntermediate1RuntimeAppUpdates, "/config"),
			),
			params.sub("/2.1.0", []string{"/2.0.0"},
				params.bootstrapVersioned(intermediates2, "2.1.0", intermediate2.gravityLoc),
				params.masters(intermediates2, intermediate2.gravityLoc, "2", "/bootstrap"),
				params.nodes(intermediates2, intermediate2.gravityLoc, "2", "/masters"),
				params.etcd(intermediates2,
					etcdVersion{installed: "2.0.0", update: "2.5.0"}),
				params.config(updates.masters(), "/etcd"),
				params.runtime(expectedIntermediate2RuntimeAppUpdates, "/config"),
			),
			params.sub("/target", []string{"/2.1.0"},
				params.bootstrap(updates, b.update.gravityLoc),
				params.coreDNS("/bootstrap"),
				params.masters(updates, b.update.gravityLoc, "3", "/coredns"),
				params.nodes(updates, b.update.gravityLoc, "3", "/masters"),
				params.etcd(updates, etcdVersion{installed: "2.5.0", update: "3.0.0"}),
				params.config(updates.masters(), "/etcd"),
				params.runtime(expectedRuntimeAppUpdates, "/config"),
			),
			params.migration("/target"),
			params.app(expectedAppUpdates, "/migration"),
			params.cleanup(),
		},
	}
	if !cmp.Equal(*obtainedPlan, expectedPlan) {
		c.Error("Plans differ:", cmp.Diff(*obtainedPlan, expectedPlan))
	}
}

func (s *PlanSuite) TestReadsExistingDNSConfig(c *check.C) {
	// setup
	runtimeLoc := newLoc("planet:1.0.0")
	runtimeConfigPackage := apptest.Package{
		Loc: newLoc("planet-config:1.0.0"),
		Items: []*archive.Item{
			archive.ItemFromString("vars.json", `{
  "PLANET_DNS_LISTEN_ADDR": "127.0.0.3",
  "PLANET_DNS_PORT": "54"
}`),
		},
		Labels: pack.RuntimeConfigPackageLabels,
	}
	runtimePackage := apptest.Package{
		Loc:    runtimeLoc,
		Labels: utils.CombineLabels(pack.RuntimePackageLabels, pack.InstalledLabels),
	}
	runtimeApp := apptest.RuntimeApplication(newLoc("runtime:1.0.0"), runtimeLoc).
		WithPackageDependencies(runtimePackage, runtimeConfigPackage).
		Build()
	apptest.CreateApplication(apptest.AppRequest{
		App:      runtimeApp,
		Apps:     s.services.Apps,
		Packages: s.services.Packages,
	}, c)

	// verify
	config, err := getExistingDNSConfig(s.services.Packages)
	c.Assert(err, check.IsNil)
	c.Assert(config, check.DeepEquals, &storage.DNSConfig{
		Addrs: []string{"127.0.0.3"},
		Port:  54,
	})
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
			Server: r.planConfig.leadMaster,
		},
	}
}

func (r *params) masters(updates updateServers, gravityPackage loc.Locator, changesetID string, requires ...string) storage.OperationPhase {
	leadMaster := updates.leader()
	otherMasters := updates.otherMasters()
	result := storage.OperationPhase{
		ID:          "/masters",
		Description: "Update master nodes",
		Requires:    requires,
		Phases: []storage.OperationPhase{
			r.leaderMasterPhase("/masters", leadMaster, updates.otherMasters(), gravityPackage, changesetID),
		},
	}
	for _, n := range otherMasters {
		result.Phases = append(result.Phases,
			r.otherMasterPhase(n, "/masters", leadMaster.Server, gravityPackage, changesetID))
	}
	return result
}

func (r *params) leaderMasterPhase(parent string, leadMaster storage.UpdateServer, otherMasters []storage.UpdateServer, gravityPackage loc.Locator, changesetID string) storage.OperationPhase {
	p := func(format string) string {
		return fmt.Sprintf(path.Join(parent, format), leadMaster.Hostname)
	}
	t := func(format string) string {
		return fmt.Sprintf(format, leadMaster.Hostname)
	}
	var phases []storage.OperationPhase
	var requires []string
	if len(otherMasters) != 0 {
		phases = []storage.OperationPhase{
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
		}
		requires = []string{p("%v/stepdown")}
	}
	phases = append(phases, []storage.OperationPhase{
		{
			ID:          p("%v/drain"),
			Executor:    drainNode,
			Description: t("Drain node %q"),
			Data: &storage.OperationPhaseData{
				Server:     &leadMaster.Server,
				ExecServer: &leadMaster.Server,
			},
			Requires: requires,
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
	}...)
	requires = []string{p("%v/system-upgrade")}
	if len(otherMasters) != 0 {
		phases = append(phases, storage.OperationPhase{
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
		})
		requires = []string{p("%v/elect")}
	}
	phases = append(phases, []storage.OperationPhase{
		{
			ID:          p("%v/health"),
			Executor:    nodeHealth,
			Description: t("Health check node %q"),
			Data: &storage.OperationPhaseData{
				Server: &leadMaster.Server,
			},
			Requires: requires,
		},
		{
			ID:          p("%v/taint"),
			Executor:    taintNode,
			Description: t("Taint node %q"),
			Data: &storage.OperationPhaseData{
				Server:     &leadMaster.Server,
				ExecServer: &leadMaster.Server,
			},
			Requires: []string{p("%v/health")},
		},
		{
			ID:          p("%v/uncordon"),
			Executor:    uncordonNode,
			Description: t("Uncordon node %q"),
			Data: &storage.OperationPhaseData{
				Server:     &leadMaster.Server,
				ExecServer: &leadMaster.Server,
			},
			Requires: []string{p("%v/taint")},
		},
	}...)
	requires = []string{p("%v/uncordon")}
	if len(otherMasters) == 0 {
		phases = append(phases, []storage.OperationPhase{
			{
				ID:          p("%v/endpoints"),
				Executor:    endpoints,
				Description: t("Wait for DNS/cluster endpoints on %q"),
				Data: &storage.OperationPhaseData{
					Server:     &leadMaster.Server,
					ExecServer: &leadMaster.Server,
				},
				Requires: []string{p("%v/uncordon")},
			},
		}...)
		requires = []string{p("%v/endpoints")}
	}
	result := storage.OperationPhase{
		ID:          p("%v"),
		Description: t("Update system software on master node %q"),
		Phases: append(phases, storage.OperationPhase{
			ID:          p("%v/untaint"),
			Executor:    untaintNode,
			Description: t("Remove taint from node %q"),
			Data: &storage.OperationPhaseData{
				Server:     &leadMaster.Server,
				ExecServer: &leadMaster.Server,
			},
			Requires: requires,
		}),
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
				ID:          p("%v/taint"),
				Executor:    taintNode,
				Description: t("Taint node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
				Requires: []string{p("%v/health")},
			},
			{
				ID:          p("%v/uncordon"),
				Executor:    uncordonNode,
				Description: t("Uncordon node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
				Requires: []string{p("%v/taint")},
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
			{
				ID:          p("%v/untaint"),
				Executor:    untaintNode,
				Description: t("Remove taint from node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster,
				},
				Requires: []string{p("%v/endpoints")},
			},
		},
	}
}

func (r *params) nodes(updates updateServers, gravityPackage loc.Locator, changesetID string, requires ...string) storage.OperationPhase {
	result := storage.OperationPhase{
		ID:            "/nodes",
		Description:   "Update regular nodes",
		Requires:      requires,
		LimitParallel: numParallelWorkers,
	}
	leadMaster := updates.leader()
	for _, n := range updates.nodes() {
		result.Phases = append(result.Phases,
			r.nodePhase(n, leadMaster, gravityPackage, "/nodes", changesetID))
	}
	return result
}

func (r *params) nodePhase(server storage.UpdateServer, leadMaster storage.UpdateServer, gravityPackage loc.Locator, parent, id string) storage.OperationPhase {
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
					ExecServer: &leadMaster.Server,
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
				ID:          p("%v/taint"),
				Executor:    taintNode,
				Description: t("Taint node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster.Server,
				},
				Requires: []string{p("%v/health")},
			},
			{
				ID:          p("%v/uncordon"),
				Executor:    uncordonNode,
				Description: t("Uncordon node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster.Server,
				},
				Requires: []string{p("%v/taint")},
			},
			{
				ID:          p("%v/endpoints"),
				Executor:    endpoints,
				Description: t("Wait for DNS/cluster endpoints on %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster.Server,
				},
				Requires: []string{p("%v/uncordon")},
			},
			{
				ID:          p("%v/untaint"),
				Executor:    untaintNode,
				Description: t("Remove taint from node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &leadMaster.Server,
				},
				Requires: []string{p("%v/endpoints")},
			},
		},
	}
}

func (r *params) bootstrap(servers []storage.UpdateServer, gravityPackage loc.Locator, requires ...string) storage.OperationPhase {
	root := storage.OperationPhase{
		ID:            "/bootstrap",
		Description:   "Bootstrap update operation on nodes",
		Requires:      requires,
		LimitParallel: numParallelPhases,
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
		ID:            "/bootstrap",
		Description:   "Bootstrap update operation on nodes",
		Requires:      requires,
		LimitParallel: numParallelPhases,
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

func (r params) etcd(updates updateServers, etcd etcdVersion) storage.OperationPhase {
	leadMaster := updates.leader()
	backupNodes := []storage.OperationPhase{r.etcdBackupNode(leadMaster.Server)}
	shutdownNodes := []storage.OperationPhase{r.etcdShutdownNode(leadMaster.Server, true)}
	upgradeNodes := []storage.OperationPhase{r.etcdUpgradeNode(leadMaster.Server)}
	migrateNodes := []storage.OperationPhase{r.etcdMigrateNode(leadMaster.Server, etcd)}
	restartNodes := []storage.OperationPhase{r.etcdRestartLeaderNode(leadMaster.Server)}
	for _, n := range updates.otherMasters() {
		backupNodes = append(backupNodes, r.etcdBackupNode(n.Server))
		shutdownNodes = append(shutdownNodes, r.etcdShutdownNode(n.Server, false))
		upgradeNodes = append(upgradeNodes, r.etcdUpgradeNode(n.Server))
		migrateNodes = append(migrateNodes, r.etcdMigrateNode(n.Server, etcd))
		restartNodes = append(restartNodes, r.etcdRestartNode(n.Server))
	}
	restartNodes = append(restartNodes, r.etcdRestartGravity(leadMaster.Server))
	return storage.OperationPhase{
		ID:          "/etcd",
		Description: fmt.Sprintf("Upgrade etcd %v to %v", etcd.installed, etcd.update),
		Phases: []storage.OperationPhase{
			{
				ID:          "/etcd/backup",
				Description: "Backup etcd data",
				Phases:      backupNodes,
			},
			{
				ID:            "/etcd/shutdown",
				Description:   "Shutdown etcd cluster",
				LimitParallel: etcdNumParallel,
				Phases:        shutdownNodes,
			},
			{
				ID:            "/etcd/upgrade",
				Description:   "Upgrade etcd servers",
				LimitParallel: etcdNumParallel,
				Phases:        upgradeNodes,
			},
			{
				ID:          "/etcd/migrate",
				Description: "Migrate etcd data to new version",
				Phases:      migrateNodes,
			},
			{
				ID:            "/etcd/restart",
				Description:   "Restart etcd servers",
				LimitParallel: etcdNumParallel,
				Phases:        restartNodes,
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
	// FIXME(dima): exercise roles migration step
	return phase
}

func (r params) config(masters updateServers, requires ...string) storage.OperationPhase {
	var configNodes []storage.OperationPhase
	for _, n := range masters {
		configNodes = append(configNodes, r.configNode(n.Server))
	}
	return storage.OperationPhase{
		ID:            "/config",
		Description:   "Update system configuration on nodes",
		Requires:      requires,
		LimitParallel: numParallelPhases,
		Phases:        configNodes,
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

func (r params) runtime(expectedUpdates []loc.Locator, requires ...string) storage.OperationPhase {
	phase := storage.OperationPhase{
		ID:          "/runtime",
		Description: "Update application runtime",
		Requires:    requires,
	}
	var deps []string
	for _, update := range expectedUpdates {
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

func (r params) app(expectedUpdates []loc.Locator, requires ...string) storage.OperationPhase {
	phase := storage.OperationPhase{
		ID:          "/app",
		Description: "Update installed application",
		Requires:    requires,
	}
	for _, update := range expectedUpdates {
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
	var phases []storage.OperationPhase
	for i, s := range r.servers {
		phases = append(phases, storage.OperationPhase{
			ID:          fmt.Sprint("/gc/", s.Hostname),
			Executor:    cleanupNode,
			Description: fmt.Sprintf("Clean up node %q", s.Hostname),
			Data: &storage.OperationPhaseData{
				Server: &r.servers[i],
			},
		})
	}
	return storage.OperationPhase{
		ID:            "/gc",
		Description:   "Run cleanup tasks",
		Requires:      []string{"/app"},
		LimitParallel: numParallelPhases,
		Phases:        phases,
	}
}

type params struct {
	// configuration
	planConfig

	installedRuntimeApp app.Application
	installedApp        app.Application
	updateRuntimeApp    app.Application
	updateApp           app.Application
	teleportLoc         loc.Locator

	// expectations
	gravityPackage loc.Locator
	etcdVersion    etcdVersion
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
	secretsPackage:        newLoc("secrets:0.0.1"),
	runtimeConfigPackage:  newLoc("planet-config:0.0.1"),
	teleportMasterPackage: newLoc("teleport-master-config:0.0.1"),
	teleportNodePackage:   newLoc("teleport-node-config:0.0.1"),
}

type testRotator struct {
	secretsPackage        loc.Locator
	runtimeConfigPackage  loc.Locator
	teleportMasterPackage loc.Locator
	teleportNodePackage   loc.Locator
}

func newWorker(name string) storage.Server {
	return storage.Server{
		AdvertiseIP: name,
		Hostname:    name,
		Role:        "node",
		ClusterRole: string(schema.ServiceRoleNode),
	}
}

func newMaster(name string) storage.Server {
	return storage.Server{
		AdvertiseIP: name,
		Hostname:    name,
		Role:        "node",
		ClusterRole: string(schema.ServiceRoleMaster),
	}
}

func newVer(v string) semver.Version {
	return *semver.New(v)
}

func newLoc(nameVersion string) loc.Locator {
	parts := strings.Split(nameVersion, ":")
	if len(parts) != 2 {
		panic("invalid package reference")
	}
	return loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       parts[0],
		Version:    parts[1],
	}
}

func newPackage(loc loc.Locator) apptest.Package {
	return apptest.Package{
		Loc: loc,
	}
}

func newRuntimePackageWithEtcd(loc loc.Locator, etcdVersion string) apptest.Package {
	return apptest.Package{
		Loc: loc,
		Items: []*archive.Item{
			archive.ItemFromString("orbit.manifest.json", fmt.Sprintf(`{
	"version": "0.0.1",
	"labels": [
		{
			"name": "version-etcd",
			"value": "v%s"
		}
	]
}`, etcdVersion)),
		},
	}
}

func testIDs(id int) idGen {
	return func() string {
		newID := id
		id++
		return fmt.Sprint(newID)
	}
}

func splitServers(servers []storage.UpdateServer) (masters, nodes []storage.UpdateServer) {
	for _, server := range servers {
		switch server.ClusterRole {
		case string(schema.ServiceRoleMaster):
			masters = append(masters, server)
		case string(schema.ServiceRoleNode):
			nodes = append(nodes, server)
		}
	}
	return masters, nodes
}

const (
	numParallelPhases  = 3
	numParallelWorkers = 4
)

var dnsConfig = storage.DNSConfig{
	Addrs: []string{"127.0.0.3"},
	Port:  54,
}

func newClusterBuilder(services appservice.TestServices) *clusterBuilder {
	installedRuntimeLoc := newLoc("planet:1.0.0")
	installedRuntimeAppLoc := newLoc("runtime:1.0.0")
	updateRuntimeLoc := newLoc("planet:3.0.0")
	// No etcd update by default
	etcdVersion := etcdVersion{
		installed: "1.0.0",
		update:    "1.0.0",
	}
	return &clusterBuilder{
		installed: clusterConfig{
			appLoc:         newLoc("app:1.0.0"),
			runtimeAppLoc:  installedRuntimeAppLoc,
			runtimeLoc:     installedRuntimeLoc,
			runtimePackage: newRuntimePackageWithEtcd(installedRuntimeLoc, etcdVersion.installed),
			gravityLoc:     newLoc("gravity:1.0.0"),
			teleportLoc:    newLoc("teleport:1.0.0"),
		},
		update: clusterConfig{
			appLoc:         newLoc("app:3.0.0"),
			runtimeAppLoc:  newLoc("runtime:3.0.0"),
			runtimeLoc:     updateRuntimeLoc,
			runtimePackage: newRuntimePackageWithEtcd(updateRuntimeLoc, etcdVersion.update),
			gravityLoc:     newLoc("gravity:3.0.0"),
			teleportLoc:    newLoc("teleport:3.0.0"),
		},
		links: []storage.OpsCenterLink{
			{
				Hostname:   "ops.example.com",
				Type:       storage.OpsCenterRemoteAccessLink,
				RemoteAddr: "ops.example.com:3024",
				APIURL:     "https://ops.example.com:32009",
				Enabled:    true,
			},
		},
		etcdVersion: etcdVersion,
		serviceUser: storage.OSUser{Name: "user", UID: "1000", GID: "1000"},
		userConfig:  UserConfig{ParallelWorkers: numParallelWorkers},
		id:          func() string { return "id" },
		services:    services,
		directUpgradeVersions: versions.Versions{
			newVer(installedRuntimeAppLoc.Version),
		},
		upgradeViaVersions: make(map[semver.Version]versions.Versions),
	}
}

type clusterBuilder struct {
	installed             clusterConfig
	update                clusterConfig
	intermediates         []intermediateConfigStep
	id                    idGen
	serviceUser           storage.OSUser
	userConfig            UserConfig
	services              appservice.TestServices
	etcdVersion           etcdVersion
	leader                storage.Server
	directUpgradeVersions versions.Versions
	upgradeViaVersions    map[semver.Version]versions.Versions

	servers []storage.Server
	links   []storage.OpsCenterLink
}

func (r *clusterBuilder) withServers(servers []storage.Server, leader storage.Server) *clusterBuilder {
	r.servers = servers
	r.leader = leader
	return r
}

func (r *clusterBuilder) withInstalledApp(appLoc, runtimeAppLoc loc.Locator) *clusterBuilder {
	r.installed.appLoc = appLoc
	r.installed.runtimeAppLoc = runtimeAppLoc
	return r
}

func (r *clusterBuilder) withIntermediateStep(s intermediateConfigStep) *clusterBuilder {
	r.directUpgradeVersions = versions.Versions{
		// Can only go directly from the last intermediate version
		newVer(s.runtimeAppLoc.Version),
	}
	from := newVer(r.installed.runtimeAppLoc.Version)
	if len(r.intermediates) != 0 {
		from = newVer(r.intermediates[len(r.intermediates)-1].runtimeAppLoc.Version)
	}
	r.upgradeViaVersions[from] = versions.Versions{
		newVer(s.runtimeAppLoc.Version),
	}
	r.intermediates = append(r.intermediates, s)
	return r
}

func (r *clusterBuilder) withUpdateApp(appLoc, runtimeAppLoc loc.Locator) *clusterBuilder {
	r.update.appLoc = appLoc
	r.update.runtimeAppLoc = runtimeAppLoc
	return r
}

func (r *clusterBuilder) withInstalledRuntimeDependencies(apps ...apptest.App) *clusterBuilder {
	r.installed.runtimeApps = apps
	return r
}

func (r *clusterBuilder) withInstalledDependencies(apps ...apptest.App) *clusterBuilder {
	r.installed.apps = apps
	return r
}

func (r *clusterBuilder) withUpdateRuntimeDependencies(apps ...apptest.App) *clusterBuilder {
	r.update.runtimeApps = apps
	return r
}

func (r *clusterBuilder) withUpdateDependencies(apps ...apptest.App) *clusterBuilder {
	r.update.apps = apps
	return r
}

func (r *clusterBuilder) withChangesetIDFrom(startID int) *clusterBuilder {
	r.id = testIDs(startID)
	return r
}

func (r *clusterBuilder) withEtcdUpdate(v etcdVersion) *clusterBuilder {
	r.etcdVersion = v
	r.installed.runtimePackage = newRuntimePackageWithEtcd(r.installed.runtimeLoc, v.installed)
	r.update.runtimePackage = newRuntimePackageWithEtcd(r.update.runtimeLoc, v.update)
	return r
}

func (r *clusterBuilder) withEmptyUpdate() *clusterBuilder {
	r.update = r.installed
	return r
}

func (r *clusterBuilder) updateServers() updateServers {
	result := make(updateServers, 0, len(r.servers))
	for _, s := range r.servers {
		result = append(result, r.newUpdateServer(s))
	}
	sort.Slice(result, func(i, j int) bool {
		// Push leader to the front
		return result[i].AdvertiseIP == r.leader.AdvertiseIP
	})
	return result
}

func (r *clusterBuilder) intermediateServers(idx int) updateServers {
	runtimeLoc, teleportLoc := r.installed.runtimeLoc, r.installed.teleportLoc
	if idx != 0 {
		prev := r.intermediates[idx-1]
		runtimeLoc, teleportLoc = prev.runtimeLoc, prev.teleportLoc
	}
	return r.intermediates[idx].servers(r.servers, r.leader, runtimeLoc, teleportLoc)
}

func (r intermediateConfigStep) servers(ss []storage.Server, leader storage.Server, installedRuntimeLoc, installedTeleportLoc loc.Locator) updateServers {
	result := make(updateServers, 0, len(ss))
	for _, s := range ss {
		result = append(result, r.newUpdateServer(s,
			installedRuntimeLoc, installedTeleportLoc))
	}
	sort.Slice(result, func(i, j int) bool {
		// Push leader to the front
		return result[i].AdvertiseIP == leader.AdvertiseIP
	})
	return result
}

func (r intermediateConfigStep) newUpdateServer(s storage.Server, installedRuntimeLoc, installedTeleportLoc loc.Locator) storage.UpdateServer {
	return storage.UpdateServer{
		Server: s,
		Runtime: storage.RuntimePackage{
			Installed:      installedRuntimeLoc,
			SecretsPackage: &testOperator.secretsPackage,
			Update: &storage.RuntimeUpdate{
				Package:       r.runtimeLoc,
				ConfigPackage: testOperator.runtimeConfigPackage,
			},
		},
		Teleport: storage.TeleportPackage{
			Installed: installedTeleportLoc,
			Update: &storage.TeleportUpdate{
				Package:           r.teleportLoc,
				NodeConfigPackage: &testOperator.teleportNodePackage,
			},
		},
	}
}

func (r updateServers) leader() storage.UpdateServer {
	return r[0]
}

func (r updateServers) masters() []storage.UpdateServer {
	masters, _ := splitServers(r)
	return masters
}

func (r updateServers) otherMasters() []storage.UpdateServer {
	masters, _ := splitServers(r)
	return masters[1:]
}

func (r updateServers) nodes() []storage.UpdateServer {
	_, nodes := splitServers(r)
	return nodes
}

type updateServers []storage.UpdateServer

func (r clusterBuilder) newUpdateServer(s storage.Server) storage.UpdateServer {
	installedRuntimeLoc := r.installed.runtimeApp.Manifest.SystemOptions.Dependencies.Runtime.Locator
	installedTeleportLoc := r.installed.teleportLoc
	if len(r.intermediates) != 0 {
		last := r.intermediates[len(r.intermediates)-1]
		installedRuntimeLoc = last.runtimeLoc
		installedTeleportLoc = last.teleportLoc
	}
	return storage.UpdateServer{
		Server: s,
		Runtime: storage.RuntimePackage{
			Installed:      installedRuntimeLoc,
			SecretsPackage: &testOperator.secretsPackage,
			Update: &storage.RuntimeUpdate{
				Package:       r.update.runtimeApp.Manifest.SystemOptions.Dependencies.Runtime.Locator,
				ConfigPackage: testOperator.runtimeConfigPackage,
			},
		},
		Teleport: storage.TeleportPackage{
			Installed: installedTeleportLoc,
			Update: &storage.TeleportUpdate{
				Package:           r.update.teleportLoc,
				NodeConfigPackage: &testOperator.teleportNodePackage,
			},
		},
	}
}

func (r *clusterBuilder) build(c *check.C) params {
	operation := storage.SiteOperation{
		AccountID:  "account-id",
		SiteDomain: "test",
		ID:         "id",
		Type:       ops.OperationUpdate,
		Update: &storage.UpdateOperationState{
			UpdatePackage: r.update.appLoc.String(),
		},
	}
	for i := range r.intermediates {
		r.intermediates[i].build(c, r.services.Apps, r.services.Packages)
	}
	r.installed.build(c, r.services.Apps, r.services.Packages)
	r.update.build(c, r.services.Apps, r.services.Packages, r.intermediates...)
	return params{
		planConfig: planConfig{
			servers:               r.servers,
			apps:                  r.services.Apps,
			packages:              r.services.Packages,
			operator:              testOperator,
			operation:             &operation,
			dnsConfig:             dnsConfig,
			leadMaster:            &r.leader,
			serviceUser:           &r.serviceUser,
			userConfig:            r.userConfig,
			currentEtcdVersion:    newVer(r.etcdVersion.installed),
			links:                 r.links,
			installedApp:          r.installed.appLoc,
			directUpgradeVersions: r.directUpgradeVersions,
			upgradeViaVersions:    r.upgradeViaVersions,
			numParallel:           numParallelPhases,
			newID:                 r.id,
		},
		gravityPackage:      r.update.gravityLoc,
		etcdVersion:         r.etcdVersion,
		installedApp:        *r.installed.app,
		installedRuntimeApp: *r.installed.runtimeApp,
		updateApp:           *r.update.app,
		updateRuntimeApp:    *r.update.runtimeApp,
		teleportLoc:         r.installed.teleportLoc,
	}
}

func (r *clusterConfig) build(c *check.C, apps app.Applications, packages pack.PackageService, steps ...intermediateConfigStep) {
	runtimeApp := apptest.RuntimeApplication(r.runtimeAppLoc, r.runtimeLoc).
		WithPackageDependencies(
			r.runtimePackage, newPackage(r.gravityLoc), newPackage(r.teleportLoc),
		).
		WithAppDependencies(r.runtimeApps...).
		Build()
	clusterApp := apptest.ClusterApplication(r.appLoc, runtimeApp).
		WithAppDependencies(r.apps...).
		Build()
	var versions []schema.IntermediateVersion
	for _, v := range steps {
		versions = append(versions, schema.IntermediateVersion{
			Version:      newVer(v.runtimeAppLoc.Version),
			Dependencies: v.runtimeApp.Manifest.Dependencies,
		})
	}
	clusterApp.Manifest.SystemOptions.Dependencies.IntermediateVersions = versions
	r.app, r.runtimeApp = apptest.CreateApplication(apptest.AppRequest{
		App:      clusterApp,
		Apps:     apps,
		Packages: packages,
	}, c)
}

func (r *intermediateConfigStep) build(c *check.C, apps app.Applications, packages pack.PackageService) {
	runtimeApp := apptest.RuntimeApplication(r.runtimeAppLoc, r.runtimeLoc).
		WithPackageDependencies(
			newRuntimePackageWithEtcd(r.runtimeLoc, r.etcdVersion),
			newPackage(r.gravityLoc), newPackage(r.teleportLoc),
		).
		Build()
	r.runtimeApp, _ = apptest.CreateApplication(apptest.AppRequest{
		App:      runtimeApp,
		Apps:     apps,
		Packages: packages,
	}, c)
}

// apps is a shortcut method to quickly build a list of apptest.App values
// from a list of package locators
func apps(locs ...string) (result []apptest.App) {
	result = make([]apptest.App, 0, len(locs))
	for _, loc := range locs {
		result = append(result, apptest.SystemApplication(newLoc(loc)).Build())
	}
	return result
}

func (r clusterConfig) String() string {
	return fmt.Sprintf("app:%q, runtimeApp:%q, runtimeLoc:%q, gravity:%q, teleport:%q",
		r.appLoc, r.runtimeAppLoc, r.runtimeLoc, r.gravityLoc, r.teleportLoc,
	)
}

type clusterConfig struct {
	appLoc        loc.Locator
	runtimeAppLoc loc.Locator
	runtimeLoc    loc.Locator
	gravityLoc    loc.Locator
	teleportLoc   loc.Locator
	// runtime application dependencies
	runtimeApps []apptest.App
	// direct application dependencies
	apps           []apptest.App
	runtimePackage apptest.Package

	// computables
	app        *app.Application
	runtimeApp *app.Application
}

type intermediateConfigStep struct {
	runtimeAppLoc loc.Locator
	runtimeLoc    loc.Locator
	etcdVersion   string
	gravityLoc    loc.Locator
	teleportLoc   loc.Locator

	// optional intermediate application snapshot,
	// set up only when necessary
	runtimeApp *app.Application
}
