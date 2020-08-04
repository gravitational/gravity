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
	"path/filepath"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewUpdatePhaseBootstrapLeader creates a new bootstrap phase executor
func NewUpdatePhaseBootstrapLeader(
	p fsm.ExecutorParams,
	operator ops.Operator,
	apps libapp.Applications,
	backend, localBackend, hostLocalBackend storage.Backend,
	localPackages, packages pack.PackageService,
	remote fsm.Remote,
	logger log.FieldLogger,
) (fsm.PhaseExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.Package == nil {
		return nil, trace.BadParameter("no application package specified for phase %v", p.Phase.ID)
	}
	if p.Phase.Data.Update == nil || len(p.Phase.Data.Update.Servers) == 0 {
		return nil, trace.BadParameter("no server specified for phase %q", p.Phase.ID)
	}
	if p.Phase.Data.Update.GravityPackage == nil {
		return nil, trace.BadParameter("no gravity package specified for phase %q", p.Phase.ID)
	}
	server := p.Phase.Data.Update.Servers[0]
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := operator.GetSiteOperation(fsm.OperationKey(p.Plan))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query application")
	}
	serviceUser, err := systeminfo.UserFromOSUser(cluster.ServiceUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	executor := updatePhaseBootstrapLeader{
		FieldLogger: logger,
		servers:     p.Phase.Data.Update.Servers,
		bootstrap: updatePhaseBootstrap{
			FieldLogger:      logger,
			Operator:         operator,
			Backend:          backend,
			LocalBackend:     localBackend,
			HostLocalBackend: hostLocalBackend,
			LocalPackages:    localPackages,
			Packages:         packages,
			Server:           server,
			Operation:        *operation,
			GravityPackage:   *p.Phase.Data.Update.GravityPackage,
			ServiceUser:      *serviceUser,
			ExecutorParams:   p,
			remote:           remote,
			clusterDNSConfig: cluster.DNSConfig,
		},
		masterIPs:      storage.Servers(p.Plan.Servers).MasterIPs(),
		packageRotator: operator,
		updateManifest: app.Manifest,
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
	executor.existingClusterConfig = configBytes
	executor.existingEnviron = env.GetKeyValues()
	return &executor, nil
}

// Execute executes the bootstrap phase locally, e.g. exports new gravity
// binary, creates new secrets/config packages in the local backend and
// initializes local operation state
func (p *updatePhaseBootstrapLeader) Execute(ctx context.Context) error {
	err := p.rotateConfigAndSecrets()
	if err != nil {
		return trace.Wrap(err)
	}
	return p.bootstrap.Execute(ctx)
}

// Rollback is no-op for this phase
func (p *updatePhaseBootstrapLeader) Rollback(context.Context) error {
	return nil
}

// PreCheck makes sure that bootstrap phase is executed on the correct node
func (p *updatePhaseBootstrapLeader) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.bootstrap.PreCheck(ctx))
}

// PostCheck is no-op for bootstrap phase
func (p *updatePhaseBootstrapLeader) PostCheck(context.Context) error {
	return nil
}

