// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package router

import (
	"context"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/e/lib/ops/service"
	"github.com/gravitational/gravity/lib/loc"
	ossops "github.com/gravitational/gravity/lib/ops"
	ossclient "github.com/gravitational/gravity/lib/ops/opsclient"
	ossrouter "github.com/gravitational/gravity/lib/ops/opsroute"
	ossservice "github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Router extends the open-source operator router
type Router struct {
	// Router is the wrapped open-source router
	*ossrouter.Router
	// Local is the local cluster operator service
	Local *service.Operator
}

// New returns a new enterprise operator router
func New(ossRouter *ossrouter.Router, local *service.Operator) *Router {
	return &Router{ossRouter, local}
}

// RegisterAgent registers an install agent
func (r *Router) RegisterAgent(req ops.RegisterAgentRequest) (*ops.RegisterAgentResponse, error) {
	return r.Local.RegisterAgent(req)
}

// RequestClusterCopy replicates the cluster specified in the provided request
// and its data from the remote Ops Center
func (r *Router) RequestClusterCopy(req ops.ClusterCopyRequest) error {
	return r.Local.RequestClusterCopy(req)
}

// GetClusterEndpoints returns the cluster management endpoints such
// as control panel advertise address and agents advertise address
func (r *Router) GetClusterEndpoints(key ossops.SiteKey) (storage.Endpoints, error) {
	client, err := r.remoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetClusterEndpoints(key)
}

// UpdateClusterEndpoints updates the cluster management endpoints
func (r *Router) UpdateClusterEndpoints(ctx context.Context, key ossops.SiteKey, endpoints storage.Endpoints) error {
	client, err := r.remoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateClusterEndpoints(ctx, key, endpoints)
}

// CheckForUpdate checks with remote OpsCenter if there is a newer version
// of the installed application
func (r *Router) CheckForUpdate(key ossops.SiteKey) (*loc.Locator, error) {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.CheckForUpdate(key)
}

// DownloadUpdate downloads the provided application version from remote
// Ops Center
func (r *Router) DownloadUpdate(ctx context.Context, req ops.DownloadUpdateRequest) error {
	client, err := r.pickClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DownloadUpdate(ctx, req)
}

// EnablePeriodicUpdates turns periodic updates for the cluster on or
// updates the interval
func (r *Router) EnablePeriodicUpdates(ctx context.Context, req ops.EnablePeriodicUpdatesRequest) error {
	client, err := r.pickClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.EnablePeriodicUpdates(ctx, req)
}

// DisablePeriodicUpdates turns periodic updates for the cluster off and
// stops the update fetch loop if it's running
func (r *Router) DisablePeriodicUpdates(ctx context.Context, key ossops.SiteKey) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DisablePeriodicUpdates(ctx, key)
}

// StartPeriodicUpdates starts periodic updates check
func (r *Router) StartPeriodicUpdates(key ossops.SiteKey) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.StartPeriodicUpdates(key)
}

// StopPeriodicUpdates stops periodic updates check without disabling it
func (r *Router) StopPeriodicUpdates(key ossops.SiteKey) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.StopPeriodicUpdates(key)
}

// PeriodicUpdatesStatus returns the status of periodic updates for the cluster
func (r *Router) PeriodicUpdatesStatus(key ossops.SiteKey) (*ops.PeriodicUpdatesStatusResponse, error) {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.PeriodicUpdatesStatus(key)
}

// UpsertTrustedCluster creates or updates a trusted cluster
func (r *Router) UpsertTrustedCluster(ctx context.Context, key ossops.SiteKey, cluster storage.TrustedCluster) error {
	return r.Local.UpsertTrustedCluster(ctx, key, cluster)
}

