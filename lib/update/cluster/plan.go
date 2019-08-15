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
	"github.com/gravitational/gravity/lib/checks"
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

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
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

	cluster, err := clusterEnv.Operator.GetLocalSite()
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
		Backend:   clusterEnv.Backend,
		Apps:      clusterEnv.Apps,
		Packages:  clusterEnv.ClusterPackages,
		Client:    clusterEnv.Client,
		DNSConfig: dnsConfig,
		Operator:  clusterEnv.Operator,
		Operation: (*ops.SiteOperation)(operation),
		Leader:    leader,
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

	installedGravityPackage, err := installedRuntime.Manifest.Dependencies.ByName(
		constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedGravityVersion, err := installedGravityPackage.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	supportsTaints := supportsTaints(*installedGravityVersion)

	gravityPackage, err := updateRuntime.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appUpdates, err := app.GetUpdatedDependencies(*installedApp, *updateApp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	runtimeUpdates, err := runtimeUpdates(*installedRuntime, *installedApp, *updateApp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateDNSAppEarly, err := shouldUpdateDNSAppEarly(config.Client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var updateDNSApp *loc.Locator
	if updateDNSAppEarly {
		for _, update := range runtimeUpdates {
			if update.Name == constants.DNSAppPackage {
				updateDNSApp = &update
				break
			}
		}
	}

	installedTeleport, err := installedApp.Manifest.Dependencies.ByName(constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateTeleport, err := updateApp.Manifest.Dependencies.ByName(constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	builder := phaseBuilder{
		planTemplate: storage.OperationPlan{
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
		installedRuntime:  *installedRuntime,
		installedTeleport: *installedTeleport,
		installedApp:      *installedApp,
		updateRuntime:     *updateRuntime,
		updateApp:         *updateApp,
		updateTeleport:    *updateTeleport,
		appUpdates:        appUpdates,
		runtimeUpdates:    runtimeUpdates,
		links:             links,
		trustedClusters:   trustedClusters,
		packageService:    config.Packages,
		updateCoreDNS:     updateCoreDNS,
		updateDNSApp:      updateDNSApp,
		supportsTaints:    supportsTaints,
		roles:             roles,
		changesetID:       uuid.New(),
	}

	etcdVersion, err := shouldUpdateEtcd(builder)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	builder.etcd = etcdVersion

	// intermediates, updates, err := configUpdatesWithIntermediateRuntime(
	// 	installedApp.Manifest, updateApp.Manifest,
	// 	config.Operator, config.Operation.Key(), servers)
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }
	// leader, err := findServer(*config.Leader, updates)
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }
	// builder.intermediateServers = intermediates
	// builder.leadMaster = *leader
	// builder.servers = updates
	// plan, err := newOperationPlanWithIntermediateUpdate(builder)
	// if err != nil {
	// 	return nil, trace.Wrap(err)
	// }
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
	Operation *ops.SiteOperation
	// Client specifies the kubernetes client
	Client *kubernetes.Clientset
	// Leader specifies the server to execute the upgrade operation on
	Leader *storage.Server
}

func newOperationPlan(builder phaseBuilder) (*storage.OperationPlan, error) {
	initPhase := *builder.init()
	checksPhase := *builder.checks().Require(initPhase)
	preUpdatePhase := *builder.preUpdate().Require(initPhase)

	var root update.Phase
	root.Add(initPhase, checksPhase, preUpdatePhase)

	if len(builder.runtimeUpdates) == 0 {
		// Fast path with no runtime updates
		root.AddSequential(builder.app(), builder.cleanup())
		return builder.newPlan(root), nil
	}

	var corednsPhase *update.Phase
	if builder.updateCoreDNS {
		corednsPhase = builder.corednsPhase()
		root.Add(*corednsPhase)
	}
	var earlyDNSAppPhase *update.Phase
	if builder.updateDNSApp != nil {
		earlyDNSAppPhase = builder.earlyDNSApp()
		root.Add(*earlyDNSAppPhase)
	}

	masters, nodes := update.SplitServers(builder.servers)
	masters = reorderServers(masters, builder.leadMaster)
	mastersPhase := *builder.masters(masters[0], masters[1:]).
		Require(checksPhase, preUpdatePhase)
	nodesPhase := *builder.nodes(masters[0], nodes).Require(mastersPhase)
	if corednsPhase != nil {
		mastersPhase.Require(*corednsPhase)
	}
	if earlyDNSAppPhase != nil {
		mastersPhase.Require(*earlyDNSAppPhase)
	}
	root.Add(mastersPhase)
	if len(nodesPhase.Phases) > 0 {
		root.Add(nodesPhase)
	}

	if builder.etcd != nil {
		// This does not depend on previous on purpose - when the etcd block is executed,
		// remote agents might be able to sync the plan before the shutdown of etcd instances
		// has begun
		root.Add(*builder.etcdPlan(serversToStorage(masters[1:]...), serversToStorage(nodes...)))
	}

	if migrationPhase := builder.migration(); migrationPhase != nil {
		root.AddSequential(*migrationPhase)
	}

	// the "config" phase pulls new teleport master config packages used
	// by gravity-sites on master nodes: it needs to run *after* system
	// upgrade phase to make sure that old gravity-sites start up fine
	// in case new configuration is incompatible, but *before* runtime
	// phase so new gravity-sites can find it after they start
	configPhase := *builder.config(serversToStorage(masters...)).Require(mastersPhase)
	runtimePhase := *builder.runtime().Require(mastersPhase)
	root.Add(configPhase, runtimePhase)
	root.AddSequential(builder.app(), builder.cleanup())

	return builder.newPlan(root), nil
}

func newOperationPlanWithIntermediateUpdate(builder phaseBuilder) (*storage.OperationPlan, error) {
	initPhase := *builder.init()
	checksPhase := *builder.checks().Require(initPhase)
	preUpdatePhase := *builder.preUpdate().Require(initPhase)

	var root update.Phase
	root.Add(initPhase, checksPhase, preUpdatePhase)

	var corednsPhase *update.Phase
	if builder.updateCoreDNS {
		corednsPhase = builder.corednsPhase()
		root.Add(*corednsPhase)
	}
	var earlyDNSAppPhase *update.Phase
	if builder.updateDNSApp != nil {
		earlyDNSAppPhase = builder.earlyDNSApp()
		root.Add(*earlyDNSAppPhase)
	}

	intermediateMasters, intermediateNodes := update.SplitServers(builder.intermediateServers)
	intermediateMasters = reorderServers(intermediateMasters, builder.leadMaster)

	intermediateMastersPhase := builder.mastersIntermediate(intermediateMasters[0], intermediateMasters[1:]).
		Require(checksPhase, preUpdatePhase)
	if corednsPhase != nil {
		intermediateMastersPhase.Require(*corednsPhase)
	}
	if earlyDNSAppPhase != nil {
		intermediateMastersPhase.Require(*earlyDNSAppPhase)
	}
	intermediateNodesPhase := *builder.nodesIntermediate(intermediateMasters[0], intermediateNodes).
		Require(*intermediateMastersPhase)
	root.Add(*intermediateMastersPhase)
	if len(intermediateNodesPhase.Phases) > 0 {
		root.Add(intermediateNodesPhase)
	}

	masters, nodes := update.SplitServers(builder.servers)
	masters = reorderServers(masters, builder.leadMaster)

	etcdPhase := *builder.etcdPlan(
		serversToStorage(masters[1:]...),
		serversToStorage(nodes...))
	// This does not depend on previous on purpose - when the etcd block is executed,
	// remote agents might be able to sync the plan before the shutdown of etcd instances
	// has begun
	root.Add(etcdPhase)

	mastersPhase := *builder.masters(masters[0], masters[1:]).Require(etcdPhase)
	nodesPhase := *builder.nodes(masters[0], nodes).Require(mastersPhase)
	root.Add(mastersPhase)
	if len(nodesPhase.Phases) > 0 {
		root.Add(nodesPhase)
	}

	if migrationPhase := builder.migration(); migrationPhase != nil {
		root.AddSequential(*migrationPhase)
	}

	// the "config" phase pulls new teleport master config packages used
	// by gravity-sites on master nodes: it needs to run *after* system
	// upgrade phase to make sure that old gravity-sites start up fine
	// in case new configuration is incompatible, but *before* runtime
	// phase so new gravity-sites can find it after they start
	configPhase := *builder.config(serversToStorage(masters...)).Require(mastersPhase)
	runtimePhase := *builder.runtime().Require(mastersPhase)
	root.Add(configPhase, runtimePhase)
	root.AddSequential(builder.app(), builder.cleanup())

	return builder.newPlan(root), nil
}

// configUpdates computes the configuration updates for the specified list of servers
func configUpdates(
	installed, update schema.Manifest,
	operator ops.Operator,
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
		secretsUpdate, err := operator.RotateSecrets(ops.RotateSecretsRequest{
			AccountID:   operation.AccountID,
			ClusterName: operation.SiteDomain,
			Server:      server,
			DryRun:      true,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		installedRuntime, err := getRuntimePackage(installed, server.Role, schema.ServiceRole(server.ClusterRole))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		installedDocker, err := ops.GetDockerConfig(operator, operation.SiteKey())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		updateServer := storage.UpdateServer{
			Server: server,
			Runtime: storage.RuntimePackage{
				Installed:      *installedRuntime,
				SecretsPackage: &secretsUpdate.Locator,
			},
			Teleport: storage.TeleportPackage{
				Installed: *installedTeleport,
			},
			Docker: storage.DockerUpdate{
				Installed: *installedDocker,
				Update: checks.DockerConfigFromSchemaValue(
					update.SystemDocker()),
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
			updateServer.Runtime.Update = &storage.RuntimeUpdate{
				Package:       *updateRuntime,
				ConfigPackage: configUpdate.Locator,
			}
		}
		if needsTeleportUpdate {
			_, nodeConfig, err := operator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
				Key:    operation,
				Server: server,
				DryRun: true,
			})
			if err != nil {
				return nil, trace.Wrap(err)
			}
			updateServer.Teleport.Update = &storage.TeleportUpdate{
				Package:           *updateTeleport,
				NodeConfigPackage: nodeConfig.Locator,
			}
		}
		updates = append(updates, updateServer)
	}
	return updates, nil
}

// configUpdatesWithIntermediateRuntime computes the configuration updates for the specified list of servers
// for the case when the plan needs to perform an intermediate runtime package update
func configUpdatesWithIntermediateRuntime(
	installedTeleport, updateTeleport loc.Locator,
	installedRuntime, updateRuntime loc.Locator,
	configurator packageConfigurator,
	operation ops.SiteOperationKey,
	servers []storage.Server,
) (updates []storage.UpdateServer, err error) {
	for _, server := range servers {
		secretsPackage, err := configurator.NewPlanetSecretsPackageName(ops.RotateSecretsRequest{
			AccountID:   operation.AccountID,
			ClusterName: operation.SiteDomain,
			Server:      server,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// FIXME: factor this out into a configuration package rotator as this
		// needs to be per step (for intermediate steps, this needs to call into
		// the corresponding version of the gravity binary)
		// FIXME: this generates the package name
		runtimeConfig, err := configurator.NewPlanetPackageName(ops.RotatePlanetConfigRequest{
			Key:    operation,
			Server: server,
			// TODO
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		teleportNodeConfig, err := configurator.NewTeleportPackageName(ops.RotateTeleportConfigRequest{
			Key:    operation,
			Server: server,
			// TODO
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		update := storage.UpdateServer{
			Server: server,
			Runtime: storage.RuntimePackage{
				Installed:      installedRuntime,
				SecretsPackage: &secretsPackage,
				Update: &storage.RuntimeUpdate{
					Package:       *updateRuntime,
					ConfigPackage: runtimeConfig,
				},
			},
			Teleport: storage.TeleportPackage{
				Installed: *installedTeleport,
				Update: &storage.TeleportUpdate{
					Package:           updateTeleport,
					NodeConfigPackage: teleportNodeConfig,
				},
			},
		}
		updates = append(updates, update)
	}
	return updates, nil
}

type packageConfigurator interface {
	newPlanetPackageName() (*loc.Locator, error)
	newTeleportPackageName() (*loc.Locator, error)
	rotatePlanetPackage() error
	rotateTeleportPackage() loc.Locator
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
	_, err := client.CoreV1().Services(constants.KubeSystemNamespace).Get("kube-dns", metav1.GetOptions{})
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
	return installedRuntimeVersion.LessThan(*updateRuntimeVersion),
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

func reorderServers(servers []storage.UpdateServer, server storage.UpdateServer) (result []storage.UpdateServer) {
	sort.Slice(servers, func(i, j int) bool {
		// Push server to the front
		return servers[i].AdvertiseIP == server.AdvertiseIP
	})
	return servers
}

func runtimeUpdates(installedRuntime, updateRuntime, updateApp app.Application) ([]loc.Locator, error) {
	allRuntimeUpdates, err := app.GetUpdatedDependencies(installedRuntime, updateRuntime)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// some system apps may need to be skipped depending on the manifest settings
	runtimeUpdates := allRuntimeUpdates[:0]
	for _, locator := range allRuntimeUpdates {
		if !schema.ShouldSkipApp(updateApp.Manifest, locator) {
			runtimeUpdates = append(runtimeUpdates, locator)
		}
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

type packageRotator interface {
	RotateSecrets(ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error)
	RotatePlanetConfig(ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error)
	RotateTeleportConfig(ops.RotateTeleportConfigRequest) (*ops.RotatePackageResponse, *ops.RotatePackageResponse, error)
}
