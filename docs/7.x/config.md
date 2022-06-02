---
title: Configuring Air-Gapped Kubernetes Cluster
description: How to configure an air-gapped Kubernetes cluster with Gravity
---

# Configuration Overview

Gravity borrows the concept of resources from Kubernetes for configuration.
Gravity uses the `gravity resource` command to update the Cluster configuration.

The currently supported resources are:

Resource Name             | Resource Description
--------------------------|---------------------
`github`                  | GitHub connector
`oidc`                    | OIDC connector (Enterprise only)
`saml`                    | SAML connector (Enterprise only)
`role`                    | User role (Enterprise only)
`user`                    | Cluster user
`token`                   | User tokens such as API keys
`logforwarder`            | Forwarding logs to a remote rsyslog server
`trusted_cluster`         | Managing access to remote Gravity Hubs (Enterprise Only)
`endpoints`               | Gravity Hub endpoints for user and Cluster traffic (Enterprise Only)
`cluster_auth_preference` | Cluster authentication settings such as second-factor
`alert`                   | Cluster monitoring alert
`alerttarget`             | Cluster monitoring alert target
`smtp`                    | Cluster monitoring SMTP configuration
`runtimeenvironment`      | Cluster runtime environment variables
`clusterconfiguration`    | General Cluster configuration
`authgateway`             | Authentication gateway configuration
`operations`              | Cluster operations


## General Cluster Configuration

It is possible to customize the Cluster per environment before the installation
or update some aspects of the Cluster using the `ClusterConfiguration` resource:

```yaml
kind: ClusterConfiguration
version: v1
spec:
  global:
    # configures the cloud provider
    cloudProvider: gce
    # free-form cloud configuration
    cloudConfig: |
      multizone=true
      gce-node-tags=demo-cluster
    # represents the IP range from which to assign service Cluster IPs
    serviceCIDR:  "10.0.0.0/24"
    # port range to reserve for services with NodePort visibility
    serviceNodePortRange: "30000-32767"
    # host port range (begin-end, single port or begin+offset, inclusive) that
    # may be consumed in order to proxy service traffic
    proxyPortRange: "0-0"
    # CIDR range for Pods in Cluster
    podCIDR: "10.0.0.0/24"
    # The size of the Pod subnet allocated to each host
    podSubnetSize: "26"
    # Enables Kubernetes high availability mode. When HA mode is enabled, Kubernetes
    # control plane components run on all master nodes.
    highAvailability: true
    # Enables or disables serf encryption mode. Enabled by default.
    serfEncryption: true
    # Enables or disables etcd-healthz checks. Enabled by default.
    etcdHealthz: true
    # A set of key=value pairs that describe feature gates for alpha/experimental features
    featureGates:
      AllAlpha: true
      APIResponseCompression: false
      BoundServiceAccountTokenVolume: false
      ExperimentalHostUserNamespaceDefaulting: true
  # kubelet configuration as described here: https://kubernetes.io/docs/tasks/administer-cluster/kubelet-config-file/
  # and here: https://github.com/kubernetes/kubelet/blob/release-1.13/config/v1beta1/types.go#L62
  kubelet:
    config:
      kind: KubeletConfiguration
      apiVersion: kubelet.config.k8s.io/v1beta1
      nodeLeaseDurationSeconds: 50
```

In order to apply the configuration immediately after the installation, supply the configuration file
to the `gravity install` command:

```bsh
root$ ./gravity install --cluster=<cluster-name> ... --config=Cluster-config.yaml
```

!!! note
    You can combine multiple kubernetes and Gravity-specific resources in the config file prior to
    running the install command to have the installer automatically create all resources upon installation.

!!! warning
    Setting feature gates overrides the value set by the runtime container by default.


In order to update configuration of an active Cluster, use the `gravity resource` command:

```bsh
root$ ./gravity resource create Cluster-config.yaml
```

The operation can be started in manual mode in which case you have the ability to review the operation
plan or cancel the operation. To put the operation into manual mode, use the `--manual` flag:

```bsh
root$ ./gravity resource create Cluster-config.yaml --manual
```

