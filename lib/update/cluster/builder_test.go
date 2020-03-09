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
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
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

	params := params{
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
		dnsConfig: storage.DefaultDNSConfig,
		// Use an alternative (other than first) master node as leader
		leadMaster: updates[1],
	}
	config := newTestPlan(c, params)

	// exercise
	obtainedPlan, err := newOperationPlan(config)
	c.Assert(err, check.IsNil)
	update.ResolvePlan(obtainedPlan)

	// verify
	c.Assert(*obtainedPlan, check.DeepEquals, storage.OperationPlan{
		OperationID:    config.operation.ID,
		OperationType:  config.operation.Type,
		AccountID:      config.operation.AccountID,
		ClusterName:    config.operation.SiteDomain,
		Servers:        servers,
		DNSConfig:      storage.DefaultDNSConfig,
		GravityPackage: gravityUpdateLoc,
		Phases: []storage.OperationPhase{
			params.init(),
			params.checks(),
			params.preUpdate(),
			params.coreDNS(),
			params.bootstrap(),
			params.masters(updates[0:1]),
			params.nodes(),
			params.etcd(updates[0:1]),
			params.migration(),
			params.config(),
			params.runtime(),
			params.app("/runtime"),
			params.cleanup(),
		},
	})
}

func (s *PlanSuite) TestPlanWithoutRuntimeUpdate(c *check.C) {
	// setup
	runtimeLoc1 := loc.MustParseLocator("gravitational.io/runtime:1.0.0")
	appLoc1 := loc.MustParseLocator("gravitational.io/app:1.0.0")
	appLoc2 := loc.MustParseLocator("gravitational.io/app:2.0.0")

	params := params{
		installedRuntime:         runtimeLoc1,
		installedApp:             appLoc1,
		updateRuntime:            runtimeLoc1, // same runtime on purpose
		updateApp:                appLoc2,
		installedRuntimeManifest: installedRuntimeManifest,
		installedAppManifest:     installedAppManifest,
		updateRuntimeManifest:    installedRuntimeManifest, // same manifest on purpose
		updateAppManifest:        updateAppManifest,
		dnsConfig:                storage.DefaultDNSConfig,
		leadMaster:               updates[0],
	}
	config := newTestPlan(c, params)

	// exercise
	obtainedPlan, err := newOperationPlan(config)
	c.Assert(err, check.IsNil)
	update.ResolvePlan(obtainedPlan)

	// verify
	c.Assert(*obtainedPlan, check.DeepEquals, storage.OperationPlan{
		OperationID:    config.operation.ID,
		OperationType:  config.operation.Type,
		AccountID:      config.operation.AccountID,
		ClusterName:    config.operation.SiteDomain,
		Servers:        servers,
		DNSConfig:      storage.DefaultDNSConfig,
		GravityPackage: gravityInstalledLoc,
		Phases: []storage.OperationPhase{
			params.init(),
			params.checks(),
			params.preUpdate(),
			params.app("/pre-update"),
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

func newTestPlan(c *check.C, params params) planConfig {
	config := planConfig{
		operator:  testOperator,
		operation: operation,
		servers:   updates,
		installedRuntime: app.Application{
			Package:  params.installedRuntime,
			Manifest: schema.MustParseManifestYAML([]byte(params.installedRuntimeManifest)),
			PackageEnvelope: pack.PackageEnvelope{
				Manifest: []byte(params.installedRuntimeManifest),
			},
		},
		installedApp: app.Application{
			Package:  params.installedApp,
			Manifest: schema.MustParseManifestYAML([]byte(params.installedAppManifest)),
			PackageEnvelope: pack.PackageEnvelope{
				Manifest: []byte(params.installedAppManifest),
			},
		},
		updateRuntime: app.Application{
			Package:  params.updateRuntime,
			Manifest: schema.MustParseManifestYAML([]byte(params.updateRuntimeManifest)),
			PackageEnvelope: pack.PackageEnvelope{
				Manifest: []byte(params.updateRuntimeManifest),
			},
		},
		updateApp: app.Application{
			Package:  params.updateApp,
			Manifest: schema.MustParseManifestYAML([]byte(params.updateAppManifest)),
			PackageEnvelope: pack.PackageEnvelope{
				Manifest: []byte(params.updateAppManifest),
			},
		},
		links:            params.links,
		trustedClusters:  params.trustedClusters,
		shouldUpdateEtcd: shouldUpdateEtcdTest,
		updateCoreDNS:    params.updateCoreDNS,
		leadMaster:       params.leadMaster,
	}
	gravityPackage, err := config.updateRuntime.Manifest.Dependencies.ByName(
		constants.GravityPackage)
	c.Assert(err, check.IsNil)
	config.plan = storage.OperationPlan{
		OperationID:    operation.ID,
		OperationType:  operation.Type,
		AccountID:      operation.AccountID,
		ClusterName:    operation.SiteDomain,
		Servers:        servers,
		GravityPackage: *gravityPackage,
		DNSConfig:      params.dnsConfig,
	}
	return config
}

func (r *params) init() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/init",
		Executor:    updateInit,
		Description: "Initialize update operation",
		Data: &storage.OperationPhaseData{
			Package:          &r.updateApp,
			ExecServer:       &r.leadMaster.Server,
			InstalledPackage: &r.installedApp,
			Update: &storage.UpdateOperationData{
				Servers: updates,
			},
		},
	}
}

func (r *params) checks() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/checks",
		Executor:    updateChecks,
		Description: "Run preflight checks",
		Requires:    []string{"/init"},
		Data: &storage.OperationPhaseData{
			Package:          &r.updateApp,
			InstalledPackage: &r.installedApp,
		},
	}
}

