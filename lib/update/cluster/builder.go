/*
Copyright 2019 Gravitational, Inc.

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

package cluster

import (
	"fmt"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	libphase "github.com/gravitational/gravity/lib/update/cluster/phases"
	"github.com/gravitational/gravity/lib/update/internal/builder"
	libbuilder "github.com/gravitational/gravity/lib/update/internal/builder"

	"github.com/coreos/go-semver/semver"
	teleservices "github.com/gravitational/teleport/lib/services"
)

func (r phaseBuilder) initPhase() *builder.Phase {
	if len(r.steps) != 0 {
		return r.steps[0].initPhase(r.leadMaster, r.installedApp.Package, r.updateApp.Package)
	}
	return r.targetStep.initPhase(r.leadMaster, r.installedApp.Package, r.updateApp.Package)
}

func (r phaseBuilder) checksPhase() *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          "checks",
		Executor:    updateChecks,
		Description: "Run preflight checks",
		Data: &storage.OperationPhaseData{
			Package:          &r.updateApp.Package,
			InstalledPackage: &r.installedApp.Package,
		},
	})
}

func (r phaseBuilder) preUpdatePhase() *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          "pre-update",
		Description: "Run pre-update application hook",
		Executor:    preUpdate,
		Data: &storage.OperationPhaseData{
			Package: &r.updateApp.Package,
		},
	})
}

func (r phaseBuilder) appPhase() *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "app",
		Description: "Update installed application",
	})
	for i, loc := range r.appUpdates {
		root.AddParallelRaw(storage.OperationPhase{
			ID:          loc.Name,
			Executor:    updateApp,
			Description: fmt.Sprintf("Update application %q to %v", loc.Name, loc.Version),
			Data: &storage.OperationPhaseData{
				Package: &r.appUpdates[i],
				Values:  r.operation.Vars().Values,
			},
		})
	}
	return root
}

// migrationPhase constructs a migration phase phase based on the plan params.
//
// If there are no migrations to perform, returns nil.
func (r phaseBuilder) migrationPhase() *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "migration",
		Description: "Perform system database migration",
	})
	var subphases []storage.OperationPhase
	// do we need to migrate links to trusted clusters?
	if len(r.links) != 0 && len(r.trustedClusters) == 0 {
		subphases = append(subphases, storage.OperationPhase{
			ID:          "links",
			Description: "Migrate remote Gravity Hub links to trusted clusters",
			Executor:    migrateLinks,
		})
	}

	// Update / reset the labels during upgrade
	subphases = append(subphases, storage.OperationPhase{
		ID:          "labels",
		Description: "Update node labels",
		Executor:    updateLabels,
	})
	// migrate roles
	if libphase.NeedMigrateRoles(r.roles) {
		subphases = append(subphases, storage.OperationPhase{
			ID:          "roles",
			Description: "Migrate cluster roles to a new format",
			Executor:    migrateRoles,
			Data: &storage.OperationPhaseData{
				ExecServer: &r.leadMaster,
			},
		})
	}
	// no migrations needed
	if len(subphases) == 0 {
		return nil
	}
	root.AddParallelRaw(subphases...)
	return root
}

func (r phaseBuilder) cleanupPhase() *builder.Phase {
	root := builder.NewPhase(storage.OperationPhase{
		ID:          "gc",
		Description: "Run cleanup tasks",
	})
	for i, server := range r.planTemplate.Servers {
		node := storage.OperationPhase{
			ID:          server.Hostname,
			Description: fmt.Sprintf("Clean up node %q", server.Hostname),
			Executor:    cleanupNode,
			Data: &storage.OperationPhaseData{
				Server: &r.planTemplate.Servers[i],
			},
		}
		root.AddParallelRaw(node)
	}
	return root
}

func (r phaseBuilder) newPlan() (*storage.OperationPlan, error) {
	var root libbuilder.Phase
	if !r.hasRuntimeUpdates() {
		root.AddSequential(
			r.checksPhase(),
			r.preUpdatePhase(),
			r.appPhase(),
			r.cleanupPhase())
		return r.newPlanFrom(&root), nil
	}

	initPhase := r.initPhase()
	checksPhase := r.checksPhase().Require(initPhase)
	preUpdatePhase := r.preUpdatePhase().Require(initPhase, checksPhase)

	root.AddParallel(initPhase, checksPhase, preUpdatePhase)

	if len(r.steps) == 0 {
		// Embed the target runtime step directly into root phase
		r.targetStep.buildInline(&root, r.leadMaster, r.installedApp.Package, r.updateApp.Package,
			checksPhase, preUpdatePhase)
	} else {
		depends := []*builder.Phase{checksPhase}
		// Otherwise, build a phase for each intermediate upgrade step
		for _, step := range r.steps {
			root.AddSequential(step.build(r.leadMaster, r.installedApp.Package, r.updateApp.Package).Require(depends...))
			depends = nil
		}
		// Followed by the final runtime upgrade step
		root.AddSequential(r.targetStep.build(r.leadMaster, r.installedApp.Package, r.updateApp.Package))
	}

	if migrationPhase := r.migrationPhase(); migrationPhase != nil {
		root.AddSequential(migrationPhase)
	}

	root.AddSequential(r.appPhase(), r.cleanupPhase())

	return r.newPlanFrom(&root), nil
}

func (r phaseBuilder) newPlanFrom(root *builder.Phase) *storage.OperationPlan {
	return builder.ResolveInline(root, r.planTemplate)
}

type phaseBuilder struct {
	// operator specifies the cluster operator
	operator libphase.PackageRotator
	// planTemplate specifies the plan to bootstrap the resulting operation plan
	planTemplate storage.OperationPlan
	// operation is the operation to generate the plan for
	operation storage.SiteOperation
	// leadMaster refers to the master server running the update operation
	leadMaster storage.Server
	// installedTeleport specifies the version of the currently installed teleport
	installedTeleport loc.Locator
	// updateTeleport specifies the version of teleport to update to
	updateTeleport loc.Locator
	// installedRuntimeApp is the installed runtime application
	installedRuntimeApp app.Application
	// installedRuntimeAppVersion specifies the version of the installed runtime application
	installedRuntimeAppVersion semver.Version
	// installedApp is the installed application
	installedApp app.Application
	// updateRuntimeApp is the update runtime application
	updateRuntimeApp app.Application
	// updateRuntimeAppVersion specifies the version of the update runtime application
	updateRuntimeAppVersion semver.Version
	// updateApp is the update application
	updateApp app.Application
	// appUpdates lists the application updates
	appUpdates []loc.Locator
	// links is a list of configured remote Ops Center links
	links []storage.OpsCenterLink
	// trustedClusters is a list of configured trusted clusters
	trustedClusters []teleservices.TrustedCluster
	// packages is a reference to the cluster package service
	packages pack.PackageService
	// apps is a reference to the cluster application service
	apps app.Applications
	// roles is the existing cluster roles
	roles []teleservices.Role
	// installedDocker specifies the Docker configuration of the installed
	// cluster
	installedDocker storage.DockerConfig
	// serviceUser defines the service user on the cluster
	serviceUser storage.OSUser
	// steps lists additional intermediate runtime update steps
	steps []intermediateUpdateStep
	// targetStep defines the final runtime update step
	targetStep targetUpdateStep
	// currentEtcdVersion specifies the etcd version of the
	// installed cluster
	currentEtcdVersion semver.Version
}

// setLeaderElection creates a phase that will change the leader election state in the cluster
// enable - the list of servers to enable election on
// disable - the list of servers to disable election on
// server - The server the phase should be executed on, and used to name the phase
// key - is the identifier of the phase (combined with server.Hostname)
// msg - is a format string used to describe the phase
func setLeaderElection(electionChanges electionChanges, server storage.Server) *builder.Phase {
	return builder.NewPhase(storage.OperationPhase{
		ID:          electionChanges.ID(),
		Executor:    electionStatus,
		Description: electionChanges.description,
		Data: &storage.OperationPhaseData{
			Server: &server,
			ElectionChange: &storage.ElectionChange{
				EnableServers:  electionChanges.enable,
				DisableServers: electionChanges.disable,
			},
		},
	})
}

func serversToStorage(updates ...storage.UpdateServer) (result []storage.Server) {
	for _, update := range updates {
		result = append(result, update.Server)
	}
	return result
}

type electionChanges struct {
	enable      []storage.Server
	disable     []storage.Server
	description string
	id          string
}

func (e electionChanges) shouldAddPhase() bool {
	if len(e.enable) != 0 || len(e.disable) != 0 {
		return true
	}
	return false
}

func (e electionChanges) ID() string {
	if e.id != "" {
		return e.id
	}
	return "elect"
}

type waitsForEndpoints bool
type enableElections bool

const etcdPhaseName = "etcd"
