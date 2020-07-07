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
	"github.com/gravitational/gravity/lib/constants"
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
	Servers []storage.UpdateServer
	// FieldLogger is used for logging
	log.FieldLogger
	// updateManifest specifies the manifest of the update application
	updateManifest schema.Manifest
	// installedApp references the installed application instance
	installedApp app.Application
	// existingDocker describes the existing Docker configuration
	existingDocker             storage.DockerConfig
	existingDNS                storage.DNSConfig
	existingEnviron            map[string]string
	existingClusterConfigBytes []byte
	existingClusterConfig      clusterconfig.Interface
}

// NewUpdatePhaseInit creates a new update init phase executor
func NewUpdatePhaseInit(
	p fsm.ExecutorParams,
	operator ops.Operator,
	apps app.Applications,
	backend, localBackend storage.Backend,
	packages pack.PackageService,
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
	if p.Phase.Data.Update == nil || len(p.Phase.Data.Update.Servers) == 0 {
		return nil, trace.BadParameter("no servers specified for phase %q", p.Phase.ID)
	}
	cluster, err := operator.GetLocalSite(context.TODO())
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

	existingDocker := checks.DockerConfigFromSchemaValue(installedApp.Manifest.SystemDocker())
	checks.OverrideDockerConfig(&existingDocker, installOperation.InstallExpand.Vars.System.Docker)

	return &updatePhaseInit{
		Backend:                    backend,
		LocalBackend:               localBackend,
		Operator:                   operator,
		Packages:                   packages,
		Users:                      users,
		Client:                     client,
		Cluster:                    *cluster,
		Operation:                  *operation,
		Servers:                    p.Phase.Data.Update.Servers,
		FieldLogger:                logger,
		updateManifest:             app.Manifest,
		installedApp:               *installedApp,
		existingDocker:             existingDocker,
		existingDNS:                p.Plan.DNSConfig,
		existingClusterConfig:      clusterConfig,
		existingClusterConfigBytes: configBytes,
		existingEnviron:            env.GetKeyValues(),
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
	for _, server := range p.Servers {
		if server.Runtime.Update != nil {
			if err := p.rotateSecrets(server); err != nil {
				return trace.Wrap(err, "failed to rotate secrets for %v", server)
			}
			if err := p.rotatePlanetConfig(server); err != nil {
				return trace.Wrap(err, "failed to rotate planet configuration for %v", server)
			}
		}
		if server.Teleport.Update != nil {
			if err := p.rotateTeleportConfig(server); err != nil {
				return trace.Wrap(err, "failed to rotate teleport configuration for %v", server)
			}
		}
	}
	return nil
}

// Rollback rolls back the init phase
func (p *updatePhaseInit) Rollback(ctx context.Context) error {
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
		state[server.AdvertiseIP] = server.Server
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
		p.Info("Config map %v already exists.", constants.ClusterInfoMap)
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

func (p *updatePhaseInit) rotateSecrets(server storage.UpdateServer) error {
	p.Infof("Generate new secrets configuration package for %v.", server)
	resp, err := p.Operator.RotateSecrets(ops.RotateSecretsRequest{
		Key:            p.Operation.ClusterKey(),
		Package:        server.Runtime.SecretsPackage,
		RuntimePackage: server.Runtime.Update.Package,
		Server:         server.Server,
		ServiceCIDR:    p.existingClusterConfig.GetGlobalConfig().ServiceCIDR,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.Packages.CreatePackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	p.Debugf("Rotated secrets package for %v: %v.", server, resp.Locator)
	return nil
}

func (p *updatePhaseInit) rotatePlanetConfig(server storage.UpdateServer) error {
	p.Infof("Generate new runtime configuration package for %v.", server)
	resp, err := p.Operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
		Key:            p.Operation.Key(),
		Server:         server.Server,
		Manifest:       p.updateManifest,
		RuntimePackage: server.Runtime.Update.Package,
		Package:        &server.Runtime.Update.ConfigPackage,
		Config:         p.existingClusterConfigBytes,
		Env:            p.existingEnviron,
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

func (p *updatePhaseInit) rotateTeleportConfig(server storage.UpdateServer) error {
	masterConf, nodeConf, err := p.Operator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
		Key:             p.Operation.Key(),
		Server:          server.Server,
		TeleportPackage: server.Teleport.Update.Package,
		NodePackage:     server.Teleport.Update.NodeConfigPackage,
		MasterIPs:       masterIPs(p.Servers),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if masterConf != nil {
		_, err = p.Packages.UpsertPackage(masterConf.Locator, masterConf.Reader, pack.WithLabels(masterConf.Labels))
		if err != nil {
			return trace.Wrap(err)
		}
		p.Debugf("Rotated teleport master config package for %v: %v.", server, masterConf.Locator)
	}
	_, err = p.Packages.UpsertPackage(nodeConf.Locator, nodeConf.Reader, pack.WithLabels(nodeConf.Labels))
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debugf("Rotated teleport node config package for %v: %v.", server, nodeConf.Locator)
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

func masterIPs(servers []storage.UpdateServer) (addrs []string) {
	for _, server := range servers {
		if server.IsMaster() {
			addrs = append(addrs, server.AdvertiseIP)
		}
	}
	return addrs
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

func rpcBackupPackage(repository string) loc.Locator {
	return loc.Locator{
		Repository: repository,
		Name:       "rpcagent-secrets-backup",
		Version:    loc.FirstVersion,
	}
}
