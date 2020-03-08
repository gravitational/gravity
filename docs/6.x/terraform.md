# Terraform Provider (OSS)

The Gravity terraform provider is used to support terraform management of open-source Gravity clusters. The provider needs to be configured with a valid token in order to manage a cluster.

## Getting Started

### Install the Gravity provider
The terraform provider will be automatically installed when getting the Gravity tools.

```bsh
curl https://get.gravitational.io/gravity/install/5.2.5 | bash
```

Please see the [getting started guide](quickstart.md#getting-the-tools) for more information.

### Example Usage

```bsh
# Configure the Gravity provider
provider "gravity" {
    host  = "https://example.com"
    token = "abcdefghi"
}

# Create a log forwarder
resource "gravity_log_forwarder" "logs" {
    # ...
}
```

### Authentication
The terraform provider uses token based authentication which must be provisioned to the cluster before being used.

See [Configuring Users & Tokens](config.md#configuring-users-tokens) for more information

## gravity_cluster_auth_preference
Configures authentication preferences for authenticating users on the cluster.

### Example Usage
```bsh
resource "gravity_cluster_auth_preference" "test" {
    type = "local"
    second_factor = "otp"
    connector_name = "test"
}
```

### Argument Reference
The following arguments are supported:

* `type` - Which type of identity provider to use.
    - local: Use local database for users / accounts.
    - oidc: Use OpenID Connect service as an identity provider.
    - saml: Use SAML service as an identity provider.
    - github: Use GitHub as an identity provider.
* `second_factor` - Whether to enable second factor authentication when using local identity provider.
    - off: Second factor authentication is disabled on the cluster.
    - otp: Use TOTP based second factor authentication.
    - u2f: Use U2F based second factor authentication.
* `connector_name` - (Optional) The name of the OIDC or SAML connector to use for providing identity. If left blank, the first provider will be used.
* `u2f_appid` - (Optional) The application ID of the cluster. See [the teleport documents](https://gravitational.com/teleport/docs/admin-guide/#fido-u2f) for more information.
* `u2f_facets` - (Optional) A list of facets for U2F authentication. See [the teleport documents](https://gravitational.com/teleport/docs/admin-guide/#fido-u2f) for more information.

## gravity_github
Configures the cluster to allow authentication using GitHub as an identity provider.

### Example Usage
```bsh
resource "gravity_github" "test" {
  name          = "github"
  display       = "Github"
  client_id     = "<client-id>"
  client_secret = "<client-secret>"
  redirect_url  = "https://<cluster-url>/portalapi/v1/github/callback"

  teams_to_logins {
    organization = "example"
    team         = "admins"
    logins       = ["@teleadmin"]
  }
}
```

### Argument Reference
The following arguments are supported:

* `name` - The name of the connector. This name must be unique.
* `client_id` - GitHub OAuth app client ID to use.
* `client_secret` - GitHub OAuth app client secret.
* `redirect_url` - URL that the cluster can be reached at for OAuth callback.
* `display` - Human readable display name that will be presented to users on the web interface when logging in.
* `teams_to_logins` - One or more maps of organization/team/logins to allow on the cluster.
    - organization - The GitHub organization a user belongs to.
    - team - The team within the organization that the user belongs to.
    - logins - A list of allowed logins for this organization/team on the cluster.

## gravity_log_forwarder
Configure log forwarding to an external syslog server.

### Example Usage
```bsh
resource "gravity_log_forwarder" "test" {
  name     = "logzer"
  address  = "192.168.1.1:514"
  protocol = "udp"
}
```

### Argument Reference
The following arguments are supported:

* `name` - A name to use for the forwarder.
* `address` - The IP address or hostname and port to send logs to in the format `<host>:<port>`.
* `protocol` - Which transport protocol to use for log forwarding.
    - tcp - Use TCP transport.
    - udp - Use UDP transport.

## gravity_tlskeypair
Apply a TLS Certificate and Key to the cluster to be used for the Web UI and API of the cluster.

### Example Usage
```bsh
resource "gravity_tlskeypair" "test" {
  cert = <<EOF
-----BEGIN CERTIFICATE-----
# ...
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
# ...
-----END CERTIFICATE-----
EOF

  private_key = <<EOF
-----BEGIN PRIVATE KEY-----
# ...
-----END PRIVATE KEY-----
EOF
}
```

### Argument Reference
The following arguments are supported:

* `cert` - The certificate chain in PEM format, with the full trust chain.
* `private_key` - The private key which matches the certificate in PEM format.

## gravity_token
A token is a static secret that can be used to login to a cluster as a user.

### Example Usage
```bsh
resource "random_id" "secret_admin_token" {
  byte_length = 32
}

resource "gravity_token" "test" {
  token = "${random_id.secret_admin_token.hex}"
  user  = "adminagent@example.com"
}
```

### Argument Reference
The following arguments are supported:

* `token` - A secret token that can be used to access the cluster.
* `user` - The user the token is for.

## gravity_user
A local cluster user

### Example Usage
```bsh
resource "random_id" "admin_password" {
  byte_length = 32
}

resource "gravity_user" "test" {
  name      = "test"
  full_name = "Test User"
  type      = "admin"
  roles     = ["@teleadmin"]
  password  = "${random_id.admin_password.hex}"
}
```

### Argument Reference
The following arguments are supported:

* `name` - An email address style username.
* `full_name` - (Optional) A friendly name for the user.
* `password` - (Optional) A password to provision for the user.
* `roles` - A customized list of roles.


# Terraform Provider (Enterprise)
The Gravity enterprise terraform provider is used to support terraform management of resources only available in the enterprise version of Gravity. This provider should be used in conjunction with the open-source Gravity provider to manage a Gravity cluster.

## Getting Started

### Install the Gravity Enterprise provider
The terraform provider will be automatically installed when getting the Gravity tools.

```bsh
curl https://get.gravitational.io/gravity/install/5.2.5 | bash
```

Please see the [getting started guide](quickstart.md#getting-the-tools) for more information.

### Example Usage

```bsh
# Configure the Gravity provider
provider "gravity" {
    host  = "https://example.com"
    token = "abcdefghi"
}

# Create a log forwarder
resource "gravity_log_forwarder" "logs" {
    # ...
}

# Configure the gravity enterprise provider
provider "gravityenterprise" {
    host  = "https://example.com"
    token = "abcdefghi"
}

# Create an oidc connector
resource "gravityenterprise_oidc" "test" {
    # ...
}
```

### Authentication
The terraform provider uses token based authentication which must be provisioned to the cluster before being used.

See [Configuring Users & Tokens](config.md#configuring-users-tokens) for more information.


## gravityenterprise_endpoints
By default an Ops Center is configured with a single endpoint set via `--ops-advertise-addr` flag passed during installation. This configuration allows creating separate endpoints for cluster management and inter-cluster communications that can be firewalled separately.

### Example Usage
```bsh
resource "gravityenterprise_endpoints" "test" {
  public_advertise_addr = "public.example.com:443"
  agents_advertise_addr = "agents.example.com:443"
}
```

### Argument Reference
The following arguments are supported:

* `public_advertise_addr` - Endpoint used by the Ops Center UI and CLI tools such as `tele` and `tsh`.
* `agents_advertise_addr` - (Optional) Endpoint used by remote clusters to connect to Ops Center as a trusted cluster.

## gravityenterprise_oidc
A Gravity enterprise cluster can be configured to use Open ID connect as an identity provider and authenticate users.

### Example Usage
```bsh
resource "gravityenterprise_oidc" "test" {
  name = "auth0"
  redirect_url = "https://example.com/portalapi/v1/oidc/callback"
  client_id = "1234"
  client_secret = "5678"
  issuer_url = "https://example.auth0.com/"
  scope = ["roles"]

  claims_to_roles {
      claim = "roles"
      value = "admins"
      roles = ["@teleadmin]
  }
}
```

### Argument Reference
The following arguments are supported:

* `name` - The name of the connector.
* `display` - (Optional) The display name of the connector as shown in the UI.
* `redirect_url` - The URL on the Gravity cluster that will process the OpenID callback. Should be in the format https://<cluster-hostname>/portalapi/v1/oidc/callback.
* `acr` - Authentication Context Class Reference value. The meaning of the ACR value is context-specific and varies for identity providers.
* `identity_provider` - (Optional)
* `client_id` - Is the client-id used by the identity provider.
* `client_secret` - Is the secret used to authenticate with the identity provider.
* `issuer_url` - Is the URL of the identity provider to direct users to.
* `scope` - Is a list of additional scopes set by the provider.
* `claims_to_roles` - Is a dictionary of claim to role mappings. Can be passed multiple times.
    - `claim` - OIDC claim name.
    - `value` - OIDC claim value to match.
    - `roles` - (Optional) A list of static roles to assign the user on login.

## gravityenterprise_role
Roles can be used to tune access permissions to the cluster.

### Example Usage
Admin access to all resources:
```bsh
resource "gravityenterprise_role" "admin" {
  name = "administrator"

  allow {
    logins = ["root"]
    namespaces = ["default"]
    node_labels = {
      "*"= "*"
    }
    rule {
      resources = ["*"]
      verbs = ["*"]
    }
  }
}
```

See [the teleport documents](https://gravitational.com/teleport/docs/ssh_adfs/#create-teleport-roles) for more information.

### Argument Reference
The following arguments are supported:

* `name` - The name of the role.
* `max_ttl` - The maximum TTL of a login session through this role. See https://golang.org/pkg/time/#ParseDuration for the format. Default: "24h0m0s".
* `port_forwarding` - Enable port forwarding for `tsh` sessions. Default: false
* `forward_agent` - Enable ssh agent forwarding for `tsh` sessions. Default: false
* `allow` - Map of allowed conditions for this role.
    - `logins` - List of logins that the user can login as.
    - `node_labels` - Map of key=value pairs that identify nodes user is able to access via `tsh`.
    - `rule` - A map of rules to apply to the condition. Multiple rules can be created.
        - `resources` - A list of resources that this rule applies to:
            - cluster_auth_preference - type of authentication for this cluster.
            - github - Github OAuth2 connector resource.
            - user - user resource.
            - token - provisioning token resource.
            - logforwarder - log forwarder resource.
            - smtp - the monitoring SMTP configuration resource.
            - alert - the monitoring alert resource.
            - alterttarget - the monitoring alert target resource.
            - tlskeypair - TLS key pair resource.
            - role - role resource.
            - oidc - OIDC connector resource.
            - saml - SAML connector resource
            - trusted_cluster - resource that contains trusted cluster configuration.
        - `verbs` - List of operation verbs can be used. Valid options are:
            - register - allow registering of new clusters within an Ops Center
            - connect - allow users to connect to clusters
            - readsecrets - allow reading of secrets
            - list - Ability to list all objects. Does not imply the ability to read a single object.
            - create - Can create an object.
            - read - Can read a single object.
            - readnosecrets - Can read a single object without secrets.
            - update - Can update a single object.
            - delete - Can remove an object.
        - `where` -
        - `actions` - A list of optional actions taken when a rule matches. Valid options are:
            - log - emits an entry when specified rule matches.
            - assignKubernetesGroups - assigns specified kubernetes groups to the role.
* `deny` - Map of deny conditions for this role.
    - `logins` - List of logins that the user can login as.
    - `node_labels` - Map of key=value pairs that identify nodes user is able to access via `tsh`.
    - `rule` - A map of rules to apply to the condition. Multiple rules can be created.
        - `resources` - A list of resources that this rule applies to:
            - cluster_auth_preference - type of authentication for this cluster.
            - github - Github OAuth2 connector resource.
            - user - user resource.
            - token - provisioning token resource.
            - logforwarder - log forwarder resource.
            - smtp - the monitoring SMTP configuration resource.
            - alert - the monitoring alert resource.
            - alterttarget - the monitoring alert target resource.
            - tlskeypair - TLS key pair resource.
            - role - role resource.
            - oidc - OIDC connector resource.
            - saml - SAML connector resource
            - trusted_cluster - resource that contains trusted cluster configuration.
        - `verbs` - List of operation verbs can be used. Valid options are:
            - register - allow registering of new clusters within an Ops Center
            - connect - allow users to connect to clusters
            - readsecrets - allow reading of secrets
            - list - Ability to list all objects. Does not imply the ability to read a single object.
            - create - Can create an object.
            - read - Can read a single object.
            - readnosecrets - Can read a single object without secrets.
            - update - Can update a single object.
            - delete - Can remove an object.
        - `where` -
        - `actions` - A list of optional actions taken when a rule matches. Valid options are:
            - log - emits an entry when specified rule matches.
            - assignKubernetesGroups - assigns specified kubernetes groups to the role.

## gravityenterprise_saml
Enables using SAML as an identity provider for cluster logins.

### Example Usage
```bsh
resource "gravityenterprise_saml" "test" {
  name = "saml"
  display = "SAML Example"
  acs = "https://example.com/portalapi/v1/saml/callback"

  attributes_to_role {
    name = "groups"
    value = "admins"
    roles = ["@teleadmin"]
  }
}
```

### Argument Reference
The following arguments are supported:

* `name` - The name of the connector.
* `display` - (Optional) The name of the connector as shown in the Web UI.
* `issuer` - A unique name (usually a URL) that the identity provider uses for SAML 2.0
* `sso` - URL of the identity provider SSO service.
* `cert` - The identity provider certificate.
* `acs` - The callback URL on the Gravity cluster. Format https://<host>/portalapi/v1/saml/callback.
* `audience` - (Optional) Uniquely identifies our service provider.
* `service_provider_issuer` - (Optional) is the issuer of the service provider
* `entity_descriptor` - (Optional) - Inline entity descriptor XML configuration.
* `entity_descriptor_url` - (Optional) Fetch entity descriptor XML from the provided URL.
* `atributes_to_roles` -  Is a dictionary of claim to role mappings. Can be passed multiple times.
    - `name` - claim name.
    - `value` - claim value to match.
    - `roles` - (Optional) A list of static roles to assign the user on login.
* `signing_key` - (Optional) Cert used to sign AuthnRequest
* `signing_cert` - (Optional) Key used to sign AuthnRequest
* `identity_provider` - (Optional) is the external identity provider.

## gravityenterprise_trusted_cluster
Trusted clusters allows connecting a standalone Gravity enterprise cluster to an Ops Center.

### Example Usage
```bsh
resource "gravityenterprise_trusted_cluster" "test" {
  name = "test"
  token = "abcdef"
  web_proxy_addr = "1.1.1.1"
  tunnel_addr = "1.1.1.1"
}
```

### Argument Reference
The following arguments are supported:

* `name` - The name of the trusted cluster connection.
* `enabled` - Boolean, allows the connection to be disabled without deleting the connection.
* `pull_updates` - Whether the cluster should automatically download application updates from the Ops Center.
* `token` - A secret token used to securely connect the cluster to the Ops Center. Can be retrieved by running  `gravity status` command on the Ops Center cluster.
* `tunnel_addr` - The address of the Ops Center reverse tunnel service as host:port. Typically exposed on port 3024.
* `web_proxy_addr` - The address which the Ops Center cluster serves its web API on.