func (r *params) preUpdate() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/pre-update",
		Executor:    preUpdate,
		Description: "Run pre-update application hook",
		Requires:    []string{"/init"},
		Data: &storage.OperationPhaseData{
			Package: &r.updateApp,
		},
	}
}

func (r *params) coreDNS() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/coredns",
		Description: "Provision CoreDNS resources",
		Executor:    coredns,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster.Server,
		},
	}
}

func (r *params) bootstrap() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/bootstrap",
		Description: "Bootstrap update operation on nodes",
		Requires:    []string{"/init"},
		Phases: []storage.OperationPhase{
			{
				ID:          "/bootstrap/node-1",
				Executor:    updateBootstrap,
				Description: `Bootstrap node "node-1"`,
				Data: &storage.OperationPhaseData{
					ExecServer:       &servers[0],
					Package:          &r.updateApp,
					InstalledPackage: &r.installedApp,
					Update: &storage.UpdateOperationData{
						Servers: updates[0:1],
					},
				},
			},
			{
				ID:          "/bootstrap/node-2",
				Executor:    updateBootstrap,
				Description: `Bootstrap node "node-2"`,
				Data: &storage.OperationPhaseData{
					ExecServer:       &servers[1],
					Package:          &r.updateApp,
					InstalledPackage: &r.installedApp,
					Update: &storage.UpdateOperationData{
						Servers: updates[1:2],
					},
				},
			},
			{
				ID:          "/bootstrap/node-3",
				Executor:    updateBootstrap,
				Description: `Bootstrap node "node-3"`,
				Data: &storage.OperationPhaseData{
					ExecServer:       &servers[2],
					Package:          &r.updateApp,
					InstalledPackage: &r.installedApp,
					Update: &storage.UpdateOperationData{
						Servers: updates[2:3],
					},
				},
			},
		},
	}
}

func (r *params) masters(otherMasters []storage.UpdateServer) storage.OperationPhase {
	t := func(format string, node storage.UpdateServer) string {
		return fmt.Sprintf(format, node.Hostname)
	}
	return storage.OperationPhase{
		ID:          "/masters",
		Description: "Update master nodes",
		Requires:    []string{"/checks", "/bootstrap", "/pre-update", "/coredns"},
		Phases: []storage.OperationPhase{
			r.leaderMasterPhase(),
			{
				ID:          t("/masters/elect-%v", r.leadMaster),
				Executor:    electionStatus,
				Description: t("Make node %q Kubernetes leader", r.leadMaster),
				Data: &storage.OperationPhaseData{
					Server: &r.leadMaster.Server,
					ElectionChange: &storage.ElectionChange{
						EnableServers:  []storage.Server{r.leadMaster.Server},
						DisableServers: serversToStorage(otherMasters...),
					},
				},
				Requires: []string{t("/masters/%v", r.leadMaster)},
			},
			r.otherMasterPhase(otherMasters[0]),
		},
	}
}

