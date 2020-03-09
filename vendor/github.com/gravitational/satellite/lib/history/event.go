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
	"context"
	"time"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
)

// DataInserter can be inserted into storage given an Execer.
type DataInserter interface {
	// Insert inserts data into storage using the provided execer.
	Insert(ctx context.Context, execer Execer) error
}

// Execer executes storage operations
type Execer interface {
	// Exec executes insert operation with provided stmt and args.
	Exec(ctx context.Context, stmt string, args ...interface{}) error
}

// ProtoBuffer can be converted into a protobuf TimelineEvent.
type ProtoBuffer interface {
	// ProtoBuf returns event as a protobuf message.
	ProtoBuf() (*pb.TimelineEvent, error)
}

// newTimelineEvent constructs a new TimelineEvent with the provided timestamp.
func newTimelineEvent(timestamp time.Time) *pb.TimelineEvent {
	return &pb.TimelineEvent{
		Timestamp: &pb.Timestamp{
			Seconds:     timestamp.Unix(),
			Nanoseconds: int32(timestamp.Nanosecond()),
		},
	}
}

// NewClusterRecovered constructs a new ClusterRecovered event with the
// provided data.
func NewClusterRecovered(timestamp time.Time) *pb.TimelineEvent {
	event := newTimelineEvent(timestamp)
	event.Data = &pb.TimelineEvent_ClusterRecovered{
		ClusterRecovered: &pb.ClusterRecovered{},
	}
	return event
}

// NewClusterDegraded constructs a new ClusterDegraded event with the provided
// data.
func NewClusterDegraded(timestamp time.Time) *pb.TimelineEvent {
	event := newTimelineEvent(timestamp)
	event.Data = &pb.TimelineEvent_ClusterDegraded{
		ClusterDegraded: &pb.ClusterDegraded{},
	}
	return event
}

// NewNodeAdded constructs a new NodeAdded event with the provided data.
func NewNodeAdded(timestamp time.Time, node string) *pb.TimelineEvent {
	event := newTimelineEvent(timestamp)
	event.Data = &pb.TimelineEvent_NodeAdded{
		NodeAdded: &pb.NodeAdded{Node: node},
	}
	return event
}

// NewNodeRemoved constructs a new NodeRemoved event with the provided data.
func NewNodeRemoved(timestamp time.Time, node string) *pb.TimelineEvent {
	event := newTimelineEvent(timestamp)
	event.Data = &pb.TimelineEvent_NodeRemoved{
		NodeRemoved: &pb.NodeRemoved{Node: node},
	}
	return event
}

// NewNodeRecovered constructs a new NodeRecovered event with the provided data.
func NewNodeRecovered(timestamp time.Time, node string) *pb.TimelineEvent {
	event := newTimelineEvent(timestamp)
	event.Data = &pb.TimelineEvent_NodeRecovered{
		NodeRecovered: &pb.NodeRecovered{Node: node},
	}
	return event
}

// NewNodeDegraded constructs a new NodeDegraded event with the provided data.
func NewNodeDegraded(timestamp time.Time, node string) *pb.TimelineEvent {
	event := newTimelineEvent(timestamp)
	event.Data = &pb.TimelineEvent_NodeDegraded{
		NodeDegraded: &pb.NodeDegraded{Node: node},
	}
	return event
}

// NewProbeSucceeded constructs a new ProbeSucceeded event with the provided
// data.
func NewProbeSucceeded(timestamp time.Time, node, probe string) *pb.TimelineEvent {
	event := newTimelineEvent(timestamp)
	event.Data = &pb.TimelineEvent_ProbeSucceeded{
		ProbeSucceeded: &pb.ProbeSucceeded{
			Node:  node,
			Probe: probe,
		},
	}
	return event

}

// NewProbeFailed constructs a new ProbeFailed event with the provided data.
func NewProbeFailed(timestamp time.Time, node, probe string) *pb.TimelineEvent {
	event := newTimelineEvent(timestamp)
	event.Data = &pb.TimelineEvent_ProbeFailed{
		ProbeFailed: &pb.ProbeFailed{
			Node:  node,
			Probe: probe,
		},
	}
	return event
}
