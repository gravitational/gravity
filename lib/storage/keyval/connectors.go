package keyval

import (
	"time"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

func (b *backend) UpsertOIDCConnector(connector teleservices.OIDCConnector) error {
	if err := connector.Check(); err != nil {
		return trace.Wrap(err)
	}
	data, err := teleservices.GetOIDCConnectorMarshaler().MarshalOIDCConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(connectorsP, connector.GetName()), data, b.ttl(connector.Expiry()))
	return trace.Wrap(err)
}

// DeleteOIDCConnector deletes OIDC Connector
func (b *backend) DeleteOIDCConnector(connectorID string) error {
	err := b.deleteKey(b.key(connectorsP, connectorID))
	return trace.Wrap(err)
}

// GetOIDCConnector returns OIDC connector data, , withSecrets adds or removes client secret from return results
func (b *backend) GetOIDCConnector(connectorID string, withSecrets bool) (teleservices.OIDCConnector, error) {
	data, err := b.getValBytes(b.key(connectorsP, connectorID))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("connector(%v) not found", connectorID)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := teleservices.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		conn.SetClientSecret("")
	}
	return conn, nil
}

// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
func (b *backend) GetOIDCConnectors(withSecrets bool) ([]teleservices.OIDCConnector, error) {
	ids, err := b.getKeys(b.key(connectorsP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := []teleservices.OIDCConnector{}
	for _, id := range ids {
		conn, err := b.GetOIDCConnector(id, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, conn)
	}
	return out, nil
}

// CreateOIDCAuthRequest creates new auth request
func (b *backend) CreateOIDCAuthRequest(req teleservices.OIDCAuthRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	if _, err := b.GetOIDCConnector(req.ConnectorID, false); err != nil {
		return trace.Wrap(err)
	}
	err := b.createVal(b.key(authRequestsP, req.StateToken), req, forever)
	return trace.Wrap(err)
}

// GetOIDCAuthRequest returns OIDC auth request if found
func (b *backend) GetOIDCAuthRequest(stateToken string) (*teleservices.OIDCAuthRequest, error) {
	var req teleservices.OIDCAuthRequest
	err := b.getVal(b.key(authRequestsP, stateToken), &req)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("OIDC Auth request %v is not found", stateToken)
		}
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// CreateSAMLConnector creates SAML Connector
func (b *backend) CreateSAMLConnector(connector teleservices.SAMLConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	data, err := teleservices.GetSAMLConnectorMarshaler().MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.createValBytes(b.key(authP, connectorsP, samlP, connector.GetName()), data, b.ttl(connector.Expiry()))
	return trace.Wrap(err)
}

// UpsertSAMLConnector upserts SAML Connector
func (b *backend) UpsertSAMLConnector(connector teleservices.SAMLConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	data, err := teleservices.GetSAMLConnectorMarshaler().MarshalSAMLConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(authP, connectorsP, samlP, connector.GetName()), data, b.ttl(connector.Expiry()))
	return trace.Wrap(err)
}

// DeleteSAMLConnector deletes SAML Connector
func (b *backend) DeleteSAMLConnector(connectorID string) error {
	err := b.deleteKey(b.key(authP, connectorsP, samlP, connectorID))
	return trace.Wrap(err)
}

// GetSAMLConnector returns SAML connector data, withSecrets adds or removes secrets from return results
func (b *backend) GetSAMLConnector(id string, withSecrets bool) (teleservices.SAMLConnector, error) {
	if id == "" {
		return nil, trace.BadParameter("missing parameter ID for SAML connector")
	}
	data, err := b.getValBytes(b.key(authP, connectorsP, samlP, id))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("SAML connector %v is not found", id)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := teleservices.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		keyPair := conn.GetSigningKeyPair()
		if keyPair != nil {
			keyPair.PrivateKey = ""
			conn.SetSigningKeyPair(keyPair)
		}
	}
	return conn, nil
}

