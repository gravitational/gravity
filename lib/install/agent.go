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

package install

import (
	"context"
	"net"
	"net/url"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/utils"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewAgent returns a new unstarted agent instance
func NewAgent(ctx context.Context, config AgentConfig) (*rpcserver.PeerServer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := FetchCloudMetadata(config.CloudProvider, &config.RuntimeConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	listener, err := net.Listen("tcp", defaults.GravityRPCAgentAddr(config.AdvertiseAddr))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			listener.Close()
		}
	}()
	peerConfig := rpcserver.PeerConfig{
		Config: rpcserver.Config{
			Listener:      listener,
			Credentials:   config.Credentials,
			RuntimeConfig: config.RuntimeConfig,
		},
		WatchCh: config.WatchCh,
		ReconnectStrategy: rpcserver.ReconnectStrategy{
			ShouldReconnect: utils.ShouldReconnectPeer,
		},
	}
	agent, err := rpcserver.NewPeer(peerConfig, config.ServerAddr, config.FieldLogger)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	// make sure that connection to the RPC server can be established
	ctx, cancel := context.WithTimeout(ctx, defaults.PeerConnectTimeout)
	defer cancel()
	// FIXME: does the agent need to be serving here?
	if err := agent.ValidateConnection(ctx); err != nil {
		// Returning agent as it needs to be Closed by the client
		return agent, trace.Wrap(err)
	}
	return agent, nil
}

// CheckAndSetDefaults validates this config object and sets defaults
func (r *AgentConfig) CheckAndSetDefaults() (err error) {
	if r.RuntimeConfig.Token == "" {
		return trace.BadParameter("access token is required")
	}
	if r.AdvertiseAddr == "" {
		r.AdvertiseAddr, err = utils.PickAdvertiseIP()
		if err != nil {
			return trace.Wrap(err,
				"failed to choose network interface on host and none was provided")
		}
	}
	return nil
}

// AgentConfig describes configuration for a stateless agent
type AgentConfig struct {
	log.FieldLogger
	rpcserver.Credentials
	CloudProvider string
	// AdvertiseAddr is the IP address to advertise
	AdvertiseAddr string
	// ServerAddr specifies the address of the agent server
	ServerAddr string
	// RuntimeConfig specifies runtime configuration
	pb.RuntimeConfig
	WatchCh chan rpcserver.WatchEvent
}

// SplitAgentURL splits agentURL into server address and token
func SplitAgentURL(agentURL string) (serverAddr, token string, err error) {
	u, err := url.ParseRequestURI(agentURL)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	addr, err := teleutils.ParseAddr("tcp://" + u.Host)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	url, err := url.ParseRequestURI(agentURL)
	if err != nil {
		return "", "", trace.Wrap(err)
	}
	token = url.Query().Get(httplib.AccessTokenQueryParam)
	return addr.Addr, token, nil
}
