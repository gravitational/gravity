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

package opsservice

import (
	"context"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// RotateSecrets rotates secrets package for the server specified in the request
func (o *Operator) RotateSecrets(req ops.RotateSecretsRequest) (resp *ops.RotatePackageResponse, err error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	node := &ProvisionedServer{
		Server: req.Server,
		Profile: schema.NodeProfile{
			ServiceRole: schema.ServiceRole(req.Server.ClusterRole),
		},
	}

	cluster, err := o.openSite(req.Key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secretsPackage := cluster.planetSecretsNextPackage(node, req.RuntimePackage.Version)
	if req.Package != nil {
		secretsPackage = *req.Package
	}

	if req.DryRun {
		return &ops.RotatePackageResponse{Locator: secretsPackage}, nil
	}

	op, err := ops.GetCompletedInstallOperation(req.Key, o)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := cluster.newOperationContext(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err = cluster.rotateSecrets(ctx, secretsPackage, node, serviceSubnet(op.InstallExpand, req.ServiceCIDR))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// RotateTeleportConfig generates teleport configuration for the server specified in the provided request
func (o *Operator) RotateTeleportConfig(req ops.RotateTeleportConfigRequest) (masterConfig *ops.RotatePackageResponse, nodeConfig *ops.RotatePackageResponse, err error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	operation, err := o.GetSiteOperation(req.Key)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	nodeProfile, err := o.getNodeProfile(*operation, req.Server)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	node := &ProvisionedServer{
		Server:  req.Server,
		Profile: *nodeProfile,
	}

	cluster, err := o.openSite(req.Key.SiteKey())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	masterConfigPackage := cluster.teleportNextMasterConfigPackage(node, req.TeleportPackage.Version)
	if req.MasterPackage != nil {
		masterConfigPackage = *req.MasterPackage
	}

	nodeConfigPackage := cluster.teleportNextNodeConfigPackage(node, req.TeleportPackage.Version)
	if req.NodePackage != nil {
		nodeConfigPackage = *req.NodePackage
	}

	ctx, err := cluster.newOperationContext(*operation)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if node.ClusterRole == string(schema.ServiceRoleMaster) {
		masterConfig, err = cluster.getTeleportMasterConfig(ctx, masterConfigPackage, node)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		// Teleport nodes on masters prefer their local auth server
		// but will try all other masters if the local gravity-site
		// isn't running.
		nodeConfig, err = cluster.getTeleportNodeConfig(ctx,
			append(req.MasterIPs, constants.Localhost),
			nodeConfigPackage,
			node)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	} else {
		nodeConfig, err = cluster.getTeleportNodeConfig(ctx, req.MasterIPs, nodeConfigPackage, node)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}

	return masterConfig, nodeConfig, nil
}

func (o *Operator) getNodeProfile(operation ops.SiteOperation, node storage.Server) (*schema.NodeProfile, error) {
	updatePackage, err := operation.Update.Package()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateApp, err := o.cfg.Apps.GetApp(*updatePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nodeProfile, err := updateApp.Manifest.NodeProfiles.ByName(node.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// keep the service role
	nodeProfile.ServiceRole = schema.ServiceRole(node.ClusterRole)
	return nodeProfile, nil
}

// RotatePlanetConfig rotates planet configuration package for the server specified in the request
func (o *Operator) RotatePlanetConfig(req ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	clusterKey := req.Key.SiteKey()
	cluster, err := o.openSite(clusterKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	nodeProfile, err := req.Manifest.NodeProfiles.ByName(req.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Keep the service role
	nodeProfile.ServiceRole = schema.ServiceRole(req.Server.ClusterRole)

	node := ProvisionedServer{
		Server:  req.Server,
		Profile: *nodeProfile,
	}

	configPackage := cluster.planetNextConfigPackage(&node, req.RuntimePackage.Version)
	if req.Package != nil {
		configPackage = *req.Package
	}

	if req.DryRun {
		return &ops.RotatePackageResponse{Locator: configPackage}, nil
	}

	runner := &localRunner{}

	memberListOutput, err := runner.Run(cluster.etcdctlCommand("member", "list")...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query etcd member list: %s", memberListOutput)
	}

	initialCluster, err := utils.EtcdInitialCluster(string(memberListOutput))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memberList, err := utils.EtcdParseMemberList(string(memberListOutput))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var etcd etcdConfig
	if memberList.HasMember(node.EtcdMemberName(cluster.domainName)) {
		etcd = etcdConfig{
			initialCluster:      initialCluster,
			initialClusterState: etcdExistingCluster,
			proxyMode:           etcdProxyOff,
		}
	} else {
		etcd = etcdConfig{
			initialCluster:      initialCluster,
			initialClusterState: etcdExistingCluster,
			proxyMode:           etcdProxyOn,
		}
	}

	installOperation, err := ops.GetCompletedInstallOperation(clusterKey, o)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var master *storage.Server
	for _, server := range cluster.servers() {
		if server.IsMaster() {
			master = &server
			break
		}
	}
	if master == nil {
		return nil, trace.NotFound("couldn't find master server: %v", req)
	}

	dockerConfig := cluster.dockerConfig()
	checks.OverrideDockerConfig(&dockerConfig,
		checks.DockerConfigFromSchema(req.Manifest.SystemOptions.DockerConfig()))

	config := planetConfig{
		master: masterConfig{
			addr:            master.AdvertiseIP,
			electionEnabled: node.IsMaster(),
		},
		manifest:      req.Manifest,
		server:        node,
		installExpand: *installOperation,
		etcd:          etcd,
		docker:        dockerConfig,
		dockerRuntime: node.Docker,
		planetPackage: req.RuntimePackage,
		configPackage: configPackage,
		env:           req.Env,
	}

	if len(req.Config) != 0 {
		clusterConfig, err := clusterconfig.Unmarshal(req.Config)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		config.config = clusterConfig
	}

	resp, err := cluster.getPlanetConfigPackage(config)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}

	log.WithFields(log.Fields{
		"server":  node.String(),
		"package": configPackage.String(),
	}).Info("Created new planet configuration.")
	return resp, nil
}

// ConfigureNode prepares the node for the upgrade, for example updates necessary directories
// permissions and creates missing ones
func (o *Operator) ConfigureNode(req ops.ConfigureNodeRequest) error {
	node := &ProvisionedServer{
		Server: req.Server,
		Profile: schema.NodeProfile{
			ServiceRole: schema.ServiceRole(req.Server.ClusterRole),
		},
	}

	cluster, err := o.GetSite(req.SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}

	operation, err := o.GetSiteOperation(req.SiteOperationKey())
	if err != nil {
		return trace.Wrap(err)
	}

	updatePackage, err := operation.Update.Package()
	if err != nil {
		return trace.Wrap(err)
	}

	updateApp, err := o.cfg.Apps.GetApp(*updatePackage)
	if err != nil {
		return trace.Wrap(err)
	}

	commands, err := remoteDirectories(*operation, node, updateApp.Manifest,
		cluster.ServiceUser.UID, cluster.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}

	runner := &localRunner{}
	for _, command := range commands {
		out, err := runner.Run(command.Args...)
		if err != nil {
			return trace.Wrap(err, "failed to run %v: %s", command, out)
		}
	}

	return nil
}

// createUpdateOperation defines the entry-point for the system/application update.
// It is responsible for creating the update operation, downloading the new version of the gravity
// binary and spawning an update controller to supervise the process.
func (s *site) createUpdateOperation(context context.Context, req ops.CreateSiteAppUpdateOperationRequest) (*ops.SiteOperationKey, error) {
	s.Infof("createUpdateOperation(%#v)", req)

	installOperation, err := ops.GetCompletedInstallOperation(s.key, s.service)
	if err != nil {
		return nil, trace.Wrap(err, "failed to find a completed install operation")
	}

	if err := s.validateUpdateOperationRequest(req, installOperation.Provisioner); err != nil {
		return nil, trace.Wrap(err)
	}

	op := ops.SiteOperation{
		ID:          uuid.New(),
		AccountID:   s.key.AccountID,
		SiteDomain:  s.key.SiteDomain,
		Type:        ops.OperationUpdate,
		Created:     s.clock().UtcNow(),
		CreatedBy:   storage.UserFromContext(context),
		Updated:     s.clock().UtcNow(),
		State:       ops.OperationStateUpdateInProgress,
		Provisioner: installOperation.Provisioner,
		Update: &storage.UpdateOperationState{
			UpdatePackage: req.App,
			Vars:          req.Vars,
		},
	}

	ctx, err := s.newOperationContext(op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer ctx.Close()

	key, err := s.getOperationGroup().createSiteOperationWithOptions(op,
		createOperationOptions{req.Force})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resetClusterState := func() {
		if err == nil {
			return
		}
		logger := log.WithField("operation", op.Key())
		// Fail the operation and reset cluster state.
		// It is important to complete the operation as subsequent same type operations
		// will not be able to complete if there's an existing incomplete one
		if errReset := ops.FailOperationAndResetCluster(context, *key, s.service, err.Error()); errReset != nil {
			logger.WithFields(log.Fields{
				log.ErrorKey: errReset,
				"operation":  key,
			}).Warn("Failed to mark operation as failed.")
		}
	}
	defer resetClusterState()

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 0,
		Message:    "initializing the operation",
	})

	if !req.StartAgents {
		return key, nil
	}

	updatePackage, err := loc.ParseLocator(req.App)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateApp, err := s.apps().GetApp(*updatePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = s.startUpdateAgent(context, ctx, updateApp)
	if err != nil {
		return key, trace.Wrap(err,
			"update operation was created but the automatic update agent failed to start. Refer to the documentation on how to proceed with manual update")
	}

	return key, nil
}

func (s *site) getRuntimeApplication(locator loc.Locator) (*app.Application, error) {
	application, err := s.apps().GetApp(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runtimeApplication, err := s.apps().GetApp(*(application.Manifest.Base()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return runtimeApplication, nil
}

func (s *site) validateUpdateOperationRequest(req ops.CreateSiteAppUpdateOperationRequest, provisioner string) error {
	currentPackage, err := s.appPackage()
	if err != nil {
		return trace.Wrap(err)
	}
	updatePackage, err := loc.ParseLocator(req.App)
	if err != nil {
		return trace.Wrap(err)
	}
	err = pack.CheckUpdatePackage(*currentPackage, *updatePackage)
	if err != nil {
		return trace.Wrap(err)
	}
	currentRuntime, err := s.getRuntimeApplication(*currentPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	updateRuntime, err := s.getRuntimeApplication(*updatePackage)
	if err != nil {
		return trace.Wrap(err)
	}
	err = checkRuntimeUpgradePath(checkRuntimeUpgradePathRequest{
		fromRuntime: currentRuntime.Package,
		toRuntime:   updateRuntime.Package,
		packages:    s.packages(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// the new package must exist in the Ops Center
	newEnvelope, err := s.packages().ReadPackageEnvelope(*updatePackage)
	if err != nil {
		return trace.Wrap(err)
	}
	return s.checkUpdateParameters(newEnvelope, provisioner)
}

// startUpdateAgent runs deploy procedure on one of the leader nodes
func (s *site) startUpdateAgent(ctx context.Context, opCtx *operationContext, updateApp *app.Application) error {
	master, err := s.getTeleportServer(schema.ServiceLabelRole, string(schema.ServiceRoleMaster))
	if err != nil {
		return trace.Wrap(err)
	}
	proxy, err := s.teleport().GetProxyClient(ctx, s.key.SiteDomain, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	nodeClient, err := proxy.ConnectToNode(ctx, master.Addr, defaults.SSHUser, false)
	if err != nil {
		return trace.Wrap(err)
	}
	defer nodeClient.Close()
	gravityPackage, err := updateApp.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	// determine the server's state dir location
	site, err := s.service.GetSite(s.key)
	if err != nil {
		return trace.Wrap(err)
	}
	advertiseIP, ok := master.Labels[ops.AdvertiseIP]
	if !ok {
		return trace.NotFound("server %v is missing %s label", master, ops.AdvertiseIP)
	}
	stateServer, err := site.ClusterState.FindServerByIP(advertiseIP)
	if err != nil {
		return trace.Wrap(err)
	}
	serverStateDir := stateServer.StateDir()
	agentExecPath := filepath.Join(state.GravityRPCAgentDir(serverStateDir), constants.GravityBin)
	secretsHostDir := filepath.Join(state.GravityRPCAgentDir(serverStateDir), defaults.SecretsDir)
	err = utils.NewSSHCommands(nodeClient.Client).
		// extract new gravity version
		C("rm -rf %s", secretsHostDir).
		C("mkdir -p %s", secretsHostDir).
		C("%s package export --file-mask=%o %s %s --ops-url=%s --insecure --quiet",
			constants.GravityBin, defaults.SharedExecutableMask,
			gravityPackage.String(), agentExecPath, defaults.GravityServiceURL).
		C("%s update init-plan", agentExecPath).
		// distribute agents and upgrade process
		C("%s agent deploy --leader=upgrade --node=sync-plan", agentExecPath).
		WithLogger(s.WithField("node", master.HostName())).
		WithOutput(opCtx.recorder).
		Run(ctx)
	return trace.Wrap(err)
}

// checkUpdateParameters checks if update parameters match
func (s *site) checkUpdateParameters(update *pack.PackageEnvelope, provisioner string) error {
	if update.Manifest == nil {
		return trace.BadParameter("application package %v does not have a manifest", update.Locator)
	}

	updateManifest, err := schema.ParseManifestYAML(update.Manifest)
	if err != nil {
		return trace.Wrap(err, "failed to parse manifest for %v", update.Locator)
	}

	manifest := s.app.Manifest

	// Verify update application has all profiles of the installed one
	for _, profile := range manifest.NodeProfiles {
		_, err = updateManifest.NodeProfiles.ByName(profile.Name)
		if err != nil {
			if trace.IsNotFound(err) {
				return trace.NotFound("profile %q not found in update manifest", profile.Name)
			}
			return trace.Wrap(err)
		}
	}

	// we also do not support switching between network types
	networkType := manifest.GetNetworkType(s.provider, provisioner)
	updateNetworkType := updateManifest.GetNetworkType(s.provider, provisioner)
	if networkType != updateNetworkType {
		return trace.BadParameter("changing network type is not supported (current %q, new %q)",
			networkType, updateNetworkType)
	}

	err = s.validateDockerConfig(*updateManifest)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.validateStorageConfig(*updateManifest)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// validateStorageConfig makes sure that persistent storage configuration
// in the new version is compatible with the old version.
func (s *site) validateStorageConfig(updateManifest schema.Manifest) error {
	installedEnabled := s.app.Manifest.OpenEBSEnabled()
	updateEnabled := updateManifest.OpenEBSEnabled()

	// OpenEBS wasn't enabled before, and isn't enabled in the new version,
	// nothing to do.
	if !installedEnabled && !updateEnabled {
		return nil
	}

	// OpenEBS wasn't enabled before, but enabled in the new version, no
	// specific checks are required.
	if !installedEnabled && updateEnabled {
		// TODO(r0mant): Check if OpenEBS was installed "out of band" and bail out?
		return nil
	}

	// OpenEBS was enabled before, but disabled in the new version, we don't
	// support uninstalling it at the moment.
	if installedEnabled && !updateEnabled {
		return trace.BadParameter(`The cluster has OpenEBS integration enabled but it's disabled in the version you're trying to upgrade to.
Disabling OpenEBS integration for existing clusters is unsupported at the moment.`)
	}

	// TODO(r0mant): At the moment we do not support upgrading storage-app,
	//               a proper upgrade procedure should be implemented first:
	//               https://github.com/openebs/openebs/tree/master/k8s/upgrades.
	installed, err := s.app.Manifest.Dependencies.ByName(defaults.StorageAppName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	update, err := updateManifest.Dependencies.ByName(defaults.StorageAppName)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// Technically, this error should never be seen by a user - it is mostly
	// meant for us to make sure proper upgrade procedure is implemented before
	// we release another version of storage-app.
	if installed != nil && update != nil && installed.Version != update.Version {
		return trace.BadParameter("Upgrading OpenEBS is unsupported at the moment.")
	}

	return nil
}

func (s *site) validateDockerConfig(updateManifest schema.Manifest) error {
	docker := updateManifest.SystemOptions.DockerConfig()
	if docker == nil {
		// No changes
		return nil
	}

	existingDocker := s.dockerConfig()
	if existingDocker.IsEmpty() {
		installOperation, err := ops.GetCompletedInstallOperation(s.key, s.service)
		if err != nil {
			return trace.Wrap(err)
		}

		defaultConfig := checks.DockerConfigFromSchemaValue(s.app.Manifest.SystemDocker())
		checks.OverrideDockerConfig(&defaultConfig, installOperation.InstallExpand.Vars.System.Docker)
		existingDocker = defaultConfig
	}

	if docker.StorageDriver != existingDocker.StorageDriver &&
		!utils.StringInSlice(constants.DockerSupportedTargetDrivers, docker.StorageDriver) {
		return trace.BadParameter(`Updating Docker storage driver to %q is not supported.
The storage driver can only be updated to one of %q.
`, docker.StorageDriver, constants.DockerSupportedTargetDrivers)
	}
	return nil
}
