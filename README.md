# Gravity

> **Warning**
> Gravity was archived 2023-07-01.
>
> Please see our [Gravitational is Teleport](https://goteleport.com/blog/gravitational-is-teleport/)
> blog post for more information.
>
> If you're looking for a similar solution, we recommend using a
> [certified Kubernetes Distribution](https://kubernetes.io/partners/#conformance).

Gravity is a [Kubernetes](https://kubernetes.io/) packaging solution.

## Introduction

Gravity is an open source toolkit for creating "images" of Kubernetes
clusters and the applications running inside the clusters. The resulting
images are called *cluster images* and they are just `.tar` files.

A cluster image can be used to re-create full replicas of the original
cluster in any environment where compliance and consistency matters, i.e. in
locked-down AWS/GCE/Azure environments or even in air-gapped server rooms.
An image can run without human supervision, as a "kubernetes appliance".

## Cluster Images

A Cluster Image produced by Gravity includes:

* All Kubernetes binaries and their dependencies.
* Built-in container registry.
* De-duplicated layers of all application containers inside a cluster.
* Built-in cluster orchestrator which guarantees HA operation, in-place
  upgrades and auto-scaling.
* Installation wizard for both CLI and web browser GUI.

An image is all one needs to re-create the complete replica of the original
Kubernetes cluster, with all deployed applications inside, even in an
air-gapped server room.

## Examples

Take a look at the [examples](examples/) directory in this repository to find
examples of how to package and deploy Kubernetes applications using Gravity.

The following examples are currently available:

* [Wordpress](examples/wordpress). Deploys Wordpress CMS with an OpenEBS-backed persistent storage.

## Building from source

Gravity is written in Go. There are two ways to build the Gravity tools from
source: by using locally installed build tools or via Docker. In both cases
you will need a Linux machine.

**Building on MacOS, even with Docker, is possible but not currently supported**

```bash
$ git clone git@github.com:gravitational/gravity.git
$ cd gravity

# Running 'make' with the default target uses Docker.
# The output will be stored in build/current/
$ make

# If you have Go 1.10+ installed, you can build without Docker which is faster.
# The output will be stored in $GOPATH/bin/
$ make install

# To remove the build artifacts:
$ make clean
```
