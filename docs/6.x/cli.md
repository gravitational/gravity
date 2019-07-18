# CLI Tools

Gravity features the following CLI commands:

| Component   | Description
|-------------|---------------------------------------------------------------------------------------------------------------------------------------------------------|
| `tele`      | The build tool. `tele` is used for building cluster images. The enterprise edition of `tele` also publishes cluster images them into the Gravity Hub.  |
| `tsh`       | Is used for remotely connecting to Gravity/Kubernetes clusters via SSH or Kubernetes API.   |
| `gravity`   | The cluster manager which runs on every cluster node. It abstracts away complexities of Kubernetes management. `gravity` is also a CLI tool to perform cluster expansion, upgrades, etc.  |

The typical Gravity workflow is as follows:

* Start by building a cluster images with `tele` command.
* Distribute a cluster image to a target environment.
* Create a new Gravity/Kubernetes cluster using the built-in cluster installer.
* Manage the cluster from by using `gravity` command on cluster nodes and/or
  using Kubernetes tools like `kubectl`.

## tele

`tele` is the Gravity CLI client and can run on macOS and Linux. With `tele` you can:

* Package Kubernetes clusters into self-installing cluster images.
* Publish cluster images into the Gravity Hub. (enterprise version only)
* Download cluster images from the Gravity Hub. (enterprise version only)

You can think of `tele` as "docker for clusters", i.e. just as `docker` can
build, push and pull containers, `tele` does the same with entire clusters. See
more details in [Building Cluster Images](pack.md) section.

## tsh

`tsh` allows to remotely connect to any Gravity cluster using SSH and
Kubernetes API. It runs on MacOS and Linux. You can use `tsh` to remotely
login into any node in a Gravity cluster, even those located behind firewalls.

See more details in [Remote Management](manage.md) section

## gravity

`gravity` only runs on Linux and is only available on the cluster nodes where
your applications are running. `gravity` is responsible for mostly abstracting
away low-level Kubernetes management. `gravity` provides commands for easy
version upgrades, adding and removing nodes to a cluster, and other common
administration tasks. 

See more details in [Cluster Management](cluster.md) section
