# Terraform Provider (OSS)

The gravity terraform provider is used to support terraform management of opensource gravity clusters. The provider needs to be configured with a valid token in order to manage a cluster.

## Getting Started

### Install the Gravity provider
TODO(knisbet)

### Example Usage

```
# Configure the Gravity provider
provider "gravity" {
    host  = "https://example.com"
    token = "abcdefghi"
}

# Create a log forwarder
resource "gravity_log_forwarder" "logs" {
    name     = "logs"
    address  = "192.168.1.1"
    protocol = "udp"
}
```

### Authentication
The terraform provider uses token based authentication which must be provisioned to the cluster before being used. 

See TODO(knisbet)

## gravity_cluster_auth_preference
Configures authentication preferences for authenticating users on the cluster.

### Example Usage
```
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
    - github: Use github as an identity provider.
* `second_factor` - Whether to enable second factor authentication when using local identity provider.
    - off: Second factor authentication is disabled on the cluster.
    - otp: Use TOTP based second factor authentication.
    - u2f: Use U2F based second factor authentication.
* `connector_name` - (Optional) The name of the OIDC or SAML connector to use for providing identity. If left blank, the first provider will be used.
* `u2f_appid` - (Optional) TODO(knisbet)
* `u2f_facets` - (Optional) A list of TODO(knisbet)

## gravity_github
Configures the cluster to allow authentication using github as an identity provider.

### Example Usage
```
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
* `client_id` - Github OAuth app client ID to use.
* `client_secret` - Github OAuth app client secret.
* `redirect_url` - URL that the cluster can be reached at for OAuth callback.
* `display` - Human readable display name that will be presented to users on the web interface when logging in.
* `teams_to_logins` - One or more maps of organization/team/logins to allow on the cluster.
    - organization - The github organization a user belongs to.
    - team - The team within the organization that the user belongs to.
    - logins - A list of allowed logins for this organization/team on the cluster.

## gravity_log_forwarder
Configure log forwarding to an external syslog server.

### Example Usage
```
resource "gravity_log_forwarder" "test" {
  name     = "logzer"
  address  = "192.168.1.1"
  protocol = "udp"
}
```

### Argument Reference
The following arguments are supported:

* `name` - A name to use for the forwarder.
* `address` - The IP address or hostname of the logging destination.
* `protocol` - Which transpor protocol to use for log forwarding.
    - tcp - Use TCP transport.
    - udp - Use UDP transport.

## gravity_tlskeypair
Apply a TLS Certificate and Key to the cluster to be used for the Web UI and API of the cluster.

