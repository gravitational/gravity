---
title: Gravity CLI Reference
description: User manual for the Gravity command line (CLI) tools
---

# CLI Tools

Gravity features the following CLI commands:

| Component   | Description
|-------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `tele`      | The build tool. `tele` is used for building Cluster images. The enterprise edition of `tele` also publishes Cluster Images them into the Gravity Hub.  |
| `tsh`       | Used for remotely connecting to Gravity/Kubernetes Clusters via SSH or Kubernetes API.   |
| `gravity`   | The Cluster manager which runs on every Cluster node. It abstracts away complexities of Kubernetes management. `gravity` is also a CLI tool to perform Cluster expansion, upgrades, etc.  |

The typical Gravity workflow is as follows:

* Start by building a Cluster Image with `tele` command.
* Distribute a Cluster Image to a target environment.
* Create a new Gravity/Kubernetes Cluster using the built-in Cluster installer.
* Manage the Cluster from by using `gravity` command on Cluster nodes and/or
  using Kubernetes tools like `kubectl`.

## tele

`tele` is the Linux Gravity CLI client. With `tele` you can:

* Package Kubernetes Clusters into self-installing Cluster Images.
* Publish Cluster Images into the Gravity Hub. (Enterprise version only)
* Download Cluster Images from the Gravity Hub. (Enterprise version only)

You can think of `tele` as "docker for Clusters". Just as `docker` can
build, push and pull containers, `tele` does the same with entire Clusters. See
more details in [Building Cluster Images](pack.md) section.

## tsh

`tsh` allows to remotely connect to any Gravity Cluster using SSH and
Kubernetes API. You can use `tsh` to remotely login into any node in a
Gravity Cluster, even those located behind firewalls.

Gravity uses Teleport for remotely accessing Clusters. See more details in the
[Teleport User Manual](https://gravitational.com/teleport/docs/user-manual/).

## gravity

`gravity` only runs on Linux and is only available on the Cluster nodes where
your applications are running. `gravity` is responsible for mostly abstracting
away low-level Kubernetes management. `gravity` provides commands for easy
version upgrades, adding and removing nodes to a Cluster, and other common
administration tasks.

See more details in [Cluster Management](cluster.md) section
