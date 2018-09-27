/*
Copyright 2018 Gravitational, Inc.

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

package status

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/roundtrip"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/monitoring"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// FromCluster collects cluster status information.
// The function returns the partial status if not all details can be collected
func FromCluster(ctx context.Context, operator ops.Operator, cluster ops.Site, operationID string) (status *Status, err error) {
	status = &Status{
		Cluster: &Cluster{
			Domain: cluster.Domain,
			// Default to degraded - reset on successful query
			State:     ops.SiteStateDegraded,
			Reason:    cluster.Reason,
			App:       cluster.App.Package,
			Extension: newExtension(),
		},
	}

	token, err := operator.GetExpandToken(cluster.Key())
	if err != nil && !trace.IsNotFound(err) {
		return status, trace.Wrap(err)
	}
	if token != nil {
		status.Token = *token
	}

	// FIXME: have status extension accept the operator/environment
	err = status.Cluster.Extension.Collect()
	if err != nil {
		return status, trace.Wrap(err)
	}

	activeOperations, err := ops.GetActiveOperations(cluster.Key(), operator)
	if err != nil && !trace.IsNotFound(err) {
		return status, trace.Wrap(err)
	}
	for _, op := range activeOperations {
		progress, err := operator.GetSiteOperationProgress(op.Key())
		if err != nil {
			return status, trace.Wrap(err)
		}
		status.ActiveOperations = append(status.ActiveOperations,
			fromOperationAndProgress(op, *progress))
	}

	var operation *ops.SiteOperation
	var progress *ops.ProgressEntry
	// if operation ID is provided, get info for that operation, otherwise
	// get info for the most recent operation
	if operationID != "" {
		operation, progress, err = ops.GetOperationWithProgress(
			cluster.OperationKey(operationID), operator)
	} else {
		operation, progress, err = ops.GetLastCompletedOperation(
			cluster.Key(), operator)
	}
	if err != nil {
		return status, trace.Wrap(err)
	}
	status.Operation = fromOperationAndProgress(*operation, *progress)

	status.Agent, err = FromPlanetAgent(ctx, cluster.ClusterState.Servers)
	if err != nil {
		return status, trace.Wrap(err, "failed to collect system status from agents")
	}

	status.State = cluster.State
	return status, nil
}

// Check classifies this cluster status as error
func (s Status) Check() error {
	// if the site is in degraded status, make sure to exit with non-0 code
	if s.State == ops.SiteStateDegraded {
		return trace.BadParameter("Cluster status: degraded")
	}
	if s.Agent != nil && s.Agent.SystemStatus != pb.SystemStatus_Running {
		return trace.BadParameter("Cluster status: degraded")
	}
	// if the operation is in bad state, return error
	if s.Operation != nil && s.Operation.isFailed() {
		return trace.BadParameter("Operation failed")
	}
	return nil
}

// FromPlanetAgent collects cluster status from the planet agent
func FromPlanetAgent(ctx context.Context, servers []storage.Server) (*Agent, error) {
	status, err := planetAgentStatus(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query cluster status from agent")
	}

	var nodes []ClusterServer
	if len(servers) != 0 {
		nodes = fromClusterState(status, servers)
	} else {
		nodes = fromSystemStatus(status)
	}

	return &Agent{
		SystemStatus: status.Status,
		Nodes:        nodes,
	}, nil
}

// Status describes the status of the cluster as a whole
type Status struct {
	// Cluster describes the operational status of the cluster
	*Cluster `json:",inline,omitempty"`
	// Agent describes the status of the system and individual nodes
	*Agent `json:",inline,omitempty"`
}

// Cluster encapsulates collected cluster status information
type Cluster struct {
	// App references the installed application
	App loc.Locator `json:"application"`
	// State describes the cluster state
	State string `json:"state"`
	// Reason specifies the reason for the state
	Reason storage.Reason `json:"reason,omitempty"`
	// Domain provides the name of the cluster domain
	Domain string `json:"domain"`
	// Token specifies the provisioning token used for joining nodes to cluster if any
	Token storage.ProvisioningToken `json:"token"`
	// Operation describes a cluster operation.
	// This can either refer to the last completed or a specific operation
	Operation *ClusterOperation `json:"operation,omitempty"`
	// ActiveOperations is a list of operations currently active in the cluster
	ActiveOperations []*ClusterOperation `json:"active_operations,omitempty"`
	// Extension is a cluster status extension
	Extension `json:",inline,omitempty"`
}

// Key returns key structure that identifies this operation
func (r ClusterOperation) Key() ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   r.accountID,
		OperationID: r.ID,
		SiteDomain:  r.siteDomain,
	}
}

// ClusterOperation describes a cluster operation.
// This can either refer to the last or a specific operation
type ClusterOperation struct {
	// Type of the operation
	Type string `json:"type"`
	// ID of the operation
	ID string `json:"id"`
	// State of the operation (completed, in progress, failed etc)
	State string `json:"state"`
	// Created specifies the time the operation was created
	Created time.Time `json:"created"`
	// Progress describes the progress of an operation
	Progress   ClusterOperationProgress `json:"progress"`
	accountID  string
	siteDomain string
}

// IsCompleted returns whether this progress entry identifies a completed
// (successful or failed) operation
func (r ClusterOperationProgress) IsCompleted() bool {
	return r.Completion == constants.Completed
}

// Progress describes the progress of an operation
type ClusterOperationProgress struct {
	// Message provides the free text associated with this entry
	Message string `json:"message"`
	// Completion specifies the progress value in percent (0..100)
	Completion int `json:"completion"`
	// Created specifies the time the progress entry was created
	Created time.Time `json:"created"`
}

// Agent specifies the status of the system and individual nodes
type Agent struct {
	// SystemStatus defines the health status of the whole cluster
	SystemStatus pb.SystemStatus_Type `json:"system_status"`
	// Nodes lists status of each individual cluster node
	Nodes []ClusterServer `json:"nodes"`
}

// ClusterServer describes the status of the cluster node
type ClusterServer struct {
	// Hostname provides the node's hostname
	Hostname string `json:"hostname"`
	// AdvertiseIP specifies the advertise IP address
	AdvertiseIP string `json:"advertise_ip"`
	// Role is the node's cluster service role (master or regular)
	Role string `json:"role"`
	// Profile is the node's profile name from application manifest
	Profile string `json:"profile"`
	// Status describes the node's status
	Status string `json:"status"`
	// FailedProbes lists all failed probes if the node is not healthy
	FailedProbes []string `json:"failed_probes,omitempty"`
}

func (r ClusterOperation) isFailed() bool {
	return r.State == ops.OperationStateFailed
}

func fromOperationAndProgress(operation ops.SiteOperation, progress ops.ProgressEntry) *ClusterOperation {
	return &ClusterOperation{
		Type:       operation.Type,
		ID:         operation.ID,
		State:      operation.State,
		Created:    operation.Created,
		Progress:   fromProgressEntry(progress),
		siteDomain: operation.SiteDomain,
		accountID:  operation.AccountID,
	}
}

func fromProgressEntry(src ops.ProgressEntry) ClusterOperationProgress {
	return ClusterOperationProgress{
		Message:    src.Message,
		Completion: src.Completion,
		Created:    src.Created,
	}
}

// fromSystemStatus returns the list of node statuses in the absence
// of the actual cluster server list so it might be missing information
// about nodes agent status did not get response back from
func fromSystemStatus(systemStatus *pb.SystemStatus) (out []ClusterServer) {
	out = make([]ClusterServer, 0, len(systemStatus.Nodes))
	for _, node := range systemStatus.Nodes {
		nodeStatus := fromNodeStatus(*node)
		out = append(out, nodeStatus)
		if nodeStatus.Status == NodeDegraded {
			systemStatus.Status = pb.SystemStatus_Degraded
		}
	}
	return out
}

// fromClusterState generates accurate node status report including nodes missing
// in the agent report
func fromClusterState(systemStatus *pb.SystemStatus, cluster []storage.Server) (out []ClusterServer) {
	out = make([]ClusterServer, 0, len(systemStatus.Nodes))
	nodes := nodes(*systemStatus)
	for _, server := range cluster {
		node, found := nodes[server.AdvertiseIP]
		if !found {
			out = append(out, emptyNodeStatus(server))
			continue
		}

		status := fromNodeStatus(*node)
		status.Hostname = server.Hostname
		status.Profile = server.Role
		out = append(out, status)
		if status.Status == NodeDegraded {
			systemStatus.Status = pb.SystemStatus_Degraded
		}
	}
	return out
}

func planetAgentStatus(ctx context.Context) (*pb.SystemStatus, error) {
	planetClient, err := httplib.GetPlanetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	httpClient := roundtrip.HTTPClient(planetClient)
	addr := fmt.Sprintf("https://%v:%v", constants.Localhost, defaults.SatelliteRPCAgentPort)
	client, err := roundtrip.NewClient(addr, "", httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := client.Get(addr, url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var status pb.SystemStatus
	err = json.Unmarshal(resp.Bytes(), &status)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &status, nil
}

// nodes returns the set of node status objects keyed by IP
func nodes(systemStatus pb.SystemStatus) (out map[string]*pb.NodeStatus) {
	out = make(map[string]*pb.NodeStatus)
	for _, node := range systemStatus.Nodes {
		publicIP := node.MemberStatus.Tags[publicIPAddrTag]
		out[publicIP] = node
	}
	return out
}

func fromNodeStatus(node pb.NodeStatus) (status ClusterServer) {
	status.AdvertiseIP = node.MemberStatus.Tags[publicIPAddrTag]
	status.Role = node.MemberStatus.Tags[roleTag]
	switch node.Status {
	case pb.NodeStatus_Unknown:
		status.Status = NodeOffline
		return status
	case pb.NodeStatus_Running:
		status.Status = NodeHealthy
	case pb.NodeStatus_Degraded:
		status.Status = NodeDegraded
	}
	for _, probe := range node.Probes {
		if probe.Status != pb.Probe_Running {
			status.FailedProbes = append(status.FailedProbes,
				probeErrorDetail(*probe))
		}
	}
	if len(status.FailedProbes) != 0 {
		status.Status = NodeDegraded
	}
	return status
}

func emptyNodeStatus(server storage.Server) ClusterServer {
	return ClusterServer{
		Status:      NodeOffline,
		Hostname:    server.Hostname,
		AdvertiseIP: server.AdvertiseIP,
	}
}

// probeErrorDetail describes the failed probe
func probeErrorDetail(p pb.Probe) string {
	if p.Checker == monitoring.DiskSpaceCheckerID {
		detail, err := diskSpaceProbeErrorDetail(p)
		if err == nil {
			return detail
		}
		logrus.Warnf(trace.DebugReport(err))
	}
	detail := p.Detail
	if p.Detail == "" {
		detail = p.Checker
	}
	return fmt.Sprintf("%v failed (%v)", detail, p.Error)
}

// diskSpaceProbeErrorDetail returns an appropriate error message for disk
// space checker probe
//
// The reason is that state directory disk space checker always checks
// /var/lib/gravity which is default path inside planet but may be different
// on host so determine the real state directory if needed
func diskSpaceProbeErrorDetail(p pb.Probe) (string, error) {
	var data monitoring.HighWatermarkCheckerData
	err := json.Unmarshal(p.CheckerData, &data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// not state directory checker, return error as-is
	if data.Path != defaults.GravityDir {
		return p.Detail, nil
	}
	// if status command was run inside planet, the default error message is fine
	if utils.CheckInPlanet() {
		return p.Detail, nil
	}
	// otherwise determine the real state directory on host and reconstruct the message
	data.Path, err = state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return data.FailureMessage(), nil
}

const (
	// NodeHealthy is the status of a healthy node
	NodeHealthy = "healthy"
	// NodeOffline is the status of an unreachable/unavailable node
	NodeOffline = "offline"
	// NodeDegraged is the status of a node with failed probes
	NodeDegraded = "degraded"

	// publicIPAddrTag is the name of the tag containing node IP
	publicIPAddrTag = "publicip"
	// roleTag is the name of the tag containing node role
	roleTag = "role"
)
