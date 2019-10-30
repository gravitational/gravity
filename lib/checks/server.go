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

package checks

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/rpc"
	rpcclient "github.com/gravitational/gravity/lib/rpc/client"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// ServerInfos is a collection of system infos from all agents
type ServerInfos []ServerInfo

// FindByIP returns the system information for the given IP
func (r ServerInfos) FindByIP(addr string) (*ServerInfo, error) {
	ip, _ := utils.SplitHostPort(addr, "")
	for _, info := range r {
		for _, iface := range info.GetNetworkInterfaces() {
			if iface.IPv4 == ip {
				return &info, nil
			}
		}
	}
	return nil, trace.NotFound("no system info found for IP %v", ip)
}

// Hostnames returns the list of all hostnames
func (r ServerInfos) Hostnames() (hostnames []string) {
	for _, info := range r {
		hostnames = append(hostnames, info.GetHostname())
	}
	return hostnames
}

// Transport returns transport-friendly representation
// of server info
func (r ServerInfo) Transport() (*RawServerInfo, error) {
	data, err := storage.MarshalSystemInfo(r.System)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &RawServerInfo{
		System:        data,
		RuntimeConfig: r.RuntimeConfig,
		LocalTime:     r.LocalTime,
		ServerTime:    r.ServerTime,
	}, nil
}

// ServerInfo describes a server
type ServerInfo struct {
	// System describes the remote system
	storage.System
	// RuntimeConfig is the server's runtime configuration
	pb.RuntimeConfig
	// LocalTime is the localtime at the point when this measurement
	// was taken
	LocalTime time.Time `json:"local_time"`
	// ServerTime is the remote time
	ServerTime time.Time `json:"server_time"`
}

// FromTransport converts from transport-friendly representation
// of server info
func (r RawServerInfo) FromTransport() (*ServerInfo, error) {
	info, err := storage.UnmarshalSystemInfo(r.System)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ServerInfo{
		System:        info,
		RuntimeConfig: r.RuntimeConfig,
		LocalTime:     r.LocalTime,
		ServerTime:    r.ServerTime,
	}, nil
}

// RawServerInfo describes a server.
// It is a transport-friendly format
type RawServerInfo struct {
	// System describes the remote system as a JSON-encoded blob
	System []byte
	// RuntimeConfig is the server's runtime configuration
	pb.RuntimeConfig
	// LocalTime is the localtime at the point when this measurement
	// was taken
	LocalTime time.Time `json:"local_time"`
	// ServerTime is the remote time
	ServerTime time.Time `json:"server_time"`
}

// GetServerInfo fetches remote server information
func GetServerInfo(ctx context.Context, client rpcclient.Client) (*ServerInfo, error) {
	info, err := client.GetSystemInfo(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := client.GetRuntimeConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localTime := time.Now().UTC()
	time, err := client.GetCurrentTime(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ServerInfo{
		System:        info,
		RuntimeConfig: *config,
		LocalTime:     localTime,
		ServerTime:    *time,
	}, nil
}

// GetServer returns a check server by retrieving runtime information for
// the provided server.
func GetServer(ctx context.Context, rpc rpc.AgentRepository, server storage.Server) (*Server, error) {
	connectCtx, cancel := context.WithTimeout(ctx, defaults.AgentConnectTimeout)
	clt, err := rpc.GetClient(connectCtx, server.AdvertiseIP)
	cancel()
	if err != nil {
		return nil, trace.Wrap(err, "failed to connect to agent on %v.\n"+
			"Make sure the node has an agent running by "+
			"issuing `gravity agent deploy` from the upgrade node",
			server.AdvertiseIP)
	}
	info, err := GetServerInfo(ctx, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Server{server, *info}, nil
}

// GetServers returns a list of check servers by retrieving runtime information
// for each of the provided servers.
func GetServers(ctx context.Context, rpc rpc.AgentRepository, servers []storage.Server) ([]Server, error) {
	result := make([]Server, 0, len(servers))
	for _, server := range servers {
		checkServer, err := GetServer(ctx, rpc, server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, *checkServer)
	}
	return result, nil
}
