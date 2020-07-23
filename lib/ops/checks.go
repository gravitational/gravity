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

package ops

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// CheckServers executes a set of preflight tests on a set of servers
// as part of the install operation given with opKey.
// agentService is the access point to the agent cluster for running remote
// commands.
// manifest specifies the application manifest with requirements.
func CheckServers(ctx context.Context,
	opKey SiteOperationKey,
	infos checks.ServerInfos,
	servers []storage.Server,
	agentService AgentService,
	manifest schema.Manifest,
) ([]*agentpb.Probe, error) {
	nodes, err := mergeServers(infos, servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	requirements, err := checks.RequirementsFromManifest(manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c, err := checks.New(checks.Config{
		Remote:       &remoteCommands{key: opKey, AgentService: agentService},
		Manifest:     manifest,
		Servers:      nodes,
		Requirements: requirements,
		Features: checks.Features{
			TestBandwidth: true,
			TestPorts:     true,
			TestEtcdDisk:  true,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.Check(ctx), nil
}

// FormatValidationError formats validation error as a human-readable text
func FormatValidationError(err error) error {
	errors := []error{err}
	if agg, ok := trace.Unwrap(err).(trace.Aggregate); ok {
		errors = agg.Errors()
	}
	var buf bytes.Buffer
	for _, err := range errors {
		fmt.Fprint(&buf, err.Error(), "\n")
	}
	return trace.BadParameter(buf.String())
}

// Exec executes an arbitrary command on the remote node specified with addr.
// The output is written into out
func (r *remoteCommands) Exec(ctx context.Context, addr string, args []string, out io.Writer) error {
	return trace.Wrap(r.AgentService.Exec(ctx, r.key, addr, args, out))
}

// CheckPorts validates the cluster port availability
func (r *remoteCommands) CheckPorts(ctx context.Context, req checks.PingPongGame) (checks.PingPongGameResults, error) {
	resp, err := r.AgentService.CheckPorts(ctx, r.key, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckBandwidth validates the cluster network bandwidth
func (r *remoteCommands) CheckBandwidth(ctx context.Context, req checks.PingPongGame) (checks.PingPongGameResults, error) {
	resp, err := r.AgentService.CheckBandwidth(ctx, r.key, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckDisks executes disk performance test on the specified node.
func (r *remoteCommands) CheckDisks(ctx context.Context, addr string, req *proto.CheckDisksRequest) (*proto.CheckDisksResponse, error) {
	res, err := r.AgentService.CheckDisks(ctx, r.key, addr, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}

// Validate validates the node given with addr against the specified manifest.
// Returns the list of failed test results.
func (r *remoteCommands) Validate(ctx context.Context, addr string, config checks.ValidateConfig) ([]*agentpb.Probe, error) {
	failed, err := r.AgentService.Validate(ctx, r.key, addr, config.Manifest, config.Profile)
	return failed, trace.Wrap(err)
}

// remoteCommands allows to execute remote commands and validate remote nodes.
// Implements checks.Remote
type remoteCommands struct {
	AgentService
	key SiteOperationKey
}

func mergeServers(infos checks.ServerInfos, servers []storage.Server) (result []checks.Server, err error) {
	result = make([]checks.Server, 0, len(servers))
	for _, server := range servers {
		info, err := infos.FindByIP(server.AdvertiseIP)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, checks.Server{Server: server, ServerInfo: *info})
	}
	return result, nil
}
