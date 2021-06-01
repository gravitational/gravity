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

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update/cluster/checks"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// updatePhaseChecks is the update phase which executes preflight checks on a set of nodes
type updatePhaseChecks struct {
	// FieldLogger specifies the logger used by the executor
	log.FieldLogger
	// operator is the cluster operator service
	operator ops.Operator
	// apps is the cluster apps service
	apps app.Applications
	// servers is the list of local cluster servers
	servers []storage.Server
	// updatePackage specifies the updated application package
	updatePackage loc.Locator
	// remote allows remote control of servers
	remote rpc.AgentRepository
}

// NewUpdatePhaseChecks creates a new preflight checks phase executor
func NewUpdatePhaseChecks(
	p fsm.ExecutorParams,
	operator ops.Operator,
	apps app.Applications,
	remote rpc.AgentRepository,
	logger log.FieldLogger,
) (fsm.PhaseExecutor, error) {
	if p.Phase.Data.Package == nil {
		return nil, trace.NotFound("no update application package specified for phase %v", p.Phase)
	}
	return &updatePhaseChecks{
		FieldLogger:   logger,
		operator:      operator,
		apps:          apps,
		servers:       p.Plan.Servers,
		updatePackage: *p.Phase.Data.Package,
		remote:        remote,
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
	p.Infof("Executing preflight checks on %v.", storage.Servers(p.servers))
	checker, err := checks.NewChecker(ctx, checks.CheckerConfig{
		ClusterOperator: p.operator,
		ClusterApps:     p.apps,
		UpgradeApps:     p.apps,
		UpgradePackage:  p.updatePackage,
		Agents:          p.remote,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = checker.Run(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is a no-op for this phase
func (p *updatePhaseChecks) Rollback(context.Context) error {
	return nil
}
