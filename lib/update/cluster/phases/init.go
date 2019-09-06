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
	"path/filepath"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// updatePhaseInit is the update init phase which performs the following:
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
	// updateManifest specifies the manifest of the update application
	updateManifest schema.Manifest
	// installedApp references the installed application instance
	installedApp app.Application
	// existingDocker describes the existing Docker configuration
	existingDocker storage.DockerConfig
	existingDNS    storage.DNSConfig
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
		return nil, trace.BadParameter("no application package specified for phase %v", p.Phase)
	}
	if p.Phase.Data.InstalledPackage == nil {
		return nil, trace.BadParameter("no installed application package specified for phase %v", p.Phase)
	}
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := operator.GetSiteOperation(fsm.OperationKey(p.Plan))
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

	existingDocker := checks.DockerConfigFromSchemaValue(installedApp.Manifest.SystemDocker())
	checks.OverrideDockerConfig(&existingDocker, installOperation.InstallExpand.Vars.System.Docker)

	return &updatePhaseInit{
		Backend:        backend,
		LocalBackend:   localBackend,
		Operator:       operator,
		Packages:       packages,
		Users:          users,
		Cluster:        *cluster,
		Operation:      *operation,
		Servers:        p.Plan.Servers,
		FieldLogger:    logger,
		updateManifest: app.Manifest,
		installedApp:   *installedApp,
		existingDocker: existingDocker,
		existingDNS:    p.Plan.DNSConfig,
	}, nil
}

// PreCheck is a no-op for this phase
func (p *updatePhaseInit) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for the init phase
func (p *updatePhaseInit) PostCheck(context.Context) error {
	return nil
}

// Execute prepares the update.
func (p *updatePhaseInit) Execute(context.Context) error {
	err := removeLegacyUpdateDirectory(p.FieldLogger)
	if err != nil {
		p.WithError(err).Warn("Failed to remove legacy update directory.")
	}
	if err := p.createAdminAgent(); err != nil {
		return trace.Wrap(err, "failed to create cluster admin agent")
	}
	if err := p.upsertServiceUser(); err != nil {
		return trace.Wrap(err, "failed to upsert service user")
	}
	if err := p.initRPCCredentials(); err != nil {
		return trace.Wrap(err, "failed to init RPC credentials")
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

func removeLegacyUpdateDirectory(log log.FieldLogger) error {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	updateDir := filepath.Join(state.GravityUpdateDir(stateDir), "gravity")
	fi, err := os.Stat(updateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return trace.Wrap(trace.ConvertSystemError(err))
	}
	if !fi.IsDir() {
		return nil
	}
	log.WithField("dir", updateDir).Debug("Remove legacy update directory.")
	return trace.ConvertSystemError(os.RemoveAll(updateDir))
}

// Rollback rolls back the init phase
func (p *updatePhaseInit) Rollback(context.Context) error {
	if err := p.removeConfiguredPackages(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// removeConfiguredPackages removes packages configured during init phase
func (p *updatePhaseInit) removeConfiguredPackages() error {
	// all packages created during this operation were marked
	// with corresponding operation-id label
	p.Info("Removing configured packages.")
	return pack.ForeachPackageInRepo(p.Packages, p.Operation.SiteDomain,
		func(e pack.PackageEnvelope) error {
			if e.HasLabel(pack.OperationIDLabel, p.Operation.ID) {
				p.Infof("Removing package %q.", e.Locator)
				return p.Packages.DeletePackage(e.Locator)
			}
			return nil
		})
}
