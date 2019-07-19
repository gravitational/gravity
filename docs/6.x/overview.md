# Introduction

Gravity is an open source toolkit that provides true portability for cloud-native applications. It allows developers to package a
Kubernetes cluster and all its applications
into a single file called a "Cluster Image".

Each Cluster Image is a dependency-free `.tar` file. It can be deployed into a variety of target infrastructure options, such as developers' own cloud infrastructure, 3rd party
cloud accounts, bare metal servers, VMware environments or even into air-gapped
servers not connected to the Internet.

The Cluster Image can be installed on any modern Linux kernal across multiple machines through a web browser GUI or CLI installation wizard to create a resilient Kubernetes cluster. This process is completely autonomous and does not require any dependencies from outside the Cluster Image. 

When a Cluster is up and running, Gravity eases the operational burden of running it. Each Gravity Cluster contains an
authentication gateway which allows ops teams to remotely troubleshoot and push
updates to many instances of the same appliance either via SSH or via
Kubernetes API, even if they are located behind a firewall.

This overview will walk you through the basic concepts of Gravity and explain
how it is used to solve the operational challenges that typically arise when
running many Kubernetes clusters across distributed teams within a large organization,
or across multiple organizations.

## Use Cases

There are two primary use cases for Gravity:

1. **Delivering Kubernetes applications to customers on-premises:** SaaS vendors often need to deploy and remotely update complex cloud applications in private data centers or public cloud accounts, like AWS, owned by their customers (aka, on-prem). Gravity reduces the time to delivery of these applications on-prem.

2. **Internal multi-cluster Kubernetes deployments:** Teams of site reliability engineers (SREs) are often tasked with providing Kubernetes-as-a-Service within their organization across multiple cloud providers or hybrid environments. The image-based approach allows them to treat Kubernetes clusters as cattle, not pets, dramatically reducing the operational overhead.

In either use case, Gravity users can create an Cluster Image that contains the Kubernetes binaries, their dependencies, application containers and their layers, a private Docker registry for autonomous operation, a monitoring system, and an authentication gateway for remotely managing the Gravity Cluster via both SSH and via the Kubernetes API.

In other words, a Gravity Cluster Image is a _self-contained, downloadable
Kubernetes appliance_ which enables true portability for cloud applications across any public or private infrastructure.

![gravity overview](/images/gravity-overview.png)

## Components

Gravity consists of the following components:

| Component   | Description
|-------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `tele`      | The build tool. `tele` is used for building Cluster Images. The enterprise edition of `tele` can also publish Cluster Images into Gravity Hub.  |
| `tsh`       | The remote access client to authenticate against a SAML/OAuth identity provider and remotely connect to Kubernetes clusters via SSH or Kubernetes API.   |
| `gravity` <small>service</small> | The Cluster manager agent which runs on every  node. It reduces the complexities of Kubernetes management.
| `gravity` <small>CLI tool</small>  | The CLI tool to perform high level Cluster administration tasks like expansion, upgrades, etc.  |
| Gravity Hub | Gravity Hub is a web portal and repository for publishing Cluster Images. Think of Gravity Hub as a catalog of Kubernetes clusters and Kubernetes applications. Gravity Hub is only available in the Gravity Enterprise edition. |

## Cluster Lifecycle

A Cluster Image may contain an "empty" Kubernetes environment or contain multiple Kubernetes applications. The typical life cycle of such applications consists of
the following:

0. First, you must prepare your application(s) to run on Kubernetes. If you do
   not have Kubernetes expertise, our Solutions Engineering Team can help.
1. Package the applications into a Cluster Image using the `tele` CLI tool.
2. Publish the Cluster Image for distribution. AWS S3 or any CDN can be used
   to publish downloadable images. Gravity Enterprise users can also use Gravity Hub to manage the publishing and distribution of Cluster Images.
3. Deploy and install the Cluster Image onto any supported Linux-based
   infrastructure ("Gravity Cluster" or "Cluster").
4. Securely connect to any cluster to monitor health, provide automatic
   updates, remote support, etc.

## Packaging

To create a Cluster Image:

