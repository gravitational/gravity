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
