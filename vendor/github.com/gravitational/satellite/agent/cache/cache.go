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

package cache

import pb "github.com/gravitational/satellite/agent/proto/agentpb"

// Cache provides access to recent health status information
// collected on a per-node basis.
// All methods are expected to be thread-safe.
type Cache interface {
	// Update updates system status from status.
	UpdateStatus(status *pb.SystemStatus) error

	// Read obtains last known system status.
	// If nil status is returned, empty system status will be
	// assumed as defined by pb.EmptyStatus.
	RecentStatus() (*pb.SystemStatus, error)

	// Recycle is a periodic request to recycle any resources
	// cache might be holding on to. The request can also be used
	// to clean up stale state.
	Recycle() error

	// Close resets the cache and closes any resources.
	Close() error
}
