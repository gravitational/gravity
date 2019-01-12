# Introduction

The Gravity _Ops Center_ is a hub which allows users to reduce operational overhead to:

* Publish _application bundles_ and manage their versions.
* Download and install _application bundles_, i.e. quickly creating Kubernetes clusters.
* Remotely manage thousands of Kubernetes clusters either via command line (CLI) or via a Web interface.

!!! warning "Version Warning":
    The Ops Center is only available to users of Gravity Enterprise.  OSS users
    can use the Ops Center for evaluation purposes only.

This chapter will guide you through the process of downloading and installing your
own instance of the _Ops Center_.

## Installing Ops Center

You need to download the [Gravity binaries](/gravity/download/). Note that you will
need an enterprise edition of Gravity. The open source edition will not work.

Before proceeding, you should have access to:

* `tele` and `tsh`: the enterprise binaries of Gravity.
* `ops-center.tar`: the _application bundle_ containing the Ops Center installer.

As with any Gravity application, you will need a Linux server to install the
Ops Center.

### Generating a Token

To establish trust between an _Ops Center_ and multiple K8s clusters, a common shared
hard-to-guess secret (token) must be generated first. Therefore, before
installing an Ops Center, a shared token needs to be generated and stored in an
environment variable named `TOKEN`:

```bsh
$ export TOKEN="$(uuidgen)"
```

Expand the Ops Center application bundle and launch the installer:

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

After provisioning, DNS records should be created with hostname at either the provisioned ELB load balancer (for AWS) or the IP of the virtual machine (for Vagrant)

!!! tip "Wildcard DNS name":
	  The Ops Center DNS records should be wildcard, both `*.opscenter.example.com` and `opscenter.example.com` should point to the IP address
	  of the Ops Center service or load balancer.

#### Setting up OIDC

After installation [OIDC provider](/cluster/#configuring-a-cluster) should be set up in order to login to the Ops Center.

#### Setting up TLS Key Pair

After installation, a valid [TLS key pair](/cluster/#configuring-tls-key-pair) should be set up in order to login to the Ops Center.

!!! tip "TLS Certificate":
    The Ops Center has to use a valid, not self-signed TLS certificate to function properly.

#### Configuring endpoints

See [Configuring Ops Center Endpoints](/cluster/#configuring-ops-center-endpoints)
for information on how to configure Ops Center management endpoints.

## Upgrading Ops Center

Log into a root terminal on the OpsCenter server.

Update the tele binary:

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

Read more about upgrade procedure [here](/cluster/#performing-upgrade).

!!! tip "Ports":
    Users who use an external load balancer may need to update their configuration after the upgrade to reference new port assignments.
