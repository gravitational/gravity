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
	"fmt"
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// updatePhaseSystem is the executor for the update master/node update phase
type updatePhaseSystem struct {
	// OperationID is the id of the current update operation
	OperationID string
	// Server is the server currently being updated
	Server storage.Server
	// GravityPath is the path to the new gravity binary
	GravityPath string
	// FieldLogger is used for logging
	log.FieldLogger
	remote fsm.Remote
	// runtimePackage specifies the runtime package to update to
	runtimePackage loc.Locator
}

// NewUpdatePhaseNode returns a new node update phase executor
func NewUpdatePhaseSystem(p fsm.ExecutorParams, remote fsm.Remote, logger log.FieldLogger) (*updatePhaseSystem, error) {
	if p.Phase.Data == nil || p.Phase.Data.Server == nil {
		return nil, trace.NotFound("no server specified for phase %q", p.Phase.ID)
	}
	if p.Phase.Data.RuntimePackage == nil {
		return nil, trace.NotFound("no runtime package specified for phase %q", p.Phase.ID)
	}
	gravityPath, err := getGravityPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &updatePhaseSystem{
		OperationID:    p.Plan.OperationID,
		Server:         *p.Phase.Data.Server,
		GravityPath:    gravityPath,
		FieldLogger:    logger,
		remote:         remote,
		runtimePackage: *p.Phase.Data.RuntimePackage,
	}, nil
}

// PreCheck makes sure the phase is being executed on the correct server
func (p *updatePhaseSystem) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.remote.CheckServer(ctx, p.Server))
}

// PostCheck is no-op for this phase
func (p *updatePhaseSystem) PostCheck(context.Context) error {
	return nil
}

// Execute runs system update on the node
func (p *updatePhaseSystem) Execute(context.Context) error {
	out, err := fsm.RunCommand([]string{p.GravityPath,
		"--insecure", "--debug", "system", "update",
		"--changeset-id", p.OperationID,
		"--runtime-package", p.runtimePackage.String(),
		"--with-status",
	})
	if err != nil {
		message := "failed to update system"
		if errUninstall, ok := trace.Unwrap(err).(*utils.ErrorUninstallService); ok {
			message = fmt.Sprintf("The %q service failed to stop."+
				"Restart this node to clean up and retry.",
				errUninstall.Package)
		}
		p.Warnf("Failed to update system: %s (%v).", out, err)
		return trace.Wrap(err, message)
	}
	p.Infof("System updated: %s.", out)
	return nil
}

// Rollback runs rolls back the system upgrade on the node
func (p *updatePhaseSystem) Rollback(context.Context) error {
	out, err := fsm.RunCommand([]string{p.GravityPath, "--insecure", "system", "rollback",
		"--changeset-id", p.OperationID, "--with-status"})
	if err != nil {
		p.Warnf("Failed to rollback system: %s (%v).", out, err)
		return trace.Wrap(err, "failed to rollback system: %s", out)
	}
	p.Infof("System rolled back: %s.", out)
	return nil
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
	err := utils.RetryTransient(ctx, b, p.pullUpdates)
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

func (p *updatePhaseConfig) pullUpdates() error {
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
	_, err = service.PullPackage(service.PackagePullRequest{
		SrcPack: p.Packages,
		DstPack: p.LocalPackages,
		Package: *update,
		Upsert:  true,
	})
	return trace.Wrap(err)
}