### Example Usage
```
resource "gravity_tlskeypair" "test" {
  cert = <<EOF
-----BEGIN CERTIFICATE-----
MIIGGDCCBQCgAwIBAgISA/wgCy4eFgnVgvgm4TGN4CCwMA0GCSqGSIb3DQEBCwUA
MEoxCzAJBgNVBAYTAlVTMRYwFAYDVQQKEw1MZXQncyBFbmNyeXB0MSMwIQYDVQQD
ExpMZXQncyBFbmNyeXB0IEF1dGhvcml0eSBYMzAeFw0xODA3MjQxNzUwMDRaFw0x
ODEwMjIxNzUwMDRaMCIxIDAeBgNVBAMTF3RmLW9wcy5ncmF2aXRhdGlvbmFsLmlv
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEApGebZgLdFzn2vy0ayewg
GS1rE85bZK/LCABkOSX8cqzTrqCM0m86KctgJDO/WskgeTUrAfPgASHmtsr1CDC2
pp+kyzQ8dV7LrI8dUDL7Mr1zD/zxiCupgw1XVvZSIPesjJRo/wAO60p3CxDeqXan
VxYhn6r41m03oilCqGMQ1LClVUqQ917P6SWnX32CeTFRZ6qeH9ATIBHP5mi5Nxtk
zjRRczT/5nuizkMp9GY5ruj/glJRTWoZIbWbBUDABEHJBIgW3kvfNHGPD6L9RExG
Mvu8KugGeeVnkk20qLak5v44FLPbMiJ2v+fJ76D4qGusVbIxWN9sRwvL8l3a/swh
7QIDAQABo4IDHjCCAxowDgYDVR0PAQH/BAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUF
BwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMB0GA1UdDgQWBBTeXA1WM6bL/7eV
NRMk8dSvjySBYDAfBgNVHSMEGDAWgBSoSmpjBH3duubRObemRWXv86jsoTBvBggr
BgEFBQcBAQRjMGEwLgYIKwYBBQUHMAGGImh0dHA6Ly9vY3NwLmludC14My5sZXRz
ZW5jcnlwdC5vcmcwLwYIKwYBBQUHMAKGI2h0dHA6Ly9jZXJ0LmludC14My5sZXRz
ZW5jcnlwdC5vcmcvMCIGA1UdEQQbMBmCF3RmLW9wcy5ncmF2aXRhdGlvbmFsLmlv
MIH+BgNVHSAEgfYwgfMwCAYGZ4EMAQIBMIHmBgsrBgEEAYLfEwEBATCB1jAmBggr
BgEFBQcCARYaaHR0cDovL2Nwcy5sZXRzZW5jcnlwdC5vcmcwgasGCCsGAQUFBwIC
MIGeDIGbVGhpcyBDZXJ0aWZpY2F0ZSBtYXkgb25seSBiZSByZWxpZWQgdXBvbiBi
eSBSZWx5aW5nIFBhcnRpZXMgYW5kIG9ubHkgaW4gYWNjb3JkYW5jZSB3aXRoIHRo
ZSBDZXJ0aWZpY2F0ZSBQb2xpY3kgZm91bmQgYXQgaHR0cHM6Ly9sZXRzZW5jcnlw
dC5vcmcvcmVwb3NpdG9yeS8wggEDBgorBgEEAdZ5AgQCBIH0BIHxAO8AdQBVgdTC
FpA2AUrqC5tXPFPwwOQ4eHAlCBcvo6odBxPTDAAAAWTNoH93AAAEAwBGMEQCIQD4
uMK6szIF920mXm3ZUeiV9Bgq1+bAePdqB/gNsU2qNgIfGmmY9GOw4I3iQ1b5R596
eXaDRUxC4tNhO8jcLYPZzgB2ACk8UZZUyDlluqpQ/FgH1Ldvv1h6KXLcpMMM9OVF
R/R4AAABZM2gf2QAAAQDAEcwRQIhAO7xwfNvY/BynMndpEpGYWTnO7P+Yg4oBG3C
gwXyrATPAiA8/YbkJFmAQCpdQmwkkQs1F+e0KtCgSMtAi3nv+/+vCDANBgkqhkiG
9w0BAQsFAAOCAQEAhRJ6H4wrhsB7uIGXttfIYYKKE459ZuW36r6ZiOrUMTG6GkGV
5Dx/iQjIaPbi3TwnlrbngGyhdhUGbw7Q9k9qaZgHntSQTmmz939VF7aLiZV+IkeO
bN5+z/9Fvsb45/5UNNARrIzyyo7DFJFIw31ve0Ya6mHi1m4wMQ7MPqgqz/s3+UVa
7p6QbOn3RGt3H5mL2BHT6LpTlKt6U8z6JKuF1PsjUbrJkGRWGwwUpwXTmDtEm5PC
+fGZENsbDu2iQZtVOGscF+cBYXrWKgfp+s4pv/RmefsA2sUuNYtXa132cicB8VDy
ljSzhTup6c4Dym6eoDtg/cpV/SlvxRG+zsNZ1A==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIEkjCCA3qgAwIBAgIQCgFBQgAAAVOFc2oLheynCDANBgkqhkiG9w0BAQsFADA/
MSQwIgYDVQQKExtEaWdpdGFsIFNpZ25hdHVyZSBUcnVzdCBDby4xFzAVBgNVBAMT
DkRTVCBSb290IENBIFgzMB4XDTE2MDMxNzE2NDA0NloXDTIxMDMxNzE2NDA0Nlow
SjELMAkGA1UEBhMCVVMxFjAUBgNVBAoTDUxldCdzIEVuY3J5cHQxIzAhBgNVBAMT
GkxldCdzIEVuY3J5cHQgQXV0aG9yaXR5IFgzMIIBIjANBgkqhkiG9w0BAQEFAAOC
AQ8AMIIBCgKCAQEAnNMM8FrlLke3cl03g7NoYzDq1zUmGSXhvb418XCSL7e4S0EF
q6meNQhY7LEqxGiHC6PjdeTm86dicbp5gWAf15Gan/PQeGdxyGkOlZHP/uaZ6WA8
SMx+yk13EiSdRxta67nsHjcAHJyse6cF6s5K671B5TaYucv9bTyWaN8jKkKQDIZ0
Z8h/pZq4UmEUEz9l6YKHy9v6Dlb2honzhT+Xhq+w3Brvaw2VFn3EK6BlspkENnWA
a6xK8xuQSXgvopZPKiAlKQTGdMDQMc2PMTiVFrqoM7hD8bEfwzB/onkxEz0tNvjj
/PIzark5McWvxI0NHWQWM6r6hCm21AvA2H3DkwIDAQABo4IBfTCCAXkwEgYDVR0T
AQH/BAgwBgEB/wIBADAOBgNVHQ8BAf8EBAMCAYYwfwYIKwYBBQUHAQEEczBxMDIG
CCsGAQUFBzABhiZodHRwOi8vaXNyZy50cnVzdGlkLm9jc3AuaWRlbnRydXN0LmNv
bTA7BggrBgEFBQcwAoYvaHR0cDovL2FwcHMuaWRlbnRydXN0LmNvbS9yb290cy9k
c3Ryb290Y2F4My5wN2MwHwYDVR0jBBgwFoAUxKexpHsscfrb4UuQdf/EFWCFiRAw
VAYDVR0gBE0wSzAIBgZngQwBAgEwPwYLKwYBBAGC3xMBAQEwMDAuBggrBgEFBQcC
ARYiaHR0cDovL2Nwcy5yb290LXgxLmxldHNlbmNyeXB0Lm9yZzA8BgNVHR8ENTAz
MDGgL6AthitodHRwOi8vY3JsLmlkZW50cnVzdC5jb20vRFNUUk9PVENBWDNDUkwu
Y3JsMB0GA1UdDgQWBBSoSmpjBH3duubRObemRWXv86jsoTANBgkqhkiG9w0BAQsF
AAOCAQEA3TPXEfNjWDjdGBX7CVW+dla5cEilaUcne8IkCJLxWh9KEik3JHRRHGJo
uM2VcGfl96S8TihRzZvoroed6ti6WqEBmtzw3Wodatg+VyOeph4EYpr/1wXKtx8/
wApIvJSwtmVi4MFU5aMqrSDE6ea73Mj2tcMyo5jMd6jmeWUHK8so/joWUoHOUgwu
X4Po1QYz+3dszkDqMp4fklxBwXRsW10KXzPMTZ+sOPAveyxindmjkW8lGy+QsRlG
PfZ+G6Z6h7mjem0Y+iWlkYcV4PIWL1iwBi8saCbGS5jN2p8M+X+Q7UNKEkROb3N6
KOqkqm57TH2H3eDJAkSnh6/DNFu0Qg==
-----END CERTIFICATE----- 
EOF

  private_key = <<EOF
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCkZ5tmAt0XOfa/
LRrJ7CAZLWsTzltkr8sIAGQ5JfxyrNOuoIzSbzopy2AkM79aySB5NSsB8+ABIea2
yvUIMLamn6TLNDx1Xsusjx1QMvsyvXMP/PGIK6mDDVdW9lIg96yMlGj/AA7rSncL
EN6pdqdXFiGfqvjWbTeiKUKoYxDUsKVVSpD3Xs/pJadffYJ5MVFnqp4f0BMgEc/m
aLk3G2TONFFzNP/me6LOQyn0Zjmu6P+CUlFNahkhtZsFQMAEQckEiBbeS980cY8P
ov1ETEYy+7wq6AZ55WeSTbSotqTm/jgUs9syIna/58nvoPioa6xVsjFY32xHC8vy
Xdr+zCHtAgMBAAECggEARwzrsNt990K6q4ZvtGJSwO7K/uVIxCvg/9VDexs6jci5
Nxf1pCAjr9pP83VVtoODgD6FFrPx1Ct1jPwLh32eAkauLo+lrUfJmArFrVpLC3Oq
nXdAXwwXlyaV32RWvB6tuJePBN1elTs6VVL2F6DK0y0iXOHD5s+OootYXnNp27DG
ZnyaLYTKps+0dsNOZLZhTk1GGadlDdmEgGlBgfA2W5Qx2ovQvc2OfDuG99vwhVv5
KZ5faLKRaqTYh9Wh7xzwG7GWRug+pFHfHAsDcJRTj/gluZ5ObDwa99xPU2gGNc1x
Ic9Z84fIlM/30wyyrUqCBsWbvKNeg0ITkLYjl68hoQKBgQDUMM9oXLovn0GcNhxf
rs8JlzcAN1/IrP6t/IlSFUmpM2xHkPUbOkWugn7ohDKdzbjJDQ0sXh3WgKOmWTD8
EuTYiUXIHbI7cPfwT59WzMp0eQzeOYUSwIJZ4jcXkH4OBGSwXUhVULlPb48XGQ+q
yYuQMV+UxhUuwYwhWooDzPEtWQKBgQDGWRqOo77F8m4fqCzD0BktHf0LgrRvddgr
qraThyT3btSt//QQ3/aJc+OJT0ZcTKu4aNrzGrgwZXxBzSHfLatwAqH7OzaEfnX2
8wuHDWe7iv1ACCU6JeGjGJcKkpUTYl9UzMF5HCxLtPWHy9WFWX3EZtmbft40iA0h
hlcrJ+NitQKBgQDMokHz8LEyKhEZOGoGsMxEAIvvbne2TXfTjR9VhOgkAE6lehuX
ZYw77ue7D3rNCs/xPN/+cMmvyyGH1K5T+1itpz0f79uqTZkfLXqKODfrOa56Rdib
LALJ8kqVNCkNFZmRKHUQqif8fqbtbKLaX0J0DdmS3bEiBVBB/lHptmTFCQKBgHB3
y9g/vwfs/EaVDLUHhY8QpxBkz7033Bh+l0I16l8nCA+Vx6Xd6KRuAwIz4lip3OEX
C7e3WeOPWBLTpzYuZjyAMasMG1CriGY70DiHAF/WYt1xAPLk0fmyEssa7M7uA9JI
vBfZQsC23lZe3Tbc1LSOASvrl0HAN3nf/ANrfcLVAoGBAMD+5zcdxzrlueKAfH4i
G4ShGDjaDXg/1HjZfi/x7Ur+umNaz868Wd7vSceQHuZlaf4tVmfrugjOBEMatSod
sax3pz5xwntVlXbluLAC3lB3wm56L38eV/OD2Y21GHjqD1A0O6aZW/YwN8mjvCi4
xDGW5r0S7Z891vy2DX4OJ9n3
-----END PRIVATE KEY-----
EOF
}
```

