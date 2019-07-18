# Installation

In this chapter we'll cover the process of creating clusters from cluster
images.  Just like a virtual machine (VM) image similar to AWS AMI can be used
to create machine instances, Gravity cluster images can be used to create
cluster instances.

#### Deployment Methods

Gravity supports two methods to create clusters from cluster images:

* Command Line (CLI) method, suitable for scripting.
* Graphical installer which serves a web UI, assisting users in cluster
  creation.

Both installation methods allow users to create a new Kubernetes cluster from a
cluster image on arbitrary Linux hosts. Because a cluster image has no external
dependencies, both installation methods will work even in air-gapped server
rooms.

#### Prerequisites

Every cluster image created with Gravity contains everything you need to 
create a production-ready cluster, but there are still some pre-requisites
to be met:

* You should have a valid cluster image, i.e. a `.tar` file. See [building images](/pack/) 
  chapter for information on how to build one.
* One or more Linux hosts with a compatible kernel. They can be bare metal hosts,
  compute instances on any cloud provider, virtual machines on a private cloud, 
  etc. Consult with [system requirements](/requirements/) to determine if your Linux hosts
  are compatible with Gravity.
* The hosts should be clean, i.e. they shouldn't contain any container
  orchestrator on them, or even Docker.
* The hosts should be able to connect to each other, i.e. to be on the same
  private network.
* You should have the ability to create DNS entries for public access points.
* You you should be able to obtain a valid SSL/TLS certificates for HTTPS.


## CLI Installation

To create a new instance of a cluster image (i.e. a Kubernetes cluster) via 
command line, you must do the following:

1. First, copy the cluster image file onto all nodes.
2. Untar it on all nodes.
3. Pick a "master node", i.e. the node which will serve as the initial
   Kubernetes master.
4. Execute `./gravity install` on the master node.
5. Execute `./gravity join` on two other nodes.

Copying files around is easy, so let's take a deeper look into steps 2 through 5.
Once the cluster image is unpacked, it is going to look similar to this on each node:

