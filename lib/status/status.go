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
	"io"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/roundtrip"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/monitoring"
	"github.com/gravitational/trace"
	"github.com/prometheus/alertmanager/api/v2/models"
	"github.com/sirupsen/logrus"
)

// FromCluster collects cluster status information.
// The function returns the partial status if not all details can be collected
func FromCluster(ctx context.Context, operator ops.Operator, cluster ops.Site, operationID string) (status *Status, err error) {
	status = &Status{
		Cluster: &Cluster{
			Domain: cluster.Domain,
			// Default to degraded - reset on successful query
			State:         ops.SiteStateDegraded,
			Reason:        cluster.Reason,
			App:           cluster.App.Package,
			ClientVersion: modules.Get().Version(),
			Extension:     newExtension(),
		},
	}

	token, err := operator.GetExpandToken(cluster.Key())
	if err != nil && !trace.IsNotFound(err) {
		return status, trace.Wrap(err)
	}
	if token != nil {
		status.Token = *token
	}

	status.Cluster.ServerVersion, err = operator.GetVersion(ctx)
	if err != nil {
		logrus.WithError(err).Warn("Failed to query server version information.")
	}

	// Collect application endpoints.
	appEndpoints, err := operator.GetApplicationEndpoints(cluster.Key())
	if err != nil {
		logrus.WithError(err).Warn("Failed to fetch application endpoints.")
		status.Endpoints.Applications.Error = err
	}
	if len(appEndpoints) != 0 {
		// Right now only 1 application is supported, in the future there
		// will be many applications each with its own endpoints.
		status.Endpoints.Applications.Endpoints = append(status.Endpoints.Applications.Endpoints,
			ApplicationEndpoints{
				Application: cluster.App.Package,
				Endpoints:   appEndpoints,
			})
	}

	// Fetch cluster endpoints.
	clusterEndpoints, err := ops.GetClusterEndpoints(operator, cluster.Key())
	if err != nil {
		logrus.WithError(err).Warn("Failed to fetch cluster endpoints.")
	}
	if clusterEndpoints != nil {
		status.Endpoints.Cluster.AuthGateway = clusterEndpoints.AuthGateways()
		status.Endpoints.Cluster.UI = clusterEndpoints.ManagementURLs()
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
		operation, err := fromOperationAndProgress(op, *progress)
		if err != nil {
			return status, trace.Wrap(err)
		}
		status.ActiveOperations = append(status.ActiveOperations, operation)
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
	status.Operation, err = fromOperationAndProgress(*operation, *progress)
	if err != nil {
		return status, trace.Wrap(err)
	}

	status.Agent, err = FromPlanetAgent(ctx, cluster.ClusterState.Servers)
	if err != nil {
		return status, trace.Wrap(err, "failed to collect system status from agents")
	}

	status.State = cluster.State

	// Collect information from alertmanager
	status.Alerts, err = FromAlertManager(ctx, cluster)
	if err != nil {
		logrus.WithError(err).Warn("Failed to collect alerts from Alertmanager.")
	}

	return status, nil
}

// FromPlanetAgent collects the cluster status from the planet agent
func FromPlanetAgent(ctx context.Context, servers []storage.Server) (*Agent, error) {
	return fromPlanetAgent(ctx, false, servers)
}

// FromLocalPlanetAgent collects the node status from the local planet agent
func FromLocalPlanetAgent(ctx context.Context) (*Agent, error) {
	return fromPlanetAgent(ctx, true, nil)
}

func fromPlanetAgent(ctx context.Context, local bool, servers []storage.Server) (*Agent, error) {
	status, err := planetAgentStatus(ctx, local)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query cluster status from agent")
	}

	var nodes []ClusterServer
	if len(servers) != 0 {
		nodes = fromClusterState(*status, servers)
	} else {
		nodes = fromSystemStatus(*status)
	}

	return &Agent{
		SystemStatus: SystemStatus(status.Status),
		Nodes:        nodes,
	}, nil
}

