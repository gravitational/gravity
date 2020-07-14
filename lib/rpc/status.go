/*
Copyright 2020 Gravitational, Inc.

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

package rpc

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// GravityAgentStatus contains a gravity agent's status information.
type GravityAgentStatus struct {
	// Node specifies the name of the node running the agent.
	Node string
	// IP specifies the IP address of the node running the agent.
	IP string
	// Status indiciates the current status of the agent. An agent is `Deployed`
	// if the gravity-agent service is active. The agent is `Offline` if it
	// fails to respond to the status request.
	Status string
	// Version describes gravity agent version.
	Version string
}

func AgentStatus(ctx context.Context, servers []string, logger log.FieldLogger, rpc AgentRepository) (
	[]GravityAgentStatus, error) {
	errs := make(chan error, len(servers))
	statusCh := make(chan GravityAgentStatus, len(servers))
	for _, srv := range servers {
		go func(host string) {
			systemInfo, err := getSystemInfo(ctx, host, rpc)
			if err != nil {
				logger.WithError(err).Errorf("Failed to collect system info on %s.", host)
			}

			version, err := getVersion(ctx, host, rpc)
			if err != nil {
				logger.WithError(err).Errorf("Failed to collect version info on %s.", host)
			}

			status := GravityAgentStatus{
				IP:      host,
				Status:  constants.GravityAgentOffline,
				Version: version.Version,
			}

			if systemInfo != nil {
				status.Node = systemInfo.GetHostname()
				status.Status = constants.GravityAgentDeployed
			}

			statusCh <- status
			errs <- trace.Wrap(err)
		}(srv)
	}

	var statusList []GravityAgentStatus
	for range servers {
		statusList = append(statusList, <-statusCh)
	}

	return statusList, trace.Wrap(utils.CollectErrors(ctx, errs))
}

func getSystemInfo(ctx context.Context, addr string, rpc AgentRepository) (storage.System, error) {
	ctx, cancel := context.WithTimeout(ctx, defaults.DialTimeout)
	defer cancel()

	clt, err := rpc.GetClient(ctx, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	systemInfo, err := clt.GetSystemInfo(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return systemInfo, nil
}

func getVersion(ctx context.Context, addr string, rpc AgentRepository) (version modules.Version, err error) {
	ctx, cancel := context.WithTimeout(ctx, defaults.DialTimeout)
	defer cancel()

	clt, err := rpc.GetClient(ctx, addr)
	if err != nil {
		return version, trace.Wrap(err)
	}

	version, err = clt.GetVersion(ctx)
	if err != nil {
		return version, trace.Wrap(err)
	}

	return version, nil
}
