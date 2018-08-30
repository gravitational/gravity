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
	"context"
	"fmt"
	"path"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	rpcclient "github.com/gravitational/gravity/lib/rpc/client"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// InitOperationPlan will initialize operation plan for an operation
func InitOperationPlan(
	ctx context.Context,
	updateEnv *localenv.LocalEnvironment,
	clusterEnv *localenv.ClusterEnvironment) (*storage.OperationPlan, error) {
	operation, err := storage.GetLastOperation(clusterEnv.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if operation.Type != ops.OperationUpdate {
		return nil, trace.BadParameter("%q does not support plans", operation.Type)
	}

	plan, err := clusterEnv.Backend.GetOperationPlan(operation.SiteDomain, operation.ID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if plan != nil {
		return nil, trace.AlreadyExists("plan is already initialized")
	}

	plan, err = NewOperationPlan(clusterEnv, *operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clusterEnv.Backend.CreateOperationPlan(*plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// all plan creation was done on the cluster, so sync it to the local backend, which will be authoritative
	// from now on
	err = SyncOperationPlan(clusterEnv.Backend, updateEnv.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return plan, nil
}

// SyncOperationPlan will synchronize the operation plan from source backend to the destination
func SyncOperationPlan(src storage.Backend, dst storage.Backend) error {
	operation, err := storage.GetLastOperation(src)
	if err != nil {
		return trace.Wrap(err)
	}

	plan, err := src.GetOperationPlan(operation.SiteDomain, operation.ID)
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := src.GetSite(operation.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = dst.CreateSite(*cluster)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = dst.CreateSiteOperation(*operation)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = dst.CreateOperationPlan(*plan)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	return trace.Wrap(syncChangelog(src, dst, plan.ClusterName, plan.OperationID))
}

// SyncOperationPlanToCluster runs gravity plan --sync on each cluster member, to force syncronization from etcd
// to the local store
func SyncOperationPlanToCluster(ctx context.Context, plan storage.OperationPlan, clientCreds credentials.TransportCredentials) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.AgentRequestTimeout)
	defer cancel()

	logger := log.WithFields(log.Fields{
		trace.Component: "fsm:sync",
	})

	for _, server := range plan.Servers {
		err := systeminfo.HasInterface(server.AdvertiseIP)
		if err == nil {
			continue
		}
		addr := defaults.GravityRPCAgentAddr(server.AdvertiseIP)
		clt, err := rpcclient.New(ctx, rpcclient.Config{ServerAddr: addr, Credentials: clientCreds})
		if err != nil {
			return trace.Wrap(err)
		}

		err = clt.GravityCommand(ctx, logger, nil, "plan", "--sync")
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// NewOperationPlan generates a new plan for the provided operation
func NewOperationPlan(env *localenv.ClusterEnvironment, op storage.SiteOperation) (*storage.OperationPlan, error) {
	if env.Client == nil {
		return nil, trace.BadParameter("Kubernetes client is required")
	}

	servers, err := storage.GetLocalServers(env.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err = checkAndSetServerDefaults(servers, env.Client.CoreV1().Nodes())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installedPackage, err := storage.GetLocalPackage(env.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installedApp, err := env.Apps.GetApp(*installedPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installedRuntime, err := env.Apps.GetApp(*(installedApp.Manifest.Base()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updatePackage, err := op.Update.Package()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateApp, err := env.Apps.GetApp(*updatePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateRuntime, err := env.Apps.GetApp(*(updateApp.Manifest.Base()))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	links, err := env.Backend.GetOpsCenterLinks(op.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedClusters, err := env.Backend.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan, err := newOperationPlan(newPlanParams{
		operation:        op,
		servers:          servers,
		installedRuntime: *installedRuntime,
		installedApp:     *installedApp,
		updateRuntime:    *updateRuntime,
		updateApp:        *updateApp,
		links:            links,
		trustedClusters:  trustedClusters,
		packageService:   env.ClusterPackages,
		shouldUpdateEtcd: shouldUpdateEtcd,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return plan, nil
}

// newPlanParams collects parameters needed to generate an update operation plan
type newPlanParams struct {
	// operation is the operation to generate the plan for
	operation storage.SiteOperation
	// servers is a list of servers from cluster state
	servers []storage.Server
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
	shouldUpdateEtcd func(newPlanParams) (bool, string, string, error)
}

func newOperationPlan(p newPlanParams) (*storage.OperationPlan, error) {
	gravityPackage, err := p.updateRuntime.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan := storage.OperationPlan{
		OperationID:    p.operation.ID,
		OperationType:  p.operation.Type,
		AccountID:      p.operation.AccountID,
		ClusterName:    p.operation.SiteDomain,
		Servers:        p.servers,
		GravityPackage: *gravityPackage,
	}

	builder := phaseBuilder{}
	initPhase := *builder.init(p.installedApp.Package, p.updateApp.Package)
	checksPhase := *builder.checks(p.installedApp.Package, p.updateApp.Package)
	preUpdatePhase := *builder.preUpdate(p.updateApp.Package).Require(initPhase)
	bootstrapPhase := *builder.bootstrap(p.servers, p.updateApp.Package).Require(initPhase)

	var masters, nodes []storage.Server
	for _, server := range p.servers {
		if fsm.IsMasterServer(server) {
			masters = append(masters, server)
		} else {
			nodes = append(nodes, server)
		}
	}

	if len(masters) == 0 {
		return nil, trace.NotFound("no master servers found")
	}

	installedGravityPackage, err := p.installedRuntime.Manifest.Dependencies.ByName(
		constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	supportsTaints, err := supportsTaints(*installedGravityPackage)
	if err != nil {
		log.Warnf("Failed to query support for taints/tolerations in installed runtime: %v.",
			trace.DebugReport(err))
	}
	if !supportsTaints {
		log.Debugf("No support for taints/tolerations for %v.", installedGravityPackage)
	}

	// Choose the first master node for upgrade to be the leader during the operation
	leadMaster := masters[0]

	mastersPhase := *builder.masters(leadMaster, masters[1:], supportsTaints,
		p.updateApp.Package).Require(checksPhase, bootstrapPhase, preUpdatePhase)
	nodesPhase := *builder.nodes(leadMaster, nodes, supportsTaints,
		p.updateApp.Package).Require(mastersPhase)

	runtimeUpdates, err := app.GetUpdatedDependencies(p.installedRuntime, p.updateRuntime)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	// this flag indicates whether rbac-app has been updated or not, because if it has
	// then its k8s resources should be created before all other hooks run
	rbacAppUpdated := false
	for _, update := range runtimeUpdates {
		if update.Name == constants.BootstrapConfigPackage {
			rbacAppUpdated = true
		}
	}

	runtimePhase := *builder.runtime(runtimeUpdates, rbacAppUpdated).Require(mastersPhase)

	appUpdates, err := app.GetUpdatedDependencies(p.installedApp, p.updateApp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appPhase := *builder.app(appUpdates)
	if len(runtimeUpdates) != 0 {
		appPhase.Require(mastersPhase)
	}
	if rbacAppUpdated {
		appPhase.RequireLiteral(runtimePhase.ChildLiteral(constants.BootstrapConfigPackage))
	}

	// check if etcd upgrade is required or not
	updateEtcd, currentVersion, desiredVersion, err := p.shouldUpdateEtcd(p)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cleanupPhase := *builder.cleanup(p.servers).Require(appPhase)

	// Order the phases
	phases := phases{initPhase, checksPhase, preUpdatePhase}
	if len(runtimeUpdates) > 0 {
		// if there are no runtime updates, then these phases are not needed
		// as we're not going to update system software
		phases = append(phases, bootstrapPhase, mastersPhase)
		if len(nodesPhase.Phases) > 0 {
			phases = append(phases, nodesPhase)
		}

		if updateEtcd {
			etcdPhase := *builder.etcdPlan(leadMaster, masters[1:], nodes, currentVersion, desiredVersion)
			phases = append(phases, etcdPhase)
		}

		if migrationPhase := builder.migration(p); migrationPhase != nil {
			phases = append(phases, *migrationPhase)
		}
		phases = append(phases, runtimePhase)
	}
	phases = append(phases, appPhase, cleanupPhase)
	plan.Phases = phases.asPhases()
	resolve(&plan)

	return &plan, nil
}

func shouldUpdateEtcd(p newPlanParams) (updateEtcd bool, installedEtcdVersion string, updateEtcdVersion string, err error) {
	// TODO: should somehow maintain etcd version invariant across runtime packages
	runtimePackage, err := p.installedRuntime.Manifest.DefaultRuntimePackage()
	if err != nil && !trace.IsNotFound(err) {
		return false, "", "", trace.Wrap(err)
	}
	if err != nil {
		runtimePackage, err = p.installedRuntime.Manifest.Dependencies.ByName(loc.LegacyPlanetMaster.Name)
		if err != nil {
			log.Warnf("Failed to fetch the runtime package: %v.", err)
			return false, "", "", trace.NotFound("runtime package not found")
		}
	}
	installedEtcdVersion, err = getPackageLabel("version-etcd", *runtimePackage, p.packageService)
	if err != nil {
		if !trace.IsNotFound(err) {
			return false, "", "", trace.Wrap(err)
		}
		// if the currently installed version doesn't have etcd version information, it needs to be upgraded
		updateEtcd = true

	}
	runtimePackage, err = p.updateRuntime.Manifest.DefaultRuntimePackage()
	if err != nil {
		return false, "", "", trace.Wrap(err)
	}
	updateEtcdVersion, err = getPackageLabel("version-etcd", *runtimePackage, p.packageService)
	if err != nil {
		return false, "", "", trace.Wrap(err)
	}
	if installedEtcdVersion != updateEtcdVersion {
		updateEtcd = true
	}

	return updateEtcd, installedEtcdVersion, updateEtcdVersion, nil
}

func getPackageLabel(searchLabel string, locator loc.Locator, packageService pack.PackageService) (string, error) {
	manifest, err := pack.GetPackageManifest(packageService, locator)
	if err != nil {
		return "", trace.Wrap(err)
	}
	for _, label := range manifest.Labels {
		if label.Name == searchLabel {
			return label.Value, nil
		}
	}
	return "", trace.NotFound("label %v not found on package %v", searchLabel, locator)
}

// setLeaderElection creates a phase that will change the leader election state in the cluster
// enable - the list of servers to enable election on
// disable - the list of servers to disable election on
// server - The server the phase should be executed on, and used to name the phase
// key - is the identifier of the phase (combined with server.Hostname)
// msg - is a format string used to describe the phase
func setLeaderElection(enable, disable []storage.Server, server storage.Server, key, msg string) phase {
	return phase{
		ID:          fmt.Sprintf("%s-%s", key, server.Hostname),
		Executor:    electionStatus,
		Description: fmt.Sprintf(msg, server.Hostname),
		Data: &storage.OperationPhaseData{
			Server: &server,
			ElectionChange: &storage.ElectionChange{
				EnableServers:  enable,
				DisableServers: disable,
			}},
	}
}

// resolve resolves dependencies between phases in the specified plan
func resolve(plan *storage.OperationPlan) {
	resolveIDs(nil, plan.Phases)
	resolveRequirements(nil, plan.Phases)
}

// resolveIDs travels the phase tree and turns relative IDs into absolute
func resolveIDs(parent *phase, phases []storage.OperationPhase) {
	for i := range phases {
		if !path.IsAbs(phases[i].ID) {
			phases[i].ID = parent.Child(phase(phases[i]))
		}
		resolveIDs((*phase)(&phases[i]), phases[i].Phases)
	}
}

// resolveRequirements travels the phase tree and resolves relative IDs in requirements into absolute
func resolveRequirements(parent *phase, phases []storage.OperationPhase) {
	for i := range phases {
		var requires []string
		for _, req := range phases[i].Requires {
			if path.IsAbs(req) {
				requires = append(requires, req)
			} else {
				requires = append(requires, parent.ChildLiteral(req))
			}
		}
		phases[i].Requires = requires
		resolveRequirements((*phase)(&phases[i]), phases[i].Phases)
	}
}

func checkAndSetServerDefaults(servers []storage.Server, client corev1.NodeInterface) ([]storage.Server, error) {
	nodes, err := utils.GetNodes(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	masterIPs := utils.GetMasters(nodes)
	// set cluster role that might have not have been set
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
					continue
				}
			}

			return nil, trace.NotFound("unable to locate kubernetes node with label %s=%s, please check each kubernetes node and re-add the %v label if it is missing",
				defaults.KubernetesAdvertiseIPLabel, server.AdvertiseIP, defaults.KubernetesAdvertiseIPLabel)
		}
		// Overwrite the Server Nodename with the name of the kubernetes node,
		// to fix any internal consistency issues that may occur in our internal data
		servers[i].Nodename = node.Name
	}
	return servers, nil
}
