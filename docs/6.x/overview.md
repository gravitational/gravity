# Introduction

Gravity is an open source toolkit which allows developers to package a 
Kubernetes (AKA, "K8s") cluster and all its applications 
into a single file called "cluster image".

Each cluster image is a dependency-free `.tar` file. It can be used to deploy
an entire K8s cluster preloaded with applications into a variety of target
infrastructure options, such as developers' own cloud infrastructure, 3rd party
cloud accounts, bare metal servers, VMWare environments or even into air-gapped
servers not connected to the Internet.

Each K8s cluster created using Gravity contains an
authentication gateway which allows ops teams to remotely troubleshoot and push
updates to many instances of the same appliance either via SSH or via
Kubernetes API, even if they are located behind a firewall.


This overview will walk you through the basic concepts of Gravity and explain
how it is used to solve the operational challenges that typically arise when
running many K8s clusters across distributed teams within a large organization,
or across multiple organizations.

!!! tip "How does Gravity compare to Kubernetes?"
    Kubernetes's job is to manage your applications. Gravity's job is to keep
    Kubernetes alive and well.

**Gravity provides true portability for cloud-native applications.**

## Use Cases

There are two primary use cases for Gravity:

1. **Deploying K8s Applications on premises:** SaaS vendors often need to
   deploy and remotely update complex cloud applications in private data
   centers or public cloud accounts, like AWS, owned by their customers. For
   them, Gravity provides true portability for cloud-native applications.

2. **Reducing Operational Overhead of Multi-Cluster Kubernetes Deployments:**
   Teams of site reliability engineers (SREs) are often tasked with providing
   Kubernetes-as-a-Service within their organization across multiple cloud
   providers or hybrid environments. The image-based approach allows them to
   treat K8s clusters as cattle, not pets, dramatically reducing the
   operational overhead.

Gravity users can create an cluster image that contains the Kubernetes binaries,
their dependencies, application containers and their layers, a private Docker 
registry for autonomous operation, a monitoring system and an authentication gateway 
for remotely managing the cluster via both SSH and via the Kubernetes API. 

In other words, a Gravity cluster image is a _self-contained, downloadable
Kubernetes appliance_ which enables true portability for cloud applications across any
public or private infrastructure.

## Components

Gravity consists of the following components:


| Component   | Description
|-------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `tele`      | The build tool. `tele` is used for building cluster images. The enterprise edition of `tele` also publishes cluster images them into the Gravity Hub.  |
| `tsh`       | Is used for remotely connecting to Gravity/Kubernetes clusters via SSH or Kubernetes API.   |
| `gravity`   | The cluster manager which runs on every cluster node. It abstracts away complexities of Kubernetes management. `gravity` is also a CLI tool to perform cluster expansion, upgrades, etc.  |
| Gravity Hub | The Hub is web portal for publishing cluster images. Think of Gravity Hub as a catalog of Kubernetes clusters and Kubernetes applications. Gravity Hub is available to Enterprise edition users only. |


## Cluster Lifecycle

A cluster image may contain an "empty", pre-packaged Kubernetes environment. Such cluster 
images are used by organizations that require an easy way to run multiple identical, compliant
Kubernetes clusters with reduced operational overhead.

A cluster image can also contain several Kubernetes applications. Such cluster
images can be used to distribute complex, Kubernetes applications as
downloadable appliances. The typical life cycle of such applications consists of 
the following:

0. First, you must prepare your application(s) to run on Kubernetes. If you do
   not have Kubernetes expertise, our Solutions Engineering Team can help.
1. Package the applications into a cluster image using `tele` CLI tool.
2. Publish the cluster image for distribution. AWS S3 or any CDN can be used
   to publish downloadable images. You can also use Gravity Hub, which is only
   available to Enterprise Edition users, to manage the publishing process of your 
   cluster images.
3. Deploy and install the cluster image onto any supported Linux-based
   infrastructure ("Gravity Cluster" or "Cluster").
4. Securely connect to any cluster to monitor health, provide automatic
   updates, remote support, etc.

## Packaging

To package a cluster into a cluster image:

