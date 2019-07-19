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

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

func validate(ctx context.Context,
	remote rpc.AgentRepository,
	servers storage.Servers,
	old, new schema.Manifest,
	docker storage.DockerConfig,
) error {
	nodes, err := checks.GetServers(ctx, remote, servers)
	if err != nil {
		return trace.Wrap(err)
	}
	requirements, err := checks.RequirementsFromManifests(old, new, servers.Profiles(), docker)
	if err != nil {
		return trace.Wrap(err)
	}
	c, err := checks.New(checks.Config{
		Remote:   checks.NewRemote(remote),
		Manifest: new,
		Servers:  nodes,
		Reqs:     requirements,
		Features: checks.Features{
			TestPorts: true,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.Run(ctx))
}