* Create Kubernetes resources describing your application(s). You can use "raw"
  Kubernetes resources as YAML files or [Helm charts](https://helm.sh/) are
  also supported.
* Provide a Cluster Image Manifest described below. The manifest is used to customize
  the Cluster Image.
* Place all of these files in the same directory and execute `tele build`

A Cluster Image manifest is required to describe hardware/system requirements of your
cluster and to customize the process of creating a new cluster instance.

!!! tip "Gravity Roadmap Tip":
    The Kubernetes community is working on finalizing the cluster API spec. Once it
    becomes production ready, we will likely adopt the
    future standard to replace the Cluster Image manifest. 

Below is a sample image manifest in YAML format. It follows the Kubernetes
configuration conventions:

```yaml
# this is an example of a small image manifest
apiVersion: cluster.gravitational.io/v2
kind: Cluster
metadata:
  name: "Example"
  resourceVersion: "1.0.0"

installer:
# a Cluster Image may optionally include the system requirements. this allows
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

The image manifest works in conjunction with [Helm charts](https://helm.sh/)
and Kubernetes resources like jobs and configuration maps. These tools provide
a high degree of flexibility for specifying how applications are installed,
updated and configured.

To create a Cluster Image you have to:

1. Place the required Kubernetes resources, Helm charts and the Cluster Image Manifest in the same directory.
2. Execute the `tele build` command to create the Cluster Image:

```bsh
$ tele build -o cluster-image.tar manifest.yaml
```

This will produce the Cluster Image called `cluster-image.tar`, which can be
deployed across any cloud providers and private data centers.

You can learn more about the image manifest in the [Packaging & Deployment](pack.md)
section of the documentation.

## Publishing

Publishing can be as simple as uploading the Cluster Image to an S3 bucket or CDN for
others to download and install.

Another option is to publish the Cluster Image into Gravity Hub, a
centralized repository of Cluster Images. If a Cluster Image is distributed via
Gravity Hub, the resulting Gravity Cluster can optionally "dial home" for automatic
updates, remote monitoring and troubleshooting.

Gravity Hub allows Cluster Image publishers to oversee how many clusters
are running and perform administration and maintenance across all of them in a
repeatable, scalable way, even if they are deployed on 3rd party
infrastructure.

!!! warning "Version Warning":
    The Gravity Hub is only available to the users of Gravity Enterprise.

## Deploying and Installing

Creating new clusters from a Cluster Image is trivial:

1. Place a Cluster Image on a Linux node and unpack ("untar") it.
2. Launch the included installer.

For more details about the installation process, please refer to [Installation Guide](installation.md).

## Cluster Management

Once a Cluster Image is installed, it becomes a fully operational and
autonomous Gravity Cluster.

For managing a running Gravity Cluster, the `gravity` tool can be used. 

The `gravity` daemon runs on every Cluster node as a "Kubernetes hypervisor", continuously monitoring the health of Kubernetes services and re-configuring them if necessary. For example, it provides automatic management of `etcd`.

`gravity` also functions as a command line (CLI) tool to perform Cluster administration tasks such as:

1. Adding or removing nodes to a Cluster.
2. Performing in-place upgrades of Kubernetes or any of the applications running inside.
3. Printing Cluster status.

When it comes to cluster management, `gravity` is a higher level replacement
for tools like `kubeadm` or `etcdadm`.  It delivers benefits such as enforcing
system requirements and delivering on the promise of lower operational overhead
by automating away many mundane Kubernetes tasks.

You can still use `kubectl` for tasks like watching logs, seeing stats for pods
or volumes, managing configuration and other operational tasks.

### Updates and Upgrades

`gravity` can perform updates of both the Kubernetes itself and the
applications running inside. The updates can be performed in one of two modes:

* **online mode** allows the `gravity` command to check for new versions of a
  Cluster Image, download them from the connected Gravity Hub, and perform a in-place
  upgrade of the Cluster.
* **offline mode** allows `gravity` to use a newer version of the Cluster Image
  to perform an in-place upgrade.

For more details on using `gravity` to manage Clusters please see the [Cluster Management](cluster.md) and [Remote Management](manage.md) sections.

