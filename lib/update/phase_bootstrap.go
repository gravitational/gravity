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

package update

import (
	"context"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
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
	// Operation is the operation being initialized
	Operation ops.SiteOperation
	// Operator is the cluster operator interface
	Operator ops.Operator
	// Backend is the cluster backend
	Backend storage.Backend
	// LocalBackend is the local state backend
	LocalBackend storage.Backend
	// GravityPath is the path to the new gravity binary
	GravityPath string
	// GravityPackage specifies the package with the gravity binary
	GravityPackage loc.Locator
	// Server specifies the bootstrap target
	Server storage.Server
	// Servers is the list of local cluster servers
	Servers []storage.Server
	// ServiceUser is the user used for services and system storage
	ServiceUser storage.OSUser
	// FieldLogger is used for logging
	log.FieldLogger
	remote fsm.Remote
	// runtimePackage specifies the runtime package to update to
	runtimePackage loc.Locator
}

// NewUpdatePhaseBootstrap creates a new bootstrap phase executor
func NewUpdatePhaseBootstrap(c FSMConfig, plan storage.OperationPlan, phase storage.OperationPhase, remote fsm.Remote) (fsm.PhaseExecutor, error) {
	if phase.Data == nil || phase.Data.Package == nil {
		return nil, trace.NotFound("no application package specified for phase %v", phase)
	}
	if phase.Data.Server == nil {
		return nil, trace.NotFound("no server specified for phase %q", phase.ID)
	}
	cluster, err := c.Operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := ops.GetLastUpdateOperation(cluster.Key(), c.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gravityPath, err := getGravityPath()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := c.Apps.GetApp(*phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query application")
	}
	runtimePackage, err := app.Manifest.RuntimePackageForProfile(phase.Data.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &updatePhaseBootstrap{
		Operator:       c.Operator,
		Backend:        c.Backend,
		LocalBackend:   c.LocalBackend,
		Packages:       c.ClusterPackages,
		GravityPackage: plan.GravityPackage,
		Server:         *phase.Data.Server,
		Servers:        plan.Servers,
		Operation:      *operation,
		GravityPath:    gravityPath,
		ServiceUser:    cluster.ServiceUser,
		FieldLogger:    log.NewEntry(log.New()),
		remote:         remote,
		runtimePackage: *runtimePackage,
	}, nil
}

// PreCheck makes sure that bootstrap phase is executed on the correct node
func (p *updatePhaseBootstrap) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.remote.CheckServer(ctx, p.Server))
}

// PostCheck is no-op for bootstrap phase
func (p *updatePhaseBootstrap) PostCheck(context.Context) error {
	return nil
}

// Execute executes the bootstrap phase locally, e.g. exports new gravity
// binary, creates new secrets/config packages in the local backend and
// initializes local operation state
func (p *updatePhaseBootstrap) Execute(context.Context) error {
	err := p.configureNode()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.exportGravity()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.syncPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.pullSystemUpdates()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is no-op for bootstrap phase
func (p *updatePhaseBootstrap) Rollback(context.Context) error {
	return nil
}

func (p *updatePhaseBootstrap) configureNode() error {
	err := p.Operator.ConfigureNode(ops.ConfigureNodeRequest{
		AccountID:   p.Operation.AccountID,
		ClusterName: p.Operation.SiteDomain,
		OperationID: p.Operation.ID,
		Server:      p.Server,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("Node %v (%v) configured.", p.Server.Hostname, p.Server.AdvertiseIP)
	return nil
}

func (p *updatePhaseBootstrap) exportGravity() error {
	_, reader, err := p.Packages.ReadPackage(p.GravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	err = os.RemoveAll(p.GravityPath)
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.CopyReaderWithPerms(p.GravityPath, reader, defaults.SharedExecutableMask)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("%v exported to %v.", p.GravityPackage, p.GravityPath)
	return nil
}

func (p *updatePhaseBootstrap) pullSystemUpdates() error {
	out, err := fsm.RunCommand(utils.PlanetCommandArgs(
		filepath.Join(defaults.GravityUpdateDir, constants.GravityBin),
		"--quiet", "--insecure", "system", "pull-updates",
		"--uid", p.ServiceUser.UID,
		"--gid", p.ServiceUser.GID,
		"--runtime-package", p.runtimePackage.String(),
		"--ops-url", defaults.GravityServiceURL))
	if err != nil {
		return trace.Wrap(err, "failed to pull system updates: %s", out)
	}
	log.Debugf("Pulled system updates: %s.", out)
	return nil
}

func (p *updatePhaseBootstrap) syncPlan() error {
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

// getGravityPath returns path to the new gravity binary
func getGravityPath() (string, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(
		stateDir, "site", "update", constants.GravityBin), nil
}