func (p *updatePhaseBootstrapLeader) rotateConfigAndSecrets() error {
	for _, server := range p.servers {
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

func (p *updatePhaseBootstrapLeader) rotateSecrets(server storage.UpdateServer) error {
	p.Infof("Generate new secrets configuration package for %v.", server)
	resp, err := p.packageRotator.RotateSecrets(ops.RotateSecretsRequest{
		Key:            p.bootstrap.Operation.ClusterKey(),
		Package:        server.Runtime.SecretsPackage,
		RuntimePackage: server.Runtime.Update.Package,
		Server:         server.Server,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.bootstrap.Packages.UpsertPackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debugf("Rotated secrets package for %v: %v.", server, resp.Locator)
	return nil
}

func (p *updatePhaseBootstrapLeader) rotatePlanetConfig(server storage.UpdateServer) error {
	p.Infof("Generate new runtime configuration package for %v.", server)
	resp, err := p.packageRotator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
		Key:            p.bootstrap.Operation.Key(),
		Server:         server.Server,
		Manifest:       p.updateManifest,
		RuntimePackage: server.Runtime.Update.Package,
		Package:        &server.Runtime.Update.ConfigPackage,
		Config:         p.existingClusterConfig,
		Env:            p.existingEnviron,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.bootstrap.Packages.UpsertPackage(resp.Locator, resp.Reader,
		pack.WithLabels(resp.Labels))
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Generated new runtime configuration package for %v: %v.", server, resp.Locator)
	return nil
}

func (p *updatePhaseBootstrapLeader) rotateTeleportConfig(server storage.UpdateServer) error {
	masterConf, nodeConf, err := p.packageRotator.RotateTeleportConfig(ops.RotateTeleportConfigRequest{
		Key:             p.bootstrap.Operation.Key(),
		Server:          server.Server,
		TeleportPackage: server.Teleport.Update.Package,
		NodePackage:     server.Teleport.Update.NodeConfigPackage,
		MasterIPs:       p.masterIPs,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if masterConf != nil {
		_, err = p.bootstrap.Packages.UpsertPackage(masterConf.Locator, masterConf.Reader, pack.WithLabels(masterConf.Labels))
		if err != nil {
			return trace.Wrap(err)
		}
		p.Debugf("Rotated teleport master config package for %v: %v.", server, masterConf.Locator)
	}
	if nodeConf != nil {
		_, err = p.bootstrap.Packages.UpsertPackage(nodeConf.Locator, nodeConf.Reader, pack.WithLabels(nodeConf.Labels))
		if err != nil {
			return trace.Wrap(err)
		}
		p.Debugf("Rotated teleport node config package for %v: %v.", server, nodeConf.Locator)
	}
	return nil
}

type updatePhaseBootstrapLeader struct {
	// FieldLogger is used for logging
	log.FieldLogger
	bootstrap updatePhaseBootstrap
	// servers lists cluster server configuration updates
	servers []storage.UpdateServer
	// masterIPs lists addresses of all master nodes in the cluster
	masterIPs []string
	// packageRotator specifies the configuration package rotator
	packageRotator        PackageRotator
	existingEnviron       map[string]string
	existingClusterConfig []byte
	// updateManifest specifies the manifest of the update application
	updateManifest schema.Manifest
}

// updatePhaseBootstrap is the executor for the update bootstrap phase.
//
// Bootstrapping entails a few activities executed on each server:
//  - exporting a copy of the new gravity binary into the auxiliary location
//	which is then used for update-related tasks
//  - ensuring that all system directories exist and have proper permissions
//  - pulling system updates
//  - synchronizing the remote operation plan with the local backend
type updatePhaseBootstrap struct {
	// FieldLogger is used for logging
	log.FieldLogger
	// Packages is the cluster package service
	Packages pack.PackageService
	// LocalPackages is the local package service
	LocalPackages pack.PackageService
	// Operation is the operation being initialized
	Operation ops.SiteOperation
	// Operator is the cluster operator interface
	Operator ops.Operator
	// Backend is the cluster backend
	Backend storage.Backend
	// LocalBackend is the local state backend
	LocalBackend storage.Backend
	// HostLocalBackend is the host-local state backend used to persist global settings
	// like DNS configuration, logins etc.
	HostLocalBackend storage.Backend
	// GravityPackage specifies the new gravity package
	GravityPackage loc.Locator
	// Server specifies the bootstrap target
	Server storage.UpdateServer
	// ServiceUser is the user used for services and system storage
	ServiceUser systeminfo.User
	// ExecutorParams stores the phase parameters
	fsm.ExecutorParams
	remote           fsm.Remote
	clusterDNSConfig storage.DNSConfig
}

// NewUpdatePhaseBootstrap creates a new bootstrap phase executor
func NewUpdatePhaseBootstrap(
	p fsm.ExecutorParams,
	operator ops.Operator,
	apps libapp.Applications,
	backend, localBackend, hostLocalBackend storage.Backend,
	localPackages, packages pack.PackageService,
	remote fsm.Remote,
	logger log.FieldLogger,
) (fsm.PhaseExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.Package == nil {
		return nil, trace.BadParameter("no application package specified for phase %v", p.Phase.ID)
	}
	if p.Phase.Data.Update == nil || len(p.Phase.Data.Update.Servers) == 0 {
		return nil, trace.BadParameter("no server specified for phase %q", p.Phase.ID)
	}
	if p.Phase.Data.Update.GravityPackage == nil {
		return nil, trace.BadParameter("no gravity package specified for phase %q", p.Phase.ID)
	}
	server := p.Phase.Data.Update.Servers[0]
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serviceUser, err := systeminfo.UserFromOSUser(cluster.ServiceUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := operator.GetSiteOperation(fsm.OperationKey(p.Plan))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &updatePhaseBootstrap{
		Operator:         operator,
		Backend:          backend,
		LocalBackend:     localBackend,
		HostLocalBackend: hostLocalBackend,
		LocalPackages:    localPackages,
		Packages:         packages,
		Server:           server,
		Operation:        *operation,
		GravityPackage:   *p.Phase.Data.Update.GravityPackage,
		ServiceUser:      *serviceUser,
		FieldLogger:      logger,
		ExecutorParams:   p,
		remote:           remote,
		clusterDNSConfig: cluster.DNSConfig,
	}, nil
}

// PreCheck makes sure that bootstrap phase is executed on the correct node
func (p *updatePhaseBootstrap) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.remote.CheckServer(ctx, p.Server.Server))
}

// PostCheck is no-op for bootstrap phase
func (p *updatePhaseBootstrap) PostCheck(context.Context) error {
	return nil
}

// Execute executes the bootstrap phase locally, e.g. exports new gravity
// binary, creates new secrets/config packages in the local backend and
// initializes local operation state
func (p *updatePhaseBootstrap) Execute(ctx context.Context) error {
	err := p.configureNode()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.updateSystemMetadata()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.syncPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.pullSystemUpdates(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.addUpdateRuntimePackageLabel()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is no-op for this phase
func (p *updatePhaseBootstrap) Rollback(context.Context) error {
	return nil
}

func (p *updatePhaseBootstrap) configureNode() error {
	err := p.Operator.ConfigureNode(ops.ConfigureNodeRequest{
		AccountID:   p.Operation.AccountID,
		ClusterName: p.Operation.SiteDomain,
		OperationID: p.Operation.ID,
		Server:      p.Server.Server,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("Node %v (%v) configured.", p.Server.Hostname, p.Server.AdvertiseIP)
	return nil
}

func (p *updatePhaseBootstrap) updateSystemMetadata() error {
	if err := p.updateDNSConfig(); err != nil {
		return trace.Wrap(err)
	}
	if err := p.updateNodeAddr(); err != nil {
		return trace.Wrap(err)
	}
	return p.updateServiceUser()
}

// updateDNSConfig persists the DNS configuration in the local backend if it has not been set
func (p *updatePhaseBootstrap) updateDNSConfig() error {
	p.Infof("Update cluster DNS configuration as %v.", p.Plan.DNSConfig)
	err := p.HostLocalBackend.SetDNSConfig(p.Plan.DNSConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// updateNodeAddr persists the node advertise IP in the local state database
func (p *updatePhaseBootstrap) updateNodeAddr() error {
	p.Infof("Update node address as %v.", p.Server.AdvertiseIP)
	return p.HostLocalBackend.SetNodeAddr(p.Server.AdvertiseIP)
}

// updateServiceUser persists the service user in the local state database
func (p *updatePhaseBootstrap) updateServiceUser() error {
	p.Infof("Update service user as %v.", p.ServiceUser)
	return p.HostLocalBackend.SetServiceUser(p.ServiceUser.OSUser())
}

func (p *updatePhaseBootstrap) pullSystemUpdates(ctx context.Context) error {
	p.Info("Pull system updates.")
	updates := []loc.Locator{p.GravityPackage}
	if p.Server.Runtime.SecretsPackage != nil {
		updates = append(updates, *p.Server.Runtime.SecretsPackage)
	}
	if p.Server.Runtime.Update != nil {
		updates = append(updates,
			p.Server.Runtime.Update.Package,
			p.Server.Runtime.Update.ConfigPackage,
		)
	}
	if p.Server.Teleport.Update != nil {
		updates = append(updates, p.Server.Teleport.Update.Package)
		if p.Server.Teleport.Update.NodeConfigPackage != nil {
			updates = append(updates, *p.Server.Teleport.Update.NodeConfigPackage)
		}
	}
	for _, update := range updates {
		p.Infof("Pulling package update: %v.", update)
		existingLabels, err := queryPackageLabels(update, p.LocalPackages)
		if err != nil {
			return trace.Wrap(err)
		}
		puller := libapp.Puller{
			SrcPack: p.Packages,
			DstPack: p.LocalPackages,
			Labels:  existingLabels,
		}
		err = puller.PullPackage(ctx, update)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		if !update.IsEqualTo(p.GravityPackage) {
			p.Infof("Unpacking package %v.", update)
			err = pack.Unpack(p.LocalPackages, update, "", nil)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	// after having pulled packages as root we need to set proper ownership
	// on the blobs dir
	// FIXME(dmitri): PullPackage API needs to accept uid/gid so this is unnecessary
	// See https://github.com/gravitational/gravity.e/issues/4209
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.Chown(filepath.Join(stateDir, defaults.LocalDir), p.ServiceUser.UID, p.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *updatePhaseBootstrap) syncPlan() error {
	p.Info("Synchronize operation plan from cluster.")
	site, err := p.Backend.GetSite(p.Operation.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := p.Backend.GetOperationPlan(p.Operation.SiteDomain, p.Operation.ID)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.LocalBackend.CreateSite(*site)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	_, err = p.LocalBackend.CreateSiteOperation(storage.SiteOperation(p.Operation))
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	_, err = p.LocalBackend.CreateOperationPlan(*plan)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	return nil
}

// addUpdateRuntimePackageLabel adds the runtime label on the runtime package from the update
// in case the installer been generated on the Ops Center that does not replicate remote
// package labels.
// See: https://github.com/gravitational/gravity.e/issues/3768
func (p *updatePhaseBootstrap) addUpdateRuntimePackageLabel() error {
	if p.Server.Runtime.Update == nil {
		return nil
	}
	err := p.LocalPackages.UpdatePackageLabels(p.Server.Runtime.Update.Package, pack.RuntimePackageLabels, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getGravityPath returns path to the new gravity binary
func getGravityPath() (string, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(
		state.GravityUpdateDir(stateDir),
		constants.GravityBin), nil
}

func queryPackageLabels(loc loc.Locator, packages pack.PackageService) (labels pack.Labels, err error) {
	env, err := packages.ReadPackageEnvelope(loc)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if env != nil {
		labels = env.RuntimeLabels
	}
	return labels, nil
}
