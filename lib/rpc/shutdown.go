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
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	rpcclient "github.com/gravitational/gravity/lib/rpc/client"
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
			err := shutdownAgent(ctx, host, rpc)
			if err != nil {
				logger.WithError(err).Errorf("failed to shut down agent at %s", host)
			} else {
				logger.Infof("shut down agent at %s", host)
			}
			errs <- trace.Wrap(err)
		}(srv)
	}

	return trace.Wrap(utils.CollectErrors(ctx, errs))
}

func shutdownAgent(ctx context.Context, addr string, rpc AgentRepository) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.DialTimeout)
	defer cancel()

	clt, err := rpc.GetClient(ctx, addr)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(clt.Shutdown(ctx))
}

// AgentRepository manages RPC connections to remote servers
type AgentRepository interface {
	// GetClient returns a client to the remote server specified with addr
	GetClient(ctx context.Context, addr string) (rpcclient.Client, error)
}
