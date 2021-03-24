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
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/rigging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
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
		log.Info("Detecting DNS configuration.")
		existingDNS, err := getExistingDNSConfig(localEnv.Packages)
		if err != nil {
			return nil, trace.Wrap(err, "failed to determine existing cluster DNS configuration")
		}
		dnsConfig = *existingDNS
	}

	plan, err = NewOperationPlan(PlanConfig{
		Backend:    clusterEnv.Backend,
		Apps:       clusterEnv.Apps,
		Packages:   clusterEnv.ClusterPackages,
		Client:     clusterEnv.Client,
		DNSConfig:  dnsConfig,
		Operator:   clusterEnv.Operator,
		Operation:  operation,
		Leader:     leader,
		UserConfig: userConfig,
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
func NewOperationPlan(config PlanConfig) (*storage.OperationPlan, error) {
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

	updateCoreDNS, err := shouldUpdateCoreDNS(config.Client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateDNSAppEarly, err := shouldUpdateDNSAppEarly(config.Client)
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
	installedRuntime, err := config.Apps.GetApp(*(installedApp.Manifest.Base()))
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
	updateRuntime, err := config.Apps.GetApp(*(updateApp.Manifest.Base()))
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

	updates, err := configUpdates(
		installedApp.Manifest, updateApp.Manifest,
		config.Operator, (*ops.SiteOperation)(config.Operation).Key(), servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	leader, err := findServer(*config.Leader, updates)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gravityPackage, err := updateRuntime.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan, err := newOperationPlan(planConfig{
		plan: storage.OperationPlan{
			OperationID:    config.Operation.ID,
			OperationType:  config.Operation.Type,
			AccountID:      config.Operation.AccountID,
			ClusterName:    config.Operation.SiteDomain,
			Servers:        servers,
			DNSConfig:      config.DNSConfig,
			GravityPackage: *gravityPackage,
		},
		operator:          config.Operator,
		operation:         *config.Operation,
		servers:           updates,
		installedRuntime:  *installedRuntime,
		installedApp:      *installedApp,
		updateRuntime:     *updateRuntime,
		updateApp:         *updateApp,
		links:             links,
		trustedClusters:   trustedClusters,
		packageService:    config.Packages,
		shouldUpdateEtcd:  shouldUpdateEtcd,
		updateCoreDNS:     updateCoreDNS,
		updateDNSAppEarly: updateDNSAppEarly,
		roles:             roles,
		leadMaster:        *leader,
		userConfig:        config.UserConfig,
	})
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
	return nil
}

// PlanConfig defines the configuration for creating a new operation plan
type PlanConfig struct {
	Backend    storage.Backend
	Packages   pack.PackageService
	Apps       app.Applications
	DNSConfig  storage.DNSConfig
	Operator   ops.Operator
	Operation  *storage.SiteOperation
	Client     *kubernetes.Clientset
	Leader     *storage.Server
	UserConfig UserConfig
}

// planConfig collects parameters needed to generate an update operation plan
type planConfig struct {
	operator packageRotator
	// plan specifies the initial plan configuration
	// this will be updated with the list of operational phases
	plan storage.OperationPlan
	// operation is the operation to generate the plan for
	operation storage.SiteOperation
	// servers is a list of servers from cluster state
	servers []storage.UpdateServer
	// installedRuntime is the runtime of the installed app
	installedRuntime app.Application
	// installedApp is the installed app
	installedApp app.Application
	// updateRuntime is the runtime of the update app
	updateRuntime app.Application
	// updateApp is the update app
	updateApp app.Application
	// links is a list of configured remote Ops Center links
	links []storage.OpsCenterLink
	// trustedClusters is a list of configured trusted clusters
	trustedClusters []teleservices.TrustedCluster
	// packageService is a reference to the clusters package service
	packageService pack.PackageService
	// shouldUpdateEtcd returns whether we should update etcd and the versions of etcd in use
	shouldUpdateEtcd func(planConfig) (bool, string, string, error)
	// updateCoreDNS indicates whether we need to run coreDNS phase
	updateCoreDNS bool
	// updateDNSAppEarly indicates whether we need to update the DNS app earlier than normal
	//	Only applicable for 5.3.0 -> 5.3.2
	updateDNSAppEarly bool
	// roles is the existing cluster roles
	roles []teleservices.Role
	// leader refers to the master server running the update operation
	leadMaster storage.UpdateServer
	// userConfig is user provided configuration to tune the upgrade
	userConfig UserConfig
}

func newOperationPlan(p planConfig) (*storage.OperationPlan, error) {
	masters, nodes := update.SplitServers(p.servers)
	if len(masters) == 0 {
		return nil, trace.NotFound("no master servers found")
	}
	otherMasters := filterServer(masters, p.leadMaster)

	if p.userConfig.SkipWorkers {
		p.servers = masters
		nodes = nil
	}

	builder := phaseBuilder{planConfig: p}
	initPhase := *builder.init(p.leadMaster.Server)
	checkDeps := []update.PhaseIder{initPhase}
	var seLinuxPhase *update.Phase
	if builder.hasSELinuxPhase() {
		seLinuxPhase = builder.bootstrapSELinux().Require(initPhase)
		checkDeps = append(checkDeps, *seLinuxPhase)
	}
	checksPhase := *builder.checks().Require(checkDeps...)
	preUpdatePhase := *builder.preUpdate().Require(initPhase)
	bootstrapPhase := *builder.bootstrap().Require(initPhase)

	installedGravityPackage, err := p.installedRuntime.Manifest.Dependencies.ByName(
		constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	supportsTaints, err := supportsTaints(*installedGravityPackage)
	if err != nil {
		log.WithError(err).Warn("Failed to query support for taints/tolerations in installed runtime.")
	}
	if !supportsTaints {
		log.Debugf("No support for taints/tolerations for %v.", installedGravityPackage)
	}

	mastersPhase := *builder.masters(p.leadMaster, otherMasters, supportsTaints).
		Require(checksPhase, bootstrapPhase, preUpdatePhase)
	nodesPhase := *builder.nodes(p.leadMaster, nodes, supportsTaints).
		Require(mastersPhase)

	runtimeUpdates, err := app.GetUpdatedDependencies(p.installedRuntime, p.updateRuntime, p.installedApp.Manifest, p.updateApp.Manifest)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	appUpdates, err := app.GetUpdatedDependencies(p.installedApp, p.updateApp, p.installedApp.Manifest, p.updateApp.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check whether etcd upgrade is required
	updateEtcd, currentVersion, desiredVersion, err := p.shouldUpdateEtcd(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check if we should enable OpenEBS integration
	enableOpenEBS := !p.installedApp.Manifest.OpenEBSEnabled() && p.updateApp.Manifest.OpenEBSEnabled()

	var root update.Phase
	root.Add(initPhase)
	if seLinuxPhase != nil {
		root.Add(*seLinuxPhase)
	}
	root.Add(checksPhase, preUpdatePhase)
	if len(runtimeUpdates) > 0 {
		if p.updateCoreDNS {
			corednsPhase := *builder.corednsPhase(p.leadMaster.Server)
			mastersPhase = *mastersPhase.Require(corednsPhase)
			root.Add(corednsPhase)
		}

		if p.updateDNSAppEarly {
			for _, update := range runtimeUpdates {
				if update.Name == constants.DNSAppPackage {
					earlyDNSAppPhase := *builder.earlyDNSApp(update)
					mastersPhase = *mastersPhase.Require(earlyDNSAppPhase)
					root.Add(earlyDNSAppPhase)
				}
			}
		}

		root.Add(bootstrapPhase, mastersPhase)
		if len(nodesPhase.Phases) > 0 {
			root.Add(nodesPhase)
		}

		if updateEtcd {
			p.plan.OfflineCoordinator = &p.leadMaster.Server
			etcdPhase := *builder.etcdPlan(p.leadMaster.Server,
				serversToStorage(otherMasters...),
				serversToStorage(nodes...),
				currentVersion, desiredVersion)
			// This does not depend on previous on purpose - when the etcd block is executed,
			// remote agents might be able to sync the plan before the shutdown of etcd instances
			// has begun
			root.Add(etcdPhase)
		}

		if migrationPhase := builder.migration(p.leadMaster.Server); migrationPhase != nil {
			root.AddSequential(*migrationPhase)
		}

		// the "config" phase pulls new teleport master config packages used
		// by gravity-sites on master nodes: it needs to run *after* system
		// upgrade phase to make sure that old gravity-sites start up fine
		// in case new configuration is incompatible, but *before* runtime
		// phase so new gravity-sites can find it after they start
		configPhase := *builder.config(serversToStorage(masters...)).Require(mastersPhase)
		root.Add(configPhase)

		// if OpenEBS has been just enabled, create its configuration before
		// the runtime phase runs and installs it
		if enableOpenEBS {
			openEBSPhase := *builder.openEBS(p.leadMaster)
			root.Add(openEBSPhase)
		}

		runtimePhase := *builder.runtime(runtimeUpdates).Require(mastersPhase)
		root.Add(runtimePhase)
	}

	root.AddSequential(*builder.app(appUpdates), *builder.cleanup())
	plan := p.plan
	plan.Phases = root.Phases
	update.ResolvePlan(&plan)

	return &plan, nil
}

// configUpdates computes the configuration updates for the specified list of servers
func configUpdates(
	installed, update schema.Manifest,
	operator packageRotator,
	operation ops.SiteOperationKey,
	servers []storage.Server,
) (updates []storage.UpdateServer, err error) {
	installedTeleport, err := installed.Dependencies.ByName(constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateTeleport, err := update.Dependencies.ByName(constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, server := range servers {
		installedRuntime, err := getRuntimePackage(installed, server.Role, schema.ServiceRole(server.ClusterRole))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateServer := storage.UpdateServer{
			Server: server,
			Runtime: storage.RuntimePackage{
				Installed: *installedRuntime,
			},
			Teleport: storage.TeleportPackage{
				Installed: *installedTeleport,
			},
		}
		needsPlanetUpdate, needsTeleportUpdate, err := systemNeedsUpdate(
			server.Role, server.ClusterRole,
			installed, update, *installedTeleport, *updateTeleport)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if needsPlanetUpdate {
			updateRuntime, err := update.RuntimePackageForProfile(server.Role)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			secretsUpdate, err := operator.RotateSecrets(ops.RotateSecretsRequest{
				Key:            operation.SiteKey(),
				Server:         server,
				RuntimePackage: *updateRuntime,
				DryRun:         true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			configUpdate, err := operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
				Key:            operation,
				Server:         server,
				Manifest:       update,
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
			_, nodeConfig, err := operator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
				Key:             operation,
				Server:          server,
				TeleportPackage: *updateTeleport,
				DryRun:          true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Teleport.Update = &storage.TeleportUpdate{
				Package:           *updateTeleport,
				NodeConfigPackage: &nodeConfig.Locator,
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
		// Overwrite the Server Nodename with the name of the kubernetes node,
		// to fix any internal consistency issues that may occur in our internal data
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

// Only applicable for 5.3.0 -> 5.3.2
// We need to update the CoreDNS app before doing rolling restarts, because the new planet will not have embedded
// coredns, and will instead point to the kube-dns service on startup. Updating the app will deploy coredns as pods.
// TODO(knisbet) remove when 5.3.2 is no longer supported as an upgrade path
func shouldUpdateDNSAppEarly(client *kubernetes.Clientset) (bool, error) {
	_, err := client.CoreV1().Services(constants.KubeSystemNamespace).
		Get(context.TODO(), "kube-dns", metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return true, trace.Wrap(err)
	}
	return false, nil
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
	installedRuntimePackage, err := getRuntimePackage(installed, profile, schema.ServiceRole(clusterRole))
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

func getRuntimePackage(manifest schema.Manifest, profileName string, clusterRole schema.ServiceRole) (*loc.Locator, error) {
	profile, err := manifest.NodeProfiles.ByName(profileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runtimePackage, err := manifest.RuntimePackage(*profile)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		return runtimePackage, nil
	}
	// Look for legacy package
	packageName := loc.LegacyPlanetMaster.Name
	if clusterRole == schema.ServiceRoleNode {
		packageName = loc.LegacyPlanetNode.Name
	}
	runtimePackage, err = manifest.Dependencies.ByName(packageName)
	if err != nil {
		logrus.Warnf("Failed to find the legacy runtime package in manifest "+
			"for profile %v and cluster role %v: %v.", profile.Name, clusterRole, err)
		return nil, trace.NotFound("runtime package for profile %v "+
			"(cluster role %v) not found in manifest",
			profile.Name, clusterRole)
	}
	return runtimePackage, nil
}

func findServer(input storage.Server, servers []storage.UpdateServer) (*storage.UpdateServer, error) {
	for _, server := range servers {
		if server.AdvertiseIP == input.AdvertiseIP {
			return &server, nil
		}
	}
	return nil, trace.NotFound("no server found with address %v", input.AdvertiseIP)
}

func filterServer(servers []storage.UpdateServer, server storage.UpdateServer) (result []storage.UpdateServer) {
	for _, s := range servers {
		if s.AdvertiseIP == server.AdvertiseIP {
			continue
		}
		result = append(result, s)
	}
	return result
}

type runtimeConfig struct {
	// DNSListenAddr specifies the configured DNS listen address
	DNSListenAddr string `json:"PLANET_DNS_LISTEN_ADDR"`
	// DNSPort specifies the configured DNS port
	DNSPort string `json:"PLANET_DNS_PORT"`
}

type packageRotator interface {
	RotateSecrets(ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error)
	RotatePlanetConfig(ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error)
	RotateTeleportConfig(ops.RotateTeleportConfigRequest) (*ops.RotatePackageResponse, *ops.RotatePackageResponse, error)
}
