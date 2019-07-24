/*
Copyright 2016 Gravitational, Inc.

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

// package health defines health checking primitives.
package health

import (
	"context"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
)

// Checker is an interface for executing a health check.
type Checker interface {
	Name() string
	// Check runs a health check and records any errors into the specified reporter.
	Check(context.Context, Reporter)
}

// Checkers is a collection of checkers.
// It implements CheckerRepository interface.
type Checkers []Checker

func (r *Checkers) AddChecker(checker Checker) {
	*r = append(*r, checker)
}

// CheckerRepository represents a collection of checkers.
type CheckerRepository interface {
	AddChecker(checker Checker)
}

// Reporter defines an obligation to report structured errors.
type Reporter interface {
	// Add adds a health probe for a specific node.
	Add(probe *pb.Probe)
	// Status retrieves the collected status after executing all checks.
	GetProbes() []*pb.Probe
	// NumProbes returns the number of probes this reporter contains
	NumProbes() int
}

// AddFrom copies probes from src to dst
func AddFrom(dst, src Reporter) {
	for _, probe := range src.GetProbes() {
		dst.Add(probe)
	}
}

// Probes is a list of probes.
// It implements the Reporter interface.
type Probes []*pb.Probe

// Add adds a health probe for a specific node.
// Implements Reporter
func (r *Probes) Add(probe *pb.Probe) {
	*r = append(*r, probe)
}

// Status retrieves the collected status after executing all checks.
// Implements Reporter
func (r Probes) GetProbes() []*pb.Probe {
	return []*pb.Probe(r)
}

// NumProbes returns the number of probes this reporter contains
// Implements Reporter
func (r Probes) NumProbes() int {
	return len(r)
}

// GetFailed returns all probes that reported an error
func (r Probes) GetFailed() []*pb.Probe {
	var failed []*pb.Probe

	for _, probe := range r {
		if probe.Status == pb.Probe_Failed {
			failed = append(failed, probe)
		}
	}

	return failed
}

// Status computes the node status based on collected probes.
func (r Probes) Status() pb.NodeStatus_Type {
	result := pb.NodeStatus_Running
	for _, probe := range r {
		if probe.Status == pb.Probe_Failed && probe.Severity != pb.Probe_Info {
			result = pb.NodeStatus_Degraded
			break
		}
	}
	return result
}