### Argument Reference
The following arguments are supported:

* `cert` - The certificate chain in pem format.
* `private_key` - The private key which matches the certificate in pem format.

## gravity_token
A token is a static secret that can be used to login to a cluster as a user.

### Example Usage
```
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

* token - A secret token that can be used to access the cluster.
* user - The user the token is for.

## gravity_user
A local cluster user

### Example Usage
```
resource "random_id" "admin_password" {
  byte_length = 32
}

resource "gravity_user" "test" {
  name      = "test"
  full_name = "Test User"
  type      = "admin"
  roles     = ["@teleadmin"]
  password  = "agent-password"
}
```

### Argument Reference
The following arguments are supported:

* `name` - An email address style username.
* `full_name` - (Optional) A friendly name for the user.
* `password` - (Optional) A password to provision for the user.
* `type` - The type of account.
    - agent - A restricted user used during OpsCenter operations.
    - admin - A user with maximum permissions.
    - regular - A standard interactive user .
    TODO(knisbet) can these description be any more vague????
* `roles` - A customized list of roles.


# Terraform Provider (Enterprise)
The gravity terraform provider is used to support terraform management of enterprise telekube resources. This provider should be used in conjunction with the gravity provider to manage a telekube cluster.

## Getting Started

### Install the Gravity provider
TODO(knisbet)

### Example Usage

```
# Configure the Telekube provider
provider "gravity" {
    host  = "https://example.com"
    token = "abcdefghi"
}

