---
title: Gravity CLI Reference
description: User manual for the Gravity command line (CLI) tools
---


# CLI Tools

Gravity uses the following CLI commands:

| Command  | Description
|----------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `tele`   | Gravity CLI client. `tele` is used for packaging Application Bundles and publishing them into the Ops Center.                                                  |
| `tsh`    | Gravity SSH client which can be used to remotely connect to a node inside of any Gravity Cluster. `tsh` is fully compatible with OpenSSH's `ssh`.     |
| `gravity`| The Kubernetes runtime engine. It manages Kubernetes daemons and their health, Cluster updates and so on. Gravity is present on every machine of a Gravity Cluster. |


The typical Gravity workflow is as follows:

* Start by building and publishing your Application Bundle with `tele` command.  Once
  a Gravity Cluster is deployed, `tele` command will let you list the active
  Clusters.
* Connect to any server inside of any Cluster using the `tsh` command.
* Manage the Cluster from within by using `gravity` command and/or
  Kubernetes tools like `kubectl`.

## tele

`tele` is the Gravity CLI client and can run on macOS and Linux. By using `tele` on your laptop you can:

* Package Kubernetes applications into self-installing tarballs ("Application Bundles").
* Publish Application Bundles into the Ops Center.
* Manage the Gravity Clusters in the Ops Center.

See more details in [Packaging & Deployment](pack.md) section.

## tsh

`tsh` is the SSH client used by Gravity and can run on macOS and Linux. You can use `tsh` to remotely login into
any server in a Gravity Cluster, even those located behind firewalls.
To achieve this, `tsh` uses the Ops Center as an "SSH bastion" or "jump host".

The `tsh` tool is a part of Gravitational Teleport, an [open source SSH server and
client](https://gravitational.com/teleport) developed and supported by
Gravitational. Teleport can be used outside of Gravity, but the supplied `tsh`
client is tightly integrated with other Gravity tools, for example `tele login`.

See more details in [Remote Management](manage.md) section

## gravity

`gravity` only runs on Linux and is only available on the target machines
where your application is running. It can be used to manage the state of a Gravity Cluster.

See more details in [Cluster Management](cluster.md) section
