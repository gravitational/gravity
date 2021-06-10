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

package checks

import (
	"fmt"
	"time"

	pb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// PingPongRequest is a ping-pong game request
type PingPongRequest struct {
	// Listen is the listening servers
	Listen []pb.Addr `json:"listen"`
	// Ping is the remote servers
	Ping []pb.Addr `json:"ping"`
	// Duration is the duration of the game
	Duration time.Duration `json:"duration"`
	// Mode is the game mode: pingpong or bandwidth
	Mode string `json:"mode"`
}

const (
	// ModePingPong is the game mode where servers send each other
	// short "ping" messages
	ModePingPong = "pingpong"
	// ModeBandwidth is the mode for testing bandwidth between servers
	ModeBandwidth = "bandwidth"
)

// Check makes sure the request is correct
func (r PingPongRequest) Check() error {
	if !utils.StringInSlice([]string{ModePingPong, ModeBandwidth}, r.Mode) {
		return trace.BadParameter("unsupported mode %q", r.Mode)
	}
	if len(r.Listen) < 1 {
		return trace.BadParameter("at least one listen address should be provided: %v", r)
	}
	if len(r.Ping) < 1 {
		return trace.BadParameter("at least one ping address should be provided: %v", r)
	}
	if r.Mode == ModePingPong {
		for _, server := range append(r.Listen, r.Ping...) {
			if !utils.StringInSlice([]string{"tcp", "udp"}, server.Network) {
				return trace.BadParameter("unsupported protocol %v, supported are: tcp, udp",
					server)
			}
		}
	}
	return nil
}

// PortsProto converts this request to protobuf format
func (r PingPongRequest) PortsProto() *pb.CheckPortsRequest {
	var listens []*pb.Addr
	for i := range r.Listen {
		listens = append(listens, &r.Listen[i])
	}
	var pings []*pb.Addr
	for i := range r.Ping {
		pings = append(pings, &r.Ping[i])
	}
	return &pb.CheckPortsRequest{
		Listen:   listens,
		Ping:     pings,
		Duration: pb.DurationProto(r.Duration),
	}
}

// BandwidthProto converts this request to protobuf format
func (r PingPongRequest) BandwidthProto() *pb.CheckBandwidthRequest {
	listen := r.Listen[0]
	var pings []*pb.Addr
	for i := range r.Ping {
		pings = append(pings, &r.Ping[i])
	}
	return &pb.CheckBandwidthRequest{
		Listen:   &listen,
		Ping:     pings,
		Duration: pb.DurationProto(r.Duration),
	}
}

// ResultFromPortsProto converts protobuf response to PingPongResult
func ResultFromPortsProto(resp *pb.CheckPortsResponse, err error) *PingPongResult {
	result := &PingPongResult{}
	if err != nil {
		result.Code = 1
		result.Message = err.Error()
	}
	for _, listen := range resp.Listen {
		result.ListenResults = append(result.ListenResults, *listen)
	}
	for _, ping := range resp.Ping {
		result.PingResults = append(result.PingResults, *ping)
	}
	return result
}

// ResultFromBandwidthProto converts protobuf response to PingPongResult
func ResultFromBandwidthProto(resp *pb.CheckBandwidthResponse, err error) *PingPongResult {
	result := &PingPongResult{BandwidthResult: resp.Bandwidth}
	if err != nil {
		result.Code = 1
		result.Message = err.Error()
	}
	return result
}

// PingPongResult is a result of a ping-pong game
type PingPongResult struct {
	// Code means that the whole operation has succeeded
	// 0 does not mean that all results are success, it just
	// means that experiment went uninterrupted, and there
	// still can be some failures in results
	Code int `json:"code"`
	// Message contains string message
	Message string `json:"message"`
	// ListenResults contains information about attempts to start listening servers
	ListenResults []pb.ServerResult `json:"listen_result"`
	// RemoteResponses is a result of attempts to reach out to remote servers
	PingResults []pb.ServerResult `json:"ping_results"`
	// BandwidthResult is the result of the bandwidth test
	BandwidthResult uint64 `json:"bandwidth_result"`
}

// FailureCount returns number of failures in the result
func (r PingPongResult) FailureCount() int {
	var count int
	for _, l := range r.ListenResults {
		if l.Code != 0 {
			count++
		}
	}
	for _, p := range r.PingResults {
		if p.Code != 0 {
			count++
		}
	}
	return count
}

// PingPongGame describes a composite request to check ports
// in an agent cluster as a mapping of agent IP -> PingPongRequest
type PingPongGame map[string]PingPongRequest

// PingPongGameResults describes a composite result of checking ports
// in an agent cluster as a mapping of agent IP -> PingPongResult
type PingPongGameResults map[string]PingPongResult

// Failures returns the list of test failures
func (p *PingPongGameResults) Failures() []string {
	out := []string{}
	for addr, result := range *p {
		if result.Code != 0 {
			out = append(out, result.Message)
		}
		for _, listen := range result.ListenResults {
			if listen.Code != 0 {
				out = append(out, fmt.Sprintf(
					"server %v failed to bind to %v:%v",
					addr, listen.Server.Network, listen.Server.Addr))
			}
		}
		for _, ping := range result.PingResults {
			if ping.Code != 0 {
				out = append(out, fmt.Sprintf(
					"server %v failed to reach out to %v:%v",
					addr, ping.Server.Addr, ping.Server.Network))
			}
		}
	}
	return out
}
