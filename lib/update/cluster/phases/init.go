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

package phases

import (
	"context"
	"os"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
)

// updatePhaseInit is the update init phase which performs the following:
//   - generate new secrets
//   - generate new planet container configuration where necessary
//   - verifies that the admin agent user exists
//   - updates the cluster with service user details
//   - cleans up state left from previous versions
type updatePhaseInit struct {
	// Backend is the cluster etcd backend
	Backend storage.Backend
	// LocalBackend is the local state backend
	LocalBackend storage.Backend
	// Operator is the local cluster ops service
	Operator ops.Operator
	// Packages is the cluster package service
	Packages pack.PackageService
	// Users is the cluster users service
	Users users.Identity
	// Cluster is the local cluster
	Cluster ops.Site
	// Operation is the current update operation
	Operation ops.SiteOperation
	// Servers is the list of local cluster servers
	Servers []storage.Server
	// FieldLogger is used for logging
	log.FieldLogger
	// app references the update application
	app app.Application
	// installedApp references the installed application instance
	installedApp app.Application
	// existingDocker describes the existing Docker configuration
	existingDocker storage.DockerConfig
	// existingDNS describes the existing DNS configuration
	existingDNS           storage.DNSConfig
	existingEnviron       map[string]string
	existingClusterConfig []byte
}