func (r *params) leaderMasterPhase() storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, r.leadMaster.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/masters/%v"),
		Description: t("Update system software on master node %q"),
		Phases: []storage.OperationPhase{
			{
				ID:          t("/masters/%v/kubelet-permissions"),
				Description: t("Add permissions to kubelet on %q"),
				Executor:    kubeletPermissions,
				Data: &storage.OperationPhaseData{
					Server: &r.leadMaster.Server,
				},
			},
			{
				ID:          t("/masters/%[1]v/stepdown-%[1]v"),
				Executor:    electionStatus,
				Description: t("Step down %q as Kubernetes leader"),
				Data: &storage.OperationPhaseData{
					Server: &r.leadMaster.Server,
					ElectionChange: &storage.ElectionChange{
						DisableServers: []storage.Server{r.leadMaster.Server},
					},
				},
				Requires: []string{t("/masters/%v/kubelet-permissions")},
			},
			{
				ID:          t("/masters/%v/drain"),
				Executor:    drainNode,
				Description: t("Drain node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &r.leadMaster.Server,
					ExecServer: &r.leadMaster.Server,
				},
				Requires: []string{t("/masters/%[1]v/stepdown-%[1]v")},
			},
			{
				ID:          t("/masters/%v/system-upgrade"),
				Executor:    updateSystem,
				Description: t("Update system software on node %q"),
				Data: &storage.OperationPhaseData{
					ExecServer: &r.leadMaster.Server,
					Update: &storage.UpdateOperationData{
						Servers: []storage.UpdateServer{r.leadMaster},
					},
				},
				Requires: []string{t("/masters/%v/drain")},
			},
			{
				ID:          t("/masters/%v/uncordon"),
				Executor:    uncordonNode,
				Description: t("Uncordon node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &r.leadMaster.Server,
					ExecServer: &r.leadMaster.Server,
				},
				Requires: []string{t("/masters/%v/system-upgrade")},
			},
		},
	}
}

func (r *params) otherMasterPhase(server storage.UpdateServer) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/masters/%v"),
		Description: t("Update system software on master node %q"),
		Requires:    []string{fmt.Sprintf("/masters/elect-%v", r.leadMaster.Hostname)},
		Phases: []storage.OperationPhase{
			{
				ID:          t("/masters/%v/drain"),
				Executor:    drainNode,
				Description: t("Drain node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &r.leadMaster.Server,
				},
			},
			{
				ID:          t("/masters/%v/system-upgrade"),
				Executor:    updateSystem,
				Description: t("Update system software on node %q"),
				Data: &storage.OperationPhaseData{
					ExecServer: &server.Server,
					Update: &storage.UpdateOperationData{
						Servers: []storage.UpdateServer{server},
					},
				},
				Requires: []string{t("/masters/%v/drain")},
			},
			{
				ID:          t("/masters/%v/uncordon"),
				Executor:    uncordonNode,
				Description: t("Uncordon node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &r.leadMaster.Server,
				},
				Requires: []string{t("/masters/%v/system-upgrade")},
			},
			{
				ID:          t("/masters/%v/endpoints"),
				Executor:    endpoints,
				Description: t("Wait for DNS/cluster endpoints on %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &r.leadMaster.Server,
				},
				Requires: []string{t("/masters/%v/uncordon")},
			},
			{
				ID:          t("/masters/%[1]v/enable-%[1]v"),
				Executor:    electionStatus,
				Description: t("Enable leader election on node %q"),
				Data: &storage.OperationPhaseData{
					Server: &server.Server,
					ElectionChange: &storage.ElectionChange{
						EnableServers: []storage.Server{server.Server},
					},
				},
				Requires: []string{t("/masters/%v/endpoints")},
			},
		},
	}
}

func (r *params) nodes() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/nodes",
		Description: "Update regular nodes",
		Requires:    []string{"/masters"},
		Phases: []storage.OperationPhase{
			r.nodePhase(updates[2]),
		},
	}
}

func (r *params) nodePhase(server storage.UpdateServer) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/nodes/%v"),
		Description: t("Update system software on node %q"),
		Phases: []storage.OperationPhase{
			{
				ID:          t("/nodes/%v/drain"),
				Executor:    drainNode,
				Description: t("Drain node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &r.leadMaster.Server,
				},
			},
			{
				ID:          t("/nodes/%v/system-upgrade"),
				Executor:    updateSystem,
				Description: t("Update system software on node %q"),
				Data: &storage.OperationPhaseData{
					ExecServer: &server.Server,
					Update: &storage.UpdateOperationData{
						Servers: []storage.UpdateServer{server},
					},
				},
				Requires: []string{t("/nodes/%v/drain")},
			},
			{
				ID:          t("/nodes/%v/uncordon"),
				Executor:    uncordonNode,
				Description: t("Uncordon node %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &r.leadMaster.Server,
				},
				Requires: []string{t("/nodes/%v/system-upgrade")},
			},
			{
				ID:          t("/nodes/%v/endpoints"),
				Executor:    endpoints,
				Description: t("Wait for DNS/cluster endpoints on %q"),
				Data: &storage.OperationPhaseData{
					Server:     &server.Server,
					ExecServer: &r.leadMaster.Server,
				},
				Requires: []string{t("/nodes/%v/uncordon")},
			},
		},
	}
}