# Create a log forwarder
resource "gravity_log_forwarder" "logs" {
    name     = "logs"
    address  = "192.168.1.1"
    protocol = "udp"
}
```

### Authentication
The terraform provider uses token based authentication which must be provisioned to the cluster before being used. 

See TODO(knisbet)

## telekube_endpoints
By default an Ops Center is configured with a single endpoint set via `--ops-advertise-addr` flag passed during installation. This configuration allows creating seperate endpoints for cluster management and inter-cluster communications that can be firewalled seperatly.

### Example Usage
```
resource "telekube_endpoints" "test" {
  public_advertise_addr = "public.example.com:443"
  agents_advertise_addr = "agents.example.com:443"
}
```

### Argument Reference
The following arguments are supported:

* `public_advertise_addr` - Endpoint used by the Ops Center UI and CLI tools such as `tele` and `tsh`.
* `agents_advertise_addr` - (Optional) Endpoint used by remote clusters to connect to Ops Center as a trusted cluster.

## telekube_oidc
A telekube cluster can be configured to use Open ID connect as an identity provider and authenticate users.

### Example Usage
```
resource "telekube_oidc" "test" {
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
* `redirect_url` - The url on the telekube cluster that will process the OpenID callback. Should be in the format https://<cluster-hostname>/portalapi/v1/oidc/callback.
* `acr` - Authentication Context Class Reference value. The meaning of the ACR value is context-specific and varies for identity providers.
* `identity_provider` - (Optional) TODO(knisbet)
* `client_id` - Is the client-id used by the identity provider.
* `client_secret` - Is the secret used to authenticate with the identity provider.
* `issuer_url` - Is the url of the identity provider to direct users to.
* `scope` - Is a list of additional scopes set by the provider.
* `claims_to_roles` - Is a dictionary of claim to role mappings. Can be passed multiple times.
    - `claim` - OIDC claim name.
    - `value` - OIDC claim value to match.
    - `roles` - (Optional) A list of static roles to assign the user on login.
    - `role_template` - Is a dictionary for generating a role that will be filled with claims from OIDC TODO(knisbet) how to use the role template
        - `name` - 
        - `max_session_ttl` -
        - `logins` -
        - `node_labels` -
        - `namespaces` -
        - `resources` -
        - `forward_agent` - 

## telekube_role
Roles provide fine grained access to the cluster.

### Example Usage
Admin access to all resources:
```
resource "telekube_role" "admin" {
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

User level access to a cluster:
TODO(knisbet) how to actually do this correctly with RoleV3?
```
resource "telekube_role" "user" {
  name = "user"
  
  allow {
    logins = ["guest"]
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

### Argument Reference
The following arguments are supported:

* `name` - The name of the role.
* `max_ttl` - The maximum TTL of a login session through this role. See https://golang.org/pkg/time/#ParseDuration for the format. Default: "24h0m0s".
* `port_forwarding` - Enable port forwarding for tsh sessions. Default: false
* `forward_agent` - Enable ssh agent forwarding for tsh sessions. Default: false
* `allow` - Map of allow conditions for this role.
    - `logins` - List of users that the user can login as.
    - `namespaces` - TODO(knisbet) how do we use namespaces in the role context?
    - `node_labels` - Map of key=value pairs the user is able to access via tsh.
    - `rule` - A map of rules to apply to the condition. Multiple rules can be created.
        - `resources` - TODO(knisbet) what are valid resources??
        - `verbs` - List of operation verbs can be used. Valid options are:
            - register - allow registering of new clusters within an Ops Center
            - connect - allow users to connect to clusters
            - readsecrets - allow reading of secrets
        - `where` - TODO(knisbet) how to specify a where condition??
        - `actions` - A list of optional actions taken when a rule matches. Valid options are:
            - `log` - TODO(knisbet) ???
* `deny` - Map of deny conditions for this role.
    - `logins` - List of users that the user can login as.
    - `namespaces` - TODO(knisbet) how do we use namespaces in the role context?
    - `node_labels` - Map of key=value pairs the user is able to access via tsh.
    - `rule` - A map of rules to apply to the condition. Multiple rules can be created.
        - `resources` - TODO(knisbet) what are valid resources??
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
        - `where` - TODO(knisbet) how to specify a where condition??
        - `actions` - A list of optional actions taken when a rule matches. Valid options are:
            - `log` - TODO(knisbet) ???

## telekube_saml
Enables using SAML as an identity provider for cluster logins.

### Example Usage
```
resource "telekube_saml" "test" {
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
* `issuer` - TODO(knisbet) ???
* `sso` - URL of the identity provider SSO service.
* `cert` - The identity provider certificate.
* `acs` - The callback url on the telekube cluster. Format https://<host>/portalapi/v1/saml/callback.
* `audience` - (Optional) Uniquely identifies our service provider.
* `service_provider_issuer` - (Optional) is the issuer of the service provider
* `entity_descriptor` - (Optional) - Can be used to supply configuration parameters in one XML file vs supply them in the individual elements. TODO(knisbet) can this be clearer?
* `entity_descriptor_url` - (Optional) URL that supplies a configuration XML
* `atributes_to_roles` -  Is a dictionary of claim to role mappings. Can be passed multiple times.
    - `name` - claim name.
    - `value` - claim value to match.
    - `roles` - (Optional) A list of static roles to assign the user on login.
    - `role_template` - Is a dictionary for generating a role that will be filled with claims from OIDC TODO(knisbet) how to use the role template
        - `name` - 
        - `max_session_ttl` -
        - `logins` -
        - `node_labels` -
        - `namespaces` -
        - `resources` -
        - `forward_agent` - 
* `signing_key` - (Optional) Cert used to sign AuthnRequest
* `signing_cert` - (Optional) Key used to sign AuthnRequest
* `identity_provider` - (Optional) is the external identity provider.

## telekube_trusted_cluster
Trusted clusters allows connecting a standalone telekube cluster to an ops center.

### Example Usage
```
resource "telekube_trusted_cluster" "test" {
  name = "test"
  token = "abcdef"
  web_proxy_addr = "1.1.1.1"
  tunnel_addr = "1.1.1.1"
}
```

### Argument Reference
The following arguments are supported:

* `name` - The name of the trusted cluster connection.
* `enabled` - Boolean, allows the connection to be disabled without deleted the connection.
* `pull_updates` - Whether the cluster should automatically download application updates from the Ops Center.
* `token` - A secret token used to securely connect the cluster to the Ops Center. Can be retrieved by running  `gravity status` command on the Ops Center cluster.
* `tunnel_addr` - The address of the Ops Center reverse tunnel service as host:port. Typically it is exposed on port 3024.
* `web_proxy_addr` - The address which the Ops Center cluster serves its web API on.