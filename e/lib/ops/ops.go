package ops

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Operator extends the open-source operator interface with additional
// enterprise functionality
type Operator interface {
	// Operator is the open-source operator interface
	ops.Operator
	// OpsCenter provides Ops Center specific methods
	OpsCenter
	// Endpoints provides cluster endpoints management methods
	Endpoints
	// PeriodicUpdates provides methods for checking/downloading updates
	PeriodicUpdates
	// TrustedCluster provides methods for managing trusted clusters
	TrustedClusters
	// RemoteSupport provides methods for managing cluster access
	RemoteSupport
	// Licenses provides cluster license management methods
	Licenses
	// Identity provides methods for managing roles and auth connectors
	Identity
}

// OpsCenter defines methods specific to installation via Ops Center
type OpsCenter interface {
	// RegisterAgent is called by install agents to determine who's installer
	// and who's joining agent when installing via Ops Center
	RegisterAgent(RegisterAgentRequest) (*RegisterAgentResponse, error)
	// RequestClusterCopy replicates the cluster specified in the provided request
	// and its data from the remote Ops Center
	//
	// It is used in Ops Center initiated installs when installer process does
	// not have the cluster and operation state locally (because the operation
	// was created in the Ops Center along with the cluster and all other data).
	//
	// The following things are replicated: cluster, install operation and its
	// progress entry, both admin and regular cluster agents, expand token.
	RequestClusterCopy(ClusterCopyRequest) error
}

// RegisterAgentRequest is a request to register install agent
type RegisterAgentRequest struct {
	// AccountID is the operation account ID
	AccountID string `json:"account_id"`
	// ClusterName is the name of the cluster being installed
	ClusterName string `json:"cluster_name"`
	// OperationID is the ID of install operation
	OperationID string `json:"operation_id"`
	// AgentID is the unique agent ID
	AgentID string `json:"agent_id"`
	// AdvertiseIP is the advertise IP of the registering agent
	AdvertiseIP string `json:"advertise_ip"`
}

// SiteOperationKey makes an operation key from this request
func (r RegisterAgentRequest) SiteOperationKey() ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   r.AccountID,
		SiteDomain:  r.ClusterName,
		OperationID: r.OperationID,
	}
}

// String returns the request's string representation
func (r RegisterAgentRequest) String() string {
	return fmt.Sprintf("RegisterAgentRequest(ClusterName=%v, OperationID=%v, AgentID=%v, AdvertiseIP=%v)",
		r.ClusterName, r.OperationID, r.AgentID, r.AdvertiseIP)
}

// RegisterAgentResponse is the agent registration response
type RegisterAgentResponse struct {
	// InstallerID is the unique ID of the installer agent
	InstallerID string `json:"installer_id"`
	// InstallerIP is the advertise IP of the current installer process
	InstallerIP string `json:"installer_ip"`
}

// String returns the response's string representation
func (r RegisterAgentResponse) String() string {
	return fmt.Sprintf("RegisterAgentResponse(InstallerID=%v, InstallerIP=%v)",
		r.InstallerID, r.InstallerIP)
}

// ClusterCopyRequest is a request to clone cluster data from remote Ops Center
type ClusterCopyRequest struct {
	// AccountID is the account ID
	AccountID string `json:"account_id"`
	// ClusterName is the name of the requested cluster
	ClusterName string `json:"cluster_name"`
	// OperationID is the install operation ID
	OperationID string `json:"operation_id"`
	// OpsURL is the URL of the remote Ops Center
	OpsURL string `json:"ops_url"`
	// OpsToken is the remote Ops Center auth token
	OpsToken string `json:"ops_token"`
}

// Endpoints defines cluster endpoints management interface
type Endpoints interface {
	// GetClusterEndpoints returns the cluster management endpoints such
	// as control panel advertise address and agents advertise address
	GetClusterEndpoints(ops.SiteKey) (storage.Endpoints, error)
	// UpdateClusterEndpoints updates the cluster management endpoints
	UpdateClusterEndpoints(context.Context, ops.SiteKey, storage.Endpoints) error
}

