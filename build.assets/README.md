Gravity
=======

Gravity is a Kubernetes packaging solution that takes the drama out of
on-premises deployments.

Project Links
==============

Gravity Website:  https://gravitational.com/gravity/
Quick Start    :  https://gravitational.com/gravity/docs/quickstart/
Gravity Source :  https://github.com/gravitational/gravity
Blog           :  https://blog.gravitational.com
Community Forum:  https://community.gravitational.com

Introduction
============

Gravity is an open source toolkit for creating "images" of Kubernetes
clusters and the applications running inside the clusters. The resulting
images are called *cluster or application images* and they are just `.tar` files.

A cluster image can be used to re-create full replicas of the original
cluster in any environment where compliance and consistency matters, i.e. in
locked-down AWS/GCE/Azure environments or even in air-gapped server rooms. A
bundle can run without human supervision, as a "kubernetes appliance".

Gravity has been running in production in major financial institutions,
government data centers and enterprises. Gravitational open sourced it in the
fall of 2018.

Installing
==========

Execute `./install.sh` script as root. It will copy `tele`, `gravity` and `tsh`
binaries into `/usr/local/bin`.

What are these binaries?

* `tele` is a tool to build cluster and application images.
* `gravity` is a tool to install cluster and application images and manage
  Gravity clusters and installed applications.
* `tsh` is a tool to remotely connect to clusters created from the images.
  tsh supports both SSH and Kubernetes API.

See the quick start to learn how to use these tools:
https://gravitational.com/gravity/docs/quickstart/

Building from source
====================
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

# To build tsh
$ make build-tsh

# To remove the build artifacts:
$ make clean
```

Talk to us
==========

* Want to join our team to hack on Gravity? https://gravitational.com/careers/
* Want to stop managing Kubernetes and have autonomous appliance-like clusters?
* Want to take your complex SaaS application and convert it into a downloadable
  appliance so your customers can run it on their own AWS account or in a colo?

Reach out to info@gravitational.com
