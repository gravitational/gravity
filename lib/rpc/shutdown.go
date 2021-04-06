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

package rpc

import (
	"bytes"
	"context"
	"io"

	"github.com/gravitational/gravity/lib/defaults"
	rpcclient "github.com/gravitational/gravity/lib/rpc/client"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ShutdownAgents takes IP host addresses to send Shutdown() RPC request
func ShutdownAgents(ctx context.Context, servers []string, logger log.FieldLogger,
	rpc AgentRepository) error {
	errs := make(chan error, len(servers))
	for _, srv := range servers {
		go func(host string) {
			err := shutdownAgent(ctx, logger, host, rpc)
			if err != nil {
				logger.WithError(err).Errorf("Failed to shut down agent on %s.", host)
			} else {
				logger.Infof("Shut down agent on %s.", host)
			}
			errs <- trace.Wrap(err)
		}(srv)
	}

	return trace.Wrap(utils.CollectErrors(ctx, errs))
}

func shutdownAgent(ctx context.Context, logger log.FieldLogger, addr string, rpc AgentRepository) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.DialTimeout)
	defer cancel()

	clt, err := rpc.GetClient(ctx, addr)
	if err != nil {
		return trace.Wrap(err)
	}
	defer clt.Close()

	var out bytes.Buffer
	err = clt.Command(ctx, logger, &out, &out, defaults.SystemctlBin, "disable", defaults.GravityRPCAgentServiceName)
	if err != nil {
		logger.WithError(err).Warnf("Failed to disable agent on %s: %s.", addr, out.String())
	}

	return trace.Wrap(clt.Shutdown(ctx, &pb.ShutdownRequest{}))
}

// AgentRepository provides an interface for creating clients for remote RPC
// agents and executing commands on them.
type AgentRepository interface {
	// RemoteRunner provides an interface for executing remote commands.
	RemoteRunner
	// GetClient returns a client to the remote server specified with addr.
	GetClient(ctx context.Context, addr string) (rpcclient.Client, error)
}

// RemoteRunner provides an interface for executing remote commands.
type RemoteRunner interface {
	io.Closer
	// Run executes a command on a remote node.
	Run(ctx context.Context, server storage.Server, command ...string) error
	// CanExecute determines whether the runner can execute a command
	// on the specified remote node.
	CanExecute(context.Context, storage.Server) error
}