// PeriodicUpdates interface provides methods for checking for and downloading
// newer app versions to gravity site as well as configuring periodic updates
type PeriodicUpdates interface {
	// EnablePeriodicUpdates turns periodic updates for the cluster on or
	// updates the interval
	EnablePeriodicUpdates(context.Context, EnablePeriodicUpdatesRequest) error
	// DisablePeriodicUpdates turns periodic updates for the cluster off and
	// stops the update fetch loop if it's running
	DisablePeriodicUpdates(context.Context, ops.SiteKey) error
	// StartPeriodicUpdates starts periodic updates check
	StartPeriodicUpdates(ops.SiteKey) error
	// StopPeriodicUpdates stops periodic updates check without disabling it
	// (so they will be resumed when the process restarts for example)
	StopPeriodicUpdates(ops.SiteKey) error
	// PeriodicUpdatesStatus returns the status of periodic updates for the
	// cluster
	PeriodicUpdatesStatus(ops.SiteKey) (*PeriodicUpdatesStatusResponse, error)
	// CheckForUpdates checks with remote OpsCenter if there is a newer version
	// of the installed application
	CheckForUpdate(ops.SiteKey) (*loc.Locator, error)
	// DownloadUpdates downloads the provided application version from remote
	// Ops Center
	DownloadUpdate(context.Context, DownloadUpdateRequest) error
}

// EnablePeriodicUpdatesRequest is a request to turn periodic updates on or update the interval
type EnablePeriodicUpdatesRequest struct {
	// AccountID is the site account ID
	AccountID string `json:"account_id"`
	// SiteDomain is the site domain name
	SiteDomain string `json:"site_domain"`
	// Interval is the periodic update interval
	Interval time.Duration `json:"interval,omitempty"`
}

// SiteKey is a shortcut to extract site key from this request
func (r EnablePeriodicUpdatesRequest) SiteKey() ops.SiteKey {
	return ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.SiteDomain}
}

// CheckAndSetDefaults verifies the request to enable periodic updates is correct
func (r *EnablePeriodicUpdatesRequest) CheckAndSetDefaults() error {
	if r.Interval == 0 {
		r.Interval = defaults.PeriodicUpdatesInterval
	}
	if r.Interval <= defaults.PeriodicUpdatesMinInterval {
		return trace.BadParameter(
			"minimum periodic updates interval is %v, got: %v",
			defaults.PeriodicUpdatesMinInterval, r.Interval)
	}
	return nil
}

// PeriodicUpdatesStatusResponse describes periodic updates status for a site
type PeriodicUpdatesStatusResponse struct {
	// Enabled is whether the periodic updates are enabled
	Enabled bool `json:"enabled"`
	// Interval is the periodic updates interval
	Interval time.Duration `json:"interval"`
	// NextCheck is the timestamp of the upcoming updates check
	NextCheck time.Time `json:"next_check"`
}

// DownloadUpdateRequest is a request to download a newer app version to gravity site
type DownloadUpdateRequest struct {
	// AccountID is the site account ID
	AccountID string `json:"account_id"`
	// SiteDomain is the site domain name
	SiteDomain string `json:"site_domain"`
	// Application is the application to download
	Application loc.Locator `json:"application"`
}

// SiteKey returns a site key from this request
func (r *DownloadUpdateRequest) SiteKey() ops.SiteKey {
	return ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.SiteDomain}
}

// TrustedClusters defines an interface for managing cluster access via
// remote Ops Centers using Teleport's trusted clusters concept
type TrustedClusters interface {
	// UpsertTrustedCluster creates or updates a trusted cluster
	UpsertTrustedCluster(context.Context, ops.SiteKey, storage.TrustedCluster) error
	// DeleteTrustedCluster deletes a trusted cluster by name
	DeleteTrustedCluster(context.Context, DeleteTrustedClusterRequest) error
	// GetTrustedClusters returns a list of configured trusted clusters
	GetTrustedClusters(ops.SiteKey) ([]storage.TrustedCluster, error)
	// GetTrustedCluster returns trusted cluster by name
	GetTrustedCluster(key ops.SiteKey, name string) (storage.TrustedCluster, error)
}

// DeleteTrustedClusterRequest is a request to delete a trusted cluster
type DeleteTrustedClusterRequest struct {
	// AccountID is the cluster account ID
	AccountID string `json:"account_id"`
	// ClusterName is the name of the local cluster
	ClusterName string `json:"cluster_name"`
	// TrustedClusterName is the name of the trusted cluster to delete
	TrustedClusterName string `json:"trusted_cluster_name"`
	// Delay, if not zero, specifies TTL for trusted cluster and
	// all related objects instead of deleting immediately
	Delay time.Duration `json:"delay"`
}