* Create Kubernetes resources describing your application(s). You can use "raw"
  Kubernetes resources as YAML files, but [Helm charts](https://helm.sh/) are
  also supported.
* Provide a cluster manifest described below. The manifest is used to customize
  the cluster image.
* Place all of these files in the same directory and execute `tele build`

A cluster manifest is required to describe hardware/system requirements of your
cluster and to customize the process of creating a new cluster instance.

!!! tip "Gravity Roadmap Tip":
    Kubernetes community is working on finalizing the cluster API spec. Once it
    becomes production ready, Gravity developers will be looking into adopting the
    future standard to replace the cluster manifest in the future. Meanwhile, 
    it continues to be the only production-ready method of describing hardware
    requirements for K8s clusters.

Below is a sample cluster manifest in YAML format. It follows Kubernetes
configuration conventions:

```yaml
# this is an example of a small cluster manifest
apiVersion: cluster.gravitational.io/v2
kind: Cluster
metadata:
  name: "Example"
  resourceVersion: "1.0.0"

installer:
# a cluster image may optionally include the system requirements. this allows
# the publisher of an image to restrict its usage only to infrastructure with
# a defined performance envelope
nodeProfiles:
  - name: node
    description: "worker node"
    requirements:
      cpu:
        min: 1
      ram:
        min: "2GB"
```

The cluster manifest works in conjunction with [Helm charts](https://helm.sh/)
and Kubernetes resources like jobs and configuration maps. These tools provide
the high degree of flexibility for specifying how applications are installed,
updated and configured. 

To create a cluster image you have to:

1. Place the required Kubernetes resources, Helm charts and the cluster
   manifest in the same directory.
2. Execute the `tele build` command to create the cluster image:

```bsh
$ tele build -o cluster-image.tar manifest.yaml
```

This will produce the cluster image called `cluster-image.tar`, which can be
deployed across any cloud providers and private data centers. 

You can learn more about the cluster manifest in the [Packaging & Deployment](pack.md) 
section of the documentation.

## Publishing

Publishing can be as simple as uploading the cluster to an S3 bucket or CDN for
others to download and install.

Another option is to publish the cluster image into the Gravity Hub, a
centralized repository of cluster images. If a cluster image is distributed via
the Gravity Hub, the resulting cluster can optionally "dial home" for automatic
updates, remote monitoring and troubleshooting.

The Gravity Hub allows cluster image publishers to oversee how many clusters
are running and perform administration and maintenance across all of them in a
repeatable, scalable way, even if they are deployed on 3rd party
infrastructure.

!!! warning "Version Warning":
    The Gravity Hub is only available to the users of Gravity Enterprise.

## Deploying and Installing

Creating a new clusters from a cluster image is trivial:

1. Place a cluster image on a Linux node and unpack ("untar") it.
2. Launch the included installer.
3. Later, a user can connect the resulting cluster to Gravity Hub to allow
   remote updates and remote administration.

For more details about the installation process, please refer to [Installation Guide](installation.md).

## Cluster Management

Once a cluster image is installed, it becomes a fully operational and
autonomous Gravity/Kubernetes cluster. It can be said that a _cluster is
an instance of a cluster image_.

Gravity comes with a tool called `gravity` which serves two purposes:

* It runs as a Linux daemon on every cluster node acting as a "Kubernetes
  hypervisor", continuously monitoring the health of Kubernetes services
  and re-configuring them if necessary. For example, it provides automatic
  management of `etcd`.
* It is available as a command line (CLI) tool to perform cluster
  administration tasks such as:
    1. Adding or removing nodes to a cluster.
    2. Performing in-place upgrades of Kubernetes or any of the applications
       running inside.
    3. Printing cluster status.

When it comes to cluster management, `gravity` is a higher level replacement
for tools like `kubeadm` or `etcdadm`.  It delivers benefits such as enforcing
system requirements and delivering on the promise of lower operational overhead
by automating away many mundane Kubernetes tasks. **It is the "magic sauce" that 
makes a Kubernetes cluster feel like a reliable appliance.**

You can still use `kubectl` for tasks like watching logs, seeing stats for pods
or volumes, managing configuration and other operational tasks.

### Updates and Upgrades

`gravity` can perform updates of both the Kubernetes itself and the
applications running inside. The updates can be performed in one of two modes: 

* **online mode** allows `gravity` command to check for new versions of a
  cluster, download them from the connected Gravity Hub, and perform a in-place
  upgrade of the cluster.
* **offline mode** allows `gravity` to use a newer version of the cluster image
  to perform an in-place upgrade.

For more details on using `gravity` to manage Clusters please see the [Cluster Management](cluster.md) and [Remote Management](manage.md) sections.

