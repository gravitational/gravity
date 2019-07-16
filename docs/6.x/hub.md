# Introduction

The Gravity Hub is a multi-cluster control plane available in the Enterprise
version of Gravity. It serves two purposes:

1. Gravity Hub acts as a central repository for cluster images, allowing an
   organization to share pre-built clusters.
2. Gravity Hub reduces the operational overhead of managing multiple Kubernetes
   clusters created from cluster images.

Users of Gravity Hub can:

* Publish cluster images and manage their versions.
* Download cluster images and quickly create production-ready clusters from them.
* Remotely manage Kubernetes clusters either via command line (CLI) or via a web interface.

!!! warning "Version Warning":
    The Gravity Hub is only available to users of Gravity Enterprise.

This chapter will guide you through the process of downloading and installing
your own instance of Gravity Hub.

## Installing Gravity Hub

In this section we'll cover how to install your own instance of Gravity Hub
on your own infrastructure. The end result will be an autonomous Kubernetes 
cluster with Gravity Hub running inside.

Gravity Hub itself is packaged and distributed as a cluster image.  Please
[contact us](https://gravitational.com/gravity/demo/) to get a trial license
key and the Gravity Hub cluster image.

As with any Gravity cluster image, you will also need a Linux server to install
Gravity Hub. Assuming you have received a copy of Gravity Hub from Gravitational,
you'll have it on a Linux server of your choice:

```bash
$ ls -lh
-rw-r--r-- 1 user user 1.3G Feb 20 13:02 gravity-hub-6.0.1.tar
```

The name of the tarball will vary based on the version of Gravity you're using,
so we'll refer to it simply as `gravity-hub.tar` below.

#### Generating a Token

To establish trust between Gravity Hub and multiple K8s clusters, a common
shared hard-to-guess secret (token) must be generated first. Therefore, before
installing Gravity Hub, a shared token needs to be generated and stored in
an environment variable named `TOKEN`:

```bsh
$ export TOKEN="$(uuidgen)"
```

Next, expand the cluster image and launch the installer:

```bsh
$ tar xvf ./gravity-hub.tar
$ ./gravity install --advertise-addr=10.1.1.5 \
                    --token=$TOKEN \
                    --flavor=standalone \
                    --cluster=hub.example.com \
                    --ops-advertise-addr=hub.example.com:443
```

* `advertise-addr` is private IPV4 address of a K8s master node (this node) which will be used by other K8s nodes to form a cluster.
* `flavor` is the cluster configuration flavor to install; choose `standalone`
  for a single-node install which is great for evaluation/development purposes
  or `ha` to install a 3-node cluster suitable for production use or
  high-availability
* `ops-advertise-addr` should be a DNS name publicly accessible via internet
* `token` is a security token for nodes to join to the cluster
* `cluster` is a unique cluster name, e.g. `hub.example.com`

## Post-provisioning

#### Setting up DNS

After provisioning of Gravity Hub cluster, create the DNS A-records pointing at
either the provisioned cloud load balancer (if the cluster was created on a
cloud account) or at the IP of the host.

!!! tip "Wildcard DNS name":
      The Gravity Hub DNS records must contain the wildcard, both `*.hub.example.com`
      and `hub.example.com` should point to the public IP address of the
      Gravity Hub cluster.

#### Setting up OIDC

After installation [OIDC provider](/cluster/#configuring-a-cluster) should be
set up in order to log into Gravity Hub.

#### Setting up TLS Key Pair

After installation, a valid [TLS key pair](/cluster/#configuring-tls-key-pair)
should be set up in order to log into Gravity Hub. Self-signed certificates are
currently not supported.

#### Configuring endpoints

See [Configuring Gravity Hub Endpoints](/cluster/#configuring-ops-center-endpoints)
for information on how to configure Gravity Hub management endpoints.

## Upgrading 

This section assumes that you have downloaded the newer version of Gravity Hub
cluster image called `new-hub.tar`. Log into a root terminal on one of the servers 
running Graivty Hub cluster and extract the tarball there:

```bsh
$ tar xvf new-hub.tar
```

Start the upgrade procedure using `upgrade` script:

```bsh
$ ./upgrade
```

Read more about upgrade procedure [here](/cluster/#performing-upgrade).

!!! tip "Ports":
    Users who use an external load balancer may need to update their
    configuration after the upgrade to reference new port assignments.

## Using Gravity Hub

Once a cluster image is built by `tele build`, it can be deployed and installed
by publishing it into the Gravity Hub. The commands below are used to manage
the publishing process.

First, you have to login into Gravity Hub via `tsh login` command.


```bash
# Use tele push to upload a cluster image to the Gravity Hub:
$ tele push [options] tarball.tar

Options:
  --force, -f  Forces to overwrite the already-published application if it exists.
```

`tele pull` will download a cluster image from the Gravity Hub:

```bash
$ tele [options] pull [application]

Options:
  -o   Name of the output tarball.
```

`tele rm app` deletes a cluster image from the Gravity Hub.

```bash
$ tele rm app [options] [application]

Options:
  --force  Do not return error if the application cannot be found or removed.
```

`tele ls` lists the cluster images currently published in the Gravity Hub:

```bash
$ tele [options] ls

Options:
  --all   Shows all available versions of images, instead of the latest versions only
```