// Check makes sure the request is valid
func (r *DeleteTrustedClusterRequest) Check() error {
	if r.AccountID == "" {
		return trace.BadParameter("missing AccountID")
	}
	if r.ClusterName == "" {
		return trace.BadParameter("missing ClusterName")
	}
	if r.TrustedClusterName == "" {
		return trace.BadParameter("missing TrustedClusterName")
	}
	if r.Delay < 0 {
		return trace.BadParameter("expected a non-negative Delay: %v", r.Delay)
	}
	return nil
}

// SiteKey returns a site key from this request
func (r *DeleteTrustedClusterRequest) SiteKey() ops.SiteKey {
	return ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.ClusterName}
}

// String returns the request's string representation
func (r DeleteTrustedClusterRequest) String() string {
	return fmt.Sprintf("DeleteTrustedClusterRequest(ClusterName=%v, TrustedClusterName=%v, Delay=%v",
		r.ClusterName, r.TrustedClusterName, r.Delay)
}

// RemoteSupport interface manages remote access to this Ops Center
type RemoteSupport interface {
	// AcceptRemoteCluster defines the handshake between a remote cluster and this
	// Ops Center.
	//
	// If the handshake is successful, the Ops Center will create a local entry
	// for the specified cluster and return a user that can be used to query
	// trust details as well as rotate (update) itself.
	AcceptRemoteCluster(AcceptRemoteClusterRequest) (*AcceptRemoteClusterResponse, error)
	// RemoveRemoteCluster removes the cluster entry specified in the request
	RemoveRemoteCluster(RemoveRemoteClusterRequest) error
}

// AcceptRemoteClusterRequest defines a request from a remote site to add itself
// as a local deployment.
// It describes how to create a local site entry and contains a handshake token
// so that the request can be verified
type AcceptRemoteClusterRequest struct {
	// Site defines everything required to create a copy of it on the remote
	// Ops Center
	Site SiteCopy `json:"site_copy"`
	// SiteAgent is a user the cluster wants to associate itself with on the Ops
	// Center side.
	//
	// OpsCenter will replicate this user locally so that the cluster can query
	// trust details as well as be able to update (rotate) the user.
	SiteAgent storage.RemoteAccessUser `json:"user"`
	// HandshakeToken specifies the token to use for handshaking
	HandshakeToken string `json:"handshake_token"`
	// TLSCertAuthorityPackage is cert authority package with the CA of the
	// remote cluster
	TLSCertAuthorityPackage []byte `json:"tls_ca_package"`
}

