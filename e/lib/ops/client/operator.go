package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Client extends the open-source operator HTTP client
type Client struct {
	// Client is the wrapped open-source operator HTTP client
	*opsclient.Client
}

// New creates a new enterprise ops client wrapping provided open-source client
func New(ossClient *opsclient.Client) *Client {
	return &Client{ossClient}
}

// NewAuthenticatedClient returns enterprise ops client using provided username
// and password for authentication
func NewAuthenticatedClient(addr, username, password string, params ...opsclient.ClientParam) (*Client, error) {
	ossClient, err := opsclient.NewAuthenticatedClient(addr, username, password, params...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Client{Client: ossClient}, nil
}

// NewBearerClient returns enterprise ops client using provided bearer token
// for authentication
func NewBearerClient(addr, password string, params ...opsclient.ClientParam) (*Client, error) {
	ossClient, err := opsclient.NewBearerClient(addr, password, params...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Client{Client: ossClient}, nil
}

// RegisterAgent registers an install agent
func (c *Client) RegisterAgent(req ops.RegisterAgentRequest) (*ops.RegisterAgentResponse, error) {
	out, err := c.PutJSON(c.Endpoint("accounts", req.AccountID, "sites", req.ClusterName, "operations", "common", req.OperationID, "register"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response ops.RegisterAgentResponse
	err = json.Unmarshal(out.Bytes(), &response)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &response, nil
}

// RequestClusterCopy replicates the cluster specified in the provided request
// and its data from the remote Ops Center
func (c *Client) RequestClusterCopy(req ops.ClusterCopyRequest) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.ClusterName, "operations", "install", req.OperationID, "copy-cluster"), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetClusterEndpoints returns the cluster management endpoints such
// as control panel advertise address and agents advertise address
func (c *Client) GetClusterEndpoints(key ossops.SiteKey) (storage.Endpoints, error) {
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "cluster-endpoints"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	endpoints, err := storage.UnmarshalEndpoints(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return endpoints, nil
}

// UpdateClusterEndpoints updates the cluster management endpoints
func (c *Client) UpdateClusterEndpoints(ctx context.Context, key ossops.SiteKey, endpoints storage.Endpoints) error {
	bytes, err := storage.MarshalEndpoints(endpoints)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PutJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "cluster-endpoints"),
		&opsclient.UpsertResourceRawReq{Resource: bytes})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CheckForUpdates checks with remote OpsCenter if there is a newer version
// of the installed application
func (c *Client) CheckForUpdate(key ossops.SiteKey) (*loc.Locator, error) {
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "updates"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var update loc.Locator
	err = json.Unmarshal(out.Bytes(), &update)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &update, nil
}

// DownloadUpdates downloads the provided application version from remote
// Ops Center
func (c *Client) DownloadUpdate(ctx context.Context, req ops.DownloadUpdateRequest) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "updates"), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// EnablePeriodicUpdates turns periodic updates for the cluster on or
// updates the interval
func (c *Client) EnablePeriodicUpdates(ctx context.Context, req ops.EnablePeriodicUpdatesRequest) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "periodicupdates", "enable"), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DisablePeriodicUpdates turns periodic updates for the cluster off and
// stops the update fetch loop if it's running
func (c *Client) DisablePeriodicUpdates(ctx context.Context, key ossops.SiteKey) error {
	_, err := c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "periodicupdates", "disable"), struct{}{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StartPeriodicUpdates starts periodic updates check
func (c *Client) StartPeriodicUpdates(key ossops.SiteKey) error {
	_, err := c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "periodicupdates", "start"), struct{}{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StopPeriodicUpdates stops periodic updates check without disabling it
func (c *Client) StopPeriodicUpdates(key ossops.SiteKey) error {
	_, err := c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "periodicupdates", "stop"), struct{}{})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PeriodicUpdatesStatus returns the status of periodic updates for the cluster
func (c *Client) PeriodicUpdatesStatus(key ossops.SiteKey) (*ops.PeriodicUpdatesStatusResponse, error) {
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "periodicupdates"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var status ops.PeriodicUpdatesStatusResponse
	err = json.Unmarshal(out.Bytes(), &status)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &status, nil
}

// UpsertTrustedCluster creates or updates a trusted cluster
func (c *Client) UpsertTrustedCluster(ctx context.Context, key ossops.SiteKey, cluster storage.TrustedCluster) error {
	bytes, err := teleservices.GetTrustedClusterMarshaler().Marshal(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "trustedclusters"),
		&opsclient.UpsertResourceRawReq{Resource: bytes})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetTrustedCluster returns trusted cluster with specified name
func (c *Client) GetTrustedCluster(key ossops.SiteKey, name string) (storage.TrustedCluster, error) {
	if name == "" {
		return nil, trace.BadParameter("missing trusted cluster name")
	}
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "trustedclusters", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := storage.UnmarshalTrustedCluster(out.Bytes())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster, nil
}

// GetTrustedClusters returns a list of configured trusted clusters
func (c *Client) GetTrustedClusters(key ossops.SiteKey) ([]storage.TrustedCluster, error) {
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "trustedclusters"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	clusters := make([]storage.TrustedCluster, len(items))
	for i, item := range items {
		cluster, err := storage.UnmarshalTrustedCluster(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters[i] = cluster
	}
	return clusters, nil
}

// DeleteTrustedCluster deletes a trusted cluster by name
func (c *Client) DeleteTrustedCluster(ctx context.Context, req ops.DeleteTrustedClusterRequest) error {
	err := req.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.DeleteWithParams(c.Endpoint("accounts", req.AccountID, "sites", req.ClusterName, "trustedclusters", req.TrustedClusterName),
		url.Values{"delay": []string{req.Delay.String()}})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AcceptRemoteCluster defines the handshake between a remote cluster and this
// Ops Center
func (c *Client) AcceptRemoteCluster(req ops.AcceptRemoteClusterRequest) (*ops.AcceptRemoteClusterResponse, error) {
	out, err := c.PutJSON(c.Endpoint("accounts", req.Site.Site.AccountID, "sites", req.Site.Domain, "accept"), req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var resp ops.AcceptRemoteClusterResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return nil, trace.Wrap(err)
	}
	return &resp, nil
}

// RemoveRemoteCluster removes the cluster entry specified in the request
func (c *Client) RemoveRemoteCluster(req ops.RemoveRemoteClusterRequest) error {
	_, err := c.PutJSON(c.Endpoint("accounts", req.AccountID, "sites", req.ClusterName, "remove"), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewLicense generates a new license signed with this Ops Center CA
func (c *Client) NewLicense(ctx context.Context, req ops.NewLicenseRequest) (string, error) {
	out, err := c.PostJSON(c.Endpoint("license", "new"), req)
	if err != nil {
		return "", trace.Wrap(err)
	}
	var resp struct {
		License string `json:"license"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		return "", trace.Wrap(err)
	}
	return resp.License, nil
}

// CheckSiteLicense checks the license installed on site
func (c *Client) CheckSiteLicense(ctx context.Context, key ossops.SiteKey) error {
	_, err := c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "license", "check"), struct{}{})
	return trace.Wrap(err)
}

// UpdateLicense updates license installed on site and runs a respective app hook
func (c *Client) UpdateLicense(ctx context.Context, req ops.UpdateLicenseRequest) error {
	_, err := c.PostJSON(c.Endpoint("accounts", req.AccountID, "sites", req.SiteDomain, "license"), req)
	return trace.Wrap(err)
}

// GetLicenseCA returns CA certificate Ops Center uses to sign licenses
func (c *Client) GetLicenseCA() ([]byte, error) {
	out, err := c.Get(c.Endpoint("license", "ca"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var response struct {
		Certificate []byte `json:"certificate"`
	}
	err = json.Unmarshal(out.Bytes(), &response)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response.Certificate, nil
}

// UpsertRole creates a new role or updates an existing one
func (c *Client) UpsertRole(ctx context.Context, key ossops.SiteKey, role teleservices.Role) error {
	data, err := teleservices.GetRoleMarshaler().MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "roles"), &opsclient.UpsertResourceRawReq{
		Resource: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetRole returns a role by name
func (c *Client) GetRole(key ossops.SiteKey, name string) (teleservices.Role, error) {
	if name == "" {
		return nil, trace.BadParameter("missing role name")
	}
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "roles", name), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleservices.GetRoleMarshaler().UnmarshalRole(out.Bytes())
}

// GetRoles returns all cluster roles
func (c *Client) GetRoles(key ossops.SiteKey) ([]teleservices.Role, error) {
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "roles"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	roles := make([]teleservices.Role, len(items))
	for i, raw := range items {
		role, err := teleservices.GetRoleMarshaler().UnmarshalRole(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles[i] = role
	}
	return roles, nil
}

// DeleteRole deletes a role by name
func (c *Client) DeleteRole(ctx context.Context, key ossops.SiteKey, name string) error {
	if name == "" {
		return trace.BadParameter("missing role name")
	}
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "roles", name))
	return trace.Wrap(err)
}

// UpsertOIDCConnector creates or updates an OIDC connector
func (c *Client) UpsertOIDCConnector(ctx context.Context, key ossops.SiteKey, connector teleservices.OIDCConnector) error {
	data, err := teleservices.GetOIDCConnectorMarshaler().MarshalOIDCConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "oidc", "connectors"), &opsclient.UpsertResourceRawReq{
		Resource: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetOIDCConnector returns an OIDC connector by name
//
// Returned connector exclude client secret unless withSecrets is true.
func (c *Client) GetOIDCConnector(key ossops.SiteKey, name string, withSecrets bool) (teleservices.OIDCConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing connector name")
	}
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "oidc", "connectors", name),
		url.Values{constants.WithSecretsParam: []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleservices.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(out.Bytes())
}

// GetOIDCConnectors returns all OIDC connectors
//
// Returned connectors exclude client secret unless withSecrets is true.
func (c *Client) GetOIDCConnectors(key ossops.SiteKey, withSecrets bool) ([]teleservices.OIDCConnector, error) {
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "oidc", "connectors"),
		url.Values{constants.WithSecretsParam: []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]teleservices.OIDCConnector, len(items))
	for i, raw := range items {
		connector, err := teleservices.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteOIDCConnector deletes an OIDC connector by name
func (c *Client) DeleteOIDCConnector(ctx context.Context, key ossops.SiteKey, name string) error {
	if name == "" {
		return trace.BadParameter("missing connector name")
	}
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "oidc", "connectors", name))
	return trace.Wrap(err)
}

// UpsertSAMLConnector creates or updates a SAML connector
func (c *Client) UpsertSAMLConnector(ctx context.Context, key ossops.SiteKey, connector teleservices.SAMLConnector) error {
	data, err := teleservices.GetSAMLConnectorMarshaler().MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = c.PostJSON(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "saml", "connectors"), &opsclient.UpsertResourceRawReq{
		Resource: data,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetSAMLConnector returns a SAML connector by name
//
// Returned connector excludes private signing key unless withSecrets is true.
func (c *Client) GetSAMLConnector(key ossops.SiteKey, name string, withSecrets bool) (teleservices.SAMLConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing connector name")
	}
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "saml", "connectors", name),
		url.Values{constants.WithSecretsParam: []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleservices.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(out.Bytes())
}

// GetSAMLConnectors returns all SAML connectors
//
// Returned connectors exclude private signing keys unless withSecrets is true.
func (c *Client) GetSAMLConnectors(key ossops.SiteKey, withSecrets bool) ([]teleservices.SAMLConnector, error) {
	out, err := c.Get(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "saml", "connectors"),
		url.Values{constants.WithSecretsParam: []string{fmt.Sprintf("%t", withSecrets)}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		return nil, trace.Wrap(err)
	}
	connectors := make([]teleservices.SAMLConnector, len(items))
	for i, raw := range items {
		connector, err := teleservices.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		connectors[i] = connector
	}
	return connectors, nil
}

// DeleteSAMLConnector deletes a SAML connector by name
func (c *Client) DeleteSAMLConnector(ctx context.Context, key ossops.SiteKey, name string) error {
	if name == "" {
		return trace.BadParameter("missing connector name")
	}
	_, err := c.Delete(c.Endpoint("accounts", key.AccountID, "sites", key.SiteDomain, "saml", "connectors", name))
	return trace.Wrap(err)
}
