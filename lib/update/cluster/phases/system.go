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
	"time"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/system"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// updatePhaseSystem is the executor for the update master/node update phase
type updatePhaseSystem struct {
	// ChangesetID specifies the ID of the system update step
	ChangesetID string
	// Server is the server currently being updated
	Server storage.UpdateServer
	// Backend specifies the backend used for the update operation
	Backend storage.Backend
	// Packages specifies the cluster package service
	Packages pack.PackageService
	// HostLocalPackages specifies the package service on local host
	HostLocalPackages update.LocalPackageService
	// GravityPackage specifies the new gravity package
	GravityPackage loc.Locator
	// FieldLogger is used for logging
	log.FieldLogger
	remote fsm.Remote
	// seLinux indicates whether the SELinux support is on on the node
	seLinux bool
}

// NewUpdatePhaseNode returns a new node update phase executor
func NewUpdatePhaseSystem(
	p fsm.ExecutorParams,
	remote fsm.Remote,
	backend storage.Backend,
	packages pack.PackageService,
	localPackages update.LocalPackageService,
	logger log.FieldLogger,
) (*updatePhaseSystem, error) {
	if p.Phase.Data.Update == nil || len(p.Phase.Data.Update.Servers) == 0 {
		return nil, trace.NotFound("no server specified for phase %q", p.Phase.ID)
	}
	if p.Phase.Data.Update.ChangesetID == "" {
		return nil, trace.BadParameter("no changeset ID specified for phase %q", p.Phase.ID)
	}
	if p.Phase.Data.Update.GravityPackage == nil {
		return nil, trace.BadParameter("no gravity package specified for phase %q", p.Phase.ID)
	}
	return &updatePhaseSystem{
		ChangesetID:       p.Phase.Data.Update.ChangesetID,
		Server:            p.Phase.Data.Update.Servers[0],
		GravityPackage:    *p.Phase.Data.Update.GravityPackage,
		Backend:           backend,
		Packages:          packages,
		HostLocalPackages: localPackages,
		FieldLogger:       logger,
		remote:            remote,
		seLinux:           p.Phase.Data.Update.Servers[0].SELinux,
	}, nil
}

// PreCheck makes sure the phase is being executed on the correct server
func (p *updatePhaseSystem) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.remote.CheckServer(ctx, p.Server.Server))
}

// PostCheck is no-op for this phase
func (p *updatePhaseSystem) PostCheck(context.Context) error {
	return nil
}

