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

There are three steps to create an _application bundle_ with Gravity:

1. Create Docker containers for application services. This step is sometimes
   called "dockerizing" an application.
2. Create Kubernetes definitions for application components, this makes an application
   capable of running on Kubernetes.
3. Create a Gravity _application manifest_ to describe the system requirements 
   for a Kubernetes cluster capable of running your application.
4. Execute `tele build` CLI command.

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

Now you can package Mattermost into an Application Bundle:

```bsh
$ tele build -o mattermost.tar mattermost/resources/app.yaml
```

!!! note
    The sample Mattermost application is packaged as a Helm chart so you will
    need `helm` installed on the machine performing the build. See [Installing Helm](https://docs.helm.sh/using_helm/#installing-helm).

The command above does the following:

* Scans Kubernetes resources for the Docker images.
* Packages (vendors) Docker images into the Application Bundle.
* Removes duplicate container image layers, reducing the size of the Application Bundle.
* Downloads Kubernetes and its dependencies, like Docker.
* Packages downloaded dependencies into the Application Bundle, as well.

This may take a while, about 2-3 minutes depending on your Internet connection.

!!! tip "Let's Review...":
    The resulting `mattermost.tar` file is about 1.8GB and it is **entirely
    self-sufficient**. It contains everything: Kubernetes, the Docker engine,
    the Docker registry and the Mattermost itself: everything one needs to get
    Mattermost up and running on any fleet of Linux servers (or into someone's
    AWS account).

The resulting bundle even has a built-in graphical installer, i.e. can be given
away as a free download to people who are not cloud experts.

### Publishing

While the resulting Application Bundle can be used as is, Gravity also allows you
to publish it in your Ops Center for distribution. Publishing it in the Ops Center also allows
you to see how many instances of the application are running and even connect to them remotely.

To publish an application, you have to connect to the Ops Center first.
You should already have an account setup with Gravitational - usually available
on `https://[yourdomain].gravitational.io`

To log in to to the Ops Center using the CLI:

```bsh
$ tele login -o [yourdomain].gravitational.io
```

!!! tip "Note":
    If you are using a guest environment, like Vagrant on macOS, you may see an error
    when attempting due to network restrictions. If so, first get a temporary key from
    your host environment with the following command:

```bsh
$ tele keys new
```

The response should include an API key to use. Now use that key to log in as follows:

```bsh
tele login -o [yourdomain].gravitational.io --key=[key returned in previous step]
```

Once you are successfully logged in, you can publish the application into the Ops Center:

```bsh
$ tele push mattermost.tar
```

This uploads the Application Bundle into the Ops Center you have logged in into.

Now the application is ready to be distributed and installed.

## Installing the Application

As shown in the [Overview](overview.md), there are two ways to install an
Application Bundle packaged with Gravity:

* Online, via the Ops Center.
* Offline, via a downloadable tarball.

### Online Installer

This method of installation is called online installation
and it assumes that the end user (a person installing the application) has access to
the Internet.

The simplest way to launch an online installer is to log in to the Ops Center
and click on "Install" in the dropdown menu for the published Application.
This will take you to a URL where the installation wizard will run.

![Gravity Online Installer](/images/installer.png)

### Offline Mode

To run an offline installation:

* Log into Ops Center and click on "Download" link in the dropdown menu.
* Upload the tarball to the server (node) you wish to install Mattermost on.
* Unpack the tarball via `tar -xf <tarball.tar>` into a Linux machine.
* Launch `./install` CLI command and follow the instructions.

The installer will copy itself to additional cluster nodes, if needed.

### Non Interactive Installation on Vagrant

To test automatic non-interactive installation on a local Vagrant box:

* `tele build -o mattermost.tar mattermost/resources/app.yaml`
* `vagrant up`

Cluster administrative control panel login will be available at https://172.28.128.101:32009/
with the username `admin@localhost` and password `abc123`.

Mattermost UI will be running on http://172.28.128.101:32010.

### Remote Access

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
