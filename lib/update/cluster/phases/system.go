/*
Copyright 2018-2019 Gravitational, Inc.

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
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/system"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// planetStart is executor that starts specified Planet service.
type planetStart struct {
	// FieldLogger is used for logging.
	log.FieldLogger
	// Node is the node where Planet service should be started.
	Node storage.UpdateServer
	// Package is the Planet package to start the service for.
	Package loc.Locator
	// Remote allows to invoke remote commands.
	Remote fsm.Remote
}

// NewPlanetStart returns executor that starts specified Planet service.
func NewPlanetStart(p fsm.ExecutorParams, remote fsm.Remote, log log.FieldLogger) (*planetStart, error) {
	node := p.Phase.Data.Update.Servers[0]
	return &planetStart{
		FieldLogger: log,
		Node:        node,
		Package:     node.Runtime.Update.Package,
		Remote:      remote,
	}, nil
}

// Execute starts specified Planet service.
func (p *planetStart) Execute(ctx context.Context) error {
	p.Infof("Starting systemd service for %v.", p.Package)
	serviceManager, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	err = serviceManager.StartPackageService(p.Package, true)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Started systemd service for %v.", p.Package)
	err = status.Wait(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Planet is running.")
	return nil
}

// Rollback stops specified Planet service.
func (p *planetStart) Rollback(ctx context.Context) error {
	p.Infof("Stopping systemd service for %v.", p.Package)
	serviceManager, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	err = serviceManager.StopPackageService(p.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Stopped systemd service for %v.", p.Package)
	return nil
}

// PreCheck makes sure the phase runs on the correct node.
func (p *planetStart) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.Remote.CheckServer(ctx, p.Node.Server))
}

// PostCheck is no-op.
func (*planetStart) PostCheck(context.Context) error { return nil }

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
	return &updatePhaseSystem{
		ChangesetID:       p.Phase.Data.Update.ChangesetID,
		Server:            p.Phase.Data.Update.Servers[0],
		GravityPackage:    p.Plan.GravityPackage,
		Backend:           backend,
		Packages:          packages,
		HostLocalPackages: localPackages,
		FieldLogger:       logger,
		remote:            remote,
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
	runtimeConfig, err := p.getInstalledRuntimeConfig()
	if err != nil {
		return trace.Wrap(err, "failed to locate runtime configuration package")
	}
	config := system.Config{
		ChangesetID: p.ChangesetID,
		Backend:     p.Backend,
		Packages:    p.HostLocalPackages,
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
	}
	if p.Server.Runtime.Update != nil {
		config.Runtime.To = p.Server.Runtime.Update.Package
		config.Runtime.ConfigPackage = &storage.PackageUpdate{
			From: *runtimeConfig,
			To:   p.Server.Runtime.Update.ConfigPackage,
		}
		config.Runtime.NoStart = p.Server.ShouldMigrateDockerDevice()
	}
	if p.Server.Teleport.Update != nil {
		// Consider teleport update only in effect when the update package
		// has been specified. This is in contrast to runtime update, when
		// we expect to update the configuration more often
		config.Teleport = &storage.PackageUpdate{
			From: p.Server.Teleport.Installed,
			To:   p.Server.Teleport.Update.Package,
			ConfigPackage: &storage.PackageUpdate{
				To: p.Server.Teleport.Update.NodeConfigPackage,
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
	// FIXME: move NoStart to system configuration as a planet-specific attribute
	err = updater.Update(ctx, !config.Runtime.NoStart)
	return trace.Wrap(err)
}

func (p *updatePhaseSystem) getInstalledRuntimeConfig() (*loc.Locator, error) {
	runtimeConfig, err := pack.FindInstalledConfigPackage(p.HostLocalPackages, p.Server.Runtime.Installed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return runtimeConfig, nil
}

// Rollback runs rolls back the system upgrade on the node
func (p *updatePhaseSystem) Rollback(ctx context.Context) error {
	updater, err := system.New(system.Config{
		ChangesetID: p.ChangesetID,
		Backend:     p.Backend,
		Packages:    p.HostLocalPackages,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.Rollback(ctx, true)
	return trace.Wrap(err)
}

type updatePhaseConfig struct {
	// Packages is the cluster package service
	Packages pack.PackageService
	// LocalPackages is the local package service
	LocalPackages pack.PackageService
	// ServiceUser is the user used for services and system storage
	ServiceUser storage.OSUser
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
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &updatePhaseConfig{
		Packages:       packages,
		LocalPackages:  localPackages,
		ServiceUser:    cluster.ServiceUser,
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
