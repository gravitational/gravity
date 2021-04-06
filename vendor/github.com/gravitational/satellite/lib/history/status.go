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

package history

import (
	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/jonboulle/clockwork"
)

// DiffCluster calculates the differences between a previous cluster status and
// a new cluster status. The differences are returned as a list of Events.
func DiffCluster(clock clockwork.Clock, old, new *pb.SystemStatus) (events []*pb.TimelineEvent) {
	oldNodes := nodeMap(old)
	newNodes := nodeMap(new)

	// Keep track of removed nodes
	removed := make(map[string]bool)
	for name := range oldNodes {
		removed[name] = true
	}

	for name, newNode := range newNodes {
		// Nodes modified
		if oldNode, ok := oldNodes[name]; ok {
			events = append(events, DiffNode(clock, oldNode, newNode)...)
			delete(removed, name)
			continue
		}

		// Nodes added to the cluster
		event := pb.NewNodeAdded(clock.Now(), name)
		events = append(events, event)
		events = append(events, DiffNode(clock, nil, newNode)...)
	}

	// Nodes removed from the cluster
	for name := range removed {
		event := pb.NewNodeRemoved(clock.Now(), name)
		events = append(events, event)
	}

	// Compare cluster status
	if old.GetStatus() == new.GetStatus() {
		return events
	}

	if new.GetStatus() == pb.SystemStatus_Running {
		events = append(events, pb.NewClusterHealthy(clock.Now()))
		return events
	}

	events = append(events, pb.NewClusterDegraded(clock.Now()))
	return events
}

// nodeMap returns the cluster's list of nodes as a map with each node mapped
// to its name.
func nodeMap(status *pb.SystemStatus) map[string]*pb.NodeStatus {
	nodes := make(map[string]*pb.NodeStatus, len(status.GetNodes()))
	for _, node := range status.GetNodes() {
		nodes[node.GetName()] = node
	}
	return nodes
}

// DiffNode calculates the differences between a previous node status and a new
// node status. The differences are returned as a list of Events.
func DiffNode(clock clockwork.Clock, old, new *pb.NodeStatus) (events []*pb.TimelineEvent) {
	oldProbes := probeMap(old)
	newProbes := probeMap(new)

	for name, newProbe := range newProbes {
		if oldProbe, ok := oldProbes[name]; ok {
			events = append(events, DiffProbe(clock, new.GetName(), oldProbe, newProbe)...)
		}
	}

	// Compare node status
	if old.GetStatus() == new.GetStatus() {
		return events
	}

	if new.GetStatus() == pb.NodeStatus_Running {
		return append(events, pb.NewNodeHealthy(clock.Now(), new.GetName()))
	}

	return append(events, pb.NewNodeDegraded(clock.Now(), new.GetName()))
}

// probeMap returns the node's list of probes as a map with each probe mapped
// to its name.
func probeMap(status *pb.NodeStatus) map[string]*pb.Probe {
	probes := make(map[string]*pb.Probe, len(status.GetProbes()))
	for _, probe := range status.GetProbes() {
		probes[probe.GetChecker()] = probe
	}
	return probes
}

// DiffProbe calculates the differences between a previous probe and a new
// probe. The differences are returned as a list of Events. The provided
// nodeName is used to specify which node the probes belong to.
func DiffProbe(clock clockwork.Clock, nodeName string, old, new *pb.Probe) (events []*pb.TimelineEvent) {
	if old.GetStatus() == new.GetStatus() {
		return events
	}

	if new.GetStatus() == pb.Probe_Running {
		return append(events, pb.NewProbeSucceeded(clock.Now(), nodeName, new.GetChecker()))
	}

	return append(events, pb.NewProbeFailed(clock.Now(), nodeName, new.GetChecker()))
}