// GetSAMLConnectors returns registered connectors, withSecrets adds or removes secret from return results
func (b *backend) GetSAMLConnectors(withSecrets bool) ([]teleservices.SAMLConnector, error) {
	ids, err := b.getKeys(b.key(authP, connectorsP, samlP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := []teleservices.SAMLConnector{}
	for _, id := range ids {
		conn, err := b.GetSAMLConnector(id, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, conn)
	}
	return out, nil
}

// CreateSAMLAuthRequest creates new auth request
func (b *backend) CreateSAMLAuthRequest(req teleservices.SAMLAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	if _, err := b.GetSAMLConnector(req.ConnectorID, false); err != nil {
		return trace.Wrap(err)
	}
	err := b.createVal(b.key(authP, authRequestsP, samlP, req.ID), req, forever)
	return trace.Wrap(err)
}

// GetSAMLAuthRequest returns SAML auth request if found
func (b *backend) GetSAMLAuthRequest(id string) (*teleservices.SAMLAuthRequest, error) {
	if id == "" {
		return nil, trace.BadParameter("SAML Auth request id is empty")
	}
	var req teleservices.SAMLAuthRequest
	err := b.getVal(b.key(authP, authRequestsP, samlP, id), &req)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("SAML Auth request %v is not found", id)
		}
		return nil, trace.Wrap(err)
	}
	return &req, nil
}

// CreateGithubConnector creates a new Github connector
func (b *backend) CreateGithubConnector(connector teleservices.GithubConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	data, err := teleservices.GetGithubConnectorMarshaler().Marshal(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.createValBytes(b.key(authP, connectorsP, githubP,
		connector.GetName()), data, b.ttl(connector.Expiry()))
	return trace.Wrap(err)
}

// UpsertGithubConnector creates or updates a new Github connector
func (b *backend) UpsertGithubConnector(connector teleservices.GithubConnector) error {
	if err := connector.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	data, err := teleservices.GetGithubConnectorMarshaler().Marshal(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(authP, connectorsP, githubP,
		connector.GetName()), data, b.ttl(connector.Expiry()))
	return trace.Wrap(err)
}

// GetGithubConnectors returns all configured Github connectors
func (b *backend) GetGithubConnectors(withSecrets bool) ([]teleservices.GithubConnector, error) {
	ids, err := b.getKeys(b.key(authP, connectorsP, githubP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []teleservices.GithubConnector
	for _, id := range ids {
		conn, err := b.GetGithubConnector(id, withSecrets)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, conn)
	}
	return out, nil
}

// GetGithubConnector returns a Github connector by its name
func (b *backend) GetGithubConnector(name string, withSecrets bool) (teleservices.GithubConnector, error) {
	if name == "" {
		return nil, trace.BadParameter("missing Github connector name")
	}
	data, err := b.getValBytes(b.key(authP, connectorsP, githubP, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("Github connector %v is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := teleservices.GetGithubConnectorMarshaler().Unmarshal(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !withSecrets {
		conn.SetClientSecret("")
	}
	return conn, nil
}

// DeleteGithubConnector deletes a Github connector by its name
func (b *backend) DeleteGithubConnector(name string) error {
	err := b.deleteKey(b.key(authP, connectorsP, githubP, name))
	return trace.Wrap(err)
}

// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
func (b *backend) CreateGithubAuthRequest(req teleservices.GithubAuthRequest, ttl time.Duration) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	if _, err := b.GetGithubConnector(req.ConnectorID, false); err != nil {
		return trace.Wrap(err)
	}
	err := b.createVal(b.key(authP, authRequestsP, githubP, req.StateToken), req, forever)
	return trace.Wrap(err)
}

// GetGithubAuthRequest retrieves Github auth request by the token
func (b *backend) GetGithubAuthRequest(stateToken string) (*teleservices.GithubAuthRequest, error) {
	if stateToken == "" {
		return nil, trace.BadParameter("Github auth request token is empty")
	}
	var req teleservices.GithubAuthRequest
	err := b.getVal(b.key(authP, authRequestsP, githubP, stateToken), &req)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("Github auth request %v not found", stateToken)
		}
		return nil, trace.Wrap(err)
	}
	return &req, nil
}