The configuration update is implemented as a Cluster operation. Once created, it is managed using
the same `gravity plan` command described in the [Managing Operations](cluster.md#managing-operations)
section.


To view the configuration:

```bsh
root$ ./gravity resource get config
```

To remove (reset to defaults) the configuration:

```bsh
root$ ./gravity resource rm config
```


!!! warning
    Updating the configuration of an active Cluster is disruptive and might necessitate the restart
    of runtime containers either on master or on all Cluster nodes. Take this into account and plan
    each update accordingly.

## Cluster Operations

Operations performed on a cluster (install, upgrade, node join or removal, etc.)
are exposed via an `operation` resource.

To see a list of cluster operations:

```bsh
$ sudo gravity resource get operations
ID                                       Description                                   State         Created
--                                       -----------                                   -----         -------
eb0d0a68-f835-471b-9ceb-500460ffcd0b     Remove node node-2 (192.168.99.103)           Completed     Tue Apr  7 21:56 UTC
7e95deca-3a71-4abe-8616-097bab26c943     Join node node-2 (192.168.99.103) as node     Completed     Tue Apr  7 21:50 UTC
b75f28bc-b8e9-403f-9cda-972013a652e8     1-node install                                Completed     Tue Apr  7 21:37 UTC
```

To view a particular operation details:

```bsh
$ sudo gravity resource get operation 7e95deca-3a71-4abe-8616-097bab26c943 --format=json
```

The `operation` resource is read-only and, as such, not supported by the
resource create and delete commands.

## Cluster Access

Gravity supports the creation of multiple users. Roles can also be created and
assigned  to users which can be mapped to Kubernetes role based access control
authorization (RBAC API). Gravity can also integrate with third party identity
providers through standard protocols like OIDC and SAML.

!!! warning "Enterprise Only Version Warning"
    The Community version of Gravity only supports local users and integration
    with Github identity. Gravity Enterprise supports additional identity
    provider integrations.

### Configuring Roles

Below is an example of a resource file with the definition of an admin role. The
admin has access to all resources, including roles, other users and authentication
settings, and belongs to a privileged Kubernetes group:

```yaml
kind: role
version: v3
metadata:
  name: administrator
spec:
  allow:
    kubernetes_groups:
    - admin
    logins:
    - root
    node_labels:
      '*': '*'
    rules:
    - resources:
      - '*'
      verbs:
      - '*'
  options:
    max_session_ttl: 30h0m0s
```

Below is an example of a non-admin role spec providing access to a particular
Cluster `example.com` and its applications:

```yaml
kind: role
version: v3
metadata:
  name: developer
spec:
  allow:
    logins:
    - root
    node_labels:
      '*': '*'
    kubernetes_groups:
    - admin
    rules:
    - resources:
      - role
      verbs:
      - read
    - resources:
      - app
      verbs:
      - list
    - resources:
      - cluster
      verbs:
      - read
      - update
      where: equals(resource.metadata.name, "example.com")
  options:
    max_session_ttl: 10h0m0s
```

To create these two roles you can execute:

```bsh
$ gravity resource create administrator.yaml
$ gravity resource create developer.yaml
```

To view all currently available roles:

```bsh
$ gravity resource get role
```

To delete the `developer` role:

```bsh
$ gravity resource delete role developer
```

### Configuring Users & Tokens

Below is an example of a resource file that creates a user called `user.yaml`.

```yaml
kind: user
version: v2
metadata:
  name: "alice@example.com"
spec:
  # "agent" type means this user is only authorized to access the Cluster
  # using the API key (token) and not using the web UI
  type: agent
  roles: ["developer"]
```

Create the user by executing `gravity resource`:

```bsh
$ gravity resource create user.yaml
```

A token can is assigned to this user by using
the following resource file called `token.yaml`:

```yaml
kind: token
version: v2
metadata:
   name: xxxyyyzzz
spec:
   user: "alice@example.com"
```

Create the token by executing `gravity resource`:

```bsh
$ gravity resource create token.yaml
```

To view available users and a user's tokens:

```bsh
$ gravity resource get user
$ gravity resource get token --user=alice@example.com
```

### Example: Provisioning A Cluster Admin User

The example below shows how to create an admin user for a Cluster.
Save the user definition into a YAML file:

```yaml
# admin.yaml
kind: user
version: v2
metadata:
  name: "admin@example.com"
spec:
  type: "admin"
  password: "Passw0rd!"
  roles: ["@teleadmin"]
```

The password will be encrypted with
[bcrypt](https://en.wikipedia.org/wiki/Bcrypt) prior to being saved into the
database. Note the role `@teleadmin` - this is a built-in system role for the
Cluster administrators.

To create the user from the YAML above, execute the following command on one of
the Cluster nodes:

```bsh
$ gravity resource create admin.yaml
```

The new user can now log into the Cluster via the Web UI with the user
credentials created above.

!!! tip "Username and Password Restrictions"
    Usernames should be composed of characters, hyphens, the at symbol and dots.
    Passwords must be between 6 and 128 characters long.


### Configuring a GitHub Connector

Gravity supports authentication and authorization via GitHub. To configure
it, create a YAML file with the resource spec based on the following example:

```yaml
kind: github
version: v3
metadata:
  name: example
spec:
  # Github OAuth app client ID
  client_id: <client-id>
  # Github OAuth app client secret
  client_secret: <client-secret>
  # Github will make a callback to this URL after successful authentication
  # Cluster-url is the address the Cluster UI is reachable at
  redirect_url: "https://<Cluster-url>/portalapi/v1/github/callback"
  # connector display name that will be appended to the title of "Login with"
  # button on the Cluster login screen so it will say "Login with Github"
  display: Github
  # mapping of Github team memberships to Gravity Cluster roles
  teams_to_logins:
    - organization: example
      team: admins
      logins:
        - "@teleadmin"
```

Create the connector:

```bsh
$ gravity resource create github.yaml
```

Once the connector has been created, the Cluster login screen will start
presenting "Login with GitHub" button.

!!! note
    When going through the Github authentication flow for the first time, the
    application must be granted the access to all organizations that are present
    in the "teams to logins" mapping, otherwise Gravity will not be able to
    determine team memberships for these organizations.

To view configured GitHub connectors:

```bsh
$ gravity resource get github
```

To remove a GitHub connector:

```bsh
$ gravity resource rm github example
```

### Configuring OpenID Connect

!!! warning "Enterprise Only Version Warning"
    The ability to configure an OIDC Connector is only available in Gravity
    Enterprise.

A Gravity Cluster can be configured to authenticate users using an
OpenID Connect (OIDC) provider such as Auth0, Okta and others.

A resource file in YAML format creates the connector.  Below is an
example of an OIDC resource for provider "Auth0" called
`oidc.yaml`:

```yaml
kind: oidc
version: v2
metadata:
  name: auth0
spec:
  redirect_url: "https://gravity-url/portalapi/v1/oidc/callback"
  client_id: <client id>
  client_secret: <client secret>
  issuer_url: "https://example.com/"
  scope: [roles]
  claims_to_roles:
    - {% raw %}{claim: "roles", value: "gravitational/admins", roles: ["@teleadmin"]}{% endraw %}
```

!!! note
    For Auth0 the "OIDC Conformant" setting should be off in Advanced Setting -> OAuth or Claims will not populate properly

Add this connector to the Cluster:

```bsh
$ gravity resource create oidc.yaml
```

To list the installed connectors:

```bsh
$ gravity resource get oidc
```

To remove the connector `auth0`:

```bsh
$ gravity resource rm oidc auth0
```

### Example: Google OIDC Connector

!!! warning "Enterprise Only Version Warning"
    The ability to configure a Google OIDC connector is only available in
    Gravity Enterprise.

Here's an example of the OIDC connector that uses Google for authentication:

```yaml
kind: oidc
version: v2
metadata:
  name: google
spec:
  redirect_url: "https://ops-advertise-url/portalapi/v1/oidc/callback"
  client_id: <client id>
  client_secret: <client secret>
  issuer_url: https://accounts.google.com
  scope: [email]
  claims_to_roles:
    - {claim: "hd", value: "example.com", roles: ["@teleadmin"]}
```

The `hd` scope contains the hosted Google suite domain of the user so in the
above example, any user who belongs to the "example.com" domain will be
allowed to log in and granted the admin role.

!!! note
    The user must belong to a hosted domain, otherwise the `hd` claim will
    not be populated.

### Configuring SAML Connector

!!! warning "Enterprise Only Version Warning"
    The ability to configure a SAML Connector is only available in Gravity
    Enterprise.

Gravity supports authentication and authorization via SAML providers. To
configure it, create a YAML file with the resource spec based on the following example:

```yaml
kind: saml
version: v2
metadata:
  name: okta
spec:
  # SAML provider will make a callback to this URL after successful authentication
  # Cluster-url is the address the Cluster UI is reachable at
  acs: https://<Cluster-url>/portalapi/v1/saml/callback
  # mapping of SAML attributes to Gravity roles
  attributes_to_roles:
    - name: groups
      value: admins
      roles:
        - @teleadmin
  # connector display name that will be appended to the title of "Login with"
  # button on the Cluster login screen so it will say "Login with Okta"
  display: Okta
  # SAML app metadata in XML format downloaded from SAML provider
  entity_descriptor: |
    ...
```

!!! note
    For an example of configuring a SAML application with Okta take a look
    at the following guide: [SSH Authentication With Okta](https://gravitational.com/teleport/docs/ssh_okta/).

Create the connector:

```bsh
$ gravity resource create saml.yaml
```

To view configured SAML connectors:

```bsh
$ gravity resource get saml
```

To remove a SAML connector:

```bsh
$ gravity resource rm saml okta
```

### Cluster Authentication Gateway

!!! warning "Version Warning":

    Authentication gateway resource is supported starting Gravity version `5.5.0`.

The Cluster authentication gateway handles the authentication and authorization
and uses the following resource:

```yaml
kind: authgateway
version: v1
spec:
  # Connection throttling settings
  connection_limits:
    # Max number of simultaneous connections
    max_connections: 1000
    # Max number of simultaneously connected users
    max_users: 250
  # Cluster authentication preferences
  authentication:
    # Auth type, can be "local", "oidc", "saml" or "github"
    type: oidc
    # Second factor auth type, can be "off", "otp" or "u2f"
    second_factor: otp
    # Default auth connector name
    connector_name: google
  # Determines if SSH sessions to Cluster nodes are forcefully terminated
  # after no activity from a client, for example "30m", "1h", "1h30m"
  client_idle_timeout: never
  # Determines if the clients will be forcefully disconnected when their
  # certificates expire in the middle of an active SSH session
  disconnect_expired_cert: false
  # DNS name that applies to all SSH, Kubernetes and web proxy endpoints
  public_addr:
    - example.com
  # DNS name of the gateway SSH proxy endpoint, overrides "public_addr"
  ssh_public_addr:
    - ssh.example.com
  # DNS name of the gateway Kubernetes proxy endpoint, overrides "public_addr"
  kubernetes_public_addr:
    - k8s.example.com
  # DNS name of the gateway web proxy endpoint, overrides "public_addr"
  web_public_addr:
    - web.example.com
```

To update authentication gateway configuration, run:

```bash
$ gravity resource create gateway.yaml
```

!!! note
    The `gravity-site` pods will be restarted upon resource creation in order
    for the new settings to take effect, so the Cluster management UI / API
    will become briefly unavailable.

When authentication gateway resource is created, only settings that were
explicitly set are applied to the current configuration. For example, to
only limit the maximum number of connections, you can create the following
resource:

```yaml
kind: authgateway
version: v1
spec:
  connection_limits:
    max_conections: 1500
```

The following command will display current authentication gateway configuration:

```bash
$ gravity resource get authgateway
```

#### Cluster Authentication Preference

!!! warning "Deprecation warning"
    Cluster authentication preference resource is obsolete starting Gravity
    version `5.5.0` and will be removed in a future version. Please use
    [Authentication Gateway](config.md#cluster-authentication-gateway)
    resource instead.

Cluster authentication preference resource allows to configure method of
authentication users will use when logging into a Gravity Cluster.

The resource has the following format:

```yaml
kind: Cluster_auth_preference
version: v2
metadata:
  name: auth-oidc
spec:
  # preferred auth type, can be "local" (to authenticate against
  # local users database) or "oidc"
  type: oidc
  # second-factor auth type, can be "off" or "otp"
  second_factor: otp
  # default authentication connector to use for tele login
  connector_name: google
```

By default the following authentication method is configured:

* For Clusters: local without second-factor authentication.
* For Gravity Hub Clusters: OIDC or local with second-factor authentication.

To update authentication preference, for example to allow local users to log
into Gravity Hub without second-factor, define the following resource:

```yaml
kind: Cluster_auth_preference
version: v2
metadata:
  name: auth-local
spec:
  type: local
  second_factor: "off"
```

Create it:

```bsh
$ gravity resource create auth.yaml
```

!!! note
    Make sure to configure a proper [OIDC connector](config.md#configuring-openid-connect)
    when using "oidc" authentication type.

To view the currently configured authentication preference:

```bsh
$ gravity resource get Cluster_auth_preference
Type      ConnectorName     SecondFactor
----      -------------     ------------
local                       off
```

### Log Forwarders

Every Gravity Cluster is automatically set up to aggregate the logs from all
running containers. By default, the logs are kept inside the Cluster but they can be configured to be shipped to a remote log collector such as a rsyslog server.

Below is a sample resource file called `forwarder.yaml` that creates a log forward:

```yaml
kind: logforwarder
version: v2
metadata:
   name: forwarder1
spec:
   address: 192.168.100.1:514
   protocol: udp
```

The `protocol` field is optional and defaults to `tcp`. Create the log forwarder:

```bsh
$ gravity resource create forwarder.yaml
```

To view currently configured log forwarders, run:

```bsh
$ gravity resource get logforwarders
```

To delete a log forwarder:

```bsh
$ gravity resource rm logforwarder forwarder1
```

### TLS Key Pair

Gravity Cluster Web UI, (Gravity Hub for Enterprise Users) and API TLS key pair can be configured using `tlskeypair` resource.

```yaml
kind: tlskeypair
version: v2
metadata:
  name: keypair
spec:
  private_key: |
    -----BEGIN RSA PRIVATE KEY-----
  cert: |
    -----BEGIN CERTIFICATE-----
```

!!! tip "Certificate chain"
    `cert` section should include all intermediate certificate PEM blocks concatenated to function properly!

To update the key pair:

```bsh
$ gravity resource create tlskeypair.yaml
```

To view the currently configured key pair:

```bsh
$ gravity resource get tls
```

To delete a TLS key pair (in this case default self-signed TLS key pair will be used instead):

```bsh
$ gravity resource rm tls keypair
```

### Monitoring and Alerts

See [the Cluster Monitoring section](monitoring.md) about details
on how to configure monitoring and alerts.

### Runtime Environment Variables

In a Gravity Cluster, each node is running a Master Container (called "Planet")
that hosts Kubernetes. All services (including Kubernetes native services like
API server or kubelet) execute within the predefined environment (set up
during installation or update). The `RuntimeEnvironment` allows you to make
changes to the runtime environment, i.e. introduce new environment variables
like `HTTP_PROXY`.

To add a new environment variable, `HTTP_PROXY`, create a file with following
contents:

```yaml
kind: RuntimeEnvironment
version: v1
spec:
  data:
    HTTP_PROXY: "example.com:8001"
```

To install a Cluster with the new runtime environment, specify the resources file as an argument
to the `install` command:

```bsh
$ sudo gravity install --cluster=<cluster-name> --config=envars.yaml
```

On an installed Cluster, create the resource with:

```bash
$ sudo gravity resource create -f envars.yaml
Updating Cluster runtime environment requires restart of runtime containers on all nodes.
The operation might take several minutes to complete depending on the Cluster size.

The operation will start automatically once you approve it.
If you want to review the operation plan first or execute it manually step by step,
run the operation in manual mode by specifying '--manual' flag.

Are you sure?
confirm (yes/no):
yes
```

Without additional parameters, the operation is executed automatically, but can be placed into manual mode with
the specification of `--manual | -m` flag to the `gravity resource`  command:

```bash
$ sudo gravity resource create -f envars.yaml --manual
```

This will allow you to control every aspect of the operation as it executes.
See [Managing Operations](cluster.md#managing-operations) for more details.


To view the currently configured runtime environment variables:

```bash
$ gravity resource get runtimeenvironment
Environment
-----------
HTTP_PROXY=example.com:8081
```

To remove the configured runtime environment variables, run:

```bash
$ gravity resource rm runtimeenvironment
```

!!! warning
    Adding or removing Cluster runtime environment variables is disruptive as it necessitates the restart
    of runtime containers on each Cluster node. Take this into account and plan each update accordingly.


### Trusted Clusters (Enterprise)

!!! warning "Enterprise Only Version Working"
    Gravity Hub and Trusted Clusters are only supported in Gravity Enterprise.
    Support for Trusted Clusters is available since Gravity version `5.0.0-alpha.5`.

Trusted Clusters is a concept for connecting standalone Gravity Clusters to
a Gravity Hub. It brings the following advantages:

* Allows agents of the remote Gravity Hub to SSH into your Cluster nodes to
  perform the remote assistance.
* Allows a Cluster to download application updates from Gravity Hub.

To configure a Trusted Cluster create the following resource:

```yaml
kind: trusted_cluster
version: v2
metadata:
  name: hub.example.com
spec:
  enabled: true
  pull_updates: true
  token: c523fd0961be71a45ceed81bdfb61b859da8963e2d9d7befb474e47d6040dbb5
  tunnel_addr: "hub.example.com:3024"
  web_proxy_addr: "hub.example.com:32009"
  role_map:
  - remote: "@teleadmin"
    local: ["@teleadmin"]
```

Let's go over the resource fields:

* `metadata.name`: The name of Gravity Hub the Cluster is being connected to.
* `spec.enabled`: Allows the agents to establish remote connection to the Cluster
  from Gravity Hub.
* `spec.pull_updates`: Whether the Cluster should be automatically downloading
  application updates from Gravity Hub.
* `spec.token`: A secret token used to securely connect the Cluster to Gravity Hub.
* `spec.tunnel_addr`: The address of Gravity Hub reverse tunnel service as
   host:port. Typically it is exposed on port `3024`.
* `spec.web_proxy_addr`: The address which Gravity Hub serves its Web
   API on. It is the same address specified via the `--hub-advertise-addr` parameter
   when [installing Gravity Hub](hub.md#installing-gravity-hub).
* `spec.role_map`: Role mapping between the Hub and the Cluster. A user that has a
   matching "remote" role assigned in the Hub, will assume all corresponding "local"
   roles on the Cluster.

!!! note "Role mapping"
    If not explicitly specified, the default role mapping will only map the remote
    admin role `@teleadmin` to the local admin role `@teleadmin` as shown in the
    example above. See Teleport documentation on [Trusted Clusters RBAC](https://gravitational.com/teleport/docs/trustedclusters/#rbac)
    for information about configuring role mapping.

Create the Trusted Cluster:

```bsh
$ gravity resource create trustedCluster.yaml
```

View the currently configured Trusted Cluster:

```bsh
$ gravity resource get trusted_Cluster
Name                Enabled     Pull Updates     Reverse Tunnel Address    Proxy Address
----                -------     ------------     ----------------------    -------------
hub.example.com     true        true             hub.example.com:3024      hub.example.com:32009
```

Once the Cluster has been created, the reverse tunnel status can be viewed and
managed using `gravity tunnel` shortcut commands:

```bsh
$ gravity tunnel status
Gravity Hub       Status
hub.example.com   enabled

$ gravity tunnel disable
Gravity Hub       Status
hub.example.com   disabled

$ gravity tunnel enable
Gravity Hub       Status
hub.example.com   enabled
```

To disconnect the Cluster Gravity Hub, remove the Trusted Cluster:

```bsh
$ gravity resource rm trustedCluster hub.example.com
Trusted Cluster "hub.example.com" has been deleted
```

### Customize Number of DNS instances on workers
Gravity ships a default DNS configuration that should be appropriate for most environments, that scales based on the
number of CPU cores and nodes within the cluster.

The scaling configuration can be customized by replacing the autoscaler configmap with the desired configuration.
```bsh
kubectl apply -f - <<EOF
kind: ConfigMap
apiVersion: v1
metadata:
  name: autoscaler-coredns-worker
  namespace: kube-system
  annotations:
    gravitational.io/customer-managed: "true"
data:
  linear: |-
    {
      "coresPerReplica": 4,
      "nodesPerReplica": 1,
      "min": 9,
      "preventSinglePointFailure": true
    }
EOF
```

The annotation `gravitational.io/customer-managed` indicates to gravity that the configuration has been overwritten,
and that cluster upgrades should not reset the configuration to default.

!!! note
    Gravity uses the cluster-proportional-autoscaler to scale the number of DNS instances on workers. For more
    information on how to configure the autoscaler, please see the [cluster-proportional-autoscaler docs](https://github.com/kubernetes-sigs/cluster-proportional-autoscaler/blob/1.8.3/README.md#control-patterns-and-configmap-formats) for information.