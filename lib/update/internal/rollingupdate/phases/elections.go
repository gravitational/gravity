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

package phases

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewElections implements the phase to manage election changes in cluster
// to support failover as the environment variables are updated
func NewElections(params libfsm.ExecutorParams, operator LocalClusterGetter, logger logrus.FieldLogger) (*elections, error) {
	if params.Phase.Data == nil || params.Phase.Data.ElectionChange == nil {
		return nil, trace.BadParameter("no election status specified for phase %q", params.Phase.ID)
	}

	return &elections{
		FieldLogger:    logger,
		Server:         *params.Phase.Data.Server,
		ElectionChange: *params.Phase.Data.ElectionChange,
		progress:       params.Progress,
		clusterName:    params.Plan.ClusterName,
		dnsAddr:        params.Plan.DNSConfig.Addr(),
	}, nil
}

func (r *elections) waitForMasterMigration(rollback bool) error {
	r.progress.NextStep("Waiting for new leader election")
	r.Info("Wait for new leader election.")
	err := utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		leaderAddr, err := utils.ResolveAddr(constants.APIServerDomainName, r.dnsAddr)
		if err != nil {
			return trace.Wrap(err, "resolving current leader IP")
		}

		servers := r.ElectionChange.DisableServers
		if rollback {
			servers = r.ElectionChange.EnableServers
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

func (r *elections) setElectionStatus(server storage.Server, enable bool) error {
	if enable {
		r.progress.NextStep("Enable leader elections on %v", server.Hostname)
	} else {
		r.progress.NextStep("Disable leader elections on %v", server.Hostname)
	}
	key := fmt.Sprintf("/planet/cluster/%s/election/%s", r.clusterName, server.AdvertiseIP)

	out, err := libfsm.RunCommand(utils.PlanetCommandArgs(defaults.EtcdCtlBin,
		"set", key, strconv.FormatBool(enable)))
	return trace.Wrap(err, "setting leader election on %q to %v: %s", server.AdvertiseIP, enable, out)
}

// PreCheck is no-op for this phase
func (r *elections) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (r *elections) PostCheck(context.Context) error {
	return nil
}

// Execute performs election change on this node
func (r *elections) Execute(context.Context) error {
	return trace.Wrap(r.updateElectionStatus(false))
}

// Rollback reverses election change performed by Execute
func (r *elections) Rollback(ctx context.Context) error {
	return trace.Wrap(r.updateElectionStatus(true))
}

func (r *elections) updateElectionStatus(rollback bool) error {
	for _, server := range r.ElectionChange.DisableServers {
		err := r.setElectionStatus(server, rollback)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	for _, server := range r.ElectionChange.EnableServers {
		err := r.setElectionStatus(server, !rollback)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(r.waitForMasterMigration(rollback))
}

// elections is the phase that manages leader elections in cluster
type elections struct {
	// FieldLogger specifies the logger for the phase
	logrus.FieldLogger
	// Server is the server currently being updated
	storage.Server
	// ElectionChange represents changes to make to the cluster elections
	storage.ElectionChange
	progress utils.Progress
	// clusterName is the name of the local cluster
	clusterName string
	dnsAddr     string
}

// LocalClusterGetter fetches data on local cluster
type LocalClusterGetter interface {
	// GetLocalSite returns the data record for the local cluster
	GetLocalSite() (*ops.Site, error)
}
