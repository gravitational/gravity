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

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// updatePhaseChecks is the update phase which executes preflight checks on a set of nodes
type updatePhaseChecks struct {
	// FieldLogger specifies the logger used by the executor
	log.FieldLogger
	// apps is the cluster apps service
	apps app.Applications
	// servers is the list of local cluster servers
	servers []storage.Server
	// updatePackage specifies the updated application package
	updatePackage loc.Locator
	// installedPackage specifies the installed application package
	installedPackage loc.Locator
	// remote allows remote control of servers
	remote fsm.AgentRepository
}

// NewUpdatePhaseChecks creates a new preflight checks phase executor
func NewUpdatePhaseChecks(
	c FSMConfig,
	plan storage.OperationPlan,
	phase storage.OperationPhase,
	remote fsm.AgentRepository,
) (*updatePhaseChecks, error) {
	if phase.Data.Package == nil {
		return nil, trace.NotFound("no update application package specified for phase %v", phase)
	}
	if phase.Data.InstalledPackage == nil {
		return nil, trace.NotFound("no installed application package specified for phase %v", phase)
	}
	return &updatePhaseChecks{
		FieldLogger:      log.NewEntry(log.New()),
		apps:             c.Apps,
		servers:          plan.Servers,
		updatePackage:    *phase.Data.Package,
		installedPackage: *phase.Data.InstalledPackage,
		remote:           remote,
	}, nil
}

// PreCheck is no-op for this phase
func (p *updatePhaseChecks) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (p *updatePhaseChecks) PostCheck(context.Context) error {
	return nil
}

// Execute runs preflight checks
func (p *updatePhaseChecks) Execute(ctx context.Context) error {
	installedApp, err := p.apps.GetApp(p.installedPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	app, err := p.apps.GetApp(p.updatePackage)
	if err != nil {
		return trace.Wrap(err)
	}

	err = validate(ctx, p.remote, p.servers, installedApp.Manifest, app.Manifest)
	return trace.Wrap(err, "failed to validate requirements")
}

// Rollback is a no-op for this phase
func (p *updatePhaseChecks) Rollback(context.Context) error {
	return nil
}