```bash
$ tar -xf cluster-image.tar
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

Next, bootstrap the master node:

```bash
# execute this on the master node, which in this case has an IP of 10.1.10.1
$ sudo ./gravity install --advertise-addr=10.1.10.1 --token=XXX --flavor="three"
```

* Note the use of `--flavor` argument which selects a cluster configuration for 3
  nodes, borrowing from the example shown in the [Image Manifest](/pack/#image-manifest) section.
* You have to select an arbitrary, hard to guess secret for `--token` and remember this
  value. It will be used to securely add additional nodes to the cluster.
* Other nodes must be able to connect to the master node via `10.1.10.1`. Make sure
  the installer ports are not blocked, see "Installer Ports" section in [System Requirements](/requirements/#network) chapter.

Next, start adding remaining nodes to the cluster:

```bash
# must be executed on the node which you want to be the "database":
$ sudo ./gravity join 10.1.10.1 --advertise-addr=10.1.10.2 --token=XXX --role="database"
```

```bash
# must be executed on the node which you want to be the "worker":
$ sudo ./gravity join 10.1.10.1 --advertise-addr=10.1.10.3 --token=XXX --role="worker"
```

!!! tip:
    The node roles in the example above are borrowed from the image manifest documented 
    in [Building Cluster Images](/pack/#image-manifest) section. The use of `--role` 
    argument is optional if the cluster image manifest did not contain node roles.

`gravity join` command will connect the worker and the database nodes to the master and you
will have a fully functioning, production-ready Kubernetes cluster up and running.

Execute `gravity install --help` to see the list of supported command line arguments, but
the most frequently used ones are listed below:

Flag               | Description
-------------------|-------------
`--token`          | Secure token which prevents rogue nodes from joining the cluster during installation. Carefully pick a hard-to-guess value.
`--advertise-addr` | The IP address this node should be visible as. **This setting is mandatory** to correctly configure Kubernetes on every node.
`--role`           | _(Optional)_ Application role of the node.
`--cluster`        | _(Optional)_ Name of the cluster. Auto-generated if not set.
`--cloud-provider` | _(Optional)_ Enable cloud provider integration: `generic` (no cloud provider integration), `aws` or `gce`. Autodetected if not set.
`--flavor`         | _(Optional)_ Application flavor. See [Image Manifest](pack/#image-manifest) for details.
`--config`         | _(Optional)_ File with Kubernetes/Gravity resources to create in the cluster during installation.
`--pod-network-cidr` | _(Optional)_ CIDR range Kubernetes will be allocating node subnets and pod IPs from. Must be a minimum of /16 so Kubernetes is able to allocate /24 to each node. Defaults to `10.244.0.0/16`.
`--service-cidr`     | _(Optional)_ CIDR range Kubernetes will be allocating service IPs from. Defaults to `10.100.0.0/16`.
`--wizard`           | _(Optional)_ Start the installation wizard.
`--state-dir`        | _(Optional)_ Directory where all Gravity system data will be kept on this node. Defaults to `/var/lib/gravity`.
`--service-uid`      | _(Optional)_ Service user ID (numeric). See [Service User](pack/#service-user) for details. A user named `planet` is created automatically if unspecified.
`--service-gid`      | _(Optional)_ Service group ID (numeric). See [Service User](pack/#service-user) for details. A group named `planet` is created automatically if unspecified.
`--dns-zone`         | _(Optional)_ Specify an upstream server for the given DNS zone within the cluster. Accepts `<zone>/<nameserver>` format where `<nameserver>` can be either `<ip>` or `<ip>:<port>`. Can be specified multiple times.
`--vxlan-port`       | _(Optional)_ Specify custom overlay network port. Default is `8472`.
`--exclude-from-cluster` | _(Optional)_ Excludes this node from the cluster, i.e. allows to bootstrap the cluster from a developer's laptop, for example. In this case the Kubernetes master will be chosen randomly.

`gravity join` command accepts the following arguments:

Flag               | Description
-------------------|-------------
`--token`          | Secure token which prevents rogue nodes from joining the cluster during installation. Carefully pick a hard-to-guess value.
`--advertise-addr` | The IP address this node should be visible as. **This setting is mandatory** to correctly configure Kubernetes on every node.
`--role`           | _(Optional)_ Application role of the node.
`--cloud-provider` | _(Optional)_ Cloud provider integration, `generic` or `aws`. Autodetected if not set.
`--mounts`         | _(Optional)_ Comma-separated list of mount points as <name>:<path>.
`--state-dir`      | _(Optional)_ Directory where all Gravity system data will be kept on this node. Defaults to `/var/lib/gravity`.
`--service-uid`    | _(Optional)_ Service user ID (numeric). See [Service User](pack/#service-user) for details. A user named `planet` is created automatically if unspecified.
`--service-gid`    | _(Optional)_ Service group ID (numeric). See [Service User](pack/#service-user) for details. A group named `planet` is created automatically if unspecified.


The result of running these commands will be a functional and self-contained
Kubernetes cluster!

## Web-based Installation

Web-based installation allows more interactive user experience, i.e. instead of
specifying installation parameters via CLI arguments, users can follow the
installation wizard using a web browser.

To illustrate how this works, let's use the same set of assumptions from the CLI 
installation section above, i.e. 

* There are 3 clean Linux machines available. One will be the "master" and the
  other two are "database" and "worker" respectively.
* A user has remote access to all 3 machines, most likely via SSH.
* A user also has their personal laptop with a web browser.

To install using the graphical wizard, a Linux computer with a browser is
required and the target servers need to be reachable via port `3012`. The node
running the wizard must have its port `61009` accessible by other servers.

The installation wizard is launched by typing `./install` script and will guide
the end user through the installation process.

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


## AWS

AWS is the most frequently used infrastructure for Gravity clusters, that's why
AWS is natively supported, i.e. the behavior of `gravity` CLI command will
change when it detects that it's running on a AWS instance.

In practice, this means that Kubernetes networking will be configured with the AWS
native network features.

## Generic Linux Hosts

In order to reliably run in any environment, Gravity aims to be infrastructure
and cloud-agnostic. Gravity makes no assumption about the nature of the network
or either the hosts are virtualized or bare metal.

## Azure

Gravity can be successfully deployed into an Azure environment using the same,
generic approach as with any Generic Linux Hosts.

## Google Compute Engine

Before installation make sure that GCE instances used for installation
satisfy all of Gravity [system requirements](/requirements). In addition to these
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

