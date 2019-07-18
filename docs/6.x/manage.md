# Remote Management

This chapter covers how to remotely manage Gravity clusters and securely access
them via Kubernetes API and via SSH using the same set of credentials.

Every Kubernetes cluster created from a Gravity cluster image can "dial home"
to a central control plane called _Gravity Hub_ to report its health, its
status and query Gravity Hub if there are any updates or security patches
available.

The ability to control large number of remote Kubernetes clusters via Gravity
Hub is called "Remote Assistance". 

The address of Gravity Hub can be configured via the cluster [image manifest](/pack/#image-manifest).
In this case, Remote Assistance will be on by default for every cluster created
from such cluster image.

If you do not mention Gravity Hub in a cluster image manifest, the resulting 
cluster image will produce clusters with Remote Assistance turned off. In this case
it's possible to turn it on via the command line arguments during [cluster installation](/installation/).

Cluster users can turn "Remote Assistance" on or off. They may want to enable
it only when they need remote assistance from the cluster image creator.

!!! warning "Version Warning":
    Only Enterprise edition of Gravity is capable of interacting with Gravity
    Hub, this means that "Remote Assistance" functionality is not available to
    the users of open source edition of Gravity.

## Gravitational Teleport

Under the hood, Gravity uses [Teleport](https://gravitational.com/teleport) to
implement Remote Assistance. Teleport is an open source privileged management
solution for both SSH and Kubernetes and it comes bundled with Gravity.

Teleport provides the following capabilities:

* Manages SSH identities and access permissions to a Gravity Cluster. Gravity Hub
  acts as a certificate authority (CA) capable of creating short-lived SSH certificates
  and Kubernetes certificates for remotely accessing clusters.

* Gravity clusters establish secure outbound management tunnels to the configured
  Gravity Hub. These tunnels are used to report application health and check for
  updates. They also allow Gravity Hub users to access Kubernetes clusters even if 
  they're running behind firewalls without any open network ports.

* Gravity Hub also acts as an "SSH jump host" (or, more correctly, "SSH
  proxy"), allowing Gravity Hub users to remotely access any node inside any
  Kubernetes cluster that is connected to the hub.

For more information, the [Teleport Architecture Document](http://gravitational.com/teleport/docs/architecture/)
covers these topics in depth.

## Logging into Gravity Hub

To login into Gravity Hub, use `tsh login` command:

```bash
$ tsh --proxy=hub.example.com login
```

Based on the Gravity Hub configuration, the login command will open the web
browser and users will have to go through a single sign-on (SSO) process with
the identity provider of their choice. 

Gravity Hub can work with any identity provider as long as it supports SAML or
OpenID Connect protocol.


## Listing Remote Clusters

To see the list of Gravity clusters available:

```bsh
$ tsh clusters
Name                          Status     Cloud Provider     Region
----                          ------     --------------     ------
east                          active     aws                us-east
west                          active     aws                us-west-2
```

Now you can make one of these clusters "current":

```bash
$ tsh login west
```

This command will automatically update your local `kubeconfig` file with
Kubernetes credentials, and `kubectl` command will automatically connect
to the cluster you've selected.

To see which cluster is current, execute `tsh status` command.

## Controlling Access To Clusters

Gravity Hub administrators can limit access to clusters using `where`
expressions in roles and user traits fetched from identity providers.

#### Cluster RBAC Using Labels

Sometimes it is necessary to limit users access to a subset of clusters via
Gravity Hub. For this, use Gravity Hub roles with `where` expressions in 
their rules:


```yaml
kind: role
version: v3
metadata:
  name: developers
spec:
  allow:
    logins:
    - developers
    namespaces:
    - default
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
      - connect
      - read
      where: contains(user.spec.traits["roles"], resource.metadata.labels["team"])
```

The role `developers` uses special property `user.spec.traits`
that contains user OIDC claims or SAML attribute statements after
users have successfully logged into Gravity Hub.

The property `resource.spec.labels["team"]` refers to cluster label `team`.
One can set cluster labels when creating clusters via UI or CLI.

And finally `where` expression `contains(user.spec.traits["roles"], resource.metadata.labels["team"])`
matches members with `developers` OIDC claim or SAML attribute statement to have `admin`
Kubernetes access to clusters marked with label `team:developers`

### Cluster RBAC With Deny Rules

Users can use `deny` rules to limit access to some privileged Clusters:

```yaml
kind: role
version: v3
metadata:
  name: deny-production
spec:
  deny:
    namespaces:
    - default
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
      - connect
      - read
      - list
      where: equals(resource.metadata.labels["env"], "production")
```

The role `deny-production` when assigned to the user, will limit access to all clusters
with label `env:production`.

## SSH Into Nodes

Users can use `tsh ssh` command to SSH into any node inside any remote clusters. 
For example:

```bsh
$ tsh --cluster=east ssh admin@node2
```

You can also copy files using secure file copy AKA `scp`:

```bsh
$ tsh --cluster=east scp example.txt admin@node2:/path/to/dest/
```

`tsh ssh` supports all the usual flags `ssh` users are used to. You can forward
ports, execute commands and so on. Run `tsh help` for more information.

