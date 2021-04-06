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
	ProtoBuf() *pb.TimelineEvent
}

// EventType specifies the type of event.
type EventType string

const (
	// ClusterDegraded indicates the overall cluster is in a degraded state.
	ClusterDegraded EventType = "ClusterDegraded"
	// ClusterHealthy indicates the overall cluster is in a healthy state.
	ClusterHealthy = "ClusterHealthy"
	// NodeAdded indicates a node was added to the cluster.
	NodeAdded EventType = "NodeAdded"
	// NodeRemoved indicates a node was removed from the cluster.
	NodeRemoved EventType = "NodeRemoved"
	// NodeDegraded indicates a node is in a degraded state.
	NodeDegraded EventType = "NodeDegraded"
	// NodeHealthy indicates a node is in a healthy state.
	NodeHealthy EventType = "NodeHealthy"
	// ProbeFailed indicates a checker has failed.
	ProbeFailed EventType = "ProbeFailed"
	// ProbeSucceeded indicates a checker has succeeded.
	ProbeSucceeded EventType = "ProbeSucceeded"
	// LeaderElected indicates a node was elected to leader.
	LeaderElected EventType = "LeaderElected"
	// UnknownEvent specifies an unknown event type.
	UnknownEvent EventType = "Unknown"
)
