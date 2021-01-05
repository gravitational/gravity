package service

import (
	"context"

	"github.com/gravitational/gravity/e/lib/events"
	"github.com/gravitational/gravity/lib/ops"
	libevents "github.com/gravitational/gravity/lib/ops/events"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// UpsertRole creates a new role
func (o *Operator) UpsertRole(ctx context.Context, key ops.SiteKey, role services.Role) error {
	if err := o.users().UpsertRole(role, 0); err != nil {
		return trace.Wrap(err)
	}
	libevents.Emit(ctx, o, events.RoleCreated, libevents.Fields{
		libevents.FieldName: role.GetName(),
	})
	return nil
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
func (o *Operator) DeleteRole(ctx context.Context, key ops.SiteKey, name string) error {
	if err := o.users().DeleteRole(name); err != nil {
		return trace.Wrap(err)
	}
	libevents.Emit(ctx, o, events.RoleDeleted, libevents.Fields{
		libevents.FieldName: name,
	})
	return nil
}

// UpsertOIDCConnector creates or updates an OIDC connector
func (o *Operator) UpsertOIDCConnector(ctx context.Context, key ops.SiteKey, connector services.OIDCConnector) error {
	if err := o.users().UpsertOIDCConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	libevents.Emit(ctx, o, events.OIDCConnectorCreated, libevents.Fields{
		libevents.FieldName: connector.GetName(),
	})
	return nil
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
func (o *Operator) DeleteOIDCConnector(ctx context.Context, key ops.SiteKey, name string) error {
	if err := o.users().DeleteOIDCConnector(name); err != nil {
		return trace.Wrap(err)
	}
	libevents.Emit(ctx, o, events.OIDCConnectorDeleted, libevents.Fields{
		libevents.FieldName: name,
	})
	return nil
}

// UpsertSAMLConnector creates or updates a SAML connector
func (o *Operator) UpsertSAMLConnector(ctx context.Context, key ops.SiteKey, connector services.SAMLConnector) error {
	if err := o.users().UpsertSAMLConnector(connector); err != nil {
		return trace.Wrap(err)
	}
	libevents.Emit(ctx, o, events.SAMLConnectorCreated, libevents.Fields{
		libevents.FieldName: connector.GetName(),
	})
	return nil
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
func (o *Operator) DeleteSAMLConnector(ctx context.Context, key ops.SiteKey, name string) error {
	if err := o.users().DeleteSAMLConnector(name); err != nil {
		return trace.Wrap(err)
	}
	libevents.Emit(ctx, o, events.SAMLConnectorDeleted, libevents.Fields{
		libevents.FieldName: name,
	})
	return nil
}
