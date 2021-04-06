---
title: Installing Cloud Applications on-prem
description: How to install a cloud application into on-premise or air-gapped environment. This also works for deploying into 3rd party cloud accounts.
---


# Installation

### Standalone Offline UI Installation

Standalone installation allows users to install into air-gapped (offline) server
clusters using a self-contained installer tarball.

 * The tarball can be generated on the Ops Center, by using the `Download` link.
 * Alternatively, the installer can be fetched from an Ops Center using `tele pull`,
see [Publishing Applications](pack.md#publishing-applications) section for details.
 * Installer tarball can be built with the `tele build` command, see
  [Packaging Applications](pack.md#packaging-applications) for details.

For example, Application Bundle, `my-app-installer.tar` will contain everything
required to install the Application `my-app`.

To install using the graphical wizard, a Linux desktop with a browser
is required and the target servers need to be reachable on port `3012`.
The node running wizard should have port `61009` reachable to other servers.

Unpacking the tarball will produce the following contents:

```bsh
$ tar -xf my-app-installer.tar
$ ls -lh
-rwxr--r-- 1 user staff 21K  Oct 24 12:01 app.yaml
-rwxr--r-- 1 user staff 56M  Oct 24 12:01 gravity
-rwxr--r-- 1 user staff 256K Oct 24 12:01 gravity.db
-rwxr--r-- 1 user staff 679  Oct 24 12:01 install
-rwxr--r-- 1 user staff 170  Oct 24 12:01 packages
-rw-r--r-- 1 user staff 1.1K Oct 24 12:01 README
-rwxr--r-- 1 user staff 170  Oct 24 12:01 upgrade
-rwxr--r-- 1 user staff 170  Oct 24 12:01 upload
```

The installation wizard is launched by typing `./install` (which executes `gravity wizard` behind the scenes) and will guide the end user
through the installation process.

![Gravity Offline Installer](images/offline-install.svg)

The `gravity wizard` command accepts the following arguments:

Flag      | Description
----------|-------------
`--token` | Secure token which prevents rogue nodes from joining the cluster during installation. A token is generated automatically if unspecified. Carefully pick a hard-to-guess value.
`--advertise-addr` | IP address the installer node should be visible at.
`--service-uid` | _(Optional)_ Service user ID (numeric). See [Service User](pack.md#service-user) for details. A user named `planet` is created automatically if unspecified.
`--service-gid` | _(Optional)_ Service group ID (numeric). See [Service User](pack.md#service-user) for details. A group named `planet` is created automatically if unspecified.

### Standalone Offline CLI Installation

Instead of running a graphical installer, an Application Bundle can be installed
through a CLI which is useful for integrations with configuration management scripts
or other types of infrastructure automation tools. Sometimes this method is called
_"unattended installation"_.

For this to work, the information needed to complete the installation has to be
supplied via the command line flags to the installer.

Let's see how to install a 3-node cluster:

1. Copy the Application Bundle onto all nodes.
1. Execute `./gravity install` on the first node.
1. Execute `./gravity join` on two other nodes.

Below is a sample `./gravity install` command for the first node:

```bsh
node-1$ sudo ./gravity install --advertise-addr=172.28.128.3 --token=XXX --flavor="three"
```

Note the use of `flavor` which, in this case, describes a configuration for 3 nodes
("three" being the name of the flavor from the [Application Manifest](pack.md#application-manifest)).

This will initiate the process of setting up a new cluster for the Application.

Below are corresponding `./gravity join` commands for the remaining nodes (`node-2` and `node-3`):

```bsh
node-2$ sudo ./gravity join 172.28.128.3 --advertise-addr=172.28.128.4 --token=XXX --role="database"
```

```bsh
node-3$ sudo ./gravity join 172.28.128.3 --advertise-addr=172.28.128.5 --token=XXX --role="worker"
```

This instructs the nodes to join a cluster initiated by `gravity install` on the node `172.28.128.3`.
Note, that nodes have also been assigned the desired roles (as defined in the Application Manifest).


The `install` command accepts the following arguments:

Flag      | Description
----------|-------------
`--token` | Secure token which prevents rogue nodes from joining the cluster during installation. A token is generated automatically if unspecified. Carefully pick a hard-to-guess value.
`--advertise-addr` | IP address this node should be visible as. This setting is needed to correctly configure Gravity on every node.
`--role` | _(Optional)_ Application role of the node.
`--cluster` | _(Optional)_ Name of the cluster. Auto-generated if not set.
`--cloud-provider` | _(Optional)_ Enable cloud provider integration: `generic` (no cloud provider integration), `aws` or `gce`. Autodetected if not set.
`--flavor` | _(Optional)_ Application flavor. See [Application Manifest](pack.md#application-manifest) for details.
`--config` | _(Optional)_ File with Kubernetes/Gravity resources to create in the cluster during installation.
`--pod-network-cidr` | _(Optional)_ CIDR range Kubernetes will be allocating node subnets and pod IPs from. Must be a minimum of /16 so Kubernetes is able to allocate /24 to each node. Defaults to `10.244.0.0/16`.
`--service-cidr` | _(Optional)_ CIDR range Kubernetes will be allocating service IPs from. Defaults to `10.100.0.0/16`.
`--state-dir` | _(Optional)_ Directory where all Gravity system data will be kept on this node. Defaults to `/var/lib/gravity`.
`--remote`  | _(Optional)_ Specifies whether the installer node should not be part of the cluster. Defaults to _false_ (i.e. installer node will be part of cluster).
`--service-uid` | _(Optional)_ Service user ID (numeric). See [Service User](pack.md#service-user) for details. A user named `planet` is created automatically if unspecified.
`--service-gid` | _(Optional)_ Service group ID (numeric). See [Service User](pack.md#service-user) for details. A group named `planet` is created automatically if unspecified.
`--dns-zone` | _(Optional)_ Specify an upstream server for the given DNS zone within the cluster. Accepts `<zone>/<nameserver>` format where `<nameserver>` can be either `<ip>` or `<ip>:<port>`. Can be specified multiple times.
`--vxlan-port` | _(Optional)_ Specify custom overlay network port. Default is `8472`.

The `join` command accepts the following arguments:

Flag      | Description
----------|-------------
`--token` | Secure token which prevents rogue nodes from joining the cluster during installation.
`--advertise-addr` | IP address this node should be visible as. This setting is needed to correctly configure Gravity on every node.
`--role` | _(Optional)_ Application role of the node.
`--cloud-provider` | _(Optional)_ Cloud provider integration, `generic`, `aws` or `gce`. Autodetected if not set.
`--mount` | _(Optional)_ Comma-separated list of mount points as <name>:<path>.
`--state-dir` | _(Optional)_ Directory where all Gravity system data will be kept on this node. Defaults to `/var/lib/gravity`.
`--service-uid` | _(Optional)_ Service user ID (numeric). See [Service User](pack.md#service-user) for details. A user named `planet` is created automatically if unspecified.
`--service-gid` | _(Optional)_ Service group ID (numeric). See [Service User](pack.md#service-user) for details. A group named `planet` is created automatically if unspecified.


!!! tip "NOTE"
    `--advertise-addr` must also be set for every node.

!!! tip "NOTE"
    `--token` must specify the same token as given for the `install` command.

!!! tip "NOTE"
    With no `role` specified, the installer uses the first role defined in the Application Manifest.

The result of running these commands will be a functional and self-contained
Kubernetes cluster!

You can learn more in the [Packaging and Deployment](pack.md) section of the
documentation.


### Troubleshooting Installs

The installation process is implemented as a state machine split into multiple steps (phases).
Every time a step fails, the install pauses and allows one to inspect and correct the cause of the failure.
See [Managing an ongoing operation](cluster.md#managing-an-ongoing-operation) section for details
on working with the operation plan.

If the installation has failed, the installer will print a warning and generate a debug report:

```bsh
root$ ./gravity install
Tue Apr 10 13:44:07 UTC	Starting installer
Tue Apr 10 13:44:09 UTC	Preparing for installation
Tue Apr 10 13:44:32 UTC	Installing application my-app:1.0.0-rc.1
Tue Apr 10 13:44:32 UTC	Starting non-interactive install
Tue Apr 10 13:44:32 UTC	Bootstrapping local state
Tue Apr 10 13:44:33 UTC	All agents have connected!
Tue Apr 10 13:44:33 UTC	Starting the installation
Tue Apr 10 13:44:34 UTC	Operation has been created
Tue Apr 10 13:44:35 UTC	Execute preflight checks
Tue Apr 10 13:44:37 UTC	Operation has failed
Tue Apr 10 13:44:37 UTC	Saving debug report to /home/user/crashreport.tgz
```

To inspect installer's progress, use the `gravity plan` command:

```bsh
root$ ./gravity plan
Phase                  Description                                                               State         Requires                  Updated
-----                  -----------                                                               -----         --------                  -------
⚠ checks               Execute preflight checks                                                  Failed        -                         Tue Apr 10 13:44 UTC
* configure            Configure packages for all nodes                                          Unstarted     -                         -
* bootstrap            Bootstrap all nodes                                                       Unstarted     -                         -
  * node-1             Bootstrap master node node-1                                              Unstarted     -                         -
* pull                 Pull configured packages                                                  Unstarted     /configure,/bootstrap     -
  * node-1             Pull packages on master node node-1                                       Unstarted     /configure,/bootstrap     -
* masters              Install system software on master nodes                                   Unstarted     /pull                     -
  * node-1             Install system software on master node node-1                             Unstarted     /pull/node-1
...

Phase Execute preflight checks (/checks) failed.
Error:
server("node-1", 192.168.121.23) failed checks:
	⚠ fs.may_detach_mounts should be set to 1 or pods may get stuck in the Terminating state, see https://www.gravitational.com/docs/faq/#kubernetes-pods-stuck-in-terminating-state
```

After fixing the error (i.e. enabling the kernel parameter in this example), resume the installation with `gravity resume`:

```bsh
root$ sysctl -w fs.may_detach_mounts=1
root$ ./gravity resume
Tue Apr 10 13:55:26 UTC	Executing "/checks" locally
Tue Apr 10 13:55:26 UTC	Running pre-flight checks
Tue Apr 10 13:55:28 UTC	Executing "/configure" locally
Tue Apr 10 13:55:28 UTC	Configuring cluster packages
Tue Apr 10 13:55:32 UTC	Executing "/bootstrap/node-1" locally
Tue Apr 10 13:55:32 UTC	Configuring system directories
Tue Apr 10 13:55:35 UTC	Configuring application-specific volumes
Tue Apr 10 13:55:36 UTC	Executing "/pull/node-1" locally
Tue Apr 10 13:55:36 UTC	Pulling user application
Tue Apr 10 13:55:46 UTC	Still pulling user application (10 seconds elapsed)
...
Tue Apr 10 14:01:07 UTC	Executing "/app/my-app" locally
Tue Apr 10 14:01:08 UTC	Executing "/election" locally
Tue Apr 10 14:01:08 UTC	Enable leader elections
Tue Apr 10 14:01:09 UTC	Executing install phase "/" finished in 5 minutes

```

To abort an installation, press Ctrl+C two times in a row in the terminal.
If the operation is aborted, the partial install state will be automatically removed.

Aborting a join (and not the installer process), will only prevent this node from joining.

!!! warning "Aborting a join"
    Aborting a join might result in installation failure.
    If the operation was aborted to correct a configuration error, it can be restarted once the error
    has been fixed.


Installer processes (that includes the main installer process and all agent processes that join additional nodes to the cluster)
execute inside a systemd service so the operation will continue in background even if the terminal session has timed out.

In order to reconnect to the installer, issue `./gravity resume`.

!!! warning "Installer state directory"
    It is important to run install commands from the directory with the original gravity binary.
    This directory contains the temporary operation state required for all commands to work properly.


#### Under the hood

Each node runs a systemd service (and may even run two). The main installer service executes
from a service called `gravity-installer` (agent nodes have a corresponding `gravity-agent`). The services are configured
from the temporary state sub-directory called `.gravity` in the directory where the installer has been extracted.

To manually remove installation state:

```bash
# Stop the installer service (or gravity-agent - depending on the node):
root$ systemctl stop gravity-installer.service
root$ systemctl disable gravity-installer.service
# Remove the state directory
root$ rm -r .gravity
```


## Installing on Google Compute Engine

!!! note
    GCE cloud provider integration is supported starting from Gravity
    version `5.1.0-alpha.1`.

Before installation make sure that GCE instances used for installation
satisfy all of Gravity [system requirements](requirements.md). In addition to these
generic requirements GCE nodes also must be configured in the following way to
ensure proper cloud provider integration:

* Network interface must have IP forwarding turned on. It is required for the
overlay network to work properly.
* Instances must be assigned a [network tag](https://cloud.google.com/vpc/docs/add-remove-network-tags)
matching the name of the cluster. It is required to ensure that created load
balancers discover proper instances.
* Cloud API access scopes must include read/write permissions for Compute Engine.

Once the nodes have been properly configured, copy the installer tarball and
launch installation as described above:

```bsh
node1$ sudo ./gravity install --advertise-addr=<addr> --token=<token> --cluster=<cluster> --cloud-provider=gce
node2$ sudo ./gravity join <installer-addr> --advertise-addr=<addr> --token=<token> --cloud-provider=gce
```

Note that the `--cloud-provider` flag is optional and, if unspecified, will be
auto-detected if install/join process is running on a GCE instance.
