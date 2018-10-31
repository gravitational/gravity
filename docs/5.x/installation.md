# Installation

### Ops Center-driven Installation

An Ops Center can be used for application deployments.
In order to deploy applications from an Ops Center, Application Bundles
are [published](pack/#publishing-applications) to an Ops Center.
Once published, they become available for deployment.
An Application can be deployed either directly from an Ops Center or
via a one-time installation link.
Once the cluster is up and running, the installer will establish a remote access
channel for maintenance:

![Ops Center Install](images/opscenter-install.svg?style=grv-image-center-md)

!!! tip "NOTE":
    The end users can close the remote channel and disconnect their Application
	from the Ops Center.


### Standalone Offline UI Installation

Standalone installation allows users to install into air-gapped (offline) server
clusters using a self-contained installer tarball.

 * The tarball can be generated on the Ops Center, by using the `Download` link.
 * Alternatively, the installer can be fetched from an Ops Center using `tele pull`,
see [Publishing Applications](pack/#publishing-applications) section for details.
 * Installer tarball can be built with the `tele build` command, see
  [Packaging Applications](pack#packaging-applications) for details.

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

The installation wizard is launched by typing `./install` and will guide the end user
through the installation process.

![Gravity Offline Installer](images/offline-install.svg?style=grv-image-center-md)

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
("three" being the name of the flavor from the [Application Manifest](pack#application-manifest)).

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
`--token` | Secure token which prevents rogue nodes from joining the cluster during installation. Carefully pick a hard-to-guess value.
`--advertise-addr` | IP address this node should be visible as. This setting is needed to correctly configure Gravity on every node.
`--role` | _(Optional)_ Application role of the node.
`--cluster` | _(Optional)_ Name of the cluster. Auto-generated if not set.
`--cloud-provider` | _(Optional)_ Enable cloud provider integration: `generic` (no cloud provider integration), `aws` or `gce`. Autodetected if not set.
`--flavor` | _(Optional)_ Application flavor. See [Application Manifest](pack/#application-manifest) for details.
`--config` | _(Optional)_ File with Kubernetes resources to create in the cluster during installation.
`--pod-network-cidr` | _(Optional)_ CIDR range Kubernetes will be allocating node subnets and pod IPs from. Must be a minimum of /16 so Kubernetes is able to allocate /24 to each node. Defaults to `10.244.0.0/16`.
`--service-cidr` | _(Optional)_ CIDR range Kubernetes will be allocating service IPs from. Defaults to `10.100.0.0/16`.
`--wizard` | _(Optional)_ Start the installation wizard.
`--state-dir` | _(Optional)_ Directory where all Gravity system data will be kept on this node. Defaults to `/var/lib/gravity`.
`--service-uid` | _(Optional)_ Service user ID (numeric). See [Service User](pack/#service-user) for details. A user named `planet` is created automatically if unspecified.
`--service-gid` | _(Optional)_ Service group ID (numeric). See [Service User](pack/#service-user) for details. A group named `planet` is created automatically if unspecified.
`--dns-zone` | _(Optional)_ Specify an upstream server for the given DNS zone within the cluster. Accepts `<zone>/<nameserver>` format where `<nameserver>` can be either `<ip>` or `<ip>:<port>`. Can be specified multiple times.
`--vxlan-port` | _(Optional)_ Specify custom overlay network port. Default is `8472`.

The `join` command accepts the following arguments:

Flag      | Description
----------|-------------
`--token` | Secure token which prevents rogue nodes from joining the cluster during installation. Carefully pick a hard-to-guess value.
`--advertise-addr` | IP address this node should be visible as. This setting is needed to correctly configure Gravity on every node.
`--role` | _(Optional)_ Application role of the node.
`--cloud-provider` | _(Optional)_ Cloud provider integration, `generic` or `aws`. Autodetected if not set.
`--mounts` | _(Optional)_ Comma-separated list of mount points as <name>:<path>.
`--state-dir` | _(Optional)_ Directory where all Gravity system data will be kept on this node. Defaults to `/var/lib/gravity`.
`--service-uid` | _(Optional)_ Service user ID (numeric). See [Service User](pack/#service-user) for details. A user named `planet` is created automatically if unspecified.
`--service-gid` | _(Optional)_ Service group ID (numeric). See [Service User](pack/#service-user) for details. A group named `planet` is created automatically if unspecified.


!!! tip "NOTE":
    `--advertise-addr` must also be set for every node, and the same value for `--token` must be used.

!!! tip "NOTE":
    With no `role` specified, the installer uses the first role defined in the Application Manifest.

The result of running these commands will be a functional and self-contained
Kubernetes cluster!

You can learn more in the [Packaging and Deployment](pack.md) section of the
documentation.


### Troubleshooting Installs

The installation process is implemented as a state machine split into multiple steps (phases).
Every time a step fails, the install pauses and allows one to inspect and correct the cause of the failure.

If the installation has failed, the installer will print a warning and pause:

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
Tue Apr 10 13:44:37 UTC	Installation failed in 4.985481556s, check ./telekube-install.log
---
Installer process will keep running so you can inspect the operation plan using
`gravity plan` command, see what failed and continue plan execution manually
using `gravity install --phase=<phase-id>` command after fixing the problem.
Once no longer needed, this process can be shutdown using Ctrl-C.
```

To inspect installer's progress, use the `plan` command:

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

After fixing the error (i.e. enabling the kernel parameter in this example), resume the installation:

```bsh
root$ sysctl -w fs.may_detach_mounts=1
root$ ./gravity install --resume
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

The following CLI flags are useful to manage the install operation:


Flag      | Description
----------|-------------
`--phase` | Specifies the name of the step to execute. Use `gravity plan` to display the list of all steps.
`--force` | Force execution of the step even it is already in-progress.
`--resume` | Resume operation after the failure. The operation is resumed from the step that failed last.
`--manual` | Launch operation in manual mode.

## Installing on Google Compute Engine

!!! note:
    GCE cloud provider integration is supported starting from Gravity
    version `5.1.0-alpha.1`.

Before installation make sure that GCE instances used for installation
satisfy all of Gravity [system requirements](/requirements). In addition to these
generic requirements GCE nodes also must be configured in the following way to
ensure proper cloud provider integration:

* Network interface must have IP forwarding turned on. It is required for the
overlay network to work properly.
* Instances must be assigned a [network tag](https://cloud.google.com/vpc/docs/add-remove-network-tags)
matching the name of the cluster. It is required to ensure that created load
balancers discover proper instances.
* Cloud API access scopes must include RW permissions for Compute Engine.

Once the nodes have been properly configured, copy the installer tarball and
launch installation as described above:

```bsh
node1$ sudo ./gravity install --advertise-addr=<addr> --token=<token> --cluster=<cluster> --cloud-provider=gce
node2$ sudo ./gravity join <installer-addr> --advertise-addr=<addr> --token=<token> --cloud-provider=gce
```

Note that the `--cloud-provider` flag is optional and, if unspecified, will be
auto-detected if install/join process is running on a GCE instance.
