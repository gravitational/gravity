---
title: Gravity System Requirements
description: Description of the system requirements for running Kubernetes in production
---

# System Requirements

This chapter outlines the system requirements for Linux hosts to be able to run
Kubernetes and Gravity.

## Linux Distributions

Gravity officially supports the following Linux distributions:

| Linux Distribution       | Version             | Docker Storage Drivers                |
|--------------------------|---------------------|---------------------------------------|
| Red Hat Enterprise Linux | 7.4-7.9, 8.0-8.3*   | `overlay`, `overlay2`                 |
| CentOS                   | 7.2-7.9, 8.0-8.3*   | `overlay`, `overlay2`                 |
| Debian                   | 9, 10               | `overlay`, `overlay2`                 |
| Ubuntu                   | 16.04, 18.04, 20.04 | `overlay`, `overlay2`                 |
| Ubuntu-Core              | 16.04               | `overlay`, `overlay2`                 |
| openSuse                 | 12-SP2 to 12-SP5    | `overlay`, `overlay2`                 |
| Suse Linux Enterprise    | 12-SP2 to 12-SP5    | `overlay`, `overlay2`                 |
| Amazon Linux             | 2                   | `overlay`, `overlay2`                 |

!!! note
    * Gravity 6.1 does not install on CentOS & Red Hat Enterprise Linux 8+ with SELinux
    enforcing due to [#2009](https://github.com/gravitational/gravity/issues/2009).
    To install on CentOS & Red Hat Enterprise Linux 8+, please use Gravity 7.0+ or disable SELinux.

### Identifying OS Distributions In Manifest

You can restrict a cluster image to certain Linux distributions via cluster
image manifest.

During infrastructure validation, the values from `name` attribute are matched
against `ID` attribute of the `/etc/os-release` file on a host.

The following table lists all officially supported Linux distributions and how they can be
specified in the manifest:

| Distribution Name        | ID                         | Version             |
|--------------------------|----------------------------|---------------------|
| Red Hat Enterprise Linux | rhel                       | 7.4-7.9, 8.0-8.3    |
| CentOS                   | centos                     | 7.2-7.9, 8.0-8.3    |
| Debian                   | debian                     | 9                   |
| Ubuntu                   | ubuntu                     | 16.04, 18.04, 20.04 |
| Ubuntu-Core              | ubuntu                     | 16.04               |
| openSuse                 | suse, opensuse, opensuse-* | 12-SP2 to 12-SP5    |
| Suse Linux Enterprise    | sles, sles_sap             | 12-SP2 to 12-SP5    |
| Amazon Linux             | amz                        | 2                   |

For example, to specify openSUSE as a dependency and support all services packs:

```yaml
nodeProfiles:
 - name: profile
   requirements:
     os:
      - name: suse # openSUSE
        versions: ["12-SP2", "12-SP3", "12-SP4", "12-SP5"]
     os:
      - name: opensuse-tumbleweed # specific openSUSE distribution
        versions: ["12-SP2", "12-SP3", "12-SP4", "12-SP5"]
```

!!! note
    During OS distribution validation, the installer considers multiple sources of release information
    when identifying the host OS. While the `/etc/os-release` file is the standard way to identify a modern
    Linux distribution, not all distributions are up to date and maintain release metadata in
    distribution-specific files.
    Additionally, the installer will use `lsb_release` if installed to query the release metadata.

## Hardware

Hardware requirements are specific to the workload running in the cluster but
the following guidelines are recommended to support bare Gravity cluster
installations.

| Role   | CPU    | RAM  | Disk                                                 |
|--------|--------|------|------------------------------------------------------|
| master | 2 vCPU | 4GiB | 100GiB SSD for Gravity data, 50GiB SSD for etcd data |
| node   | 1 vCPU | 2GiB | 100GiB SSD for Gravity data                          |

## Network

#### Network backends

Gravity supports two networking backends in production:

* VPC and routing tables based network for `AWS` cloud provider.
* VXLAN based network for `generic` provider to be used on generic linux installations.

See [Cluster Image Manifest](pack.md#image-manifest) section for details on how to select
the network type.

#### Air-gapped installs

Gravity Clusters do not need internet access to operate by default and ships all containers and binaries
with every install or update.

### Installer Ports

These ports are required during initial installation and can be closed after the install is complete:

| Port                     | Protocol  | Description               |
|--------------------------|-----------|---------------------------|
| 61009                    | HTTPS     | Install wizard UI access  |
| 61008-61010, 61022-61024 | HTTPS     | Installer agent ports     |
| 4242                     | TCP       | Bandwidth checker utility |

### Default Subnets

By default Gravity clusters are configured to use the following network subnets:

| Subnet          | Description               |
|-----------------|---------------------------|
| 10.244.0.0/16   | Pod IPv4 addresses        |
| 10.100.0.0/16   | Services IPv4 addresses   |

Both subnets are customizable via installer flags as explained in the [Installation guide](installation.md)

### Cluster Ports

These ports are used for Cluster operation and should be open between cluster nodes:

| Port                    | Protocol - Layer 4 | Protocol - Layer 5   | Source      | Destination | Description                              |
|-------------------------|--------------------|----------------------|-------------|-------------|------------------------------------------|
| 53                      | TCP/UDP            | DNS                  | localhost   | localhost   | Internal cluster DNS                     |
| 8472                    | UDP                | VXLAN                | all         | all         | Overlay network                          |
| 7496                    | TCP/UDP            | HTTPs                | all         | all         | Serf (Health check agents) peer to peer  |
| 7373                    | TCP                | RPC                  | localhost   | localhost   | Serf RPC - peer to peer                  |
| 7575                    | TCP                | gRPC                 | all         | all         | Cluster status gRPC API                  |
| 2379, 2380, 4001, 7001  | TCP                | HTTPS                | all         | controllers | Etcd server communications               |
| 6443                    | TCP                | HTTPS                | all         | controllers | Kubernetes API Server                    |
| 30000 - 32767           | N/A                | Application specific | all         | all         | Kubernetes internal services range       |
| 10248 - 10250, 10255    | TCP                | HTTPS                | all         | all         | Kubernetes components                    |
| 5000                    | TCP                | HTTPS                | all         | controllers | Docker registry                          |
| 3022-3025               | TCP                | SSH                  | all         | all         | Teleport internal SSH control panel      |
| 3080                    | TCP                | HTTPS                | ext         | controllers | Teleport Web  UI                         |
| 3008-3012, 6060         | TCP                | HTTPS / gRPC         | all         | all         | Internal Gravity services                |
| 32009                   | TCP                | HTTPS                | ext         | controllers | Gravity Cluster/OpsCenter Admin UI (ext) |
| 32009                   | TCP                | HTTPS                | controllers | all         | Gravity internal API                     |
| 3012                    | TCP                | HTTPS                | all         | all         | Gravity RPC agent                        |

!!! note "Source/Destination Legend"
    * all - Any node which is a member of the cluster
    * ext - Any source outside the cluster
    * localhost - The port is only used within the host where the request started
    * controllers - Nodes which are designated "controller" (aka "master") role

!!! note "Custom vxlan port"
    If the default overlay network port (`8472`) was changed by supplying
    `--vxlan-port` flag to `gravity install` command, it will be checked
    instead of the default one.

## Kernel Modules

The following kernel modules are essential for Kubernetes cluster to properly
function.

!!! note
    The Gravity installer includes a set of pre-flight checks that alert the
    user if any of the required modules are not loaded.

### br_netfilter module

The bridge netfilter kernel module is required for Kubernetes iptables-based
proxy to work correctly. Kernels prior to version 3.18 had this module built
in:

```bsh
root$ cat /lib/modules/$(uname -r)/modules.builtin | grep netfilter
```

Starting from kernel 3.18 it became a separate module. To check that it is
loaded run:

```bsh
root$ lsmod | grep netfilter
br_netfilter           24576  0
```

If the above command didn't produce any result, then the module is not loaded.
Use the following commands to load the module and make sure it is loaded on boot:

```bsh
root$ modprobe br_netfilter
root$ echo 'br_netfilter' > /etc/modules-load.d/netfilter.conf
```

When the module is loaded, check the iptables setting and, if required, enable
it as well:

```bsh
root$ sysctl net.bridge.bridge-nf-call-iptables
net.bridge.bridge-nf-call-iptables = 0
root$ sysctl -w net.bridge.bridge-nf-call-iptables=1
root$ echo net.bridge.bridge-nf-call-iptables=1 >> /etc/sysctl.d/10-bridge-nf-call-iptables.conf
```

Note, that in CentOS 7.2, the module is called `bridge`.

See our blog post [Troubleshooting Kubernetes Networking](https://gravitational.com/blog/troubleshooting-kubernetes-networking/)
for more information about possible network-related issues.

### overlay module

The overlay kernel module is required if the application is using overlay or
overlay2 Docker storage driver (see [Cluster Image Manifest](pack.md#image-manifest))
for information on how to configure the storage driver). To check that it's
loaded:

```bsh
root$ lsmod | grep overlay
overlay                49152  29
```

To load the module and make it persist across reboots:

```bsh
root$ modprobe overlay
root$ echo 'overlay' > /etc/modules-load.d/overlay.conf
```

### ebtable_filter module

This kernel module is required if the application is configuring Hairpin NAT
(see [Hairpin NAT](cluster.md#hairpin-nat) to enable services to load-balance to themselves
and setting up docker bridge in "promiscuous-bridge" mode.

To see if the module is loaded:

```bsh
root$ lsmod | grep ebtable
ebtable_filter         12827  0
ebtables               35009  1 ebtable_filter
```

To load the module and make it persist across reboots:

```bsh
root$ modprobe ebtable_filter
root$ echo 'ebtable_filter' > /etc/modules-load.d/network.conf
```

### iptables modules

The following modules also need to be loaded to make sure firewall rules
that Kubernetes sets up function properly:

```bsh
root$ modprobe ip_tables
root$ modprobe iptable_filter
root$ modprobe iptable_nat
```

### Kernel Module Matrix

Following table summarizes the required kernel modules per OS distribution.
Gravity requires that these modules are loaded prior to installation.

| Linux Distribution                   | Version             | Modules                                         |
|--------------------------------------|---------------------|-------------------------------------------------|
| CentOS                               | 7.2                 | bridge, ebtable_filter, iptables, overlay       |
| CentOS                               | 7.3-7.9, 8.0-8.3    | br_netfilter, ebtable_filter, iptables, overlay |
| RedHat Linux                         | 7.3-7.9, 8.0-8.3    | br_netfilter, ebtable_filter, iptables, overlay |
| Debian                               | 9, 10               | br_netfilter, ebtable_filter, iptables, overlay |
| Ubuntu                               | 16.04, 18.04, 20.04 | br_netfilter, ebtable_filter, iptables, overlay |
| Ubuntu-Core                          | 16.04               | br_netfilter, ebtable_filter, iptables, overlay |
| Suse Linux (openSUSE and Enterprise) | 12-SP2 to 12-SP5    | br_netfilter, ebtable_filter, iptables, overlay |

### Inotify watches

Kubelet configures multiple inotify watches per container so it's recommended
to increase the `max_user_watches` kernel parameter. Gravity's built-in
monitoring system checks for inotify watches exhaustion but we recommended setting
it to some large value to avoid running out of limits:

```bsh
$ sysctl -w fs.inotify.max_user_watches=1048576
```

To make the change persistent so it survives the node reboots, set the setting
in a file inside `/etc/sysctl.d` directory, for example:

```bsh
$ cat /etc/sysctl.d/inotify.conf
fs.inotify.max_user_watches=1048576
```

See the [sysctl.d man page](https://www.freedesktop.org/software/systemd/man/sysctl.d.html)
for more information about applying the settings.

## Time Drift

Being a distributed system Gravity/Kubernetes clusters require the system time
to be kept in sync between the cluster nodes.

When installing a multi-node cluster or joining a new node to the cluster,
Gravity will execute a preflight check to make sure that time drift between
the nodes doesn't exceed `300ms` and will fail the operation otherwise. The
same check is executed on the deployed clusters and if any node's time drifts
off by `300ms` or more, the cluster will move to a degraded state.

To avoid time drift issues it is highly recommended to run time synchronization
software on the cluster nodes such as `ntpd` or `chronyd`.

## AWS IAM Policy

When deploying on AWS, the supplied keys should have a set of EC2/ELB/IAM permissions
to be able to provision required infrastructure on your AWS account.

<details><summary>Click here to view an example IAM policy.</summary>
```js
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "autoscaling:*",
                "ec2:*",
                "elasticloadbalancing:*",
                "iam:AddRoleToInstanceProfile",
                "iam:CreateInstanceProfile",
                "iam:CreateRole",
                "iam:DeleteInstanceProfile",
                "iam:DeleteRole",
                "iam:DeleteRolePolicy",
                "iam:GetInstanceProfile",
                "iam:GetRole",
                "iam:GetRolePolicy",
                "iam:ListInstanceProfiles",
                "iam:ListInstanceProfilesForRole",
                "iam:ListRoles",
                "iam:PassRole",
                "iam:PutRolePolicy",
                "iam:RemoveRoleFromInstanceProfile",
                "kms:DescribeKey",
                "kms:ListAliases",
                "kms:ListKeys",
                "s3:*",
                "sqs:ChangeMessageVisibility",
                "sqs:ChangeMessageVisibilityBatch",
                "sqs:CreateQueue",
                "sqs:DeleteMessage",
                "sqs:DeleteMessageBatch",
                "sqs:DeleteQueue",
                "sqs:GetQueueAttributes",
                "sqs:GetQueueUrl",
                "sqs:ListDeadLetterSourceQueues",
                "sqs:ListQueueTags",
                "sqs:ListQueues",
                "sqs:PurgeQueue",
                "sqs:ReceiveMessage",
                "sqs:SendMessage",
                "sqs:SendMessageBatch",
                "sqs:SetQueueAttributes",
                "sqs:TagQueue",
                "sqs:UntagQueue",
                "ssm:DeleteParameter",
                "ssm:DeleteParameters",
                "ssm:DescribeParameters",
                "ssm:GetParameter",
                "ssm:GetParameters",
                "ssm:ListTagsForResource",
                "ssm:PutParameter"
            ],
            "Resource": "*"
        }
    ]
}
```
</details>

!!! note
    The exact list of required permissions may depend on the provisioner you're
    using. The IAM policy shown above is an example for
    [gravitational/provisioner](https://github.com/gravitational/provisioner).

## GCE IAM Policy

When installing on Google Compute Engine instances with cloud integration enabled,
the instances should be granted R/W access to Compute Engine API in order to be
able to provision routes, load balancers, persistent volumes and use other cloud
integration APIs.

If instead of granting the full access to Compute Engine you'd like to set up a
more limited access, create a [service account](https://cloud.google.com/iam/docs/service-accounts)
with permissions listed below.

!!! note
    The list of permissions may be not exhaustive, the applications deployed in the
    cluster may require a broader set of permissions.

<details><summary>Click here to view required GCE permissions.</summary>
```
compute.addresses.create
compute.addresses.delete
compute.addresses.get
compute.addresses.use
compute.disks.create
compute.disks.delete
compute.disks.get
compute.disks.resize
compute.disks.use
compute.firewalls.create
compute.firewalls.get
compute.firewalls.update
compute.forwardingRules.create
compute.forwardingRules.get
compute.httpHealthChecks.create
compute.httpHealthChecks.get
compute.httpHealthChecks.useReadOnly
compute.instances.attachDisk
compute.instances.detachDisk
compute.instances.get
compute.instances.list
compute.instances.use
compute.networks.get
compute.networks.updatePolicy
compute.routes.create
compute.routes.delete
compute.routes.get
compute.subnetworks.list
compute.targetPools.create
compute.targetPools.get
compute.targetPools.use
compute.zoneOperations.get
compute.zones.list
iam.serviceAccounts.actAs
```
</details>

## Etcd Disk

Gravity Clusters make high use of Etcd, both for the Kubernetes cluster and for
the application's own bookkeeping with respect to e.g. deployed clusters' health
and reachability. As a result, it is helpful to have a reliable, performance
isolated disk.

To achieve this, by default Gravity master nodes look for a disk mounted at
`/var/lib/gravity/planet/etcd`. We recommend each master mounts a dedicated disk there,
`ext4` formatted with at least 50GiB of free space. A reasonably high performance
SSD is preferred. On AWS, we recommend an `io1` class EBS volume with at least
1500 provisioned IOPS.

If your Etcd disk is `xvdf`, you can have the following `/etc/fstab` entry to
make sure it's mounted upon machine startup:

```
/dev/xvdf  /var/lib/gravity/planet/etcd  ext4  defaults   0  2
```

### Etcd Disk Performance Requirements

Prior to installation, Gravity will perform a preflight check to assess performance
characteristics of the disk used for etcd data on each master node.

The check uses `fio` tool and looks at the following benchmarks:

* Sequential write IOPS:
    * If the detected IOPS is below `50`, a warning is triggered.
    * If the detected IOPS is below `10`, a failure is triggered
* Fsync latency:
    * If the detected latency is higher than `50ms`, a warning is triggered.
    * If the detected latency is higher than `150ms`, a failure is triggered.

If necessary, these thresholds can be overridden at install/join time using
[environment variables](installation.md#environment-variables).
