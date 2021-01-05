package service

import (
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/teleport/lib/services"
)

// UpsertRole creates a new role
func (o *Operator) UpsertRole(key ops.SiteKey, role services.Role) error {
	return o.users().UpsertRole(role, 0)
}

// GetRole returns a role by name
func (o *Operator) GetRole(key ops.SiteKey, name string) (services.Role, error) {
	return o.users().GetRole(name)
}

// GetRoles returns all roles
func (o *Operator) GetRoles(key ops.SiteKey) ([]services.Role, error) {
	return o.users().GetRoles()
}

// DeleteRole deletes a role by name
func (o *Operator) DeleteRole(key ops.SiteKey, name string) error {
	return o.users().DeleteRole(name)
}

// UpsertOIDCConnector creates or updates an OIDC connector
func (o *Operator) UpsertOIDCConnector(key ops.SiteKey, connector services.OIDCConnector) error {
	return o.users().UpsertOIDCConnector(connector)
}

// GetOIDCConnector returns an OIDC connector by name
//
// Returned connector exclude client secret unless withSecrets is true.
func (o *Operator) GetOIDCConnector(key ops.SiteKey, name string, withSecrets bool) (services.OIDCConnector, error) {
	return o.users().GetOIDCConnector(name, withSecrets)
}

// GetOIDCConnectors returns all OIDC connectors
//
// Returned connectors exclude client secret unless withSecrets is true.
func (o *Operator) GetOIDCConnectors(key ops.SiteKey, withSecrets bool) ([]services.OIDCConnector, error) {
	return o.users().GetOIDCConnectors(withSecrets)
}

// DeleteOIDCConnector deletes an OIDC connector by name
func (o *Operator) DeleteOIDCConnector(key ops.SiteKey, name string) error {
	return o.users().DeleteOIDCConnector(name)
}

// UpsertSAMLConnector creates or updates a SAML connector
func (o *Operator) UpsertSAMLConnector(key ops.SiteKey, connector services.SAMLConnector) error {
	return o.users().UpsertSAMLConnector(connector)
}

// GetSAMLConnector returns a SAML connector by name
//
// Returned connector excludes private signing key unless withSecrets is true.
func (o *Operator) GetSAMLConnector(key ops.SiteKey, name string, withSecrets bool) (services.SAMLConnector, error) {
	return o.users().GetSAMLConnector(name, withSecrets)
}

// GetSAMLConnectors returns all SAML connectors
//
// Returned connectors exclude private signing keys unless withSecrets is true.
func (o *Operator) GetSAMLConnectors(key ops.SiteKey, withSecrets bool) ([]services.SAMLConnector, error) {
	return o.users().GetSAMLConnectors(withSecrets)
}

// DeleteSAMLConnector deletes a SAML connector by name
func (o *Operator) DeleteSAMLConnector(key ops.SiteKey, name string) error {
	return o.users().DeleteSAMLConnector(name)
}
