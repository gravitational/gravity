# Setting up an Ops Center

An Ops Center controls access and lifecycle of Applicaion Clusters and provides a distribution endpoint
for Application Bundles to be installed on Telekube Clusters.

## Pre-requisites

 - [Telekube binaries](quickstart.md)
 - The Gravitational [Quickstart Repository](https://github.com/gravitational/quickstart)

## Generating a token

To install an Ops Center, a shared token needs to be generated to allow multiple nodes of a Cluster to join. This token will be used in an environment variable named `TOKEN`:

```bash
$ export TOKEN="$(uuidgen)"
```
## Automatic Provisioning

Included in the [Quickstart repository](https://github.com/gravitational/quickstart/tree/master/opscenter) is a configuration to provision a Vagrant VM, as well as an AWS instance to run the Ops Center.

### Manual Provisioning

Install Telekube:

```bash
$ curl https://get.gravitational.io/telekube/install | bash
```

Pull the latest Ops Center:

```bash
$ tele pull opscenter -o installer.tar
$ tar xvf ./installer.tar
```

Run the standalone install:

```bash
$ ./gravity install --advertise-addr=(server IP address) --token=(TOKEN) --flavor=(flavor) --cluster=(cluster name) --ops-advertise-addr=example.com:443
```

* `advertise-addr` is private IPV4 address used for nodes to communicate to each other
* `flavor` is the cluster configuration flavor to install; choose `standalone`
for a single-node install which is great for evaluation/development purposes or
`ha` to install a 3-node cluster suitable for production use or high-availability
* `ops-advertise-addr` should be a DNS name publicly accessible via internet
* `token` is a security token for nodes to join to the cluster
* `cluster` is a unique cluster name, e.g. `example.com`

## Post-provisioning

#### Setting up DNS

After provisioning, DNS records should be created with hostname at either the provisioned ELB load balancer (for AWS) or the IP of the virtual machine (for Vagrant)

!!! tip "Wildcard DNS name"
	  The Ops Center DNS records should be wildcard, both `*.opscenter.example.com` and `opscenter.example.com` should point to the IP address
	  of the Ops Center service or load balancer.

#### Setting up OIDC

After installation [OIDC provider](cluster.md#configuring-a-cluster) should be set up in order to login to the Ops Center.

#### Setting up TLS Key Pair

After installation, a valid [TLS key pair](cluster.md#configuring-tls-key-pair) should be set up in order to login to the Ops Center.

!!! tip "TLS Certificate"
    The Ops Center has to use a valid, not self-signed TLS certificate to function properly.

#### Configuring endpoints

See [Configuring Ops Center Endpoints](cluster.md#configuring-ops-center-endpoints)
for information on how to configure Ops Center management endpoints.

## Upgrading Ops Center

Log into a root terminal on the OpsCenter server.

Update the tele binary:

```
$ curl -LO https://get.gravitational.io/telekube/bin/{VERSION}/linux/x86_64/tele
$ chmod +x ./tele
```

Fetch the latest Ops Center application using `tele`:

```
$ ./tele pull opscenter:{VERSION} -o installer.tar
```

This will automatically download into the current directory as `installer.tar`.

This archive provides all dependencies required for the update, including new `gravity` binaries,
`install` and `upgrade` scripts.

Extract the tarball:

```
$ tar xvf installer.tar
```

Start the upgrade procedure using `upgrade` script:

```
$ ./upgrade
```

Read more about upgrade procedure [here](cluster.md#performing-upgrade).

!!! tip "Ports"
    Users who use an external load balancer may need to update their configuration after the upgrade to reference new port assignments.
