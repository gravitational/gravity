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

package agent

import (
	"time"
)

// MemberStatus describes the state of a serf node.
type MemberStatus string

const (
	MemberAlive   MemberStatus = "alive"
	MemberLeaving              = "leaving"
	MemberLeft                 = "left"
	MemberFailed               = "failed"
)

// Role describes the agent's server role.
type Role string

const (
	RoleMaster Role = "master"
	RoleNode        = "node"
)

// Timeout values
const (
	// lastSeenTTLSeconds specifies the time to live for the stored lastSeen values.
	// This ensures agents do not hold on to unused information when a member
	// leaves the cluster.
	lastSeenTTLSeconds = 180 // 3 minutes

	// lastSeenCapacity specifies the max number of values that can be stored in
	// the ttl map.
	lastSeenCapacity = 1000

	// timelineInitTimeout specifies the amount of time to wait for the
	// timeline to initialize.
	timelineInitTimeout = time.Minute

	// updateTimelineTimeout specifies the amount of time to wait for events
	// to be stored into the timeline.
	updateTimelineTimeout = 5 * time.Second

	// StatusUpdateTimeout is the amount of time to wait between status update collections.
	StatusUpdateTimeout = 30 * time.Second

	// recycleTimeout is the amount of time to wait between recycle attempts.
	// Recycle is a request to clean up / remove stale data that backends can choose to
	// implement.
	recycleTimeout = 10 * time.Minute

	// statusQueryReplyTimeout specifies the amount of time to wait for the cluster
	// status query reply.
	statusQueryReplyTimeout = 30 * time.Second

	// nodeStatusTimeoutLocal specifies the amount of time to wait for a node status
	// query reply. The timeout is smaller than the statusQueryReplyTimeout so
	// that the node status collection step can return results before the
	// deadline.
	nodeStatusTimeoutLocal = statusQueryReplyTimeout - (5 * time.Second)

	// nodestatusTimeoutRemote specifies the amount of time to wait for a node status from a remote node.
	// This timeout is set low, because the remote node pre-caches the status, and as such is expected to respond
	// quickly.
	nodeStatusTimeoutRemote = time.Second

	// checksTimeout specifies the amount of time to wait for a check to complete.
	// The checksTimeout is smaller than the nodeStatusTimeout so that the checks
	// can return results before the deadline.
	checksTimeout = nodeStatusTimeoutLocal - (5 * time.Second)

	// probeTimeout specifies the amount of time to wait for a probe to complete.
	// The probeTimeout is smaller than the checksTimeout so that the probe
	// collection step can return results before the deadline.
	probeTimeout = checksTimeout - (5 * time.Second)
)

// maxConcurrentCheckers specifies the maximum number of checkers active at
// any given time.
const maxConcurrentCheckers = 10
