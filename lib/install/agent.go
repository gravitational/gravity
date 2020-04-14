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
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/pack/webpack"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewAgent returns a new unstarted agent instance
func NewAgent(ctx context.Context, config AgentConfig, log log.FieldLogger, watchCh chan rpcserver.WatchEvent) (rpcserver.Server, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	addr := config.PackageAddr
	// Assume addr to be a complete address if it's prefixed with `http`
	if !strings.Contains(addr, "http") {
		host, port := utils.SplitHostPort(addr, strconv.Itoa(defaults.GravitySiteNodePort))
		addr = fmt.Sprintf("https://%v:%v", host, port)
	}

	httpClient := roundtrip.HTTPClient(httplib.GetClient(true))
	packages, err := webpack.NewBearerClient(addr, config.Token, httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := LoadRPCCredentials(ctx, packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listener, err := net.Listen("tcp", defaults.GravityRPCAgentAddr(config.AdvertiseAddr))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	peerConfig := rpcserver.PeerConfig{
		Config: rpcserver.Config{
			Listener:      listener,
			Credentials:   *creds,
			RuntimeConfig: config.RuntimeConfig,
		},
		WatchCh: watchCh,
		ReconnectStrategy: rpcserver.ReconnectStrategy{
			ShouldReconnect: utils.ShouldReconnectPeer,
		},
	}

	agent, err := rpcserver.NewPeer(peerConfig, config.ServerAddr, log)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}

	return agent, nil
}

// CheckAndSetDefaults validates this config object and sets defaults
func (r *AgentConfig) CheckAndSetDefaults() (err error) {
	if r.PackageAddr == "" {
		return trace.BadParameter("package service address is required")
	}
	if r.Token == "" {
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
	// PackageAddr references the endpoint to bootstrap credentials from
	PackageAddr string
	// AdvertiseAddr is the IP address to advertise
	AdvertiseAddr string
	// ServerAddr specifies the address of the agent server
	ServerAddr string
	// RuntimeConfig specifies runtime configuration
	pb.RuntimeConfig
}
