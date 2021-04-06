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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewAgent returns a new unstarted agent instance
func NewAgent(config AgentConfig) (*rpcserver.PeerServer, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := FetchCloudMetadata(config.CloudProvider, &config.RuntimeConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	config.WithFields(log.Fields{
		"provider": config.CloudProvider,
		"metadata": config.RuntimeConfig.CloudMetadata,
	}).Info("Fetched cloud metadata.")
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
			FieldLogger:   config.FieldLogger,
			Listener:      listener,
			Credentials:   config.Credentials,
			RuntimeConfig: config.RuntimeConfig,
			AbortHandler:  config.AbortHandler,
			StopHandler:   config.StopHandler,
		},
		WatchCh:           config.WatchCh,
		ReconnectStrategy: *config.ReconnectStrategy,
	}
	agent, err := rpcserver.NewPeer(peerConfig, config.ServerAddr)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

// checkAndSetDefaults validates this config object and sets defaults
func (r *AgentConfig) checkAndSetDefaults() (err error) {
	if r.AdvertiseAddr == "" {
		r.AdvertiseAddr, err = utils.PickAdvertiseIP()
		if err != nil {
			return trace.Wrap(err,
				"failed to choose network interface on host and none was provided")
		}
	}
	if r.ReconnectStrategy == nil {
		r.ReconnectStrategy = &rpcserver.ReconnectStrategy{
			ShouldReconnect: utils.ShouldReconnectPeer,
		}
	}
	return nil
}

// AgentConfig describes configuration for a stateless agent
type AgentConfig struct {
	log.FieldLogger
	rpcserver.Credentials
	*rpcserver.ReconnectStrategy
	CloudProvider string
	// AdvertiseAddr is the IP address to advertise
	AdvertiseAddr string
	// ServerAddr specifies the address of the agent server
	ServerAddr string
	// RuntimeConfig specifies runtime configuration
	pb.RuntimeConfig
	// WatchCh specifies the channel to receive peer reconnect updates
	WatchCh chan rpcserver.WatchEvent
	// StopHandler specifies an optional handler for when the agent is stopped.
	// completed indicates whether this is the result of a successfully completed operation
	StopHandler func(ctx context.Context, completed bool) error
	// AbortHandler specifies an optional handler for abort requests
	AbortHandler func(context.Context) error
}

// getTokenFromURL extracts authorization token from the specified URL
func getTokenFromURL(agentURL string) (token string, err error) {
	url, err := url.ParseRequestURI(agentURL)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return url.Query().Get(httplib.AccessTokenQueryParam), nil
}