// IsDegraded returns whether the cluster is in degraded state
func (r Status) IsDegraded() bool {
	return (r.Cluster == nil ||
		r.Cluster.State == ops.SiteStateDegraded ||
		r.Agent == nil ||
		r.Agent.GetSystemStatus() != pb.SystemStatus_Running)
}

// Status describes the status of the cluster as a whole
type Status struct {
	// Cluster describes the operational status of the cluster
	*Cluster `json:",inline,omitempty"`
	// Agent describes the status of the system and individual nodes
	*Agent `json:",inline,omitempty"`
	// Alerts is a list of alerts collected by prometheus alertmanager
	Alerts []*models.GettableAlert `json:"alerts,omitempty"`
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
	// Endpoints contains cluster and application endpoints.
	Endpoints Endpoints `json:"endpoints"`
	// Extension is a cluster status extension
	Extension `json:"inline,omitempty"`
	// ServerVersion is version of the server the operator is talking to.
	ServerVersion *modules.Version `json:"server_version,omitempty"`
	// ClientVersion is version of the binary collecting the status.
	ClientVersion modules.Version `json:"client_version"`
}

// Endpoints contains information about cluster and application endpoints.
type Endpoints struct {
	// Applications contains endpoints for installed applications.
	Applications ApplicationsEndpoints `json:"applications,omitempty"`
	// Cluster contains system cluster endpoints.
	Cluster ClusterEndpoints `json:"cluster"`
}

// ClusterEndpoints describes cluster system endpoints.
type ClusterEndpoints struct {
	// AuthGateway contains addresses that users should specify via --proxy
	// flag to tsh commands (essentially, address of gravity-site service)
	AuthGateway []string `json:"auth_gateway,omitempty"`
	// UI contains URLs of the cluster control panel.
	UI []string `json:"ui,omitempty"`
}

// WriteTo writes cluster endpoints to the provided writer.
func (e ClusterEndpoints) WriteTo(w io.Writer) (n int64, err error) {
	var errors []error
	errors = append(errors, fprintf(&n, w, "Cluster endpoints:\n"))
	errors = append(errors, fprintf(&n, w, "    * Authentication gateway:\n"))
	for _, e := range e.AuthGateway {
		errors = append(errors, fprintf(&n, w, "        - %v\n", e))
	}
	errors = append(errors, fprintf(&n, w, "    * Cluster management URL:\n"))
	for _, e := range e.UI {
		errors = append(errors, fprintf(&n, w, "        - %v\n", e))
	}
	return n, trace.NewAggregate(errors...)
}

// ApplicationsEndpoints contains endpoints for multiple applications.
type ApplicationsEndpoints struct {
	// Endpoints lists the endpoints of all applications
	Endpoints []ApplicationEndpoints `json:"endpoints,omitempty"`
	// Error indicates whether there was an error fetching endpoints
	Error error `json:"-"`
}

// ApplicationEndpoints contains endpoints for a single application.
type ApplicationEndpoints struct {
	// Application is the application locator.
	Application loc.Locator `json:"application"`
	// Endpoints is a list of application endpoints.
	Endpoints []ops.Endpoint `json:"endpoints"`
}

// WriteTo writes all application endpoints to the provided writer.
func (e ApplicationsEndpoints) WriteTo(w io.Writer) (n int64, err error) {
	if len(e.Endpoints) == 0 {
		if e.Error != nil {
			err := fprintf(&n, w, "Application endpoints: <unable to fetch>")
			return n, trace.Wrap(err)
		}
		return 0, nil
	}
	var errors []error
	errors = append(errors, fprintf(&n, w, "Application endpoints:\n"))
	for _, app := range e.Endpoints {
		errors = append(errors, fprintf(&n, w, "    * %v:%v:\n",
			app.Application.Name, app.Application.Version))
		for _, ep := range app.Endpoints {
			errors = append(errors, fprintf(&n, w, "        - %v:\n", ep.Name))
			for _, addr := range ep.Addresses {
				errors = append(errors, fprintf(&n, w, "            - %v\n", addr))
			}
		}
	}
	return n, trace.NewAggregate(errors...)
}

