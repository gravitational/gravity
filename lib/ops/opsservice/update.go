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
	"path/filepath"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// RotateSecrets rotates secrets package for the server specified in the request
func (o *Operator) RotateSecrets(req ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error) {
	node := &ProvisionedServer{
		PackageSet: NewPackageSet(),
		Server:     req.Server,
		Profile: schema.NodeProfile{
			ServiceRole: schema.ServiceRole(req.Server.ClusterRole),
		},
	}

	op, err := ops.GetCompletedInstallOperation(req.SiteKey(), o)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := o.openSite(req.SiteKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, err := cluster.newOperationContext(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := cluster.rotateSecrets(ctx, node, *op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// RotatePlanetConfig rotates planet configuration package for the server specified in the request
func (o *Operator) RotatePlanetConfig(req ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error) {
	operation, err := o.GetSiteOperation(req.SiteOperationKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updatePackage, err := operation.Update.Package()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateApp, err := o.cfg.Apps.GetApp(*updatePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nodeProfile, err := updateApp.Manifest.NodeProfiles.ByName(req.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Keep the service role
	nodeProfile.ServiceRole = schema.ServiceRole(req.Server.ClusterRole)

	node := &ProvisionedServer{
		PackageSet: NewPackageSet(),
		Server:     req.Server,
		Profile:    *nodeProfile,
	}

	runner := &localRunner{}

	cluster, err := o.openSite(req.SiteKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
			initialClusterState: etcdNewCluster,
			proxyMode:           etcdProxyOff,
		}
	} else {
		etcd = etcdConfig{
			initialCluster:      initialCluster,
			initialClusterState: etcdExistingCluster,
			proxyMode:           etcdProxyOn,
		}
	}

	ctx, err := cluster.newOperationContext(*operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	installOperation, err := ops.GetCompletedInstallOperation(req.SiteKey(), o)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var master *storage.Server
	for _, server := range req.Servers {
		if server.ClusterRole == string(schema.ServiceRoleMaster) {
			master = &server
			break
		}
	}
	if master == nil {
		return nil, trace.NotFound("couldn't find master server: %v", req)
	}

	ctx.update = updateContext{
		masterIP:  master.AdvertiseIP,
		installOp: *installOperation,
		app:       *updateApp,
		gravityPath: filepath.Join(
			state.GravityUpdateDir(node.StateDir()), constants.GravityBin),
	}

	resp, err := cluster.configurePlanetOnNode(
		ctx, runner, node, etcd, updateApp.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// ConfigureNode prepares the node for the upgrade, for example updates necessary directories
// permissions and creates missing ones
func (o *Operator) ConfigureNode(req ops.ConfigureNodeRequest) error {
	node := &ProvisionedServer{
		PackageSet: NewPackageSet(),
		Server:     req.Server,
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
func (s *site) createUpdateOperation(req ops.CreateSiteAppUpdateOperationRequest) (*ops.SiteOperationKey, error) {
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
		Updated:     s.clock().UtcNow(),
		State:       ops.OperationStateUpdateInProgress,
		Provisioner: installOperation.Provisioner,
		Update: &storage.UpdateOperationState{
			UpdatePackage: req.App,
		},
	}

	ctx, err := s.newOperationContext(op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer ctx.Close()

	key, err := s.getOperationGroup().createSiteOperation(op)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create update operation")
	}

	resetSiteState := func() {
		if err == nil {
			return
		}
		errReset := s.setSiteState(ops.SiteStateActive)
		if errReset != nil {
			log.Warningf("failed to reset site state: %v", trace.DebugReport(errReset))
		}
	}
	defer resetSiteState()

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 0,
		Message:    "initializing the operation",
	})

	return key, nil
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
	// the new package must exist in the Ops Center
	newEnvelope, err := s.packages().ReadPackageEnvelope(*updatePackage)
	if err != nil {
		return trace.Wrap(err)
	}
	return s.checkUpdateParameters(newEnvelope, provisioner)
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

	if err = s.validateDockerConfig(*updateManifest); err != nil {
		return trace.Wrap(err)
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
