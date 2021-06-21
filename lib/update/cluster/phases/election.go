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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// phaseElectionChange is the executor for the update master update phase
type phaseElectionChange struct {
	// OperationID is the id of the current update operation
	OperationID string
	// Server is the server currently being updated
	Server storage.Server
	// SiteName is local cluster name
	ClusterName string
	// ElectionChange represents changes to make to the cluster elections
	ElectionChange storage.ElectionChange
	// ExecutorParams stores the phase parameters
	fsm.ExecutorParams
	// FieldLogger is used for logging
	logrus.FieldLogger
	remote fsm.Remote
}

// NewPhaseElectionChange is a phase for modifying cluster elections during upgrades
func NewPhaseElectionChange(
	p fsm.ExecutorParams,
	operator ops.Operator,
	remote fsm.Remote,
	logger logrus.FieldLogger,
) (fsm.PhaseExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.ElectionChange == nil {
		return nil, trace.BadParameter("no election status specified for phase %q", p.Phase.ID)
	}

	return &phaseElectionChange{
		OperationID:    p.Plan.OperationID,
		Server:         *p.Phase.Data.Server,
		ClusterName:    p.Plan.ClusterName,
		ElectionChange: *p.Phase.Data.ElectionChange,
		ExecutorParams: p,
		FieldLogger:    logger,
		remote:         remote,
	}, nil
}

func (p *phaseElectionChange) waitForMasterMigration(rollback bool) error {
	p.Info("Wait for new leader election.")
	err := utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		leaderAddr, err := utils.ResolveAddr(constants.APIServerDomainName, p.Plan.DNSConfig.Addr())
		if err != nil {
			return trace.Wrap(err, "resolving current leader IP")
		}

		servers := p.ElectionChange.DisableServers
		if rollback {
			servers = p.ElectionChange.EnableServers
		}

		if len(servers) == 0 {
			return nil
		}

		// If we disabled a server and it was leader, wait for another server to get elected
		for _, server := range servers {
			if leaderAddr == server.AdvertiseIP {
				return utils.Continue("waiting for %q to step down as k8s leader", leaderAddr)
			}
		}
		return nil
	})
	return trace.Wrap(err)
}

func (p *phaseElectionChange) setElectionStatus(server storage.Server, enable bool) error {
	key := fmt.Sprintf("/planet/cluster/%s/election/%s", p.ClusterName, server.AdvertiseIP)

	out, err := fsm.RunCommand(utils.PlanetCommandArgs(defaults.EtcdCtlBin,
		"set", key, fmt.Sprintf("%v", enable)))
	return trace.Wrap(err, "setting leader election on %q to %v: %s", server.AdvertiseIP, enable, out)
}

// PreCheck makes sure the phase is being executed on the correct server
func (p *phaseElectionChange) PreCheck(ctx context.Context) error {
	return trace.Wrap(p.remote.CheckServer(ctx, p.Server))
}

// PostCheck is no-op for this phase
func (p *phaseElectionChange) PostCheck(context.Context) error {
	return nil
}

// Execute performs election change on this node
func (p *phaseElectionChange) Execute(context.Context) error {
	return trace.Wrap(p.updateElectionStatus(false))
}

// Rollback performs reverse operation
func (p *phaseElectionChange) Rollback(ctx context.Context) error {
	return trace.Wrap(p.updateElectionStatus(true))
}

func (p *phaseElectionChange) updateElectionStatus(rollback bool) error {
	for _, server := range p.ElectionChange.DisableServers {
		err := p.setElectionStatus(server, rollback)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	for _, server := range p.ElectionChange.EnableServers {
		err := p.setElectionStatus(server, !rollback)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(p.waitForMasterMigration(rollback))
}