func (r params) etcd(otherMasters []storage.UpdateServer) storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/etcd",
		Description: "Upgrade etcd 1.0.0 to 2.0.0",
		Phases: []storage.OperationPhase{
			{
				ID:          "/etcd/backup",
				Description: "Backup etcd data",
				Phases: []storage.OperationPhase{
					r.etcdBackupNode(r.leadMaster),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdBackupNode(otherMasters[0]),
				},
			},
			{
				ID:          "/etcd/shutdown",
				Description: "Shutdown etcd cluster",
				Phases: []storage.OperationPhase{
					r.etcdShutdownNode(r.leadMaster, true),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdShutdownNode(otherMasters[0], false),
					r.etcdShutdownWorkerNode(updates[2]),
				},
			},
			{
				ID:          "/etcd/upgrade",
				Description: "Upgrade etcd servers",
				Phases: []storage.OperationPhase{
					r.etcdUpgradeNode(r.leadMaster),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdUpgradeNode(otherMasters[0]),
					// upgrade regular nodes
					r.etcdUpgradeNode(updates[2]),
				},
			},
			{
				ID:          "/etcd/restore",
				Description: "Restore etcd data from backup",
				Executor:    updateEtcdRestore,
				Data: &storage.OperationPhaseData{
					Server: &r.leadMaster.Server,
				},
				Requires: []string{"/etcd/upgrade"},
			},
			{
				ID:          "/etcd/restart",
				Description: "Restart etcd servers",
				Phases: []storage.OperationPhase{
					r.etcdRestartLeaderNode(),
					// FIXME: assumes len(otherMasters) == 1
					r.etcdRestartNode(otherMasters[0]),
					// upgrade regular nodes
					r.etcdRestartNode(updates[2]),
					r.etcdRestartGravity(),
				},
			},
		},
	}
}

func (r params) etcdBackupNode(server storage.UpdateServer) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/backup/%v"),
		Description: t("Backup etcd on node %q"),
		Executor:    updateEtcdBackup,
		Data: &storage.OperationPhaseData{
			Server: &server.Server,
		},
	}
}

func (r params) etcdShutdownNode(server storage.UpdateServer, isLeader bool) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/shutdown/%v"),
		Description: t("Shutdown etcd on node %q"),
		Executor:    updateEtcdShutdown,
		Requires:    []string{t("/etcd/backup/%v")},
		Data: &storage.OperationPhaseData{
			Server: &server.Server,
			Data:   strconv.FormatBool(isLeader),
		},
	}
}

func (r params) etcdShutdownWorkerNode(server storage.UpdateServer) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/shutdown/%v"),
		Description: t("Shutdown etcd on node %q"),
		Executor:    updateEtcdShutdown,
		Data: &storage.OperationPhaseData{
			Server: &server.Server,
			Data:   "false",
		},
	}
}

func (r params) etcdUpgradeNode(server storage.UpdateServer) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/upgrade/%v"),
		Description: t("Upgrade etcd on node %q"),
		Executor:    updateEtcdMaster,
		Requires:    []string{t("/etcd/shutdown/%v")},
		Data: &storage.OperationPhaseData{
			Server: &server.Server,
		},
	}
}

func (r params) etcdRestartLeaderNode() storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, r.leadMaster.Hostname)
	}
	return storage.OperationPhase{
		ID:          t("/etcd/restart/%v"),
		Description: t("Restart etcd on node %q"),
		Executor:    updateEtcdRestart,
		Requires:    []string{"/etcd/restore"},
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster.Server,
			Master: &r.leadMaster.Server,
		},
	}
}

func (r params) etcdRestartNode(server storage.UpdateServer) storage.OperationPhase {
	t := func(format string) string {
		return fmt.Sprintf(format, server.Hostname)
	}

	return storage.OperationPhase{
		ID:          t("/etcd/restart/%v"),
		Description: t("Restart etcd on node %q"),
		Executor:    updateEtcdRestart,
		Requires:    []string{t("/etcd/upgrade/%v")},
		Data: &storage.OperationPhaseData{
			Server: &server.Server,
			Master: &r.leadMaster.Server,
		},
	}
}

func (r params) etcdRestartGravity() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          fmt.Sprint("/etcd/restart/", constants.GravityServiceName),
		Description: fmt.Sprint("Restart ", constants.GravityServiceName, " service"),
		Executor:    updateEtcdRestartGravity,
		Data: &storage.OperationPhaseData{
			Server: &r.leadMaster.Server,
		},
	}
}

