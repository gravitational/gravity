package acl

import (
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	oss "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// OperatorACL extends ACL operator from open-source
type OperatorACL struct {
	// OperatorACL is the wrapped open-source ACL operator
	*oss.OperatorACL
	// operator is the enterprise operator service
	operator ops.Operator
}

// OperatorWithACL returns a new enterprise ACL operator
func OperatorWithACL(operatorACL *oss.OperatorACL, operator ops.Operator) *OperatorACL {
	return &OperatorACL{
		OperatorACL: operatorACL,
		operator:    operator,
	}
}

// RegisterAgent registers an install agent
func (o *OperatorACL) RegisterAgent(req ops.RegisterAgentRequest) (*ops.RegisterAgentResponse, error) {
	if err := o.ClusterAction(req.ClusterName, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.RegisterAgent(req)
}

// RequestClusterCopy replicates the cluster specified in the provided request
// and its data from the remote Ops Center
func (o *OperatorACL) RequestClusterCopy(req ops.ClusterCopyRequest) error {
	if err := o.Action(storage.KindCluster, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.RequestClusterCopy(req)
}

// GetClusterEndpoints returns the cluster management endpoints such
// as control panel advertise address and agents advertise address
func (o *OperatorACL) GetClusterEndpoints(key oss.SiteKey) (storage.Endpoints, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetClusterEndpoints(key)
}

// UpdateClusterEndpoints updates the cluster management endpoints
func (o *OperatorACL) UpdateClusterEndpoints(key oss.SiteKey, endpoints storage.Endpoints) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateClusterEndpoints(key, endpoints)
}

// CheckForUpdates checks with remote OpsCenter if there is a newer version
// of the installed application
func (o *OperatorACL) CheckForUpdate(key oss.SiteKey) (*loc.Locator, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CheckForUpdate(key)
}

// DownloadUpdates downloads the provided application version from remote
// Ops Center
func (o *OperatorACL) DownloadUpdate(req ops.DownloadUpdateRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DownloadUpdate(req)
}

// EnablePeriodicUpdates turns periodic updates for the cluster on or
// updates the interval
func (o *OperatorACL) EnablePeriodicUpdates(req ops.EnablePeriodicUpdatesRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.EnablePeriodicUpdates(req)
}

// DisablePeriodicUpdates turns periodic updates for the cluster off and
// stops the update fetch loop if it's running
func (o *OperatorACL) DisablePeriodicUpdates(key oss.SiteKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DisablePeriodicUpdates(key)
}

// StartPeriodicUpdates starts periodic updates check
func (o *OperatorACL) StartPeriodicUpdates(key oss.SiteKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.StartPeriodicUpdates(key)
}

// StopPeriodicUpdates stops periodic updates check without disabling it
func (o *OperatorACL) StopPeriodicUpdates(key oss.SiteKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.StopPeriodicUpdates(key)
}

// PeriodicUpdatesStatus returns the status of periodic updates for the cluster
func (o *OperatorACL) PeriodicUpdatesStatus(key oss.SiteKey) (*ops.PeriodicUpdatesStatusResponse, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.PeriodicUpdatesStatus(key)
}

// UpsertTrustedCluster creates or updates a trusted cluster
func (o *OperatorACL) UpsertTrustedCluster(key oss.SiteKey, cluster storage.TrustedCluster) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpsertTrustedCluster(key, cluster)
}

// DeleteTrustedCluster deletes a trusted cluster by name
func (o *OperatorACL) DeleteTrustedCluster(req ops.DeleteTrustedClusterRequest) error {
	if err := o.ClusterAction(req.ClusterName, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteTrustedCluster(req)
}

// GetTrustedClusters returns a list of configured trusted clusters
func (o *OperatorACL) GetTrustedClusters(key oss.SiteKey) ([]storage.TrustedCluster, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetTrustedClusters(key)
}

// GetTrustedCluster returns trusted cluster by name
func (o *OperatorACL) GetTrustedCluster(key oss.SiteKey, name string) (storage.TrustedCluster, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetTrustedCluster(key, name)
}

// AcceptRemoteCluster defines the handshake between a remote cluster and this
// Ops Center
func (o *OperatorACL) AcceptRemoteCluster(req ops.AcceptRemoteClusterRequest) (*ops.AcceptRemoteClusterResponse, error) {
	if err := o.Action(storage.KindCluster, storage.VerbRegister); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.AcceptRemoteCluster(req)
}

// RemoveRemoteCluster removes the cluster entry specified in the request
func (o *OperatorACL) RemoveRemoteCluster(req ops.RemoveRemoteClusterRequest) error {
	if err := o.Action(storage.KindCluster, storage.VerbRegister); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.RemoveRemoteCluster(req)
}

// NewLicense generates a new license signed with this Ops Center CA
func (o *OperatorACL) NewLicense(req ops.NewLicenseRequest) (string, error) {
	if err := o.Action(storage.KindLicense, teleservices.VerbCreate); err != nil {
		return "", trace.Wrap(err)
	}
	return o.operator.NewLicense(req)
}

// CheckSiteLicense makes sure the license installed on cluster is correct
func (o *OperatorACL) CheckSiteLicense(key oss.SiteKey) error {
	// the "update" permission is required here because license check may deactivate the site
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CheckSiteLicense(key)
}

// UpdateLicense updates license installed on cluster and runs a respective app hook
func (o *OperatorACL) UpdateLicense(req ops.UpdateLicenseRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateLicense(req)
}

// GetLicenseCA returns CA certificate Ops Center uses to sign licenses
func (o *OperatorACL) GetLicenseCA() ([]byte, error) {
	return o.operator.GetLicenseCA()
}

// UpsertRole creates a new role
func (o *OperatorACL) UpsertRole(key oss.SiteKey, role teleservices.Role) error {
	if err := o.roleActions(teleservices.VerbCreate, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	if role.GetMetadata().Labels[constants.SystemLabel] == constants.True {
		return trace.AccessDenied("system roles can't be created")
	}
	return o.operator.UpsertRole(key, role)
}

// GetRole returns a role by name
func (o *OperatorACL) GetRole(key oss.SiteKey, name string) (teleservices.Role, error) {
	if err := o.roleActions(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetRole(key, name)
}

// GetRoles returns all roles
func (o *OperatorACL) GetRoles(key oss.SiteKey) ([]teleservices.Role, error) {
	if err := o.roleActions(teleservices.VerbList, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetRoles(key)
}

// DeleteRole deletes a role by name
func (o *OperatorACL) DeleteRole(key oss.SiteKey, name string) error {
	if err := o.roleActions(teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	role, err := o.operator.GetRole(key, name)
	if err != nil {
		return trace.Wrap(err)
	}
	if role.GetMetadata().Labels[constants.SystemLabel] == constants.True {
		return trace.AccessDenied("system roles can't be deleted")
	}
	return o.operator.DeleteRole(key, name)
}

// UpsertOIDCConnector creates or updates an OIDC connector
func (o *OperatorACL) UpsertOIDCConnector(key oss.SiteKey, connector teleservices.OIDCConnector) error {
	if err := o.AuthConnectorActions(teleservices.KindOIDCConnector, teleservices.VerbCreate, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpsertOIDCConnector(key, connector)
}

// GetOIDCConnector returns an OIDC connector by name
//
// Returned connector exclude client secret unless withSecrets is true.
func (o *OperatorACL) GetOIDCConnector(key oss.SiteKey, name string, withSecrets bool) (teleservices.OIDCConnector, error) {
	if err := o.AuthConnectorActions(teleservices.KindOIDCConnector, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetOIDCConnector(key, name, withSecrets)
}

// GetOIDCConnectors returns all OIDC connectors
//
// Returned connectors exclude client secret unless withSecrets is true.
func (o *OperatorACL) GetOIDCConnectors(key oss.SiteKey, withSecrets bool) ([]teleservices.OIDCConnector, error) {
	if err := o.AuthConnectorActions(teleservices.KindOIDCConnector, teleservices.VerbList, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetOIDCConnectors(key, withSecrets)
}

// DeleteOIDCConnector deletes an OIDC connector by name
func (o *OperatorACL) DeleteOIDCConnector(key oss.SiteKey, name string) error {
	if err := o.AuthConnectorActions(teleservices.KindOIDCConnector, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteOIDCConnector(key, name)
}

// UpsertSAMLConnector creates or updates a SAML connector
func (o *OperatorACL) UpsertSAMLConnector(key oss.SiteKey, connector teleservices.SAMLConnector) error {
	if err := o.AuthConnectorActions(teleservices.KindSAMLConnector, teleservices.VerbCreate, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpsertSAMLConnector(key, connector)
}

// GetSAMLConnector returns a SAML connector by name
//
// Returned connector excludes private signing key unless withSecrets is true.
func (o *OperatorACL) GetSAMLConnector(key oss.SiteKey, name string, withSecrets bool) (teleservices.SAMLConnector, error) {
	if err := o.AuthConnectorActions(teleservices.KindSAMLConnector, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSAMLConnector(key, name, withSecrets)
}

// GetSAMLConnectors returns all SAML connectors
//
// Returned connectors exclude private signing keys unless withSecrets is true.
func (o *OperatorACL) GetSAMLConnectors(key oss.SiteKey, withSecrets bool) ([]teleservices.SAMLConnector, error) {
	if err := o.AuthConnectorActions(teleservices.KindSAMLConnector, teleservices.VerbList, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSAMLConnectors(key, withSecrets)
}

// DeleteSAMLConnector deletes a SAML connector by name
func (o *OperatorACL) DeleteSAMLConnector(key oss.SiteKey, name string) error {
	if err := o.AuthConnectorActions(teleservices.KindSAMLConnector, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteSAMLConnector(key, name)
}

// roleActions checks access to the specified actions on the "role" resource
func (o *OperatorACL) roleActions(actions ...string) error {
	for _, action := range actions {
		if err := o.Action(teleservices.KindRole, action); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
