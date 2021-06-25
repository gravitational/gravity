/*
Copyright 2019 Gravitational, Inc.

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
	"encoding/json"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/rpc/client"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Remote defines an interface for validating remote nodes.
type Remote interface {
	// Exec executes the command remotely on the specified node.
	Exec(ctx context.Context, addr string, command []string, stdout, stderr io.Writer) error
	// CheckPorts executes network test to test port availability.
	CheckPorts(context.Context, PingPongGame) (PingPongGameResults, error)
	// CheckBandwidth executes network bandwidth test.
	CheckBandwidth(context.Context, PingPongGame) (PingPongGameResults, error)
	// CheckDisks executes disk performance test on the specified node.
	CheckDisks(ctx context.Context, addr string, req *proto.CheckDisksRequest) (*proto.CheckDisksResponse, error)
	// Validate performs local checks on the specified node.
	Validate(ctx context.Context, addr string, config ValidateConfig) ([]*agentpb.Probe, error)
}

// ValidateConfig specifies validation data to validate node against.
type ValidateConfig struct {
	// Manifest is the manifest to validate against.
	Manifest schema.Manifest
	// Profile is the node profile name to validate against.
	Profile string
	// Docker is the Docker configuration to validate.
	Docker storage.DockerConfig
}

// NewRemote creates a remote node validator from the provided agents repository.
func NewRemote(agents rpc.AgentRepository) *Remoter {
	return &Remoter{
		agents: agents,
		log: logrus.WithField(trace.Component,
			"checks:remote"),
	}
}

// Remoter allows to execute remote commands and validate remote nodes.
//
// Implements Remote.
type Remoter struct {
	// agents provides access to running RPC agents.
	agents rpc.AgentRepository
	// log is used for logging.
	log logrus.FieldLogger
}

// Exec executes the command remotely on the specified node.
//
// The command's output is written to the provided writer.
func (r *Remoter) Exec(ctx context.Context, addr string, command []string, stdout, stderr io.Writer) error {
	clt, err := r.agents.GetClient(ctx, addr)
	if err != nil {
		return trace.Wrap(err)
	}
	err = clt.Command(ctx, r.log, stdout, stderr, command...)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CheckPorts executes network test to test port availability.
func (r *Remoter) CheckPorts(ctx context.Context, req PingPongGame) (PingPongGameResults, error) {
	resp, err := pingPong(ctx, r.agents, req, ports)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckBandwidth executes network bandwidth test.
func (r *Remoter) CheckBandwidth(ctx context.Context, req PingPongGame) (PingPongGameResults, error) {
	resp, err := pingPong(ctx, r.agents, req, bandwidth)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckDisks executes disk performance test.
func (r *Remoter) CheckDisks(ctx context.Context, addr string, req *proto.CheckDisksRequest) (*proto.CheckDisksResponse, error) {
	clt, err := r.agents.GetClient(ctx, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	res, err := clt.CheckDisks(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return res, nil
}

// Validate performs local checks on the specified node.
//
// Returns a list of failed test results.
func (r *Remoter) Validate(ctx context.Context, addr string, config ValidateConfig) ([]*agentpb.Probe, error) {
	clt, err := r.agents.GetClient(ctx, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	bytes, err := json.Marshal(config.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := proto.ValidateRequest{
		Manifest: bytes,
		Profile:  config.Profile,
		Docker: &proto.Docker{
			StorageDriver: config.Docker.StorageDriver,
		},
	}
	failed, err := clt.Validate(ctx, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return failed, nil
}

func pingPong(ctx context.Context, remote rpc.AgentRepository, game PingPongGame, fn pingpongHandler) (PingPongGameResults, error) {
	resultsCh := make(chan pingpongResult)
	for addr, req := range game {
		clt, err := remote.GetClient(ctx, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		go fn(ctx, rpc.AgentAddr(addr), clt, req, resultsCh)
	}
	results := make(PingPongGameResults, len(game))
	for _, req := range game {
		select {
		case result := <-resultsCh:
			if result.err != nil {
				return nil, trace.Wrap(result.err)
			}
			results[result.addr] = *result.resp
		case <-time.After(2 * req.Duration):
			return nil, trace.LimitExceeded("timeout waiting for servers")
		}
	}
	return results, nil
}

func ports(ctx context.Context, addr string, clt client.Interface, req PingPongRequest, resultsCh chan<- pingpongResult) {
	resp, err := clt.CheckPorts(ctx, req.PortsProto())
	if err != nil {
		resultsCh <- pingpongResult{addr: addr, err: err}
		return
	}
	resultsCh <- pingpongResult{addr: addr, resp: ResultFromPortsProto(resp, nil)}
}

func bandwidth(ctx context.Context, addr string, clt client.Interface, req PingPongRequest, resultsCh chan<- pingpongResult) {
	resp, err := clt.CheckBandwidth(ctx, req.BandwidthProto())
	if err != nil {
		resultsCh <- pingpongResult{addr: addr, err: err}
		return
	}
	resultsCh <- pingpongResult{addr: addr, resp: ResultFromBandwidthProto(resp, nil)}
}

type pingpongHandler func(ctx context.Context, addr string, clt client.Interface,
	req PingPongRequest, resultsCh chan<- pingpongResult)

type pingpongResult struct {
	addr string
	resp *PingPongResult
	err  error
}