// DeleteTrustedCluster deletes a trusted cluster by name
func (r *Router) DeleteTrustedCluster(ctx context.Context, req ops.DeleteTrustedClusterRequest) error {
	client, err := r.pickClient(req.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteTrustedCluster(ctx, req)
}

// GetTrustedClusters returns a list of configured trusted clusters
func (r *Router) GetTrustedClusters(key ossops.SiteKey) ([]storage.TrustedCluster, error) {
	return r.Local.GetTrustedClusters(key)
}

// GetTrustedCluster returns trusted cluster by name
func (r *Router) GetTrustedCluster(key ossops.SiteKey, name string) (storage.TrustedCluster, error) {
	return r.Local.GetTrustedCluster(key, name)
}

// AcceptRemoteCluster defines the handshake between a remote cluster and this
// Ops Center
func (r *Router) AcceptRemoteCluster(req ops.AcceptRemoteClusterRequest) (*ops.AcceptRemoteClusterResponse, error) {
	return r.Local.AcceptRemoteCluster(req)
}

// RemoveRemoteCluster removes the cluster entry specified in the request
func (r *Router) RemoveRemoteCluster(req ops.RemoveRemoteClusterRequest) error {
	return r.Local.RemoveRemoteCluster(req)
}

// NewLicense generates a new license signed with this Ops Center CA
func (r *Router) NewLicense(ctx context.Context, req ops.NewLicenseRequest) (string, error) {
	return r.Local.NewLicense(ctx, req)
}

// CheckSiteLicense makes sure the license installed on cluster is correct
func (r *Router) CheckSiteLicense(ctx context.Context, key ossops.SiteKey) error {
	client, err := r.remoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.CheckSiteLicense(ctx, key)
}

// UpdateLicense updates license installed on cluster and runs a respective app hook
func (r *Router) UpdateLicense(ctx context.Context, req ops.UpdateLicenseRequest) error {
	client, err := r.remoteClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateLicense(ctx, req)
}

// GetLicenseCA returns CA certificate Ops Center uses to sign licenses
func (r *Router) GetLicenseCA() ([]byte, error) {
	return r.Local.GetLicenseCA()
}

// UpsertRole creates a new role
func (r *Router) UpsertRole(ctx context.Context, key ossops.SiteKey, role teleservices.Role) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpsertRole(ctx, key, role)
}

// GetRole returns a role by name
func (r *Router) GetRole(key ossops.SiteKey, name string) (teleservices.Role, error) {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetRole(key, name)
}

// GetRoles returns all roles
func (r *Router) GetRoles(key ossops.SiteKey) ([]teleservices.Role, error) {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetRoles(key)
}

// DeleteRole deletes a role by name
func (r *Router) DeleteRole(ctx context.Context, key ossops.SiteKey, name string) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteRole(ctx, key, name)
}

// UpsertOIDCConnector creates or updates an OIDC connector
func (r *Router) UpsertOIDCConnector(ctx context.Context, key ossops.SiteKey, connector teleservices.OIDCConnector) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpsertOIDCConnector(ctx, key, connector)
}

// GetOIDCConnector returns an OIDC connector by name
//
// Returned connector exclude client secret unless withSecrets is true.
func (r *Router) GetOIDCConnector(key ossops.SiteKey, name string, withSecrets bool) (teleservices.OIDCConnector, error) {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetOIDCConnector(key, name, withSecrets)
}

// GetOIDCConnectors returns all OIDC connectors
//
// Returned connectors exclude client secret unless withSecrets is true.
func (r *Router) GetOIDCConnectors(key ossops.SiteKey, withSecrets bool) ([]teleservices.OIDCConnector, error) {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetOIDCConnectors(key, withSecrets)
}

// DeleteOIDCConnector deletes an OIDC connector by name
func (r *Router) DeleteOIDCConnector(ctx context.Context, key ossops.SiteKey, name string) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteOIDCConnector(ctx, key, name)
}

// UpsertSAMLConnector creates or updates a SAML connector
func (r *Router) UpsertSAMLConnector(ctx context.Context, key ossops.SiteKey, connector teleservices.SAMLConnector) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpsertSAMLConnector(ctx, key, connector)
}

// GetSAMLConnector returns a SAML connector by name
//
// Returned connector excludes private signing key unless withSecrets is true.
func (r *Router) GetSAMLConnector(key ossops.SiteKey, name string, withSecrets bool) (teleservices.SAMLConnector, error) {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSAMLConnector(key, name, withSecrets)
}

// GetSAMLConnectors returns all SAML connectors
//
// Returned connectors exclude private signing keys unless withSecrets is true.
func (r *Router) GetSAMLConnectors(key ossops.SiteKey, withSecrets bool) ([]teleservices.SAMLConnector, error) {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSAMLConnectors(key, withSecrets)
}

// DeleteSAMLConnector deletes a SAML connector by name
func (r *Router) DeleteSAMLConnector(ctx context.Context, key ossops.SiteKey, name string) error {
	client, err := r.pickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteSAMLConnector(ctx, key, name)
}

func (r *Router) pickClient(clusterName string) (ops.Operator, error) {
	cluster, err := r.Backend.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cluster.Local {
		return r.Local, nil
	}
	operator, err := r.PickClient(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r.wrap(operator)
}

func (r *Router) remoteClient(clusterName string) (ops.Operator, error) {
	operator, err := r.RemoteClient(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return r.wrap(operator)
}

// wrap wraps an open-source operator into an appropriate enterprise operator
func (r *Router) wrap(operator ossops.Operator) (ops.Operator, error) {
	// it can be either local ops service or a remote ops client
	switch o := operator.(type) {
	case *ossservice.Operator:
		return service.New(o), nil
	case *ossclient.Client:
		return client.New(o), nil
	}
	return nil, trace.BadParameter("unexpected operator type: %T", operator)
}