func fprintf(n *int64, w io.Writer, format string, a ...interface{}) error {
	written, err := fmt.Fprintf(w, format, a...)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	*n += int64(written)
	return nil
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
	// Description is the human friendly operation description
	Description string `json:"description"`
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

// GetSystemStatus returns the status of the system
func (r Agent) GetSystemStatus() pb.SystemStatus_Type {
	return pb.SystemStatus_Type(r.SystemStatus)
}

// Agent specifies the status of the system and individual nodes
type Agent struct {
	// SystemStatus defines the health status of the whole cluster
	SystemStatus SystemStatus `json:"system_status"`
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
	// SELinux indicates whether the SELinux support is on on the node
	SELinux *bool `json:"selinux,omitempty"`
	// FailedProbes lists all failed probes if the node is not healthy
	FailedProbes []string `json:"failed_probes,omitempty"`
	// WarnProbes lists all warning probes
	WarnProbes []string `json:"warn_probes,omitempty"`
}

func fromOperationAndProgress(operation ops.SiteOperation, progress ops.ProgressEntry) (*ClusterOperation, error) {
	resource, err := ops.NewOperation(storage.SiteOperation(operation))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ClusterOperation{
		Type:        operation.Type,
		ID:          operation.ID,
		State:       operation.State,
		Created:     operation.Created,
		Description: ops.DescribeOperation(resource),
		Progress:    fromProgressEntry(progress),
		siteDomain:  operation.SiteDomain,
		accountID:   operation.AccountID,
	}, nil
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
func fromSystemStatus(systemStatus pb.SystemStatus) (out []ClusterServer) {
	out = make([]ClusterServer, 0, len(systemStatus.Nodes))
	for _, node := range systemStatus.Nodes {
		out = append(out, fromNodeStatus(*node))
	}
	return out
}

// fromClusterState generates accurate node status report including nodes missing
// in the agent report
func fromClusterState(systemStatus pb.SystemStatus, cluster []storage.Server) (out []ClusterServer) {
	out = make([]ClusterServer, 0, len(systemStatus.Nodes))
	nodes := nodes(systemStatus)
	for _, server := range cluster {
		node, found := nodes[server.AdvertiseIP]
		if !found {
			out = append(out, emptyNodeStatus(server))
			continue
		}

		status := fromNodeStatus(*node)
		status.Hostname = server.Hostname
		status.Profile = server.Role
		status.SELinux = utils.BoolPtr(server.SELinux)
		out = append(out, status)
	}
	return out
}

func planetAgentStatus(ctx context.Context, local bool) (*pb.SystemStatus, error) {
	urlFormat := "https://%v:%v"
	if local {
		urlFormat = "https://%v:%v/local"
	}
	planetClient, err := httplib.GetPlanetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	httpClient := roundtrip.HTTPClient(planetClient)
	addr := fmt.Sprintf(urlFormat, constants.Localhost, defaults.SatelliteRPCAgentPort)
	client, err := roundtrip.NewClient(addr, "", httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := client.Get(ctx, addr, url.Values{})
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
	case pb.NodeStatus_Running:
		status.Status = NodeHealthy
	case pb.NodeStatus_Degraded:
		status.Status = NodeDegraded
	}
	for _, probe := range node.Probes {
		if probe.Status != pb.Probe_Running {
			if probe.Severity != pb.Probe_Warning {
				status.FailedProbes = append(status.FailedProbes,
					probeErrorDetail(*probe))
			} else {
				status.WarnProbes = append(status.WarnProbes,
					probeErrorDetail(*probe))
			}
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
	if p.Error == "" {
		return detail
	}
	return fmt.Sprintf("%v (%v)", detail, p.Error)
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

// String returns a textual representation of this system status
func (r SystemStatus) String() string {
	switch pb.SystemStatus_Type(r) {
	case pb.SystemStatus_Running:
		return "running"
	case pb.SystemStatus_Degraded:
		return "degraded"
	case pb.SystemStatus_Unknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// GoString returns a textual representation of this system status
func (r SystemStatus) GoString() string {
	return r.String()
}

// SystemStatus is an alias for system status type
type SystemStatus pb.SystemStatus_Type

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
