---
title: Gravity Manage
description: How Gravity can remotely manage Gravity Clusters and securely access them via SSH.
---

# Remote Management

This chapter covers how Gravity can remotely manage Gravity Clusters
and securely access them via SSH.

Every application deployed via Gravity can "dial home" to an Ops Center
to report its health status and query for updates. The address of the Ops Center
can be configured in the Application Manifest, or it can be set via command line
arguments during application installation.

This capability is called "Remote Assistance" in the Gravity Cluster GUI.

* The end users can turn "Remote Assistance" on or off. Sometimes they may
  want to enable it only when they need vendor help troubleshooting.
* You can package an application without binding it to an Ops Center.
  Such applications do not "dial home" by default.

## Gravitational Teleport

Gravity uses [Teleport](https://gravitational.com/teleport) to implement
Remote Assistance. Teleport is an open source SSH server bundled with Gravity
and it provides the following capabilities:

* It manages SSH identities and access rights to a Gravity Cluster.
  The Ops Center acts as a certificate authority (CA)
  capable of creating short-lived SSH sessions keys for accessing Clusters.

* Every Gravity Cluster runs in its own Kubernetes cluster and also
  runs a local SSH certificate authority (CA) which manages remote access
  rights to that cluster.

* Gravity Clusters establish outbound SSH tunnels to the
  Ops Center. These tunnels are used to report application health and check for
  updates.

* The Ops Center also acts as an "SSH jump host" (aka, "SSH bastion"), allowing
  the Ops Center to remotely access any server inside the Gravity Clusters.

For more information, the [Teleport Architecture Document](http://gravitational.com/teleport/docs/architecture/)
covers these topics in depth.

## Creating Gravity Clusters

The command `tele create` command can be used to provision a new remote cluster. The new
cluster will be remotely accessible and managed via the Ops Center.

!!! note
	The `tele create` operation is only available for creating clusters on programmable
	infrastructure. At this time only AWS is supported.

An Gravity Cluster is defined by two things:

1. The Application Manifest [manifest](pack.md#application-manifest).
2. The Cluster Spec.

The _Application Manifest_ declares **what** should be inside a cluster:

* Kubernetes binaries.
* The binaries of Kubernetes dependencies such as Docker or etcd.
* Minimum system requirements of a cluster.
* Pre-packaged applications that have to be pre-deployed, popular choices
  are CI/CD tools, databases, etc.

The _Cluster Spec_ provides the infrastructure resources that satisfy the requirements
defined by the Application Manifest. Remember that in case of a [manual installation](quickstart.md#installing)
of an application bundle the user is responsible to provide the same information manually
to create a cluster.

To provision a new Gravity Cluster on AWS:

1. Declare the Cluster Spec in a YAML file, for example `cluster.yaml`
2. Execute `$ tele create cluster.yaml`
3. Optional: provisioning of a cluster can be customized with user-supplied
   scripts based on tools like [Terraform](https://www.terraform.io/) or
   [Cloud Formation](https://aws.amazon.com/cloudformation/).

Below is the example of a Cluster Spec definition:

```yaml
# cluster.yaml
kind: cluster
version: v2
metadata:
  # test is a unique cluster name
  name: test
spec:
  # The application bundle name which defines the binaries for Kubernetes and all of its
  # dependencies. It may also contain the provisioning customization steps described below.
  # The application must be published into the Ops Center ahead of time.
  app: example:1.0.0
  provider: aws
  aws:
    # AWS region
    region: us-west-2
    # optional VPC ID. if not specified a new VPC will be created
    vpc: vpc-abc123
    # the ssh key name must exist in the selected AWS region
    keyName: key-name
  nodes:
    # profile is node profile that should be
    # specified in the app "example"
  - profile: node
    # number of nodes to provision:
    count: 1
    # AWS instance type
    instanceType: m4.xlarge
```

To provision the Gravity Cluster defined above, make sure to log into the Ops Center and
execute:

```bsh
$ tele create cluster.yaml
```

!!! tip "Important"
    `tele create` only supports AWS clusters and does not allow updating clusters
    after they have been created. This capability is still evolvling and user feedback is welcome.
    `tele create` requires `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables
    to be set on the client and will use them to provision AWS instances.

### Verifying Node Requirements

The following command will verify that a node is compatible with requirements defined by given
profile, such as CPU, RAM and volume storage.

```bsh
$ gravity check --profile=node app.yaml
```

If the node does not satisfy any of the requirements, the command will output
a list of failed checks and exit with a non-0 return code.

If the list of failed checks includes unloaded kernel modules and unset kernel
parameters required for installation (see [System Requirements](requirements.md#kernel-modules)),
this command can be re-run with `--autofix` flag to attempt to fix those issues:

```bsh
$ gravity check --profile=node --autofix app.yaml
```

During installation the `--autofix` flag is implied so kernel modules/parameters
will be loaded by all install agents automatically.

### Customized Cluster Provisioning

Cluster provisioning can be customized by the [Application Manifest](pack.md#application-manifest)
author. This is achieved by implementing four _provisioning hooks_ and listing them in the
Application Manifest. A provisioning hook is a Kubernetes job and can be
implemented using any language.

The supported hooks are:

```yaml
# fragment of an application bundle manifest
hooks:
  # Provisions the entire cluster. This job will run inside the ops center.
  clusterProvision:
      # job.yaml must contain the standard Kubernetes job resource
      job: file://path/to/job.yaml

  # Deprovisions the entire cluster. This job will run inside the ops center.
  clusterDeprovision:
      job: file://path/to/job.yaml

  # Provisions one or several nodes. This job will run inside the target cluster.
  nodesProvision:
      job: file://path/to/job.yaml

  # Deprovisions one or several nodes. This job will run inside the target cluster.
  nodesDeprovision:
      job: file://path/to/job.yaml
```

As shown in the [Application Manifest](pack.md#application-manifest) documentation, a hook must be implemented as a Kubernetes job.
A job must be declared using standard [Kubernetes job syntax](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/#writing-a-job-spec),
either in a separate YAML file as shown above, or inline.


!!! tip "Important"
    * Gravity requires all 4 hooks when using custom provisioning.
    * Docker images in custom provisioning jobs are not embedded into the application
      and are pulled from the registries directly. When using private registries, use
      [special secrets](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)
      to specify credentials.


**Hook Parameters**

All provisioning hooks are implemented as Kubernetes jobs and their parameters are passed either as an environment
variables (env) or as a Kubernetes secret mounted as a file.

The following parameters are common for all cloud providers:

| Parameter                                      | Type  |  Description                                   | Example Value                          |  Hooks                               |
|------------------------------------------------|-------|------------------------------------------------|----------------------------------------|--------------------------------------|
| CLOUD_PROVIDER                                 | env   | Cloud provider specified by the caller         | `aws`                                  | all                                  |
| TELEKUBE_OPS_URL                               | env   | URL with the script to start the install agent | `https://example.com/t`                | `clusterProvision`, `nodesProvision` |
| `/var/lib/telekube/token`                      | file  | The secret token used by the install agent     | `autogenerated random string`          | `clusterProvision`, `nodesProvision` |
| TELEKUBE_FLAVOR                                | env | Installation flavor                            | `demo`                                 | `clusterProvision`                   |
| TELEKUBE_CLUSTER_NAME                          | env | Cluster name                                   | `cluster1`                             | all                                  |
| TELEKUBE_NODE_PROFILES                         | env | Comma-delimited list of the node profiles      | `db,node`                              | `clusterProvision`, `nodesProvision` |
| TELEKUBE_NODE_PROFILE_COUNT_[profile]          | env | Requested node count of profile `[profile]`    | `3`                                    | `clusterProvision`, `nodesProvision` |
| TELEKUBE_NODE_PROFILE_ADD_COUNT_[profile]      | env | Requested node count to add of profile `[profile]`   | `1`                                    | `nodesProvision`                     |
| TELEKUBE_NODE_PROFILE_INSTANCE_TYPE_[profile]  | env | Instance type of profile `[profile]`           | `m4.xlarge`                            | `clusterProvision`, `nodesProvision` |

The following parameters are specific to AWS:

| Parameter                                      | Type |  Description                                    | Example Value                          |  Hooks                               |
|------------------------------------------------|------|-------------------------------------------------|----------------------------------------|--------------------------------------|
| `/var/lib/telekube/aws-credentials`            | file | AWS credentials if supplied by caller           | [credentials file format](http://docs.aws.amazon.com/cli/latest/userguide/cli-config-files.html)          | all |
| AWS_REGION                                     | env | AWS region name                                 | `us-west-2`                                | all                                  |
| AWS_AMI                                        | env | AWS AMI (optional)                              | `ami-abcdefg`                              | `clusterProvision`, `nodesProvision` |
| AWS_VPC_ID                                     | env | AWS VPC ID (optional) if omitted, create new    | `vpc-abcdefg`                              | `clusterProvision`, `nodesProvision` |
| AWS_KEY_NAME                                   | env | AWS Key name to assign to instance              | `example`                                  | all                                  |
| AWS_INSTANCE_PRIVATE_IP                        | env | Private IP address of instance to deprovision   | `10.0.1.5`                                 | `nodesDeprovision`                   |
| AWS_INSTANCE_PRIVATE_DNS                       | env | Private DNS name of the instance to deprovision | `ip-10-0-1-5.us-west-2.compute.internal`   | `nodesDeprovision`                   |


**Error Handling**

In case of an error the hook author must write a useful error message to
`stdout` and to return a non-0 exit code. Gravity will collect and report all
such errors when a user checks the status of the cluster provisioning
operation.

To check the status of the cluster provisioning operation (as well as other
ongoing operations) on the Ops Center, a user must SSH into a machine running
the Ops Center and execute:

```bsh
$ gravity status
```

Future versions of Gravity will allow listing Ops Center operations remotely
via `tele` command.

## Listing Gravity Clusters

To see the list of Gravity Clusters available:

```bsh
$ tele get clusters
```

Example output:

```bsh
Name                            Status     Cloud Provider     Region
----                            ------     --------------     ------
hello1                          active     aws                us-west-2

```

Use `tsh ls` command to list all nodes (servers) inside of a given Gravity Cluster:

```bsh
$ tsh --cluster=name ls
```

## Removing Clusters

Use `tele rm clusters [cluster-name]` command to start a non-blocking operation
that de-provisions a Cluster.

```bsh
$ tele rm clusters clustername
```

## Controlling Access To Clusters

Ops Center administrators can limit access to Clusters using `where`
expressions in roles and user traits fetched from identity providers.

### Cluster RBAC Using Labels

Sometimes it is necessary to limit users access to a subset
of clusters via Ops Center. Ops Center roles make it possible using
`where` expressions in rules:


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
users have successfully logged into the Ops Center.

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

Users can use `tsh ssh` command to SSH into individual Clusters. For example:

```bsh
$ tsh --cluster=example.east ssh admin@node2
```

You can also copy files using secure file copy (scp):

```bsh
$ tsh --cluster=example.east scp <src> admin@node2:<dest>
```

!!! tip "Tip"
    `tsh` remembers the flags of every successfully executed command. This means
    you do not need to specify `--cluster=name` every time and do something as simple
    as `tsh ssh login@node`


`tsh` supports all the usual flags `ssh` users are used to. You can forward ports,
execute commands and so on. Run `tsh help` for more information.
