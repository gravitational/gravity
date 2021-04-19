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

// Package membership provides an interface for querying cluster membership
// status.
package membership

import (
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
)

// Cluster interface is used to query cluster members.
type Cluster interface {
	// Members returns the list of cluster members.
	Members() ([]*pb.MemberStatus, error)
	// Member returns the member with the specified name.
	Member(name string) (*pb.MemberStatus, error)
}
