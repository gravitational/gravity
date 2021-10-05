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

package cluster

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"sort"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update/cluster/phases"
	"github.com/gravitational/gravity/lib/update/cluster/versions"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/teleport/lib/services"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// UserConfig holds configuration parameters provided by the user triggering the update operation.
type UserConfig struct {
	ParallelWorkers int
	SkipWorkers     bool
}

// InitOperationPlan will initialize operation plan for an operation
func InitOperationPlan(
	ctx context.Context,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	opKey ops.SiteOperationKey,
	leader *storage.Server,
	userConfig UserConfig,
) (*storage.OperationPlan, error) {
	operation, err := storage.GetOperationByID(clusterEnv.Backend, opKey.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if operation.Type != ops.OperationUpdate {
		return nil, trace.BadParameter("expected update operation but got %q", operation.Type)
	}

	plan, err := clusterEnv.Backend.GetOperationPlan(operation.SiteDomain, operation.ID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if plan != nil {
		return nil, trace.AlreadyExists("plan is already initialized")
	}

	cluster, err := clusterEnv.Operator.GetLocalSite(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dnsConfig := cluster.DNSConfig
	if dnsConfig.IsEmpty() {
		logrus.Info("Detecting DNS configuration.")
		existingDNS, err := getExistingDNSConfig(localEnv.Packages)
		if err != nil {
			return nil, trace.Wrap(err, "failed to determine existing cluster DNS configuration")
		}
		dnsConfig = *existingDNS
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	currentEtcdVersion, err := getCurrentEtcdVersion(ctx, stateDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logrus.WithField("version", currentEtcdVersion.String()).Info("Current etcd version.")

	servers, err := storage.GetLocalServers(clusterEnv.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err = checkAndSetServerDefaults(servers, clusterEnv.Client.CoreV1().Nodes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	links, err := clusterEnv.Backend.GetOpsCenterLinks(operation.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := clusterEnv.Backend.GetRoles()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedApp, err := storage.GetLocalPackage(clusterEnv.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trustedClusters, err := clusterEnv.Backend.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan, err = newOperationPlan(ctx, planConfig{
		apps:               clusterEnv.Apps,
		packages:           clusterEnv.ClusterPackages,
		dnsConfig:          dnsConfig,
		operator:           clusterEnv.Operator,
		operation:          operation,
		leadMaster:         leader,
		userConfig:         userConfig,
		serviceUser:        &cluster.ServiceUser,
		currentEtcdVersion: *currentEtcdVersion,
		servers:            servers,
		links:              links,
		roles:              roles,
		trustedClusters:    trustedClusters,
		installedApp:       *installedApp,
		numParallel:        NumParallel(),
		newID:              newUUID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clusterEnv.Backend.CreateOperationPlan(*plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return plan, nil
}

// newOperationPlan generates a new plan for the provided operation
func newOperationPlan(ctx context.Context, config planConfig) (*storage.OperationPlan, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	installedApp, err := config.apps.GetApp(config.installedApp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedRuntimeApp, err := config.apps.GetApp(*(installedApp.Manifest.Base()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installedRuntimeAppVersion, err := installedRuntimeApp.Package.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updatePackage, err := config.operation.Update.Package()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateApp, err := config.apps.GetApp(*updatePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateRuntimeApp, err := config.apps.GetApp(*(updateApp.Manifest.Base()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateRuntimeAppVersion, err := updateRuntimeApp.Package.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedTeleport, err := installedApp.Manifest.Dependencies.ByName(constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateTeleport, err := updateApp.Manifest.Dependencies.ByName(constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gravityPackage, err := updateRuntimeApp.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedDeps, err := app.GetDirectApplicationDependencies(*installedApp)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateDeps, err := app.GetDirectApplicationDependencies(*updateApp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appUpdates, err := loc.GetUpdatedDependencies(
		installedDeps,
		updateDeps)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	builder := phaseBuilder{
		planConfig:                 config,
		gravityPackage:             *gravityPackage,
		appUpdates:                 appUpdates,
		installedApp:               *installedApp,
		updateApp:                  *updateApp,
		installedRuntimeApp:        *installedRuntimeApp,
		installedRuntimeAppVersion: *installedRuntimeAppVersion,
		updateRuntimeApp:           *updateRuntimeApp,
		updateRuntimeAppVersion:    *updateRuntimeAppVersion,
		installedTeleport:          *installedTeleport,
		updateTeleport:             *updateTeleport,
	}

	err = builder.initSteps(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return builder.newPlan(), nil
}

func (r *planConfig) checkAndSetDefaults() error {
	if r.apps == nil {
		return trace.BadParameter("application service is required")
	}
	if r.packages == nil {
		return trace.BadParameter("package service is required")
	}
	if r.operator == nil {
		return trace.BadParameter("cluster operator is required")
	}
	if r.operation == nil {
		return trace.BadParameter("cluster operation is required")
	}
	if r.leadMaster == nil {
		return trace.BadParameter("operation leader node is required")
	}
	if r.serviceUser == nil {
		return trace.BadParameter("cluster service user is required")
	}
	return nil
}

// planConfig defines the configuration for creating a new operation plan
type planConfig struct {
	// packages specifies the cluster package service
	packages pack.PackageService
	// apps specifies the cluster application service
	apps app.Applications
	// operator specifies the cluster service operator
	operator phases.PackageRotator
	// dnsConfig specifies the cluster DNS configuration
	dnsConfig storage.DNSConfig
	// operation specifies the operation to generate the plan for
	operation *storage.SiteOperation
	// leadMaster specifies the server to execute the upgrade operation on
	leadMaster *storage.Server
	// serviceUser specifies the cluster's service user
	serviceUser *storage.OSUser
	// userConfig combines operation-specific custom configuration
	userConfig UserConfig
	// currentEtcdVersion specifies the current version of etcd
	currentEtcdVersion semver.Version
	// servers lists the cluster servers
	servers []storage.Server
	// links optionally lists additional remote hub references
	links []storage.OpsCenterLink
	// roles lists cluster roles and permissions to migrate
	roles []services.Role
	// trustedClusters lists trusted clusters connected to this cluster
	trustedClusters []services.TrustedCluster
	// installedApp identifies the installed cluster application
	installedApp loc.Locator
	// directUpgradeVersions optionally specifies custom list of versions
	// we can upgrade directly. If unset, versions.DirectUpgradeVersions
	// is used.
	directUpgradeVersions versions.Versions
	// upgradeViaVersions optionally specifies custom version mapping in case
	// no direct upgrade is possible. If unset, versions.UpgradeViaVersions
	// is used.
	upgradeViaVersions map[semver.Version]versions.Versions
	// numParallel limits the number of concurrent sub-phase invocations
	numParallel int
	// newID generates new changeset IDs
	newID idGen
}

// configUpdates computes the configuration updates for the specified list of servers
func (r phaseBuilder) configUpdates(
	installedTeleport loc.Locator,
	installedRuntimeFunc, updateRuntimeFunc runtimePackageGetterFunc,
) (updates []storage.UpdateServer, err error) {
	for _, server := range r.planConfig.servers {
		installedRuntime, err := installedRuntimeFunc(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateServer := storage.UpdateServer{
			Server: server,
			Runtime: storage.RuntimePackage{
				Installed: *installedRuntime,
			},
			Teleport: storage.TeleportPackage{
				Installed: installedTeleport,
			},
		}
		needsPlanetUpdate, needsTeleportUpdate, err := systemNeedsUpdate(
			server.Role, server.ClusterRole,
			r.installedApp.Manifest, r.updateApp.Manifest,
			installedTeleport, r.updateTeleport)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if needsPlanetUpdate {
			updateRuntime, err := updateRuntimeFunc(server)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			secretsUpdate, err := r.operator.RotateSecrets(ops.RotateSecretsRequest{
				Key:            (ops.SiteOperation)(*r.operation).ClusterKey(),
				Server:         server,
				RuntimePackage: *updateRuntime,
				DryRun:         true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			configUpdate, err := r.operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
				Key:            (ops.SiteOperation)(*r.operation).Key(),
				Server:         server,
				Manifest:       r.updateApp.Manifest,
				RuntimePackage: *updateRuntime,
				DryRun:         true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Runtime.SecretsPackage = &secretsUpdate.Locator
			updateServer.Runtime.Update = &storage.RuntimeUpdate{
				Package:       *updateRuntime,
				ConfigPackage: configUpdate.Locator,
			}
		}
		if needsTeleportUpdate {
			_, nodeConfig, err := r.operator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
				Key:             (ops.SiteOperation)(*r.operation).Key(),
				Server:          server,
				TeleportPackage: r.updateTeleport,
				DryRun:          true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Teleport.Update = &storage.TeleportUpdate{
				Package: r.updateTeleport,
			}
			if nodeConfig != nil {
				updateServer.Teleport.Update.NodeConfigPackage = &nodeConfig.Locator
			}
		}
		updates = append(updates, updateServer)
	}
	return updates, nil
}

func checkAndSetServerDefaults(servers []storage.Server, client corev1.NodeInterface) ([]storage.Server, error) {
	nodes, err := utils.GetNodes(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	masterIPs := utils.GetMasters(nodes)
	// set cluster role that might have not have been set
L:
	for i, server := range servers {
		if utils.StringInSlice(masterIPs, server.AdvertiseIP) {
			servers[i].ClusterRole = string(schema.ServiceRoleMaster)
		} else {
			servers[i].ClusterRole = string(schema.ServiceRoleNode)
		}

		// Check that we're able to locate the node in the kubernetes node list
		node, ok := nodes[server.AdvertiseIP]
		if !ok {
			// The server is missing it's advertise-ip label,
			// however, if we're able to match the Nodename, our internal state is likely correct
			// and we can continue without trying to repair the Nodename
			for _, node := range nodes {
				if node.Name == server.Nodename {
					continue L
				}
			}

			return nil, trace.NotFound("unable to locate kubernetes node with label %s=%s,"+
				" please check each kubernetes node and re-add the %v label if it is missing",
				defaults.KubernetesAdvertiseIPLabel,
				server.AdvertiseIP,
				defaults.KubernetesAdvertiseIPLabel)
		}
		// Store the name of the kubernetes node in case it has been left unspecified
		servers[i].Nodename = node.Name
	}
	return servers, nil
}

func getExistingDNSConfig(packages pack.PackageService) (*storage.DNSConfig, error) {
	_, configPackage, err := pack.FindAnyRuntimePackageWithConfig(packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, rc, err := packages.ReadPackage(*configPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer rc.Close()
	var configBytes []byte
	err = archive.TarGlob(tar.NewReader(rc), "", []string{"vars.json"}, func(_ string, r io.Reader) error {
		configBytes, err = ioutil.ReadAll(r)
		if err != nil {
			return trace.Wrap(err)
		}

		return archive.ErrAbort
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var runtimeConfig runtimeConfig
	if configBytes != nil {
		err = json.Unmarshal(configBytes, &runtimeConfig)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	dnsPort := defaults.DNSPort
	if len(runtimeConfig.DNSPort) != 0 {
		dnsPort, err = strconv.Atoi(runtimeConfig.DNSPort)
		if err != nil {
			return nil, trace.Wrap(err, "expected integer value but got %v", runtimeConfig.DNSPort)
		}
	}
	var dnsAddrs []string
	if runtimeConfig.DNSListenAddr != "" {
		dnsAddrs = append(dnsAddrs, runtimeConfig.DNSListenAddr)
	}
	dnsConfig := &storage.DNSConfig{
		Addrs: dnsAddrs,
		Port:  dnsPort,
	}
	if dnsConfig.IsEmpty() {
		*dnsConfig = storage.LegacyDNSConfig
	}
	logrus.Infof("Detected DNS configuration: %v.", dnsConfig)
	return dnsConfig, nil
}

// systemNeedsUpdate determines whether planet or teleport services need
// to be updated by comparing versions of respective packages in the
// installed and update application manifest
// FIXME(dmitri): should consider runtime update if runtime applications have changed
// between versions
func systemNeedsUpdate(
	profile, clusterRole string,
	installed, update schema.Manifest,
	installedTeleportPackage, updateTeleportPackage loc.Locator,
) (planetNeedsUpdate, teleportNeedsUpdate bool, err error) {
	updateProfile, err := update.NodeProfiles.ByName(profile)
	if err != nil {
		return false, false, trace.Wrap(err)
	}
	updateRuntimePackage, err := update.RuntimePackage(*updateProfile)
	if err != nil {
		return false, false, trace.Wrap(err)
	}
	updateRuntimeVersion, err := updateRuntimePackage.SemVer()
	if err != nil {
		return false, false, trace.Wrap(err)
	}
	installedRuntimePackage, err := schema.GetRuntimePackage(installed, profile, schema.ServiceRole(clusterRole))
	if err != nil {
		return false, false, trace.Wrap(err)
	}
	installedRuntimeVersion, err := installedRuntimePackage.SemVer()
	if err != nil {
		return false, false, trace.Wrap(err)
	}
	installedTeleportVersion, err := installedTeleportPackage.SemVer()
	if err != nil {
		return false, false, trace.Wrap(err)
	}
	updateTeleportVersion, err := updateTeleportPackage.SemVer()
	if err != nil {
		return false, false, trace.Wrap(err)
	}
	logrus.WithFields(logrus.Fields{
		"installed-runtime":  installedRuntimePackage,
		"update-runtime":     updateRuntimePackage,
		"installed-teleport": installedTeleportPackage,
		"update-teleport":    updateTeleportPackage,
	}).Debug("Check if system packages need to be updated.")
	return installedRuntimeVersion.LessThan(*updateRuntimeVersion) || update.SystemSettingsChanged(installed),
		installedTeleportVersion.LessThan(*updateTeleportVersion), nil
}

func reorderServers(servers []storage.UpdateServer, server storage.Server) (result []storage.UpdateServer) {
	result = make([]storage.UpdateServer, len(servers))
	copy(result, servers)
	sort.Slice(result, func(i, j int) bool {
		// Push server to the front
		return result[i].AdvertiseIP == server.AdvertiseIP
	})
	return result
}

func runtimeUpdates(installedRuntime, updateRuntime, updateApp app.Application) ([]loc.Locator, error) {
	installedDeps, err := app.GetDirectApplicationDependencies(installedRuntime)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateDeps, err := app.GetDirectApplicationDependencies(updateRuntime)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installedDeps = updateApp.Manifest.FilterDisabledDependencies(installedDeps)
	updateDeps = updateApp.Manifest.FilterDisabledDependencies(updateDeps)
	runtimeUpdates, err := loc.GetUpdatedDependencies(installedDeps, updateDeps)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	sort.Slice(runtimeUpdates, func(i, j int) bool {
		// Push RBAC package update to front
		return runtimeUpdates[i].Name == constants.BootstrapConfigPackage
	})
	return runtimeUpdates, nil
}

type runtimeConfig struct {
	// DNSListenAddr specifies the configured DNS listen address
	DNSListenAddr string `json:"PLANET_DNS_LISTEN_ADDR"`
	// DNSPort specifies the configured DNS port
	DNSPort string `json:"PLANET_DNS_PORT"`
}

func newUUID() string {
	return uuid.New()
}

// idGen generates new unique IDs
type idGen func() string
