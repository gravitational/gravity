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
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// AgentStatus contains a gravity agent's status information.
type AgentStatus struct {
	// Hostname specifies the hostname of the node running the agent.
	Hostname string
	// Address specifies the IP address of the node running the agent.
	Address string
	// Status indicates the current status of the agent. An agent is `Deployed`
	// if the gravity-agent service is active. The agent is `Offline` if it
	// fails to respond to the status request.
	Status string
	// Version describes gravity agent version.
	Version string
	// Error contains an error that might have occurred when requesting agent status.
	Error error
}

// CollectAgentStatus collects the status from the specified agents.
func CollectAgentStatus(ctx context.Context, servers storage.Servers, rpc AgentRepository) []AgentStatus {
	statusCh := make(chan AgentStatus, len(servers))
	for _, srv := range servers {
		go func(server storage.Server) {
			statusCh <- getAgentStatus(ctx, server, rpc)
		}(srv)
	}

	var statusList []AgentStatus
	for range servers {
		statusList = append(statusList, <-statusCh)
	}

	return statusList
}

func getAgentStatus(ctx context.Context, server storage.Server, rpc AgentRepository) AgentStatus {
	agentStatus := AgentStatus{
		Hostname: server.Hostname,
		Address:  server.AdvertiseIP,
		Status:   constants.GravityAgentOffline,
	}

	version, err := getVersion(ctx, server.AdvertiseIP, rpc)
	if err != nil {
		agentStatus.Error = err
		return agentStatus
	}

	agentStatus.Version = version.Version
	agentStatus.Status = constants.GravityAgentDeployed
	return agentStatus
}

func getVersion(ctx context.Context, addr string, rpc AgentRepository) (*proto.Version, error) {
	ctxDial, cancelDial := context.WithTimeout(ctx, defaults.DialTimeout)
	defer cancelDial()

	clt, err := rpc.GetClient(ctxDial, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clt.Close()

	ctxVersion, cancelVersion := context.WithTimeout(ctx, defaults.AgentRequestTimeout)
	defer cancelVersion()

	version, err := clt.GetVersion(ctxVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return version, nil
}
