/*
Copyright 2017 Gravitational, Inc.

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

package monitoring

import (
	"context"
	"fmt"
	"runtime"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"

	"github.com/cloudfoundry/gosigar"
	"github.com/dustin/go-humanize"
)

// NewHostChecker returns a new checker to validate host environment
func NewHostChecker(config HostConfig) health.Checker {
	return &hostChecker{
		HostConfig: config,
		getMemory:  realGetMemory,
		getCPU:     realGetCPU,
	}
}

// HostConfig describes host environment requirements
type HostConfig struct {
	// MinCPU specifies the minimum amount of logical CPUs to expect
	MinCPU int
	// MinRAMBytes specifies the minimum amount of RAM to expect
	MinRAMBytes uint64
}

// hostChecker validates CPU and RAM requirements
type hostChecker struct {
	HostConfig
	getMemory memoryGetterFunc
	getCPU    cpuGetterFunc
}

// Name returns checker id
// Implements health.Checker
func (c *hostChecker) Name() string {
	return hostCheckerID
}

// Check validates that the host has enough RAM/CPU.
// Implements health.Checker
func (c *hostChecker) Check(ctx context.Context, reporter health.Reporter) {
	var probes health.Probes
	err := c.check(ctx, &probes)
	if err != nil && !trace.IsNotFound(err) {
		reporter.Add(NewProbeFromErr(c.Name(), "failed to validate host environment", err))
		return
	}

	health.AddFrom(reporter, &probes)
	if probes.NumProbes() != 0 {
		return
	}

	reporter.Add(NewSuccessProbe(c.Name()))
}

func (c *hostChecker) check(ctx context.Context, reporter health.Reporter) error {
	mem, err := c.getMemory()
	if err != nil {
		return trace.Wrap(err, "failed to query memory info")
	}

	if mem.Total < c.MinRAMBytes {
		reporter.Add(&pb.Probe{
			Checker: c.Name(),
			Detail: fmt.Sprintf("at least %s of RAM required, only %s available",
				humanize.Bytes(c.MinRAMBytes), humanize.Bytes(mem.Total)),
			Status: pb.Probe_Failed,
		})
	}

	if c.getCPU() < c.MinCPU {
		reporter.Add(&pb.Probe{
			Checker: c.Name(),
			Detail: fmt.Sprintf("at least %d CPUs required, only %d available",
				c.MinCPU, c.getCPU()),
			Status: pb.Probe_Failed,
		})
	}

	return nil
}

type memoryGetterFunc func() (*sigar.Mem, error)

func realGetMemory() (*sigar.Mem, error) {
	var mem sigar.Mem
	if err := mem.Get(); err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return &mem, nil
}

type cpuGetterFunc func() (numCPU int)

func realGetCPU() (numCPU int) {
	return runtime.NumCPU()
}

const (
	hostCheckerID = "cpu-ram"
)
