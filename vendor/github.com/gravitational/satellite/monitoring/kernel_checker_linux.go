/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	kernelCheckerID = "kernel-check"
)

// kernelChecker is a checker that verifies that the linux kernel version is
// supported by Gravity.
type kernelChecker struct {
	// MinKernelVersion specifies the minimum supported kernel version.
	MinKernelVersion KernelVersion
	// kernelVersionReader specifies the kernel version reader function.
	kernelVersionReader
}

// NewKernelChecker returns a new instance of kernel checker.
func NewKernelChecker(version KernelVersion) health.Checker {
	return &kernelChecker{
		MinKernelVersion:    version,
		kernelVersionReader: realKernelVersionReader,
	}
}

// Name returns the checker name.
// Implements health.Checker.
func (r *kernelChecker) Name() string {
	return kernelCheckerID
}

// Check verifies kernel version.
// Implements health.Checker.
func (r *kernelChecker) Check(ctx context.Context, reporter health.Reporter) {
	if err := r.check(ctx, reporter); err != nil {
		log.WithError(err).Debug("Failed to verify kernel version.")
		return
	}
}

func (r *kernelChecker) check(_ context.Context, reporter health.Reporter) error {
	release, err := r.kernelVersionReader()
	if err != nil {
		return trace.Wrap(err, "failed to read kernel version")
	}

	kernelVersion, err := parseKernelVersion(release)
	if err != nil {
		return trace.Wrap(err, "failed to determine kernel version: %s", release)
	}

	if !r.isSupportedVersion(*kernelVersion) {
		reporter.Add(r.warningProbe(release))
		return nil
	}

	reporter.Add(r.successProbe(release))
	return nil
}

func (r *kernelChecker) successProbe(installedVersion string) *pb.Probe {
	return &pb.Probe{
		Checker: r.Name(),
		Detail:  fmt.Sprintf("Installed Linux kernel is supported: %s.", installedVersion),
		Status:  pb.Probe_Running,
	}
}

func (r *kernelChecker) warningProbe(installedVersion string) *pb.Probe {
	return &pb.Probe{
		Checker: r.Name(),
		Detail: fmt.Sprintf("Minimum recommended kernel version is %s (%s is installed).",
			r.MinKernelVersion.String(), installedVersion),
		Status:   pb.Probe_Failed,
		Severity: pb.Probe_Warning,
	}
}

// isSupportedVersion returns true if the provided linux kernel version is
// supported by Gravity.
func (r *kernelChecker) isSupportedVersion(version KernelVersion) bool {
	return !KernelVersionLessThan(r.MinKernelVersion)(version)
}