// Execute runs system update on the node
func (p *updatePhaseSystem) Execute(ctx context.Context) error {
	runtimeConfig, err := p.getInstalledConfigPackage(p.Server.Runtime.Installed)
	if err != nil {
		return trace.Wrap(err, "failed to locate runtime configuration package")
	}
	teleportConfig, err := p.getInstalledConfigPackage(p.Server.Teleport.Installed)
	if err != nil {
		return trace.Wrap(err, "failed to locate teleport configuration package")
	}
	config := system.Config{
		ChangesetID: p.ChangesetID,
		Backend:     p.Backend,
		Packages:    p.HostLocalPackages,
		ClusterRole: p.Server.ClusterRole,
		PackageUpdates: system.PackageUpdates{
			Gravity: &storage.PackageUpdate{
				To: p.GravityPackage,
			},
			Runtime: storage.PackageUpdate{
				From: p.Server.Runtime.Installed,
				// Overwritten below if an update exists
				To: p.Server.Runtime.Installed,
			},
		},
		SELinux: p.seLinux,
	}
	if p.Server.Runtime.Update != nil {
		config.Runtime.To = p.Server.Runtime.Update.Package
		config.Runtime.ConfigPackage = &storage.PackageUpdate{
			From: *runtimeConfig,
			To:   p.Server.Runtime.Update.ConfigPackage,
		}
	}
	if p.Server.Teleport.Update != nil {
		// Consider teleport update only in effect when the update package
		// has been specified. This is in contrast to runtime update, when
		// we expect to update the configuration more often
		configPackage := p.Server.Teleport.Update.NodeConfigPackage
		if configPackage == nil {
			// No update necessary
			configPackage = teleportConfig
		}
		config.Teleport = &storage.PackageUpdate{
			From: p.Server.Teleport.Installed,
			To:   p.Server.Teleport.Update.Package,
			ConfigPackage: &storage.PackageUpdate{
				From: *teleportConfig,
				To:   *configPackage,
			},
		}
	}
	if p.Server.Runtime.SecretsPackage != nil {
		config.RuntimeSecrets = &storage.PackageUpdate{
			To: *p.Server.Runtime.SecretsPackage,
		}
	}
	updater, err := system.New(config)
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.Update(ctx, true)
	if err != nil {
		return trace.Wrap(err)
	}
	// Regardless of whether the teleport itself is being updated or not,
	// update the tctl script to make sure it is present when upgrading
	// from older versions and also to account for any possible changes
	// made to the script itself.
	if p.Server.IsMaster() {
		err = updater.UpdateTctlScript(p.Server.Teleport.Package())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (p *updatePhaseSystem) getInstalledConfigPackage(loc loc.Locator) (*loc.Locator, error) {
	configPackage, err := pack.FindInstalledConfigPackage(p.HostLocalPackages, loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return configPackage, nil
}

// Rollback runs rolls back the system upgrade on the node
func (p *updatePhaseSystem) Rollback(ctx context.Context) error {
	updater, err := system.New(system.Config{
		ChangesetID: p.ChangesetID,
		Backend:     p.Backend,
		Packages:    p.HostLocalPackages,
		SELinux:     p.seLinux,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return updater.Rollback(ctx, true)
}

type updatePhaseConfig struct {
	// Packages is the cluster package service
	Packages pack.PackageService
	// LocalPackages is the local package service
	LocalPackages pack.PackageService
	// ServiceUser is the user used for services and system storage
	ServiceUser systeminfo.User
	// ExecutorParams is common phase executor parameters
	fsm.ExecutorParams
	// FieldLogger specifies the logger
	log.FieldLogger
	// remote is the remote executor
	remote fsm.Remote
}

// NewUpdatePhaseConfig returns executor that pulls local configuration packages
func NewUpdatePhaseConfig(
	p fsm.ExecutorParams,
	operator ops.Operator,
	packages pack.PackageService,
	localPackages pack.PackageService,
	remote fsm.Remote,
	logger log.FieldLogger,
) (*updatePhaseConfig, error) {
	cluster, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serviceUser, err := systeminfo.UserFromOSUser(cluster.ServiceUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &updatePhaseConfig{
		Packages:       packages,
		LocalPackages:  localPackages,
		ServiceUser:    *serviceUser,
		ExecutorParams: p,
		FieldLogger:    logger,
		remote:         remote,
	}, nil
}

// Execute pulls rotated teleport master config package to the local package store
func (p *updatePhaseConfig) Execute(ctx context.Context) error {
	b := utils.NewExponentialBackOff(5 * time.Minute)
	err := utils.RetryTransient(ctx, b, func() error {
		return p.pullUpdates(ctx)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// after having pulled as root, update ownership on the blobs dir
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.Chown(filepath.Join(stateDir, defaults.LocalDir),
		p.ServiceUser.UID, p.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback removes teleport master config packages pulled during this
// operation from the local package store
func (p *updatePhaseConfig) Rollback(context.Context) error {
	labels := map[string]string{
		pack.AdvertiseIPLabel: p.Phase.Data.Server.AdvertiseIP,
		pack.OperationIDLabel: p.Plan.OperationID,
		pack.PurposeLabel:     pack.PurposeTeleportMasterConfig,
	}
	return pack.ForeachPackage(p.LocalPackages, func(e pack.PackageEnvelope) error {
		if e.HasLabels(labels) {
			p.Infof("Removing package %v.", e.Locator)
			return p.LocalPackages.DeletePackage(e.Locator)
		}
		return nil
	})
}

// PreCheck makes sure the phase is being executed on the correct server
func (p *updatePhaseConfig) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.remote.CheckServer(ctx, *p.Phase.Data.Server))
}

// PostCheck is no-op for this phase
func (p *updatePhaseConfig) PostCheck(context.Context) error {
	return nil
}

func (p *updatePhaseConfig) pullUpdates(ctx context.Context) error {
	update, err := pack.FindLatestPackageWithLabels(
		p.Packages, p.Plan.ClusterName, map[string]string{
			pack.AdvertiseIPLabel: p.Phase.Data.Server.AdvertiseIP,
			pack.OperationIDLabel: p.Plan.OperationID,
			pack.PurposeLabel:     pack.PurposeTeleportMasterConfig,
		})
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		p.Info("No teleport master config update found.")
		return nil
	}
	p.Infof("Pulling teleport master config update: %v.", update)
	puller := libapp.Puller{
		SrcPack: p.Packages,
		DstPack: p.LocalPackages,
		Upsert:  true,
	}
	err = puller.PullPackage(ctx, *update)
	return trace.Wrap(err)
}
