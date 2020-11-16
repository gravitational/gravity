---
title: Introduction to Gravity
description: Introduction to Gravity, the toolkit for packaging and deploying cloud application on-premises, into remote, restricted and regulated environments.
---

# Introduction

Gravity is an open source toolkit which allows developers to
package their [Kubernetes](http://k8s.io) (aka, "K8s") clusters and applications as
downloadable "appliances" ("Application Bundle" or "Bundle").

Each Application Bundle is a single, dependency-free `.tar` file. It can be used to deploy an
entire K8s cluster preloaded with applications into a variety of target
infrastructure options, such as developers' own cloud infrastructure, 3rd party cloud accounts or even into air-gapped, bare metal infrastructure.

Each K8s cluster created using Gravity contains an
authentication gateway which allows ops teams to remotely troubleshoot and push
updates to many instances of the same appliance either via SSH or via
Kubernetes API, even if they are located behind a firewall.

This overview will walk you through the basic concepts of Gravity and explain how
it is used to solve the operational challenges that typically arise when running multiple K8s clusters.

!!! tip "How does Gravity compare to Kubernetes?"
    Kubernetes's job is to manage your applications. Gravity's job is to keep Kubernetes alive and well.

## Use Cases

There are two primary use cases for Gravity:

1. **On-Premise Deployments of K8s Applications:** Software vendors (including
   SaaS applications) often need to deploy and remotely update complex
   software in private data centers or public cloud accounts, like AWS, owned
   by their customers.

2. **Reducing Operational Overhead:** Ops teams supporting many distributed
   product teams are often tasked with providing Kubernetes-as-a-Service
   within their organization across multiple hosting regions and multiple
   hosting providers. Gravity's image-based approach allows them to treat
   K8s clusters as cattle, not pets, dramatically reducing operational overhead.

For these use cases, Gravity is used to create an Application Bundle that includes the Kubernetes binaries,
their dependencies, a private Docker registry for autonomous operation, a
monitoring system and an SSH bastion for remotely managing the cluster either
via simple SSH or via Kubernetes API. These components are customizable by the user and additional components may be included.

In other words, a Gravity Application Bundle is a _self-contained, downloadable
Kubernetes appliance_ which enables true application portability across any
public or private infrastructure.

## Components

Gravity consists of these major components:


* [gravity](cluster.md#gravity-tool): the CLI tool used to manage the cluster.
Gravity is only available inside the cluster so you have to either `tsh ssh` into the cluster or use web UI to execute `gravity` commands.
* **`tele`**: the CLI tool which is used for packaging and publishing applications.
* **`tsh`**: the SSH client for establishing secure SSH connections between the
  Ops Center and the application instances running behind firewalls on private clouds.
  `tsh` is a part of [Gravitational Teleport](http://gravitational.com/teleport/),
  a free open source SSH server developed, maintained and supported by Gravitational.
  It can be used independently from Gravity.
* [Ops Center](opscenter.md): the Web UI for managing published applications and remotely
  accessing private cloud deployments.

## Cluster Lifecycle

Application Bundles may contain "empty", pre-packaged Kubernetes for
centralized management of Kubernetes resources within an organization or may
also contain other applications running on Kubernetes.

The typical cluster lifecycle consists of the following:

0. Prepare your application(s) to run on [Kubernetes](https://k8s.io). If
   you do not have Kubernetes expertise, our Implementation Services team can help.
1. Package the applications into a deployable `tar` tarball ("Application Bundle" or
   "Bundle") using Gravity's `tele` tool.
2. Publish the Application Bundle for distribution. AWS S3 or any CDN can be used
   to publish downloadable images. The enterprise edition of Gravity also comes with
   a web application ("Ops Center") which can be used to publish and manage
   Application Bundles.
3. Deploy and install the Application Bundle onto any supported Linux-based
   infrastructure ("Gravity Cluster" or "Cluster").
4. Securely connect to any Cluster to monitor health and roll out updates.

## Packaging

To prepare an Application Bundle for distribution via Gravity, you have to:

* Create Kubernetes resources describing your application(s). [Helm charts](https://helm.sh/) are supported for this.
* Write an Application Manifest described below.
* Place all of these files in the same location.

The Application Manifest is required to describe hardware/system requirements
of your Cluster and to customize the process of creating a new
Cluster (i.e. the installation of the Cluster).

!!! tip "Gravity Roadmap Tip"
    Kubernetes community is working on finalizing the cluster API spec. Once it
    becomes production ready, Gravity developers will be looking into adopting the
    future standard to replace the application manifest in the future. Meanwhile,
    it continues to be the only production-ready method of describing hardware
    requirements for K8s clusters.

Below is a sample Application Manifest in YAML format. It follows Kubernetes
configuration conventions:

```yaml
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: telekube
  resourceVersion: "1.0.0"

# Applications can be white-labeled with vendor's look and feel.
logo: "http://example.com/logo.jpg"

installer:
  # An application can be configured with multiple "flavors", perhaps letting
  # the end user of a cluster to customize its shape and size.
  flavors:
    prompt: "Select a flavor"
    items:
      - name: "one"
        description: "1 node"
        nodes:
          - profile: node
            count: 1

# An application must define its system requirements, i.e. if an application
# needs certain amounts of RAM/CPU/storage to run, they can be listed here.
nodeProfiles:
  - name: node
    description: "worker node"
    requirements:
      cpu:
        min: 1
      ram:
        min: "2GB"
```

The Application Manifest works in conjunction with [Helm charts](https://helm.sh/)
and Kubernetes resources like jobs and configuration maps. These
tools provide a high degree of flexibility for specifying how applications are
installed, updated and configured.

To create an Application Bundle you have to:

1. Place the required Kubernetes resources, Helm charts and the Application Manifest in the same directory.
2. Execute the `tele build` command to create the Application Bundle:

```bsh
$ tele build -o my-kubernetes-appliance.tar manifest.yaml
```

This will produce the Application Bundle called `my-kubernetes-appliance.tar`, which can
be deployed across cloud providers and private data centers.

You can learn more about the Application Manifest in the [Packaging & Deployment](pack.md)
section of the documentation.

## Publishing

Publishing can be as simple as uploading the Application Bundle to an S3 bucket or CDN
for others to download and install.

Another option is to publish the Application Bundle into the Gravity [Ops
Center](opscenter.md), a centralized repository of your Application Bundles and deployed Clusters. If an Application Bundle is distributed via the Ops
Center, the resulting Cluster can optionally "phone home" for automating
updates, remote monitoring and troubleshooting.

The Ops Center allows Application Bundle publishers to oversee how many Clusters are
running and perform administration and maintenance across all of them in a
repeatable, scalable way, even if they are deployed on 3rd party infrastructure.

!!! warning "Version Warning"
    The Ops Center is only available to users of Gravity Enterprise.

## Deploying and Installing

The Gravity Ops Center supports two methods for deploying Application Bundles:

* **Online Installation**: The Ops Center generates a URL to an installer which
  can be shared with users to install the application into their infrastructure.
  When using online mode, the Cluster can be remotely managed and updated.

* **Offline Installation**: The Application Bundle can be simply copied to
  the target infrastructure. This infrastructure could even be air-gapped and not
  connected to the Internet. The end user can then unpack the Application Bundle and
  launch a self-contained installer.

In either mode, the Gravity installer can run as a command line (CLI) tool or by
opening a browser-based graphical installation wizard.

For more details about the installation process, please refer to [Installation Guide](installation.md).

## Ongoing Management

Once an Application Bundle is deployed and installed, it becomes a fully operational and autonomous Kubernetes cluster ("Gravity
Cluster" or "Cluster"). All Clusters can optionally "phone home" to a centralized control plane ("Ops Center") to be remotely managed.

Gravity uses a command line (CLI) tool called `gravity` to manage a cluster. `gravity`
must be executed on any cluster node and it allows a cluster administrator to:

1. Add new nodes to a cluster.
2. Remove nodes from a cluster.
3. Print cluster cluster status.
4. Perform updates to the cluster runtime (i.e. K8s itself), as well as upgrades
   of individual applications in the Clusters.

`gravity` is a higher level replacement for tools like `kubeadm` or `etcdadm`. It provides automatic, hypervisor-like management of `etcd`. Additionally, it delivers
benefits such as enforcing system requirements and delivering on the promise of
lower operational overhead for managing many Kubernetes clusters. You can also still use `kubectl` for tasks like watching logs, seeing
stats for pods or volumes, managing configuration and other operational tasks.

For Applications running on remote infrastructure, the Ops Center can be used to
provide remote access to the clusters (assuming the remote administrators allow it).

### Updates and Upgrades

`gravity` can perform updates of both the Kubernetes itself and the
applications running inside. The updates can be performed in one of two modes:

* **online mode** allows `gravity` command to check for new versions of a
  cluster, download them and perform a in-place upgrade of the cluster.
* **offline mode** allows `gravity` to use a newer version of the _application
  bundle_ tarball to perform an in-place upgrade.

For more details on using `gravity` to manage Clusters please see the [Cluster Management](cluster.md) and [Remote Management](manage.md) sections.
