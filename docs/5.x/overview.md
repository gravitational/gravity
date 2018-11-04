# Introduction

!!! tip "Information"
    As of October 10, 2018, Telekube has been renamed to Gravity.

Welcome to Gravity, by Gravitational. Gravity allows developers to package
and deploy their complex multi-node, multi-tier application clusters into a
variety of target infrastructure options, such as their own AWS accounts, 3rd
party AWS accounts or even into air-gapped, bare metal server clusters.

Gravity uses Google's [Kubernetes](http://k8s.io) as a portable cluster runtime
and allows ops teams to remotely manage many instances of a cluster either
via SSH or via Kubernetes API, even if they are located behind
a firewall.

This overview will walk you through the basic concepts of Gravity and explain how
it is used to solve the operational challenges that arise in the use cases below.

If you already have a good understanding of Gravity you can choose to jump right in with our [Quick Start](/quickstart) guide or dive deeper into Gravity documentation through the table of contents.

## Use Cases

There are two primary use cases for Gravity:

1. **Private SaaS:** Software vendors (including SaaS applications) often
   need to deploy their complex software into private datacenters or 3rd
   party cloud accounts owned by their enterprise customers.
   Gravity allows them to snapshot, publish and distribute updates to the
   enterprise customers.

2. **Multi-Region Kubernetes:** Ops teams in large companies with many
   distributed product teams often need to provide Kubernetes-as-a-Service
   within their organization across multiple hosting regions and multiple
   hosting providers. Gravity allows them to pre-package supported Kubernetes
   configurations into "applications" and streamline their management and
   cross-organizational trust.

In both use cases, an application _usually_ includes the Kubernetes binaries,
their dependencies, a private Docker registry for autonomous operation, a
monitoring system and an SSH bastion for remotely managing the cluster either
via simple SSH or via Kubernetes API.


## Components

Gravity consists of these major components:


* [gravity](./cluster/#gravity-tool): the CLI tool used to manage the cluster. Gravity is only available inside the cluster so you have to `tsh ssh` into the cluster to execute `gravity` commands.
* **`tele`**: the CLI tool which is used for packaging and publishing applications.
* **`tsh`**: the SSH client for establishing secure SSH connections between the
  Ops Center and the application instances running behind firewalls on private clouds.
  `tsh` is a part of [Gravitational Teleport](http://gravitational.com/teleport/),
  a free open source SSH server developed, maintained and supported by Gravitational.
  It can be used independently from Gravity.
* [Ops Center](./opscenter): the Web UI for managing published applications and remotely
  accessing private cloud deployments.

## Workflow

The typical Gravity workflow is as follows:

* Start by building and publishing your Application Bundle with `tele` command.  Once
  a Gravity Cluster is deployed, `tele` command will let you list the active
  Clusters.
* Connect to any server inside of any Cluster using the `tsh` command.
* Manage the Cluster from within by using `gravity` command and/or
  Kubernetes tools like `kubectl`.

## Application Lifecycle

A Gravity application is a snapshot of a Kubernetes cluster, stored
as a compressed `.tar` file ("Application Bundle" or "Application"). Application
Bundles may contain nothing but pre-packaged Kubernetes for centralized management
of Kubernetes resources within an organization or may also contain other
applications running on Kubernetes. This allows a user to replicate complex cloud
native stacks in multiple environments.

The typical application lifecycle using Gravity consists of the following:

0. Prepare your application to run on [Kubernetes](https://k8s.io). If
   you do not have Kubernetes expertise, our Implementation Services team can help.
1. Package the application into a deployable `tar` tarball ("Application Bundle" or
   "Bundle").
2. Publish the Application Bundle for distribution.
3. Deploy and install the Application Bundle onto Linux based server cluster
   ("Gravity Cluster" or "Cluster").
4. Securely connect to any Cluster to monitor health and roll out
   updates.

## Packaging

Gravity works only with applications running on Kubernetes. This means the following
prerequisites apply in order to use Gravity:

* The application is packaged into containers.
* You have Kubernetes resource definitions for application services, pods, etc.

To prepare a Kubernetes application for distribution via Gravity, you have to write
an Application Manifest. Below is a sample of a simple Application Manifest in YAML format. It follows Kubernetes configuration conventions:

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

The sample above is intentionally simplistic to illustrate the concept. The
Applicaion Manifest's purpose is to describe the infrastructure requirements and
custom steps required for installing the Bundle and updating the Cluster.

The Application Manifest works in conjunction with Kubernetes jobs and
configuration maps. Together, these tools provide a high degree of
flexibility for specifying how applications are installed, updated and
configured. You can learn more about the Application Manifest in the
[Packaging & Deployment](pack.md) section of the documentation.

As shown in the diagram below, the build machine (often times a
developer's laptop or a CI/CD server) should contain everything needed to
build a self-sufficient portable package to distribute the Application Bundle:

![Gravity Build Diagram](images/build.svg?style=grv-image-center-md)

Gravity's `build` command creates the Application Bundle tarball:

```bsh
$ tele build -o app-name.tar app.yaml
```

This will produce a self-sufficient tarball, `app-name.tar`, which can be deployed
across cloud providers, or onto infrastructure like private data centers or
private clouds.

## Publishing

Publishing can be as simple as uploading the Application Bundle to an
S3 bucket for others to download and install.

Another option is to publish the Application Bundle into the Gravity "Ops Center", a
centralized repository of your Application Bundles and its deployed Clusters. If an
Application Bundle is distributed via the Ops Center, the resulting Cluster can
optionally "phone home" for automating updates, remote monitoring and trouble
shooting.

The Ops Center allows Application Bundle publishers to oversee how many Clusters are
running and perform administration and maitenance across all of them in a
repeatable, scalable way, even if they are deployed on 3rd party infrastructure.

## Deploying and Installing

Once an Application Bundle is deployed and installed it becomes a fully operational,
autonomous, self-regulating and self-updating Kubernetes cluster ("Gravity
Cluster" or "Cluster"). All Clusters can optionally "phone home" to a centralized control plane ("Ops Center") and be remotely managed.

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

For more details about the installation process, refer to [Installation Guide](installation.md).

## Ongoing Management

Gravity uses a component called "Gravity" to manage Clusters. Gravity commands are available through a CLI and can:

1. Add new nodes to a Cluster.
2. Remove nodes from a Cluster.
3. Monitor a Cluster
4. Perform updates to the Cluster.

Kubernetes tools such as `kubectl` can also be used to perform Cluster management
tasks such as watching logs, seeing stats for pods or volumes, managing
configuration and so on.

Kubernetes's job is to manage your application. Gravity's job is to manage Kubernetes.

See more details in [Cluster Management](cluster.md) section.

For Applications running on remote infrastructure, Ops Center can be used to
get remote access to the Application (assuming the end users allow it).

If the Application is offline, the Gravity offline updating mechanism can be used to perform updates.

See more details in [Remote Management](manage.md) section.