func (r *params) migration() storage.OperationPhase {
	phase := storage.OperationPhase{
		ID:          "/migration",
		Description: "Perform system database migration",
		Requires:    []string{"/etcd"},
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

func (r params) config() storage.OperationPhase {
	return storage.OperationPhase{
		ID:          "/config",
		Description: "Update system configuration on nodes",
		Requires:    []string{"/masters"},
		Phases: []storage.OperationPhase{
			{
				ID:          "/config/node-1",
				Executor:    config,
				Description: `Update system configuration on node "node-1"`,
				Data: &storage.OperationPhaseData{
					Server: &servers[0],
				},
			},
			{
				ID:          "/config/node-2",
				Executor:    config,
				Description: `Update system configuration on node "node-2"`,
				Data: &storage.OperationPhaseData{
					Server: &servers[1],
				},
			},
		},
	}
}

func (r params) runtime() storage.OperationPhase {
	rbacLoc := loc.MustParseLocator("gravitational.io/rbac-app:2.0.0")
	runtimeDepLoc := loc.MustParseLocator("gravitational.io/runtime-dep-2:2.0.0")
	runtimeLoc := loc.MustParseLocator("gravitational.io/runtime:2.0.0")
	return storage.OperationPhase{
		ID:          "/runtime",
		Description: "Update application runtime",
		Requires:    []string{"/masters"},
		Phases: []storage.OperationPhase{
			{
				ID:          "/runtime/rbac-app",
				Executor:    updateApp,
				Description: `Update system application "rbac-app" to 2.0.0`,
				Data: &storage.OperationPhaseData{
					Package: &rbacLoc,
				},
			},
			{
				ID:          "/runtime/runtime-dep-2",
				Executor:    updateApp,
				Description: `Update system application "runtime-dep-2" to 2.0.0`,
				Data: &storage.OperationPhaseData{
					Package: &runtimeDepLoc,
				},
				Requires: []string{"/runtime/rbac-app"},
			},
			{
				ID:          "/runtime/runtime",
				Executor:    updateApp,
				Description: `Update system application "runtime" to 2.0.0`,
				Data: &storage.OperationPhaseData{
					Package: &runtimeLoc,
				},
				Requires: []string{"/runtime/runtime-dep-2"},
			},
		},
	}
}

func (r params) app(requires ...string) storage.OperationPhase {
	appLoc := loc.MustParseLocator("gravitational.io/app:2.0.0")
	appDepLoc := loc.MustParseLocator("gravitational.io/app-dep-2:2.0.0")
	return storage.OperationPhase{
		ID:          "/app",
		Description: "Update installed application",
		Requires:    requires,
		Phases: []storage.OperationPhase{
			{
				ID:          "/app/app-dep-2",
				Executor:    updateApp,
				Description: `Update application "app-dep-2" to 2.0.0`,
				Data: &storage.OperationPhaseData{
					Package: &appDepLoc,
				},
			},
			{
				ID:          "/app/app",
				Executor:    updateApp,
				Description: `Update application "app" to 2.0.0`,
				Data: &storage.OperationPhaseData{
					Package: &appLoc,
				},
			},
		},
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
					Server: &updates[0].Server,
				},
			},
			{
				ID:          "/gc/node-2",
				Executor:    cleanupNode,
				Description: `Clean up node "node-2"`,
				Data: &storage.OperationPhaseData{
					Server: &updates[1].Server,
				},
			},
			{
				ID:          "/gc/node-3",
				Executor:    cleanupNode,
				Description: `Clean up node "node-3"`,
				Data: &storage.OperationPhaseData{
					Server: &updates[2].Server,
				},
			},
		},
	}
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
	leadMaster               storage.UpdateServer
	dnsConfig                storage.DNSConfig
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

var runtimePackage = storage.RuntimePackage{
	Update: &storage.RuntimeUpdate{
		Package: loc.MustParseLocator("gravitational.io/planet:2.0.0"),
	},
}
var gravityInstalledLoc = loc.MustParseLocator("gravitational.io/gravity:1.0.0")
var gravityUpdateLoc = loc.MustParseLocator("gravitational.io/gravity:2.0.0")
var servers = []storage.Server{
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

var updates = []storage.UpdateServer{
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

var operation = storage.SiteOperation{
	AccountID:  "000",
	SiteDomain: "test",
	ID:         "123",
	Type:       ops.OperationUpdate,
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
