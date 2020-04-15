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

// Package history provides a Timeline interfaces that can be used for keeping
// track of cluster status history.
package history

import (
	"context"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
)

// Timeline keeps a temporary record of events. This can be used to track the
// history of changes to the status of a gravity cluster.
type Timeline interface {
	// RecordEvents records the provided event into the current timeline.
	// Duplicate events will be ignored.
	RecordEvents(ctx context.Context, events []*pb.TimelineEvent) error
	// GetEvents returns a filtered list of events based on the provided params.
	// Events will be returned in sorted order by timestamp.
	GetEvents(ctx context.Context, params map[string]string) ([]*pb.TimelineEvent, error)
}
