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
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
)

// Check makes sure the request is correct
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

// Check makes sure the request is correct
func (r CheckBandwidthRequest) Check() error {
	if len(r.Ping) < 1 {
		return trace.BadParameter("at least one ping address should be provided: %v", r)
	}

	return nil
}

// CheckAndSetDefaults validates the request and sets defaults.
func (r *CheckDisksRequest) CheckAndSetDefaults() error {
	for _, job := range r.Jobs {
		if err := job.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	// During operations such as install or join fio binary is placed
	// in state directory on the nodes so look there if the path
	// wasn't specified explicitly.
	if r.FioPath == "" {
		stateDir, err := state.GetStateDir()
		if err != nil {
			return trace.Wrap(err)
		}
		r.FioPath = filepath.Join(stateDir, constants.FioBin)
	}
	if _, err := utils.StatFile(r.FioPath); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Check validates the fio job spec.
func (r FioJobSpec) Check() error {
	// Generally fio does not mandate filename and will generate one if it
	// is not provided, but we require it to simplify things like cleanup.
	if r.Filename == "" {
		return trace.BadParameter("missing file name for %q test", r.Name)
	}
	return nil
}

// Flags returns command-line flags for this fio job spec.
func (r FioJobSpec) Flags() (flags []string) {
	if r.Name != "" {
		flags = append(flags, fmt.Sprint("--name=", r.Name))
	}
	if r.ReadWrite != "" {
		flags = append(flags, fmt.Sprint("--rw=", r.ReadWrite))
	}
	if r.IoEngine != "" {
		flags = append(flags, fmt.Sprint("--ioengine=", r.IoEngine))
	}
	if r.Fdatasync {
		flags = append(flags, "--fdatasync=1")
	}
	if r.Filename != "" {
		flags = append(flags, fmt.Sprint("--filename=", r.Filename))
	}
	if r.Size_ != "" {
		flags = append(flags, fmt.Sprint("--size=", r.Size_))
	}
	if r.Runtime != nil {
		flags = append(flags, fmt.Sprintf("--runtime=%vs", r.Runtime.GetSeconds()))
	}
	return flags
}

// GetWriteIOPS returns number of write iops.
func (r FioJobResult) GetWriteIOPS() float64 {
	return r.Write.Iops
}

// GetFsyncLatency returns 99th percentile of fsync latency in milliseconds.
func (r FioJobResult) GetFsyncLatency() int64 {
	return r.Sync.Latency.Percentile[bucket99] / 1000000
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
			failures++
		}
	}
	for _, ping := range r.Ping {
		if ping.Code != 0 {
			failures++
		}
	}
	return failures
}

// Result returns a text representation of this result
func (r ServerResult) Result() string {
	if r.Code == 0 {
		return fmt.Sprintf("success from %v", r.Server.Address())
	}
	return fmt.Sprintf("failure(code:%v) from %v: %v", r.Code, r.Server.Address(), r.Error)
}

// DurationFromProto returns a time.Duration from the given protobuf value
func DurationFromProto(d *types.Duration) (time.Duration, error) {
	return types.DurationFromProto(d)
}

// DurationProto returns a protobuf equivalent for the specified time.Duration
func DurationProto(d time.Duration) *types.Duration {
	return types.DurationProto(d)
}

// bucket99 is the name of the fio's 99th percentile bucket.
const bucket99 = "99.000000"
