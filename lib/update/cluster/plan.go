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
	"strings"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	libphase "github.com/gravitational/gravity/lib/update/cluster/phases"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// InitOperationPlan will initialize operation plan for an operation
func InitOperationPlan(
	ctx context.Context,
	localEnv, updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment,
	opKey ops.SiteOperationKey,
	leader *storage.Server,
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
		log.Info("Detecting DNS configuration.")
		existingDNS, err := getExistingDNSConfig(localEnv.Packages)
		if err != nil {
			return nil, trace.Wrap(err, "failed to determine existing cluster DNS configuration")
		}
		dnsConfig = *existingDNS
	}

	plan, err = NewOperationPlan(ctx, PlanConfig{
		Backend:     clusterEnv.Backend,
		Apps:        clusterEnv.Apps,
		Packages:    clusterEnv.ClusterPackages,
		Client:      clusterEnv.Client,
		DNSConfig:   dnsConfig,
		Operator:    clusterEnv.Operator,
		Operation:   operation,
		Leader:      leader,
		ServiceUser: &cluster.ServiceUser,
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

// NewOperationPlan generates a new plan for the provided operation
func NewOperationPlan(ctx context.Context, config PlanConfig) (*storage.OperationPlan, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := storage.GetLocalServers(config.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err = checkAndSetServerDefaults(servers, config.Client.CoreV1().Nodes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedPackage, err := storage.GetLocalPackage(config.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedApp, err := config.Apps.GetApp(*installedPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedRuntimeApp, err := config.Apps.GetApp(*(installedApp.Manifest.Base()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installedRuntimeAppVersion, err := installedRuntimeApp.Package.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updatePackage, err := config.Operation.Update.Package()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateApp, err := config.Apps.GetApp(*updatePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateRuntimeApp, err := config.Apps.GetApp(*(updateApp.Manifest.Base()))
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

	links, err := config.Backend.GetOpsCenterLinks(config.Operation.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	trustedClusters, err := config.Backend.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := config.Backend.GetRoles()
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
	appUpdates, err := loc.GetUpdatedDependencies(installedDeps, updateDeps)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	builder := phaseBuilder{
		planTemplate: storage.OperationPlan{
			OperationID:        config.Operation.ID,
			OperationType:      config.Operation.Type,
			AccountID:          config.Operation.AccountID,
			ClusterName:        config.Operation.SiteDomain,
			Servers:            servers,
			DNSConfig:          config.DNSConfig,
			GravityPackage:     *gravityPackage,
			OfflineCoordinator: config.Leader,
		},
		operator:                   config.Operator,
		operation:                  *config.Operation,
		appUpdates:                 appUpdates,
		links:                      links,
		trustedClusters:            trustedClusters,
		packages:                   config.Packages,
		apps:                       config.Apps,
		roles:                      roles,
		leadMaster:                 *config.Leader,
		installedApp:               *installedApp,
		updateApp:                  *updateApp,
		installedRuntimeApp:        *installedRuntimeApp,
		installedRuntimeAppVersion: *installedRuntimeAppVersion,
		updateRuntimeApp:           *updateRuntimeApp,
		updateRuntimeAppVersion:    *updateRuntimeAppVersion,
		installedTeleport:          *installedTeleport,
		updateTeleport:             *updateTeleport,
		serviceUser:                *config.ServiceUser,
	}

	err = builder.initSteps(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan, err := builder.newPlan()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

func (r *PlanConfig) checkAndSetDefaults() error {
	if r.Client == nil {
		return trace.BadParameter("Kubernetes client is required")
	}
	if r.Apps == nil {
		return trace.BadParameter("application service is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("package service is required")
	}
	if r.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if r.Operator == nil {
		return trace.BadParameter("cluster operator is required")
	}
	if r.Operation == nil {
		return trace.BadParameter("cluster operation is required")
	}
	if r.Leader == nil {
		return trace.BadParameter("operation leader node is required")
	}
	if r.ServiceUser == nil {
		return trace.BadParameter("cluster service user is required")
	}
	return nil
}

// PlanConfig defines the configuration for creating a new operation plan
type PlanConfig struct {
	// Backend specifies the cluster backend for low-level queries
	Backend storage.Backend
	// Packages specifies the cluster package service
	Packages pack.PackageService
	// Apps specifies the cluster application service
	Apps app.Applications
	// Operator specifies the cluster service operator
	Operator ops.Operator
	// DNSConfig specifies the cluster DNS configuration
	DNSConfig storage.DNSConfig
	// Operation specifies the operation to generate the plan for
	Operation *storage.SiteOperation
	// Client specifies the kubernetes client
	Client *kubernetes.Clientset
	// Leader specifies the server to execute the upgrade operation on
	Leader *storage.Server
	// ServiceUser specifies the cluster's service user
	ServiceUser *storage.OSUser
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

		return archive.Abort
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

func shouldUpdateCoreDNS(client *kubernetes.Clientset) (bool, error) {
	_, err := client.RbacV1().ClusterRoles().Get(libphase.CoreDNSResourceName, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	_, err = client.RbacV1().ClusterRoleBindings().Get(libphase.CoreDNSResourceName, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	_, err = client.CoreV1().ConfigMaps(constants.KubeSystemNamespace).Get("coredns", metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	return false, nil
}

func shouldUpdateEtcd(installedRuntimePackage, updateRuntimePackage loc.Locator, packages pack.PackageService) (*etcdVersion, error) {
	// TODO: should somehow maintain etcd version invariant across runtime packages
	var updateEtcd bool
	installedVersion, err := getEtcdVersion(installedRuntimePackage, packages)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		// if the currently installed version doesn't have etcd version information, it needs to be upgraded
		updateEtcd = true
	}
	updateVersion, err := getEtcdVersion(updateRuntimePackage, packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if installedVersion == nil || installedVersion.Compare(*updateVersion) < 0 {
		updateEtcd = true
	}
	if !updateEtcd {
		return nil, nil
	}
	result := etcdVersion{
		update: updateVersion.String(),
	}
	if installedVersion != nil {
		result.installed = installedVersion.String()
	}
	return &result, nil
}

func getEtcdVersion(locator loc.Locator, packageService pack.PackageService) (*semver.Version, error) {
	const searchLabel = "version-etcd"
	manifest, err := pack.GetPackageManifest(packageService, locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, label := range manifest.Labels {
		if label.Name == searchLabel {
			versionS := strings.TrimPrefix(label.Value, "v")
			version, err := semver.NewVersion(versionS)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return version, nil
		}
	}
	return nil, trace.NotFound("package manifest for %q does not have label %v",
		locator, searchLabel)
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

func findServer(input storage.Server, servers []storage.UpdateServer) (*storage.UpdateServer, error) {
	for _, server := range servers {
		if server.AdvertiseIP == input.AdvertiseIP {
			return &server, nil
		}
	}
	return nil, trace.NotFound("no server found with address %v", input.AdvertiseIP)
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
	installedDeps = updateApp.Manifest.FilterDisabledDependencies(installedDeps)
	updateDeps, err := app.GetDirectApplicationDependencies(updateRuntime)
	if err != nil {
		return nil, trace.Wrap(err)
	}
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