// NewUpdatePhaseInit creates a new update init phase executor
func NewUpdatePhaseInit(
	p fsm.ExecutorParams,
	operator ops.Operator,
	apps app.Applications,
	backend, localBackend storage.Backend,
	packages pack.PackageService,
	users users.Identity,
	logger log.FieldLogger,
) (*updatePhaseInit, error) {
	if p.Phase.Data == nil || p.Phase.Data.Package == nil {
		return nil, trace.NotFound("no application package specified for phase %v", p.Phase)
	}
	if p.Phase.Data.InstalledPackage == nil {
		return nil, trace.NotFound("no installed application package specified for phase %v", p.Phase)
	}
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := ops.GetLastUpdateOperation(cluster.Key(), operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installOperation, err := ops.GetCompletedInstallOperation(cluster.Key(), operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query application")
	}
	installedApp, err := apps.GetApp(*p.Phase.Data.InstalledPackage)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query installed application")
	}
	env, err := operator.GetClusterEnvironmentVariables(operation.ClusterKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterConfig, err := operator.GetClusterConfiguration(operation.ClusterKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configBytes, err := clusterconfig.Marshal(clusterConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger.Infof("Existing DNS configuration: %v.", p.Plan.DNSConfig)

	existingDocker := checks.DockerConfigFromSchemaValue(installedApp.Manifest.SystemDocker())
	checks.OverrideDockerConfig(&existingDocker, installOperation.InstallExpand.Vars.System.Docker)

	return &updatePhaseInit{
		Backend:               backend,
		LocalBackend:          localBackend,
		Operator:              operator,
		Packages:              packages,
		Users:                 users,
		Cluster:               *cluster,
		Operation:             *operation,
		Servers:               p.Plan.Servers,
		FieldLogger:           logger,
		app:                   *app,
		installedApp:          *installedApp,
		existingDocker:        existingDocker,
		existingDNS:           p.Plan.DNSConfig,
		existingClusterConfig: configBytes,
		existingEnviron:       env.GetKeyValues(),
	}, nil
}

// PreCheck makes sure that init phase is being executed from one of master nodes
func (p *updatePhaseInit) PreCheck(context.Context) error {
	return trace.Wrap(fsm.CheckMasterServer(p.Servers))
}

// PostCheck is no-op for the init phase
func (p *updatePhaseInit) PostCheck(context.Context) error {
	return nil
}

// Execute prepares the update.
func (p *updatePhaseInit) Execute(context.Context) error {
	err := removeLegacyUpdateDirectory(p.FieldLogger)
	if err != nil {
		return trace.Wrap(err, "failed to remove legacy update directory")
	}
	if err := p.createAdminAgent(); err != nil {
		return trace.Wrap(err, "failed to create cluster admin agent")
	}
	if err := p.upsertServiceUser(); err != nil {
		return trace.Wrap(err, "failed to upsert service user")
	}
	if err := p.initRPCCredentials(); err != nil {
		return trace.Wrap(err, "failed to update RPC credentials")
	}
	if err := p.updateClusterRoles(); err != nil {
		return trace.Wrap(err, "failed to update RPC credentials")
	}
	if err := p.updateClusterDNSConfig(); err != nil {
		return trace.Wrap(err, "failed to update DNS configuration")
	}
	if err := p.updateDockerConfig(); err != nil {
		return trace.Wrap(err, "failed to update Docker configuration")
	}
	for _, server := range p.Servers {
		if err := p.rotateSecrets(server); err != nil {
			return trace.Wrap(err, "failed to rotate secrets for %v", server)
		}
		updatePlanet, err := planetNeedsUpdate(server.Role, p.installedApp, p.app)
		if err != nil {
			return trace.Wrap(err)
		}
		if !updatePlanet {
			continue
		}
		runtimePackage, err := p.app.Manifest.RuntimePackageForProfile(server.Role)
		if err != nil {
			return trace.Wrap(err)
		}
		err = p.rotatePlanetConfig(server, *runtimePackage)
		if err != nil {
			return trace.Wrap(err, "failed to rotate planet configuration for %v", server)
		}
	}
	return nil
}

// Rollback is no-op for the init phase
func (p *updatePhaseInit) Rollback(context.Context) error {
	return nil
}

func (p *updatePhaseInit) initRPCCredentials() error {
	// FIXME: the secrets package is currently only generated once.
	// Even though the package is generated with some time buffer in advance,
	// we need to make sure if the existing package needs to be rotated (i.e.
	// as expiring soon).
	// This will ether need to generate a new package version and then the
	// problem becomes how the agents will know the name of the package.
	// Or, the package version is recycled and then we need to make sure
	// to restart the cluster controller (gravity-site) to make sure it has
	// reloaded its copy of the credentials.
	// See: https://github.com/gravitational/gravity/issues/3607.
	pkg, err := rpc.InitRPCCredentials(p.Packages)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	if trace.IsAlreadyExists(err) {
		p.Info("RPC credentials already initialized.")
	} else {
		p.Infof("Initialized RPC credentials: %v.", pkg)
	}

	return nil
}

func (p *updatePhaseInit) updateClusterRoles() error {
	p.Info("Update cluster roles.")
	cluster, err := p.Backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	state := make(map[string]storage.Server, len(p.Servers))
	for _, server := range p.Servers {
		state[server.AdvertiseIP] = server
	}

	for i, server := range cluster.ClusterState.Servers {
		if server.ClusterRole != "" {
			continue
		}
		stateServer := state[server.AdvertiseIP]
		cluster.ClusterState.Servers[i].ClusterRole = stateServer.ClusterRole
	}

	_, err = p.Backend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *updatePhaseInit) updateClusterDNSConfig() error {
	p.Info("Update cluster DNS configuration.")
	cluster, err := p.Backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	if cluster.DNSConfig.IsEmpty() {
		cluster.DNSConfig = p.existingDNS
	}

	_, err = p.Backend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// updateDockerConfig persists the Docker configuration
// of the currently installed application
func (p *updatePhaseInit) updateDockerConfig() error {
	cluster, err := p.Backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	if !cluster.ClusterState.Docker.IsEmpty() {
		// Nothing to do
		return nil
	}

	cluster.ClusterState.Docker = p.existingDocker
	_, err = p.Backend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *updatePhaseInit) upsertServiceUser() error {
	cluster, err := p.Backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	if !cluster.ServiceUser.IsEmpty() {
		// Nothing to do
		return nil
	}

	p.Info("Create service user.")
	user, err := install.GetOrCreateServiceUser(defaults.ServiceUserID, defaults.ServiceGroupID)
	if err != nil {
		return trace.Wrap(err,
			"failed to lookup/create service user %q", defaults.ServiceUser)
	}

	cluster.ServiceUser.Name = user.Name
	cluster.ServiceUser.UID = strconv.Itoa(user.UID)
	cluster.ServiceUser.GID = strconv.Itoa(user.GID)

	_, err = p.Backend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *updatePhaseInit) createAdminAgent() error {
	p.Info("Create admin agent user.")
	// create a cluster admin agent as it may not exist yet
	// when upgrading from older versions
	email := storage.ClusterAdminAgent(p.Cluster.Domain)
	_, err := p.Users.CreateClusterAdminAgent(p.Cluster.Domain, storage.NewUser(email, storage.UserSpecV2{
		AccountID:   p.Cluster.AccountID,
		ClusterName: p.Cluster.Domain,
	}))
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	return nil
}

func (p *updatePhaseInit) rotateSecrets(server storage.Server) error {
	p.Infof("Generate new secrets configuration package for %v.", server)
	resp, err := p.Operator.RotateSecrets(ops.RotateSecretsRequest{
		AccountID:   p.Operation.AccountID,
		ClusterName: p.Operation.SiteDomain,
		Server:      server,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.Packages.CreatePackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debugf("Rotated secrets package for %v: %v.", server, resp.Locator)
	return nil
}

func (p *updatePhaseInit) rotatePlanetConfig(server storage.Server, runtimePackage loc.Locator) error {
	p.Infof("Generate new runtime configuration package for %v.", server)
	resp, err := p.Operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
		AccountID:   p.Operation.AccountID,
		ClusterName: p.Operation.SiteDomain,
		OperationID: p.Operation.ID,
		Server:      server,
		Manifest:    p.app.Manifest,
		Package:     runtimePackage,
		Config:      p.existingClusterConfig,
		Env:         p.existingEnviron,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.Packages.UpsertPackage(resp.Locator, resp.Reader,
		pack.WithLabels(resp.Labels))
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Generated new runtime configuration package for %v: %v.", server, resp.Locator)
	return nil
}

func removeLegacyUpdateDirectory(log log.FieldLogger) error {
	const updateDir = "/var/lib/gravity/site/update/gravity"

	fi, err := os.Stat(updateDir)
	err = trace.ConvertSystemError(err)
	if trace.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return trace.Wrap(err)
	}

	if !fi.IsDir() {
		return nil
	}

	log.Debugf("Removing legacy update directory %v.", updateDir)
	err = os.RemoveAll(updateDir)
	return trace.ConvertSystemError(err)
}

// planetNeedsUpdate returns true if the planet version in the update application is
// greater than in the installed one for the specified node profile
func planetNeedsUpdate(profile string, installed, update app.Application) (needsUpdate bool, err error) {
	installedProfile, err := installed.Manifest.NodeProfiles.ByName(profile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateProfile, err := update.Manifest.NodeProfiles.ByName(profile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateRuntimePackage, err := update.Manifest.RuntimePackage(*updateProfile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateVersion, err := updateRuntimePackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	runtimePackage, err := getRuntimePackage(installed.Manifest, *installedProfile, schema.ServiceRoleMaster)
	if err != nil {
		return false, trace.Wrap(err)
	}

	version, err := runtimePackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	logrus.Debugf("Runtime installed: %v, runtime to update to: %v.", runtimePackage, updateRuntimePackage)
	updateNewer := updateVersion.Compare(*version) > 0
	return updateNewer, nil
}

func getRuntimePackage(manifest schema.Manifest, profile schema.NodeProfile, clusterRole schema.ServiceRole) (*loc.Locator, error) {
	runtimePackage, err := manifest.RuntimePackage(profile)
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