// Check validates this request
func (r *AcceptRemoteClusterRequest) Check() error {
	if err := r.SiteAgent.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// String returns a string representation of a request
func (r AcceptRemoteClusterRequest) String() string {
	return fmt.Sprintf("AcceptRemoteClusterRequest(ClusterName=%v, Agent=%v)",
		r.Site.Domain, r.SiteAgent.Email)
}

// AcceptRemoteClusterResponse defines the response of the OpsCenter accepting a new remote site
// after it has validated the request
type AcceptRemoteClusterResponse struct {
	// User defines the user OpsCenter created as a result of accepting this site.
	// After a successful handshake, the site will replicate this user locally
	// and use it to pull trust details and rotate the user itself
	User storage.RemoteAccessUser
}

// RemoveRemoteClusterRequest is a request that a cluster sends to the Ops Center
// when disconnecting itself from it
type RemoveRemoteClusterRequest struct {
	// AccountID is the system account ID
	AccountID string `json:"account_id"`
	// ClusterName is the name of the cluster to remove
	ClusterName string `json:"cluster_name"`
	// HandshakeToken is the authorization token
	HandshakeToken string `json:"handshake_token"`
}

// SiteKey returns a SiteKey from this request
func (r *RemoveRemoteClusterRequest) SiteKey() ops.SiteKey {
	return ops.SiteKey{
		AccountID:  r.AccountID,
		SiteDomain: r.ClusterName,
	}
}

// String returns a string representation of a request
func (r RemoveRemoteClusterRequest) String() string {
	return fmt.Sprintf("RemoveRemoteClusterRequest(ClusterName=%v)", r.ClusterName)
}

// SiteCopy defines a subset of attributes necessary to replicate a cluster
// in a remote Ops Center
type SiteCopy struct {
	// Site is the cluster to replicate
	storage.Site `json:"site"`
	// SiteOperation is the cluster install operation
	storage.SiteOperation `json:"operation"`
	// ProgressEntry is the cluster install operation progress
	storage.ProgressEntry `json:"entry"`
}

// Licenses defines available operations with cluster licenses
type Licenses interface {
	// NewLicense generates a new license signed with this Ops Center CA
	NewLicense(context.Context, NewLicenseRequest) (string, error)
	// CheckSiteLicense makes sure the license installed on cluster is correct
	CheckSiteLicense(context.Context, ops.SiteKey) error
	// UpdateLicense updates license installed on cluster and runs a respective app hook
	UpdateLicense(context.Context, UpdateLicenseRequest) error
	// GetLicenseCA returns CA certificate Ops Center uses to sign licenses
	GetLicenseCA() ([]byte, error)
}

// NewLicenseRequest is a request to generate a new license.
type NewLicenseRequest struct {
	// MaxNodes is a maximum amount of nodes supported by the license.
	MaxNodes int `json:"max_nodes"`
	// ValidFor is a validity duration for the license, in Go's duration format.
	ValidFor time.Duration `json:"valid_for"`
	// StopApp indicates whether an application should be stopped when license expires
	StopApp bool `json:"stop_app"`
}

// CheckLicenseRequest is a request to check a license
type CheckLicenseRequest struct {
	// License is a license string to check
	License string `json:"license"`
	// Type is an optional license type
	Type string `json:"type,omitempty"`
}

// Validate makes sure that request for a new license is sane.
func (r NewLicenseRequest) Validate() error {
	if r.MaxNodes < 1 {
		return trace.BadParameter("maximum number of server should be 1 or more")
	}
	if time.Now().Add(r.ValidFor).Before(time.Now()) {
		return trace.BadParameter("expiration date can't be in the past")
	}
	return nil
}

// UpdateLicenseRequest is a request to update site's license
type UpdateLicenseRequest struct {
	// AccountID is the ID of the account the site belongs to
	AccountID string `json:"account_id"`
	// SiteDomain is the site name to update the license for
	SiteDomain string `json:"site_domain"`
	// License is the new license
	License string `json:"license"`
}

// Identity provides methods for managing roles and auth connectors
type Identity interface {
	// UpsertRole creates a new role or updates an existing one
	UpsertRole(ctx context.Context, key ops.SiteKey, role services.Role) error
	// GetRole returns a role by name
	GetRole(key ops.SiteKey, name string) (services.Role, error)
	// GetRoles returns all roles
	GetRoles(key ops.SiteKey) ([]services.Role, error)
	// DeleteRole deletes a role by name
	DeleteRole(ctx context.Context, key ops.SiteKey, name string) error
	// UpsertOIDCConnector creates or updates an OIDC connector
	UpsertOIDCConnector(ctx context.Context, key ops.SiteKey, connector services.OIDCConnector) error
	// GetOIDCConnector returns an OIDC connector by name
	GetOIDCConnector(key ops.SiteKey, name string, withSecrets bool) (services.OIDCConnector, error)
	// GetOIDCConnectors returns all OIDC connectors
	GetOIDCConnectors(key ops.SiteKey, withSecrets bool) ([]services.OIDCConnector, error)
	// DeleteOIDCConnector deletes an OIDC connector by name
	DeleteOIDCConnector(ctx context.Context, key ops.SiteKey, name string) error
	// UpsertSAMLConnector creates or updates a SAML connector
	UpsertSAMLConnector(ctx context.Context, key ops.SiteKey, connector services.SAMLConnector) error
	// GetSAMLConnector returns a SAML connector by name
	GetSAMLConnector(key ops.SiteKey, name string, withSecrets bool) (services.SAMLConnector, error)
	// GetSAMLConnectors returns all SAML connectors
	GetSAMLConnectors(key ops.SiteKey, withSecrets bool) ([]services.SAMLConnector, error)
	// DeleteSAMLConnector deletes a SAML connector by name
	DeleteSAMLConnector(ctx context.Context, key ops.SiteKey, name string) error
}
