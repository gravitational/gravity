# Quick Start

The purpose of this Quick Start Guide is to quickly evaluate Telekube by using a real world example.

We believe in learning by doing!

## Introduction

This guide will use [Mattermost](https://www.mattermost.org/), an open sourced chat application for
teams, as a sample application. Mattermost represents a fairly typical web application
and consists of an HTTP request handling process which connects to a PostgreSQL
instance.

This Quick Start Guide will walk you through the process of packaging
Mattermost for private deployments. Before we start, please take a look at the
[Telekube Overview](index.md) to get familiar with basic concepts of the
Telekube system.

### System Requirements


Telekube is a Linux-based toolkit. By default it supports 64-bit versions of the
following Linux distributions as [specified here](requirements.md#distributions)

If you have a need to support a Linux distribution not listed above,
Gravitational offers Implementation Services that may be able to assist you.

Additionally, this guide requires Docker version 1.8 or newer. Run `docker info`
before continuing to make sure you have Docker up and running on your
system.

You will also need `git` to be able to download Mattermost code from Github.

## Getting the Tools

Telekube consists of three major components:

* **`tele`**: the CLI tool which is used for packaging and publishing applications.
* **`tsh`**: the SSH client for establishing secure SSH connections between the
  Ops Center and the application instances running behind firewalls on private clouds.
  `tsh` is a part of [Gravitational Teleport](https://gravitational.com/teleport/),
  a free open source SSH server developed, maintained and supported by Gravitational.
  It can be used independently from Telekube.
* **Ops Center**: the Web UI for managing published applications and remotely
  accessing private cloud deployments.

### Installing Telekube Tools

Let's start by installing `tele` and `tsh` CLI tools onto your machine. You
need to have `sudo` priviliges:

```bash
# Download the latest version:
$ curl https://get.gravitational.io/telekube/install | bash

# ... or, if a specific version is needed:
$ curl https://get.gravitational.io/telekube/install/4.36.0 | bash
```
To make sure the installation succeeded, try typing `tele version`.

Next, download Mattermost, the sample application used for this tutorial:

```bash
$ git clone https://github.com/gravitational/quickstart.git
```

## Packaging the Application

There are three major logical steps in preparing an application to be deployed
via Telekube:

1. Creating Docker containers for application components.
2. Creating Kubernetes definitions for application components, this enables your
   application to run on Kubernetes.
3. Packaging "kubernetized" application into a single self-deployable tarball (an
   "Application Bundle").

Once you complete these 3 steps, you can use the resulting tarball to distribute
application manually, or you can publish it by uploading it into the Ops Center.

### Building Containers

Before an application (Mattermost in our case) can be packaged and published
via Telekube, you need to "containerize" it first.

The sample project you have fetched via `git` above contains Docker files
for Mattermost, as well as its Kubernetes resources.

Run this to build Mattermost containers:

```bash
$ cd quickstart/mattermost/worker
$ docker build -t mattermost-worker:2.2.0 .
```

When the docker build is done, return to the quickstart home directory.

```bash
$ cd ../..
$ pwd
/home/user/quickstart
```

### Migrating to Kubernetes

Making Mattermost run on Kubernetes is easy. The quickstart repository you have
cloned above includes the YAML definitions of Kubernetes objects in
[resources/mattermost.yaml](https://github.com/gravitational/quickstart/blob/master/mattermost/resources/charts/mattermost/templates/mattermost.yaml)
but you are welcome to change it to your liking.

### Packaging

Now you can package Mattermost into an Application Bundle:

```bash
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

The resulting `mattermost.tar` Application Bundle will be about 800MB and it is entirely
self-sufficient. It contains everything needed to install Mattermost onto a fleet
of Linux servers.

### Publishing

While the resulting Application Bundle can be used as is, Telekube also allows you
to publish it in your Ops Center for distribution. Publishing it in the Ops Center also allows
you to see how many instances of the application are running and even connect to them remotely.

To publish an application, you have to connect to the Ops Center first.
You should already have an account setup with Gravitational - usually available
on `https://[yourdomain].gravitational.io`

To log in to to the Ops Center using the CLI:

```bash
$ tele login -o [yourdomain].gravitational.io
```

!!! tip "Note"
    If you are using a guest environment, like Vagrant on macOS, you may see an error
    when attempting due to network restrictions. If so, first get a temporary key from
    your host environment with the following command:

```bash
$ tele keys new
```

The response should include an API key to use. Now use that key to log in as follows:

```bash
tele login -o [yourdomain].gravitational.io --key=[key returned in previous step]
```

Once you are successfully logged in, you can publish the application into the Ops Center:

```bash
$ tele push mattermost.tar
```

This uploads the Application Bundle into the Ops Center you have logged in into.

Now the application is ready to be distributed and installed.

## Installing the Application

As shown in the [Overview](index.md), there are two ways to install an
Application Bundle packaged with Telekube:

* Online, via the Ops Center.
* Offline, via a downloadable tarball.

### Online Installer

This method of installation is called online installation
and it assumes that the end user (a person installing the application) has access to
the Internet.

The simplest way to launch an online installer is to log in to the Ops Center
and click on "Install" in the dropdown menu for the published Application.
This will take you to a URL where the installation wizard will run.

![Telekube Online Installer](images/installer.png)

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
via SSH. See more in [Remote Management](manage.md) section.

## Conclusion

This is a brief overview of how publishing and distributing an application
works using Telekube. Feel free to dive further into the documentation
for more details.

Telekube avoids proprietary configuration formats or closed source tools.
Outside of having a small Telekube-specific application manifest, your
application is just a regular Kubernetes deployment. Telekube simply makes
it portable and deployable into private infrastructure.

If you need help packaging your application into Docker containers, or if you
need help packaging your application for Kubernetes, our implementation
services team can help (info@gravitational.com).
