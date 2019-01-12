# Introduction

This Quick Start Guide will help you to quickly evaluate Gravity by using a
real world example. We believe in learning by doing!

This guide will use [Mattermost](https://www.mattermost.org/), an open sourced
chat application for teams, as a sample application. Mattermost represents a
fairly typical web application and consists of an HTTP request handling process
which connects to a PostgreSQL instance.

This Quick Start Guide will walk you through the process of packaging
Mattermost for private deployments. Before we start, please take a look at the
[Gravity Overview](overview.md) to get familiar with basic concepts of the
Gravity system.

## System Requirements

Gravity is a Kubernetes packaging solution so it only runs on computers capable
of running Kubernetes. For this QuickStart you will need:

* A x86_64 GNU/Linux Linux machine. We recommend taking a look at the [list of supported distributions](requirements/#distributions).
* [Docker](https://get.docker.io) version 1.8 or newer. Run `docker info` before continuing to make sure 
  you have Docker up and running. 
* You must be a member of the `docker` group. Run `groups` command to make sure
  `docker` group is listed.
* You must install [Helm](https://docs.helm.sh/using_helm/#installing-helm).
* You must have `git` installed to clone the example application repo.
* You must have `sudo` privileges.

Obviously, all of this can be accomplished on a Windows or MacOS machine if it's
capable of running a Linux VM which satisfies the criteria above.

## Getting the Tools

Start by [downloading Gravity](https://gravitational.com/gravity/download/) and
unpacking the archive, you should see:

```
$ ls -l
-rwxr-xr-x 1 user user      128 Dec  3 13:07 install.sh
-rwxr-xr-x 1 user user 50562960 Dec  3 13:07 tele
-rwxr-xr-x 1 user user 21417992 Dec  3 13:07 tsh
```

You can execute `install.sh` if you want to copy `tele` and `tsh` binaries to
`/usr/local/bin/`.

* `tele`: the CLI tool which is used for creating _application bundles_.
* `tsh`: the [Teleport client](https://gravitational.com/teleport/) for establishing 
   SSH connections to remote Kubernetes clusters. This QuickStart does not use it.

Try typing:

```
$ tele version
Edition:	open-source
Version:	5.2.4
Git Commit:	708a1f155da4633774281bf8660c7e6cca6e0ff1
```

Next, let's clone the sample Git repository which contains Kubernetes resources for
[Mattermost](https://www.mattermost.org/), the open source, self-hosted Slack alternative 
which we are using in this tutorial as a sample application:

```bsh
$ git clone https://github.com/gravitational/quickstart.git
$ cd quickstart
```

## Building Application Bundle 

To create an _application bundle_ the following steps must be performed:

1. Create Docker containers for application services. This step is sometimes
   called "dockerizing" an application.
2. Create Kubernetes definitions for application components, this makes an application
   capable of running on Kubernetes.
3. Create a Gravity _application manifest_ to describe the system requirements 
   for a Kubernetes cluster capable of running your application.
4. Execute `tele build` CLI command.

Let's follow them one by one.

### Step 1: Containerizing

Run the following to build the Mattermost containers:

```bsh
$ cd mattermost/worker
$ docker build -t mattermost-worker:2.2.0 .
```

When Docker finishes building the container, you should be able to see it listed:

```
$ docker images | grep mattermost
mattermost-worker      2.2.0       ce3ead6dff48     43 seconds ago      405MB
```

You can now return to the quickstart home directory.

### Step 2: Creating Kubernetes Resources

Making Mattermost run on Kubernetes is easy. The quickstart repository you have
cloned above includes the YAML definitions of Kubernetes objects. We'll use
a Helm chart for this:

```
$ tree mattermost/resources/charts/mattermost/
mattermost/resources/charts/mattermost/
├── Chart.yaml
├── templates
│   ├── _helpers.tpl
│   └── mattermost.yaml
└── values.yaml
```
The most interesting file to take a look at is [mattermost.yaml](https://github.com/gravitational/quickstart/blob/master/mattermost/resources/charts/mattermost/templates/mattermost.yaml)
You are welcome to change it to your liking.

**NOTE:** in this tutorial we're packaging a single Helm chart but it's
possible to have several of them packaged into a single application bundle.

### Step 3: Creating Application Manifest

In this step we must create a _application manifest_ which describes the system
requirements for a Kubernetes cluster for our application.

The cloned repo already has one in `mattermost/resources/app.yaml`. [Click here](https://github.com/gravitational/quickstart/blob/master/mattermost/resources/app.yaml) 
to open it on Github for convenience. We have commented the most important fields
in this example manifest.

### Step 4: Building a Bundle

Before we build our first application bundle, let's make sure [Helm](https://helm.sh/) is properly 
initialized and [helm-template](https://github.com/technosophos/helm-template) plugin is installed:

```bash
$ helm init --client-only
$ helm plugin install https://github.com/technosophos/helm-template
```

Now you let's build the application bundle which will consist of a Kubernetes
cluster with Mattermost pre-installed inside:

```bsh
$ tele build -o mattermost.tar mattermost/resources/app.yaml

# the output:
* [1/6] Selecting application runtime
	Will use latest runtime version 5.2.4
* [2/6] Downloading dependencies from s3://hub.gravitational.io
	Still downloading dependencies from s3://hub.gravitational.io (10 seconds elapsed)
	Still downloading dependencies from s3://hub.gravitational.io (20 seconds elapsed)
* [3/6] Embedding application container images
	Detected application manifest app.yaml
	Detected resource file clusterDeprovision.yaml
	Detected resource file clusterProvision.yaml
	Detected resource file install.yaml
	Detected resource file nodesDeprovision.yaml
	Detected resource file nodesProvision.yaml
	Detected Helm chart charts/mattermost
	Using local image quay.io/gravitational/debian-tall:0.0.1
	Using local image quay.io/gravitational/debian-tall:0.0.1
	Using local image quay.io/gravitational/provisioner:ci.82
	Using local image mattermost-worker:2.2.0
	Using local image postgres:9.4.4
	Vendored image gravitational/debian-tall:0.0.1
	Vendored image gravitational/provisioner:ci.82
	Still embedding application container images (10 seconds elapsed)
	Vendored image mattermost-worker:2.2.0
	Vendored image postgres:9.4.4
* [4/6] Using runtime version 5.2.4
	Still using runtime version 5.2.4 (10 seconds elapsed)
* [5/6] Generating the cluster snapshot
* [6/6] Saving the snapshot as mattermost.tar
* [6/6] Build completed in 2 minutes 
```

Let's review what just happened there. `tele build` did the following:

* Downloaded Kubernetes binaries and Gravity tooling (these artifacts are called "runtime") from `s3://hub.gravitational.io`.
* It scanned the current directory and the subdirectories for Kubernetes resources and Helm charts.
* Downloaded external container images referenced in the resources discovered in the previous step.
* Packaged (or vendored) Docker images into the application bundle.
* Removed the duplicate container image layers, reducing the size of the application bundle.
* Saved the snapshot AKA _application bundle_ as `mattermost.tar`.

!!! warning "Slow Operation Warning"
    `tele build` needs to download hundreds of megabytes of binary dependencies which
    can take considerable amount of time, depending on your Internet connection speed.

The resulting `mattermost.tar` file is about 1.6GB and it is **entirely
self-sufficient** and dependency-free. It contains everything: the Kubernetes
binaries, the Docker engine, the Docker registry and the Mattermost itself:
everything one needs to get Mattermost up and running on any fleet of Linux
servers (or into an AWS/GCE/Azure account).

Congratulations! You have created your first self-installing **Kubernetes
virtual appliance** which contains Mattermost inside!


## Installing 

Installing `mattermost.tar` _application bundle_ means creating a Kubernetes
cluster with the application pre-loaded in it. This file is the only artifact
one needs to create a Kubernetes cluster with Mattermost running inside. 

Copy `mattermost.tar` to a clean Linux machine, let's call it `host`. This node
will be used to bootstrap the cluster. Let's untar it and look inside:

```bash
$ tar -xf mattermost.tar
$ tree
├── app.yaml
├── gravity
├── gravity.db
├── install
├── packages
│   ├── blobs
│   │   ├── 0e1
│   │   ...
│   │   └── ff1
│   │       └── ff19bcf2dc62f037e0016d5d065150d195f714be3c97a301791365a4ec5a43f0
│   ├── tmp
│   └── unpacked
├── README
├── upgrade
└── upload
```

There are a few interesting bits in here:

File Name    | Description
-------------|------------------------
`gravity`    | Gravity cluster manager which is a Linux binary (executable). It's responsible for installing, upgrading and managing clusters.
`app.yaml`   | The application manifest which we've defined earlier and fed to `tele build`. You'll notice that the build process populated the manifest with additional metadata.
`packages`   | The database of Docker image layers for all containers and other binary artifacts, like Kubernetes binaries.
`gravity.db` | The metadata of what's stored in `packages`.
`upgrade`, `install`, `upload` | Helpful bash wrappers around `gravity` command.
`README`     | Instructions for the end user.

We're ready to install our _first application bundle_ now! But first, it's important
to note that Gravity supports two modes of installation:

* **CLI mode** also known as non-interactive mode is useful for advanced users
  and for scripting. It allows clusters to be created programmatically or via a
  command line.
  supported by the open source edition of Gravity.
* **Web mode** uses a web browser to guide a user through an install wizard.
  This mode is useful for non-technical users, sales demos, etc.

### Installing via CLI

To install a cluster via CLI, you have to execute `./gravity install` command and 
supply two mandatory flags:

Flag              | Description
-------------------|---------------------------------
`--token`          | A secret token of your choosing which will be used to add additional nodes to this cluster in the future. We'll use word "secret" here.
`--advertise-addr` | The IP address this host will be visible on by other nodes in this cluster. We'll use `10.5.5.28`.

The command below will create a single-node Kubernetes cluster with Mattermost running inside:

```
# We are executing this on the node named 'host' with IP address of 10.5.5.28
$ sudo ./gravity install \
        --advertise-addr=10.5.5.28 \
        --token=secret
# Output:
Sat Jan 12 05:30:44 UTC Starting installer
Sat Jan 12 05:30:44 UTC Preparing for installation...
Sat Jan 12 05:31:09 UTC Installing application mattermost:2.2.0
Sat Jan 12 05:31:09 UTC Starting non-interactive install
Fri Jan 11 21:31:09 UTC Auto-loaded kernel module: br_netfilter
Fri Jan 11 21:31:09 UTC Auto-loaded kernel module: iptable_nat
Fri Jan 11 21:31:09 UTC Auto-loaded kernel module: iptable_filter
Fri Jan 11 21:31:09 UTC Auto-loaded kernel module: ebtables
Fri Jan 11 21:31:09 UTC Auto-set kernel parameter: net.ipv4.ip_forward=1
Fri Jan 11 21:31:09 UTC Auto-set kernel parameter: net.bridge.bridge-nf-call-iptables=1
Sat Jan 12 05:31:10 UTC All agents have connected!
Sat Jan 12 05:31:10 UTC Starting the installation
Sat Jan 12 05:31:11 UTC Operation has been created
Sat Jan 12 05:31:12 UTC Execute preflight checks
Sat Jan 12 05:31:16 UTC Configure packages for all nodes
Sat Jan 12 05:31:20 UTC Bootstrap master node host
Sat Jan 12 05:31:24 UTC Pull packages on master node host
Sat Jan 12 05:32:20 UTC Install system package teleport:2.4.7 on master node host
Sat Jan 12 05:32:22 UTC Install system package planet:5.2.19-11105 on master node host
Sat Jan 12 05:32:48 UTC Wait for system services to start on all nodes
Sat Jan 12 05:33:24 UTC Label and taint master node host
Sat Jan 12 05:33:25 UTC Bootstrap Kubernetes roles and PSPs
Sat Jan 12 05:33:26 UTC Export applications layers to Docker registries
Sat Jan 12 05:33:27 UTC Populate Docker registry on master node host
Sat Jan 12 05:34:24 UTC Install system application dns-app:0.1.0
Sat Jan 12 05:34:31 UTC Install system application logging-app:5.0.2
Sat Jan 12 05:35:05 UTC Install system application tiller-app:5.2.1
Sat Jan 12 05:35:43 UTC Install system application site:5.2.4
Sat Jan 12 05:36:59 UTC Install system application kubernetes:5.2.4
Sat Jan 12 05:37:00 UTC Install application mattermost:2.2.0
Sat Jan 12 05:37:10 UTC Enable elections
Sat Jan 12 05:37:12 UTC Operation has completed
Sat Jan 12 05:37:13 UTC Installation succeeded in 6m3.257480586s
```

**Congratulations!** You have created a fully functional, highly available
Kubernetes cluster with Mattermost running inside. If a single node cluster 
is not enough, you can add additional nodes to it:

1. Copy `gravity` binary from the boostrapping node above to another host, let's assume it's IP is `10.5.5.29`.
2. Execute `gravity join` command as shown below. Note that this command will
   "think" in silence for a few seconds before dumping any output.

```bash
# Execute this on the second node with an IP 10.5.5.29
$ sudo ./gravity join 10.5.5.28 --advertise-addr=10.5.5.29 --token=secret

# Output:
Sat Jan 12 06:00:16 UTC	Connecting to cluster
Fri Jan 11 22:00:16 UTC	Auto-loaded kernel module: br_netfilter
Fri Jan 11 22:00:16 UTC	Auto-loaded kernel module: iptable_nat
Fri Jan 11 22:00:16 UTC	Auto-loaded kernel module: iptable_filter
Fri Jan 11 22:00:16 UTC	Auto-loaded kernel module: ebtables
Fri Jan 11 22:00:16 UTC	Auto-set kernel parameter: net.ipv4.ip_forward=1
Fri Jan 11 22:00:16 UTC	Auto-set kernel parameter: net.bridge.bridge-nf-call-iptables=1
Sat Jan 12 06:00:16 UTC	Connected to existing cluster at 10.5.5.28
Sat Jan 12 06:00:17 UTC	Operation has been created
Sat Jan 12 06:00:18 UTC	Configure packages for the joining node
Sat Jan 12 06:00:20 UTC	Bootstrap the joining node
Sat Jan 12 06:00:21 UTC	Pull packages on the joining node
Sat Jan 12 06:01:18 UTC	Install system package teleport:2.4.7
Sat Jan 12 06:01:19 UTC	Install system package planet:5.2.19-11105
Sat Jan 12 06:01:41 UTC	Start RPC agent on the master node 10.5.5.28
Sat Jan 12 06:01:45 UTC	Add the joining node to the etcd cluster
Sat Jan 12 06:01:49 UTC	Wait for the planet to start
Sat Jan 12 06:02:24 UTC	Stop RPC agent on the master node 10.5.5.28
Sat Jan 12 06:02:25 UTC	Enable leader election on the joined node
Sat Jan 12 06:02:26 UTC	Operation has completed
Sat Jan 12 06:02:26 UTC	Joined cluster in 2m10.547146946s
```

Now you have a two-node Kubernetes cluster! And you can see Mattermost running on
`https://10.5.5.28:3009/web/login`

### Installing via Web Browser


Obviously, take a look at the `README` file, it explains how to launch an
installer. Basically it will tell you to execute the `install` script. 

```bash
$ sudo ./install
OPEN THIS IN BROWSER: https://host:61009/web/installer/new/gravitational.io/mattermost/2.2.0?install_token=2a9de4a72ede
```

The `install` launches a daemon which serves a web UI and acts as a
bootstrapping agent to create a new Kubernetes cluster. It will print a web URL
for you to click on.

The browser-based installer will ask for the following:

* Name of your cluster. We recommend FQDN-like names like
  `mattermost.example.com`.
* The network interface to use. This must be the interface which Kubernetes
  nodes will use to talk to each other.
* The "flavor" of the cluster, i.e. 1, 2 or 3 nodes. The installer will offer a CLI
  command for each node to copy to and execute.
* Once all nodes report into the cluster the installer will proceed setting up
  Kubernetes.

!!! tip "Tip":
    The installer will ask you to copy and paste a CLI command for each node to join the 
    cluster. If you select a single-node install, you have to open a second terminal session
    into your node to paste and execute the command in.

The final step is to select the user name and password for the cluster
administrator. You will be able to change it later (or configure the SSO).
After that you will be placed in Gravity's cluster management UI, where you
will find the HTTP end point of Mattermost.

Now you can press `Ctrl+C` in the `node`'s terminal to stop the installer.


## Remote Access

Now with at least one instance of Mattermost running, you can go back to the
machine where you packaged it with the `tele` tool and execute:

```
$ tsh clusters
Cluster Name                     Status
------------                     ------
yourcompany.gravitational.io     online
mattermost                       online
```

You can now see the "mattermost" cluster and you can connect to it by running:

```
$ tele login mattermost
```

Now you can run `tsh ls` to see the nodes and `tsh ssh` for connecting to them
via SSH. See more in [Remote Management](manage) section.

## Publishing

While the resulting _application bundle_ can be used to create Kubernetes
clusters, Gravity also allows to publish the bundle in the _Gravity Ops Center_
for distribution, so other users of the Ops Center will see the published or
updated versions of application bundles and install them, creating new clusters.

The Ops Center also allows to keep track of remote clusters created from
published application bundles and perform remote administration of them.

!!! warning "Version Warning":
    The Ops Center is only available to users of Gravity Enterprise.  OSS users
    can skip "publishing" subsection and move on to [installing](#installing) below.

To publish the application bundle, you have to connect `tele` tool to your Ops Center.
Let's assume your _Ops Center_ is running on `https://opscenter.example.com`

```bsh
$ tele login -o https://opscenter.example.com
```

The command above performs the interactive login, i.e. opens a web browser and
takes a user through their corporate single sign on (SSO) process.

!!! tip "Note":
    If you are using a guest environment, like Vagrant on macOS, you may see an error
    when attempting due to network restrictions. If so, first get a temporary key from
    your host environment with the following command:

Next, generate a new API key for the _Ops Center_ you've just connected to. The API
key will allow you to login into the Ops Center automatically in the future without
having to go through the SSO flow: 

```bsh
$ tele keys new
```

The response will include the API key, let's assume it's called `secret`. 
We can use the key to login into the Ops Center non-interactively:

```bash
tele login -o https://opscenter.example.com --key=secret
```

Now you can publish the application bundle we just built:

```bash
$ tele push mattermost.tar
```

... and Mattermost is now visible to all users of the Ops Center, so they can
install it.


## Conclusion

This is a brief overview of how publishing and distributing an application
works using Gravity. Feel free to dive further into the documentation
for more details.

Gravity avoids proprietary configuration formats or closed source tools.
Outside of having a small Gravity-specific application manifest, your
application is just a regular Kubernetes deployment. Gravity simply makes
it portable and deployable into private infrastructure.

If you need help packaging your application into Docker containers, or if you
need help packaging your application for Kubernetes, our implementation
services team can help (info@gravitational.com).
