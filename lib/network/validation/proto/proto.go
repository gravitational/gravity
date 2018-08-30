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

package proto

import (
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
)

// Checks makes sure the request is correct
func (r CheckPortsRequest) Check() error {
	if len(r.Listen) == 0 {
		return trace.BadParameter("at least one listen address should be provided: %v", r)
	}

	if len(r.Ping) == 0 {
		return trace.BadParameter("at least one ping address should be provided: %v", r)
	}

	for _, server := range append(r.Listen, r.Ping...) {
		if !utils.StringInSlice([]string{"tcp", "udp"}, server.Network) {
			return trace.BadParameter("unsupported protocol %v, supported are: tcp, udp",
				server)
		}
	}

	return nil
}

// Checks makes sure the request is correct
func (r CheckBandwidthRequest) Check() error {
	if len(r.Ping) < 1 {
		return trace.BadParameter("at least one ping address should be provided: %v", r)
	}

	return nil
}

// Address returns a text representation of this server
func (r Addr) Address() string {
	return fmt.Sprintf("%v@%v", r.Network, r.Addr)
}

// FailureCount returns number of failures in the response
func (r CheckPortsResponse) FailureCount() int {
	var failures int
	for _, listen := range r.Listen {
		if listen.Code != 0 {
			failures += 1
		}
	}
	for _, ping := range r.Ping {
		if ping.Code != 0 {
			failures += 1
		}
	}
	return failures
}

// Result returns a text representation of this result
func (r ServerResult) Result() string {
	if r.Code == 0 {
		return fmt.Sprintf("success from %v", r.Server.Address())
	} else {
		return fmt.Sprintf("failure(code:%v) from %v: %v", r.Code, r.Server.Address(), r.Error)
	}
}

// DurationFromProto returns a time.Duration from the given protobuf value
func DurationFromProto(d *types.Duration) (time.Duration, error) {
	return types.DurationFromProto(d)
}

// DurationProto returns a protobuf equivalent for the specified time.Duration
func DurationProto(d time.Duration) *types.Duration {
	return types.DurationProto(d)
}
