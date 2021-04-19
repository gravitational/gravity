/*
Copyright 2016-2020 Gravitational, Inc.

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
	"bytes"
	"errors"
	"fmt"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
)

// unknownNodeStatus creates an `unknown` node status for a node specified with member.
func unknownNodeStatus(member *pb.MemberStatus) *pb.NodeStatus {
	return &pb.NodeStatus{
		Name:         member.Name,
		Status:       pb.NodeStatus_Unknown,
		MemberStatus: member,
	}
}

// emptyNodeStatus creates an empty node status.
func emptyNodeStatus(name string) *pb.NodeStatus {
	return &pb.NodeStatus{
		Name:         name,
		Status:       pb.NodeStatus_Unknown,
		MemberStatus: &pb.MemberStatus{Name: name},
	}
}

// emptySystemStatus creates an empty system status.
func emptySystemStatus() *pb.SystemStatus {
	return &pb.SystemStatus{
		Status: pb.SystemStatus_Unknown,
	}
}

// setSystemStatus combines the status of individual nodes into the status of the
// cluster as a whole.
// It additionally augments the cluster status with human-readable summary.
func setSystemStatus(status *pb.SystemStatus, members []*pb.MemberStatus) {
	var foundMaster bool

	missing := make(memberMap)
	for _, member := range members {
		missing[member.Name] = struct{}{}
	}

	status.Status = pb.SystemStatus_Running
	for _, node := range status.Nodes {
		if !foundMaster && isMaster(node.MemberStatus) {
			foundMaster = true
		}
		if status.Status == pb.SystemStatus_Running {
			status.Status = nodeToSystemStatus(node.Status)
		}
		if node.MemberStatus.Status == pb.MemberStatus_Failed {
			status.Status = pb.SystemStatus_Degraded
		}
		delete(missing, node.Name)
	}
	if !foundMaster {
		status.Status = pb.SystemStatus_Degraded
		status.Summary = errNoMaster.Error()
	}
	if len(missing) != 0 {
		status.Status = pb.SystemStatus_Degraded
		status.Summary = fmt.Sprintf(msgNoStatus, missing)
	}
}

// isMaster returns true if member has role master.
func isMaster(member *pb.MemberStatus) bool {
	value, ok := member.Tags["role"]
	return ok && value == string(RoleMaster)
}

// nodeToSystemStatus converts the provided node status into a system status.
func nodeToSystemStatus(status pb.NodeStatus_Type) pb.SystemStatus_Type {
	switch status {
	case pb.NodeStatus_Running:
		return pb.SystemStatus_Running
	case pb.NodeStatus_Degraded:
		return pb.SystemStatus_Degraded
	default:
		return pb.SystemStatus_Unknown
	}
}

// isDegraded returns true if the provided system status is degraded.
func isDegraded(status pb.SystemStatus) bool {
	switch status.Status {
	case pb.SystemStatus_Unknown, pb.SystemStatus_Degraded:
		return true
	}
	return false
}

// isNodeDegraded returns true if the provided node status is degraded.
func isNodeDegraded(status pb.NodeStatus) bool {
	switch status.Status {
	case pb.NodeStatus_Unknown, pb.NodeStatus_Degraded:
		return true
	}
	return false
}

// String returns a string representation of the memeber map.
func (r memberMap) String() string {
	var buf bytes.Buffer
	for member := range r {
		buf.WriteString(member)
		buf.WriteString(",")
	}
	return buf.String()
}

type memberMap map[string]struct{}

var errNoMaster = errors.New("master node unavailable")

const msgNoStatus = "no status received from nodes (%v)"
