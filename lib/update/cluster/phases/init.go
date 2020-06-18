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
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
	// Client is the cluster Kubernetes client
	Client *kubernetes.Clientset
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
	existingDocker        storage.DockerConfig
	// existingDNS is the existing DNS configuration
	existingDNS storage.DNSConfig
	// init specifies the optional server-specific initialization
	init *updatePhaseInitServer
}

// NewUpdatePhaseInitLeader creates a new update init phase executor
func NewUpdatePhaseInitLeader(
	p fsm.ExecutorParams,
	operator ops.Operator,
	apps app.Applications,
	backend, localBackend storage.Backend,
	packages, localPackages pack.PackageService,
	users users.Identity,
	client *kubernetes.Clientset,
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

	var init *updatePhaseInitServer
	if p.Phase.Data.Update != nil && len(p.Phase.Data.Update.Servers) != 0 {
		init = &updatePhaseInitServer{
			FieldLogger:   logger,
			localPackages: localPackages,
			clusterName:   cluster.Domain,
			server:        p.Phase.Data.Update.Servers[0],
		}
	}

	return &updatePhaseInit{
		Backend:               backend,
		LocalBackend:          localBackend,
		Operator:              operator,
		Packages:              packages,
		Users:                 users,
		Client:                client,
		Cluster:               *cluster,
		Operation:             *operation,
		Servers:               p.Plan.Servers,
		FieldLogger:           logger,
		updateManifest:        app.Manifest,
		installedApp:          *installedApp,
		existingDocker:        existingDocker,
		existingDNS:           p.Plan.DNSConfig,
		init:           init,
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
func (p *updatePhaseInit) Execute(ctx context.Context) error {
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
	if err := p.updateRPCCredentials(); err != nil {
		return trace.Wrap(err, "failed to update RPC credentials")
	}
	if err := p.updateClusterRoles(); err != nil {
		return trace.Wrap(err, "failed to update RPC credentials")
	}
	if err := p.updateClusterDNSConfig(); err != nil {
		return trace.Wrap(err, "failed to update DNS configuration")
	}
	if err := p.updateClusterInfoMap(); err != nil {
		return trace.Wrap(err, "failed to update cluster info config map")
	}
	if err := p.updateDockerConfig(); err != nil {
		return trace.Wrap(err, "failed to update Docker configuration")
	}
	if p.init != nil {
		if err := p.init.Execute(ctx); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Rollback rolls back the init phase
func (p *updatePhaseInit) Rollback(ctx context.Context) error {
	if p.init != nil {
		if err := p.init.Rollback(ctx); err != nil {
			return trace.Wrap(err)
		}
	}
	err := p.restoreRPCCredentials()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err := p.removeConfiguredPackages(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// updateRPCCredentials rotates the RPC credentials used for install/expand/leave operations.
func (p *updatePhaseInit) updateRPCCredentials() error {
	// This assumes that the cluster controller Pods are eventually restarted
	// by the upcoming phase for these changes to take effect.
	//
	// Currently the upgrade short-circuits the application-only upgrades by not
	// including the init phase so this is safe.
	//
	// Keep it in mind for future changes.
	// See https://github.com/gravitational/gravity/issues/3607 for more details when we had
	// to be careful about it previously.
	p.Info("Update RPC credentials")
	err := p.backupRPCCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	loc, err := rpc.UpsertCredentials(p.Packages)
	if err != nil {
		return trace.Wrap(err)
	}
	p.WithField("package", loc.String()).Info("Update RPC credentials.")
	return nil
}

func (p *updatePhaseInit) backupRPCCredentials() error {
	p.Info("Backup RPC credentials")
	env, rc, err := rpc.LoadCredentialsData(p.Packages)
	if err != nil {
		return trace.Wrap(err)
	}
	defer rc.Close()
	_, err = p.Packages.UpsertPackage(rpcBackupPackage(p.Operation.SiteDomain), rc, pack.WithLabels(env.RuntimeLabels))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *updatePhaseInit) restoreRPCCredentials() error {
	p.Info("Restore RPC credentials from backup")
	env, rc, err := p.Packages.ReadPackage(rpcBackupPackage(p.Operation.SiteDomain))
	if err != nil {
		return trace.Wrap(err)
	}
	defer rc.Close()
	delete(env.RuntimeLabels, pack.OperationIDLabel)
	err = rpc.UpsertCredentialsFromData(p.Packages, rc, env.RuntimeLabels)
	if err != nil {
		return trace.Wrap(err)
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

// updateClusterInfoMap updates the cluster info config map or creates it
// if it doesn't exist yet.
func (p *updatePhaseInit) updateClusterInfoMap() error {
	p.Infof("Update %v config map.", constants.ClusterInfoMap)
	_, err := p.Client.CoreV1().ConfigMaps(constants.KubeSystemNamespace).Get(
		constants.ClusterInfoMap, metav1.GetOptions{})
	if err == nil {
		p.Infof("Config map %v already exists.", constants.ClusterInfoMap)
		return nil
	}
	err = rigging.ConvertError(err)
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// Cluster info config map doesn't exist yet, create it.
	configMap := ops.MakeClusterInfoMap(ops.ConvertOpsSite(p.Cluster))
	_, err = p.Client.CoreV1().ConfigMaps(constants.KubeSystemNamespace).Create(configMap)
	if err != nil {
		return rigging.ConvertError(err)
	}
	p.Infof("Created %v config map.", configMap.Name)
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

// NewUpdatePhaseInit creates a new update init phase executor
func NewUpdatePhaseInitServer(
	p fsm.ExecutorParams,
	localPackages pack.PackageService,
	clusterName string,
	logger log.FieldLogger,
) (*updatePhaseInitServer, error) {
	if p.Phase.Data.Update == nil || len(p.Phase.Data.Update.Servers) == 0 {
		return nil, trace.BadParameter("no server specified for phase %q", p.Phase.ID)
	}
	return &updatePhaseInitServer{
		FieldLogger:   logger,
		localPackages: localPackages,
		clusterName:   clusterName,
		server:        p.Phase.Data.Update.Servers[0],
	}, nil
}

// updateExistingPackageLabels updates labels on existing packages
// so the system package pull step can find and pull correct package updates.
//
// For legacy runtime packages ('planet-master' and 'planet-node')
// the sibling runtime package (i.e. 'planet-master' on a regular node
// and vice versa), will be updated to _not_ include the installed label
// to simplify the search
func (p *updatePhaseInitServer) updateExistingPackageLabels() error {
	installedRuntime := p.server.Runtime.Installed
	runtimeConfigLabels, err := updateRuntimeConfigLabels(p.localPackages, installedRuntime)
	if err != nil {
		return trace.Wrap(err)
	}
	teleportConfigLabels, err := updateTeleportConfigLabels(p.localPackages, p.clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	secretLabels, err := updateRuntimeSecretLabels(p.localPackages)
	if err != nil {
		return trace.Wrap(err)
	}
	updates := append(runtimeConfigLabels, secretLabels...)
	updates = append(updates, teleportConfigLabels...)
	updates = append(updates, pack.LabelUpdate{
		Locator: installedRuntime,
		Add:     utils.CombineLabels(pack.RuntimePackageLabels, pack.InstalledLabels),
	})
	// FIXME: remove legacy package handling as 5.5 being the minimum does not have them
	if loc.IsLegacyRuntimePackage(installedRuntime) {
		var runtimePackageToClear loc.Locator
		switch installedRuntime.Name {
		case loc.LegacyPlanetMaster.Name:
			runtimePackageToClear = loc.LegacyPlanetNode.WithLiteralVersion(installedRuntime.Version)
		case loc.LegacyPlanetNode.Name:
			runtimePackageToClear = loc.LegacyPlanetMaster.WithLiteralVersion(installedRuntime.Version)
		}
		updates = append(updates, pack.LabelUpdate{
			Locator: runtimePackageToClear,
			Add:     pack.RuntimePackageLabels,
			Remove:  []string{pack.InstalledLabel},
		})
	}
	for _, update := range updates {
		p.Info(update.String())
		err := p.localPackages.UpdatePackageLabels(update.Locator, update.Add, update.Remove)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Execute prepares the update on the specified server.
func (p *updatePhaseInitServer) Execute(context.Context) error {
	if err := p.updateExistingPackageLabels(); err != nil {
		return trace.Wrap(err, "failed to update existing package labels")
	}
	return nil
}

// Rollback is a no-op for this phase
func (p *updatePhaseInitServer) Rollback(context.Context) error {
	return nil
}

// PreCheck is a no-op for this phase
func (p *updatePhaseInitServer) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for the init phase
func (p *updatePhaseInitServer) PostCheck(context.Context) error {
	return nil
}

type updatePhaseInitServer struct {
	log.FieldLogger
	server        storage.UpdateServer
	localPackages pack.PackageService
	clusterName   string
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

func updateRuntimeConfigLabels(packages pack.PackageService, installedRuntime loc.Locator) ([]pack.LabelUpdate, error) {
	runtimeConfig, err := pack.FindInstalledConfigPackage(packages, installedRuntime)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if runtimeConfig != nil {
		// No update necessary
		return nil, nil
	}
	// Fall back to first configuration package
	runtimeConfig, err = pack.FindConfigPackage(packages, installedRuntime)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Mark this configuration package as installed
	return []pack.LabelUpdate{{
		Locator: *runtimeConfig,
		Add:     pack.InstalledLabels,
	}}, nil
}

func updateTeleportConfigLabels(packages pack.PackageService, clusterName string) ([]pack.LabelUpdate, error) {
	labels := map[string]string{
		pack.PurposeLabel:   pack.PurposeTeleportNodeConfig,
		pack.InstalledLabel: pack.InstalledLabel,
	}
	configEnv, err := pack.FindPackage(packages, func(e pack.PackageEnvelope) bool {
		return e.HasLabels(labels)
	})
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if configEnv != nil {
		// No update necessary
		return nil, nil
	}
	// Fall back to latest available package
	configPackage, err := pack.FindLatestPackageCustom(pack.FindLatestPackageRequest{
		Packages:   packages,
		Repository: clusterName,
		Match: func(e pack.PackageEnvelope) bool {
			return e.Locator.Name == constants.TeleportNodeConfigPackage
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Mark this configuration package as installed
	return []pack.LabelUpdate{{
		Locator: *configPackage,
		Add:     labels,
	}}, nil
}

func updateRuntimeSecretLabels(packages pack.PackageService) ([]pack.LabelUpdate, error) {
	secretsPackage, err := pack.FindSecretsPackage(packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = pack.FindInstalledPackage(packages, *secretsPackage)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		// No update necessary
		return nil, nil
	}
	// Mark this secrets package as installed
	return []pack.LabelUpdate{{
		Locator: *secretsPackage,
		Add:     pack.InstalledLabels,
	}}, nil
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
		return trace.ConvertSystemError(err)
	}
	if !fi.IsDir() {
		return nil
	}
	log.WithField("dir", updateDir).Debug("Remove legacy update directory.")
	return trace.ConvertSystemError(os.RemoveAll(updateDir))
}

func rpcBackupPackage(repository string) loc.Locator {
	return loc.Locator{
		Repository: repository,
		Name:       "rpcagent-secrets-backup",
		Version:    loc.FirstVersion,
	}
}
