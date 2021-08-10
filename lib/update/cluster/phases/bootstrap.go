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
	"io"
	"path/filepath"

	appservice "github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// updatePhaseBootstrap is the executor for the update bootstrap phase.
//
// Bootstrapping entails a few activities executed on each server:
//  - exporting a copy of the new gravity binary into the auxiliary location
//	which is then used for update-related tasks
//  - ensuring that all system directories exist and have proper permissions
//  - pulling system updates
//  - synchronizing the remote operation plan with the local backend
type updatePhaseBootstrap struct {
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
	// GravityPath is the path to the new gravity binary
	GravityPath string
	// GravityPackage specifies the new gravity package
	GravityPackage loc.Locator
	// Server specifies the bootstrap target
	Server storage.UpdateServer
	// ServiceUser is the user used for services and system storage
	ServiceUser systeminfo.User
	// FieldLogger is used for logging
	log.FieldLogger
	// ExecutorParams stores the phase parameters
	fsm.ExecutorParams
	remote fsm.Remote
}

// NewUpdatePhaseBootstrap creates a new bootstrap phase executor
func NewUpdatePhaseBootstrap(
	p fsm.ExecutorParams,
	operator ops.Operator,
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
	cluster, err := operator.GetLocalSite(context.TODO())
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
	gravityPath, err := getGravityPath()
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
		Server:           p.Phase.Data.Update.Servers[0],
		Operation:        *operation,
		GravityPath:      gravityPath,
		GravityPackage:   p.Plan.GravityPackage,
		ServiceUser:      *serviceUser,
		FieldLogger:      logger,
		ExecutorParams:   p,
		remote:           remote,
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
	exportCtx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	err = p.exportGravity(exportCtx)
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
	err = p.updateExistingPackageLabels()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.pullSystemUpdates()
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

func (p *updatePhaseBootstrap) exportGravity(ctx context.Context) error {
	p.Infof("Export gravity binary to %v.", p.GravityPath)
	err := utils.CopyWithRetries(ctx, p.GravityPath,
		func() (io.ReadCloser, error) {
			_, rc, err := p.Packages.ReadPackage(p.Plan.GravityPackage)
			return rc, trace.Wrap(err)
		},
		utils.PermOption(defaults.SharedExecutableMask),
		utils.OwnerOption(p.ServiceUser.UID, p.ServiceUser.GID),
	)
	return trace.Wrap(err)
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

func (p *updatePhaseBootstrap) pullSystemUpdates() error {
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
		_, err = appservice.PullPackage(appservice.PackagePullRequest{
			SrcPack: p.Packages,
			DstPack: p.LocalPackages,
			Package: update,
			Upsert:  true,
			Labels:  existingLabels,
		})
		if err != nil {
			return trace.Wrap(err)
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

// updateExistingPackageLabels updates labels on existing packages
// so the system package pull step can find and pull correct package updates.
//
// For legacy runtime packages ('planet-master' and 'planet-node')
// the sibling runtime package (i.e. 'planet-master' on a regular node
// and vice versa), will be updated to _not_ include the installed label
// to simplify the search
func (p *updatePhaseBootstrap) updateExistingPackageLabels() error {
	installedRuntime := p.Server.Runtime.Installed
	runtimeConfigLabels, err := updateRuntimeConfigLabels(p.LocalPackages, installedRuntime)
	if err != nil {
		return trace.Wrap(err)
	}
	teleportConfigLabels, err := updateTeleportConfigLabels(p.LocalPackages, p.Plan.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	secretLabels, err := updateRuntimeSecretLabels(p.LocalPackages)
	if err != nil {
		return trace.Wrap(err)
	}
	updates := append(runtimeConfigLabels, secretLabels...)
	updates = append(updates, teleportConfigLabels...)
	updates = append(updates, pack.LabelUpdate{
		Locator: installedRuntime,
		Add:     utils.CombineLabels(pack.RuntimePackageLabels, pack.InstalledLabels),
	})
	if loc.IsLegacyRuntimePackage(installedRuntime) {
		var runtimePackageToClear loc.Locator
		switch installedRuntime.Name {
		case loc.LegacyPlanetMaster.Name:
			runtimePackageToClear = withVersion(loc.LegacyPlanetNode, installedRuntime.Version)
		case loc.LegacyPlanetNode.Name:
			runtimePackageToClear = withVersion(loc.LegacyPlanetMaster, installedRuntime.Version)
		}
		updates = append(updates, pack.LabelUpdate{
			Locator: runtimePackageToClear,
			Add:     pack.RuntimePackageLabels,
			Remove:  []string{pack.InstalledLabel},
		})
	}
	for _, update := range updates {
		p.Info(update.String())
		err := p.LocalPackages.UpdatePackageLabels(update.Locator, update.Add, update.Remove)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
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

func withVersion(filter loc.Locator, version string) loc.Locator {
	return loc.Locator{
		Repository: filter.Repository,
		Name:       filter.Name,
		Version:    version,
	}
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
			return e.Locator.Name == constants.TeleportNodeConfigPackage &&
				(e.HasLabels(pack.TeleportNodeConfigPackageLabels) ||
					e.HasLabels(pack.TeleportLegacyNodeConfigPackageLabels))
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
