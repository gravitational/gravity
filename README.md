# Support Notice

**The Gravity project is no longer under active development.** 
The project's development has been limited to maintenance and support for our
commercial customers until maintenance agreements expire.

Please see our blog post for more information:
https://goteleport.com/blog/gravitational-is-teleport/

---

<a href='https://gravitational.com/gravity/download/'>
    <img src='https://gravitational.com/gravitational/images/logos/logo-gravity-x-large.png' alt='Gravity'>
</a>

Gravity is a [Kubernetes](https://kubernetes.io/) packaging solution
that takes the drama out of deploying and running applications in someone
else's cloud accounts, on-premise data centers, edge locations and other
"uncharted territory" environments.

With Gravity, Kubernetes apps can run and be regularly updated anywhere in
the world without a massive DevOps team.

|Project Links| Description
|---|----
| [Gravity Downloads](https://goteleport.com/gravity/download/)  | The latest binaries |
| [Gravity Documentation](https://gravitational.com/gravity/docs/)  | Gravity Documentation |
| [Gravity Examples](examples/) | Examples of applications packaged with Gravity |
| [Blog](http://blog.gravitational.com) | Our blog, where we publish Gravity news |
| [Security and Release Updates](https://goteleport.com/gravity/docs/changelog/) | Gravity Security and Release Updates |

## Introduction

Gravity is an open source toolkit for creating "images" of Kubernetes
clusters and the applications running inside the clusters. The resulting
images are called *cluster images* and they are just `.tar` files.

A cluster image can be used to re-create full replicas of the original
cluster in any environment where compliance and consistency matters, i.e. in
locked-down AWS/GCE/Azure environments or even in air-gapped server rooms.
An image can run without human supervision, as a "kubernetes appliance".

Gravity has been running in production in major financial institutions,
government data centers and enterprises. Gravitational open sourced it in the
fall of 2018.

<a href='https://gravitational.com/gravity/download/'>
    <img src='https://gravitational.com/gravitational/images/graphics/gravity-oss-hero.png' alt='Gravity'>
</a>

## Gravity vs ...

There are plenty of Kubernetes distributions out there. Most of them aim to be
flexible, general purpose platforms. Gravity has a more narrow **focus
on compliance and reducing the overhead of managing Kubernetes**:

* Gravity clusters are idempotent, i.e. clusters created from the same image
  are _always identical_. There is no configuration drift over time; no
  "special snowflakes".
* Gravity clusters are always "wrapped" with a privileged access gateway called
  [Teleport](https://gravitational.com/teleport), which unifies k8s and SSH
  authentication, integrates with SSO and keeps a detailed audit log for compliance
  purposes. It even records the interactive SSH and `kubectl exec` sessions.
* Gravity clusters deployed world-wide can be remotely managed via built-in
  reverse SSH tunnels, i.e. developers can have access to thousands of k8s API
  endpoints even if they're located behind NAT/firewalls.
* Gravity includes tools to perform _infrastructure validation_ prior to
  cluster provisioning. This allows cluster designers to prevent users from
  installing clusters on infrastructure that does not meet the system requirements.
* Gravity clusters only allow Kubernetes components that have been thoroughly
  tested by [Gravitational Inc](https://gravitational.com) for compatibility
  and stability. These components are called a "Kubernetes Runtime". Users can
  pick a Runtime but Gravity does not allow any customization of
  individual components of Kubernetes.

## Who is Gravity for?

We have seen the following primary use cases for using a image-based Kubernetes approach
(there may be others):

* Deploying and running complex SaaS applications into on-premises enterprise environments.
* Deploying and running complex SaaS applications in thousands of edge locations (retail, transportation, energy, etc).

Anyone who needs Kubernetes best practices out of the box, without having to
proactively manage it can benefit from Gravity. It allows you to focus on
building your product instead of managing Kubernetes.

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

## How do Initial Deployments work?

A cluster image created with Gravity can be used for:

1. Creating many Kubernetes clusters from scratch, on any infrastructure.
2. Installing applications contained in the cluster image into an existing
   Kubernetes cluster, like OpenShift.

## How do Updates work?

Developers can continuously update their applications using different methods:

1. Vanilla CI/CD using Kubernetes APIs, which is available for every cluster.
   This is probably what you're already doing.
2. Via "polling model", when each Gravity cluster will automatically download
   updates from a Gravity Hub, letting cluster users decide when/if they want
   to upgrade. This method is recommended for traditional on-premise
   environments when developers do not have access to each deployment site.
3. Offline method, when a developer prepares a new cluster image which can be
   distributed via offline media. This method is suitable for air-gapped
   environments.

## Remote Access and Compliance

Each cluster provisioned with Gravity includes the built-in SSH/Kubernetes gateway
called [Teleport](https://github.com/gravitational/teleport). Teleport provides the
following benefits:

* One-step authentication which issues credentials for both k8s API and SSH.
* Ability to implement compliance rules like "developers must never touch production data".
* Ability to grant remote access to the cluster via SSH or via k8s API, even if the
  cluster is located behind NAT with no open ports.
* Keeps a detailed audit log (including fully recorded interactive sessions)
  for all SSH commands and all `kubectl` commands executed on cluster nodes.

Teleport can also be used independently without Gravity, it has been [audited
multiple times](https://gravitational.com/resources/audits/) by reputable
cyber security companies and it has been deployed in production in [multiple
organizations](https://gravitational.com/teleport).

## Is Gravity Production Ready?

Yes!

Fully autonomous Gravity clusters are running inside of large banks, government
institutions, enterprises, etc. We use Gravity to run our own infrastructure.

## Why did We Build Gravity?

Gravity is built by [Teleport](https://goteleport.com).

The original use case for Gravity was to allow Kubernetes applications to be
deployed into 3rd party environments, like on-premises datacenters. That's why
Gravity includes features like the built-in, graphical cluster installer,
infrastructure validation and a built-in privileged access manager (Teleport)
for providing remote support.

These features also resonated with security-minded teams who need to run
applications in environments where _compliance matters_. Gravity clusters are
always identical and do not allow any configuration drift over time. This
allows _cluster architects_ (aka, Devops or SREs) to "publish" clusters that are approved for
production and allow multiple teams within the organization to rapidly scale their
Kubernetes adoption without having to become security and Kubernetes experts themselves.

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

## Contributing

To contribute, please read the [contribution guidelines](./CONTRIBUTING.md).

Want to join our team? [We are always hiring!](https://jobs.lever.co/teleport)
