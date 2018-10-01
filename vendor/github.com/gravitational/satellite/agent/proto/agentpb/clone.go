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
package agentpb

// Clone creates a deep-copy of this SystemStatus
func (r *SystemStatus) Clone() (status *SystemStatus) {
	status = new(SystemStatus)
	*status = *r
	status.Timestamp = status.Timestamp.Clone()
	for i, node := range r.Nodes {
		status.Nodes[i] = node.Clone()
	}
	return status
}

// Clone creates a deep-copy of this NodeStatus
func (r *NodeStatus) Clone() (status *NodeStatus) {
	status = new(NodeStatus)
	*status = *r
	status.MemberStatus = status.MemberStatus.Clone()
	for i, probe := range r.Probes {
		status.Probes[i] = new(Probe)
		*status.Probes[i] = *probe
	}
	return status
}

// Clone creates a deep-copy of this MemberStatus
func (r *MemberStatus) Clone() (status *MemberStatus) {
	status = new(MemberStatus)
	*status = *r
	status.Tags = make(map[string]string)
	for name, value := range r.Tags {
		status.Tags[name] = value
	}
	return status
}
