# Gravity

Gravity is a highly opinionated configuration of [upstream Kubernetes](https://kubernetes.io/), along with tooling to improve compliance, security, simplicity and reduce operational overhead.

|Project Links| Description
|---|----
| [Gravity Website](http://gravitational.com/telekube)  | The official website of the enterprise version of Gravity called Telekube |
| [Gravity Documentation](http://gravitational.com/telekube/documentation)  | Gravity Documentation (aka, Telekube)  |
| [Blog](http://blog.gravitational.com) | Our blog, where we publish Gravity news |

## Introduction

Gravity is an open source tooklit for creating "snapshots" of Kubernetes
clusters and the applications running inside the clusters. The resulting snapshots are called
*application bundles*. 

An application bundle can be used to re-create full replicas of the original
cluster in any environment where compliance matters, even in air-gapped server
rooms.

## Gravity vs ...

There are plenty of Kubernetes distributions out there. Most of them aim to be
flexible, general purpose platforms. Gravity has a more narrow focus
on compliance and autonomous operations:

* Gravity clusters are idempotent, i.e. clusters created from the same bundle
  are _always identical_. There is no configuration drift over time; no
  "special snowflakes".
* Gravity clusters are always "wrapped" with a privileged access gateway called
  [Teleport](https://gravitational.com/teleport), which unifies k8s and SSH authentication 
  and keeps a detailed audit log for compliance purposes.
* Gravity includes tools to perform _infrastructure validation_ prior to
  cluster provisioning. This allows cluster designers to prevent users from
  installing clusters on infrastructure that does not meet the system requirements.
* Gravity clusters only allow Kubernetes components that have been thoroughly
  tested by [Gravitational Inc](https://gravitational.com) for compatibility
  and stability. These components are called a "Kubernetes Runtime". Users can
  pick a Runtime but Gravity does not allow any customization of
  individual components of Kubernetes.

## Who is Gravity for?

We have seen three primary use cases for using a snapshot-based Kubernetes approach
(there may be others):

* Deploying complex SaaS applications into on-premise enterprise environments.
* Managing many idempotent Kubernetes clusters in environments where
  compliance and security matters. An example would be if you want the same,
  compliant Kubernetes environment across a variety of organizations or infrastructure
  environments.
* Environments where autonomous Kubernetes is required, such as large multi-node
  hardware appliances, production floors, edge deployments, etc.

## Application Bundles

An Application Bundle produced by Gravity includes:

* All Kubernetes binaries and their dependencies.
* Built-in container registry.
* De-duplicated layers of all application containers inside a cluster.
* Built-in cluster orchestrator which guarantees HA operation, in-place
  upgrades and auto-scaling.
* Installation wizard for both CLI and web browser GUI.

A bundle is all one needs to re-create the complete replica of the original
Kubernetes cluster, with all deployed applications inside, even in an
air-gapped server room.

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

Teleport can also be used independently without Gravity, it has been audited
multiple times by [reputable](https://gravitational.com/blog/teleport-security-audit/) 
cyber security companies and it has been deployed in production in [multiple](https://gravitational.com/teleport) 
organizations.

## Is Gravity Production Ready?

Yes! Even though Gravity was open sourced in September 2018, it started life
much earlier, as a component of a larger, proprietary system called
[Telekube](https://gravitational.com/telekube). 

Fully autonomous Gravity clusters are running inside of large banks, government
institutions, enterprises, etc. Some of the commercial users of Gravity are
listed on the [Gravitational web site](https://gravitational.com)

## Why did We Build Gravity?

Gravity was built by [Gravitational Inc](https://gravitational.com), a company
based in Oakland, California. Gravitational's mission is to allow software
creators to freely share their products with customers without having to
convert themselves into software operators. 

The original use case for Gravity was to allow Kubernetes applications to be
deployed into 3rd party environments. That's why Gravity includes features like
the built-in, graphical cluster installer, infrastructure validation and a
built-in privileged access manager (Teleport) for providing remote support.

These features also resonated with security-minded teams who need to run
applications in environments where _compliance matters_. Gravity clusters are
always identical and do not allow any configuration drift over time, which
allows _cluster designers_ (aka, Devops or SREs) to "publish" clusters that are approved for
production and allow multiple teams within the organization to rapidly scale their
Kubernetes adoption without having to become security and Kubernetes experts themselves.

## Building from source

Gravity is written in Go. There are two ways to build the Gravity tools from
source: by using locally installed build tools or via Docker.

```bash
$ git clone git@github.com:gravitational/gravity.git
$ cd gravity

# Running 'make' with the default target uses Docker.
# The output will be stored in build/<version>/
$ make

# If you have Go 1.10+ installed, you can build without Docker which is faster.
# The output will be stored in $GOPATH/bin/
$ make install

# To remove the build artifacts:
$ make clean
```

## Known Issues

While the code is open source, we're still working on updating the
documentation to reflect the differences between the proprietary and
community/OSS editions of the software. We are also working on providing open
source users with pre-built binaries on a regular basis.

Gravity OSS is currently in private, preview mode. We will be making this
repository public once we have updated the documentation, polished user experience
and discussed feedback from preview users.

## Questions?

For more information reach out to `info@gravitational.com`

