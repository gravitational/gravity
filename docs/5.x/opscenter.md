---
title: Gravity OpsCenter
description: Gravity Ops Center is a multi-cluster control plane available in the Enterprise version of Gravity.
---

# Introduction

The Gravity Ops Center is a multi-cluster control plane available in the Enterprise version of Gravity. It reduces the operational overhead of managing multiple Gravity Clusters and Application Bundles by allowing users to:

* Publish Application Bundles and manage their versions.
* Download and install Application Bundles, i.e. quickly creating Kubernetes clusters.
* Remotely manage thousands of Kubernetes clusters either via command line (CLI) or via a Web interface.

!!! warning "Version Warning"
    The Ops Center is only available to users of Gravity Enterprise.

This chapter will guide you through the process of downloading and installing your own instance of the _Ops Center_.

## Installing Ops Center

The Ops Center only works with the Gravity Enterprise Edition license key and
the Application Bundle for the Ops Center. You can [contact us](https://gravitational.com/gravity/demo/)
to get a trial license key and the Ops Center Application Bundle.

As with any Gravity application, you will also need a Linux server to install the Ops Center.

### Downloading Ops Center

The Ops Center itself is packaged as a Gravity application bundle, i.e. it's a
tarball which contains a fully functional K8s cluster. To download the Ops Center,
run the following command:

```bash
# This command only works with the enterprise edition of 'tele' command.
# It may take 1-2 minutes to download the bundle, depending on the
# quality of your Internet connection:
$ tele pull opscenter

# output:
* [1/3] Requesting application installer from https://get.gravitational.io
* [2/3] Downloading opscenter:5.5.0
	Still downloading opscenter:5.5.0 (10 seconds elapsed)
	Still downloading opscenter:5.5.0 (20 seconds elapsed)
	Still downloading opscenter:5.5.0 (30 seconds elapsed)
	Still downloading opscenter:5.5.0 (40 seconds elapsed)
	Still downloading opscenter:5.5.0 (50 seconds elapsed)
	Still downloading opscenter:5.5.0 (1 minute elapsed)
* [3/3] Application opscenter downloaded
* [3/3] Download completed in 1 minute

# See the result:
$ ls -lh
-rw-r--r-- 1 user user 1.3G Feb 20 13:02 opscenter-5.5.0.tar
```

The name of the tarball will vary based on the version of `tele` you're using,
so we'll refer to it simply as `ops-center.tar` below.

### Generating a Token

To establish trust between the Ops Center and multiple K8s clusters, a common shared
hard-to-guess secret (token) must be generated first. Therefore, before
installing the Ops Center, a shared token needs to be generated and stored in
an environment variable named `TOKEN`:

```bsh
$ export TOKEN="$(uuidgen)"
```

Next, expand the Ops Center Application Bundle `ops-center.tar` and launch the installer:

```bsh
$ tar xvf ./ops-center.tar
$ ./gravity install --advertise-addr=10.1.1.5 \
                    --token=$TOKEN \
                    --flavor=standalone \
                    --cluster=opscenter.example.com \
                    --ops-advertise-addr=opscenter.example.com:443
```

* `advertise-addr` is private IPV4 address of a K8s master node (this node) which will be used by other K8s nodes to form a cluster.
* `flavor` is the cluster configuration flavor to install; choose `standalone`
  for a single-node install which is great for evaluation/development purposes
  or `ha` to install a 3-node cluster suitable for production use or
  high-availability
* `ops-advertise-addr` should be a DNS name publicly accessible via internet
* `token` is a security token for nodes to join to the cluster
* `cluster` is a unique cluster name, e.g. `opscenter.example.com`

## Post-provisioning

#### Setting up DNS

After provisioning, DNS records should be created with hostname at either the provisioned ELB load balancer (for AWS) or the IP of the virtual machine (for Vagrant).

!!! tip "Wildcard DNS name"
	  The Ops Center DNS records should be wildcard, both `*.opscenter.example.com` and `opscenter.example.com` should point to the IP address
	  of the Ops Center service or load balancer.

#### Setting up OIDC

After installation [OIDC provider](cluster.md#configuring-a-cluster) should be set up in order to log into the Ops Center.

#### Setting up TLS Key Pair

After installation, a valid [TLS key pair](cluster.md#configuring-tls-key-pair) should be set up in order to log into the Ops Center. The Ops Center has to use a valid, not self-signed TLS certificate to function properly.

#### Configuring endpoints

See [Configuring Ops Center Endpoints](cluster.md#configuring-ops-center-endpoints)
for information on how to configure Ops Center management endpoints.

## Upgrading Ops Center

Log into a root terminal on the Ops Center server.

Update the `tele` binary:

```bsh
$ curl -LO https://get.gravitational.io/telekube/bin/{VERSION}/linux/x86_64/tele
$ chmod +x ./tele
```

Fetch the latest Ops Center application using `tele`:

```bsh
$ ./tele pull opscenter:{VERSION} -o installer.tar
```

This will automatically download into the current directory as `installer.tar`.

This archive provides all dependencies required for the update, including new `gravity` binaries,
`install` and `upgrade` scripts.

Extract the tarball:

```bsh
$ tar xvf installer.tar
```

Start the upgrade procedure using `upgrade` script:

```bsh
$ ./upgrade
```

Read more about upgrade procedure [here](cluster.md#performing-upgrade).

!!! tip "Ports"
    Users who use an external load balancer may need to update their configuration after the upgrade to reference new port assignments.
