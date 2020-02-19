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

// Package history provides interfaces for keeping track of cluster status history.
package history

import (
	"context"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
)

// Timeline can be used to record changes in the system status and retrieve them
// as a list of Events.
type Timeline interface {
	// RecordStatus records any changes that have occurred since the previous
	// recorded status.
	RecordStatus(ctx context.Context, status *pb.NodeStatus) error
	// RecordTimeline merges the provided events into the current timeline.
	// Duplicate events will be ignored.
	RecordTimeline(ctx context.Context, events []*pb.TimelineEvent) error
	// GetEvents returns a filtered list of events based on the provided params.
	// Events will be returned in sorted order by timestamp.
	GetEvents(ctx context.Context, params map[string]string) ([]*pb.TimelineEvent, error)
}
