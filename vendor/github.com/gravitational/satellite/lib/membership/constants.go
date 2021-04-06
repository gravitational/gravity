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

package membership

// MemberStatus describes the state of a serf node.
type MemberStatus string

// Possible membership statuses.
const (
	// MemberAlive indicates serf member is active.
	MemberAlive MemberStatus = "alive"
	// MemberLeaving indicates serf member is in the process of leaving the cluster.
	MemberLeaving MemberStatus = "leaving"
	// MemberLeft indicates serf member has left the cluster.
	MemberLeft MemberStatus = "left"
	// MemberFailed indicates failure has been detected on serf member.
	MemberFailed MemberStatus = "failed"
)
