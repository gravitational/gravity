# Gravity Enterprise

This repository contains the enterprise features of `gravity`. 
The rest of this file (below) has been migrated from the old
`telekube` repository and probably needs updating.

## Versioning

Please follow [Versioning spec](https://github.com/gravitational/wiki/blob/master/docs/engineering.md#releasing-and-versioning).

## Development Quickstart

**Install Telekube on Vagrant**

Change working directory to `vagrant` folder and execute:

```sh
$ cd vagrant
$ vagrant up
```

This will bring 3 VMs properly configured for development

Build telekube:

```sh
$ make production telekube
```

Install telekube:

```sh
$ cd vagrant
$ make ansible-install
```

You can read more in  [Ansible and Vagrant development guide](vagrant/README.md)

## Ops Center Development Quickstart (Linux)

WARNING: this is hard section, most likely you will get stuck without extra help

* [Install libvirt](https://help.ubuntu.com/lts/serverguide/libvirt.html)
* [Install Vagrant](https://www.vagrantup.com/downloads.html)
* [Install Vagrant libvirt plugin](https://github.com/vagrant-libvirt/vagrant-libvirt)
* Golang [version 1.7.x](https://golang.org/dl/)
* Make sure you have `aws` CLI tool installed and configured with your AWS
  credentials. If `aws s3 ls` works, it's good sign.
* Build Telekube Ops Center and start Gravity
* Ubuntu 15.04+, Debian 8.0+

```
# builds and compiles everything
make production
# installs gravity and teleport on your system
make install
# runs an etcd instance for ops center
make etcd
# imports packages into opscenter
make packages
# starts gravity ops center
make start
```

### Quick recompile

If you only need to recompile telekube binaries (tele and gravity):

```
# goinstall rebuilds binary
# start starts the ops center
make goinstall start
```

### Rebuild dependencies

If you have updated any dependencies, e.g. you want to rebuild with a particular
version of teleport or planet:

```
TELEPORT_TAG=v0.2.0-beta.7 PLANET_TAG=v0.0.27 make production packages
```


It will update binaries on all hosts and redeploy the daemonset.


## Telekube explained

Telekube currently features two modes:

* Ops Center - where vendors upload packages, and from where you can install
  apps. When an application is installed, a new remote site is created.
  The staging version of OpsCenter is always deployed to https://portal.gravitational.io

* Cluster - the component that runs on customers infrastructure. It implements
  the necessary functionality to support the following cluster-local workflows:

    * Update/Uninstall the application
    * Run/Stop the application
    * Enforce cluster-specific access/usage policies
    * Collect events and implement site audit
    * Interact with on-site monitoring

## AWS Deployments

### Authoring new AWS AMIs

Launching instances on AWS requires a choice of an AMI (Amazon Machine Image) to boot it with.
When choosing an AMI, one has to keep in mind that AMIs are bound to certain
regions - in order to use the image in another region one can copy it into that
region prior to use.

AMIs come in three flavors: marketplace, public and private images. There's a
big caveat to a marketplace image which presents a problem to using open source
images. If you want another account to use a marketplace image, that account is
required to accept a license agreement (EULA) which can only be done by
visiting a web page. Obviously this complicates deployments across AWS
accounts.

One way to solve this problem is by cloning the official open-source image to
remove the marketplace binding.

[This link](https://bugs.centos.org/view.php?id=6228) describes a solution
(although it disregards an important concern and should not be followed
verbatim). The basic idea is to clone a root volume of an instance launched
from an official AMI to another volume and use that to bootstrap another
instance which can be used for creating an image without marketplace binding.

Here are the steps for creating an AMI:

  - Spin up an instance using a marketplace image
  - Make any necessary changes to the system that you want in the new AMI
  - Stop this instance and detach its root volume
  - Spin up another instance (from an AMI of your choice) and attach the above volume to it (for example, as /dev/xvdf)
  - Create a new EBS volume (no snapshot) with characteristics matching the
    root volume from above - 8 GiB standard gp2 24/3000 IOPS volumes are used
    as root volumes by default
  - Attach this new volume to the previously started instance  (for example, as /dev/xvdj)
  - It now should have a root volume and two attached volumes: /dev/xvdf and /dev/xvdj
    (Note, /dev/xvdj is arbitrary - volume mounts might be different in each case)
  - Format the second volume: `mkfs -t xfs /dev/xvdj`
  - Copy the other (attached previously as a root) volume using dd: `dd bs=65536 if=/dev/xvdf of=/dev/xvdj`
  - Re-attach the cloned volume to the stopped instance as a root value (i.e. as /dev/sda1)
  - Start the previously stopped instance (this is optional)
  - Create an image from the instance

Currently, the `ami-73619113` (based on CentOS Linux 7 x86_64 HVM EBS 1602 with
no marketplace binding) is the image of choice for deployments in `us-west-2`.

Please, refrain from using `ami-e3da2a83` (CentOS Linux 7 x86_64 HVM EBS 1602_gravitational) - it has been
cloned from a running system and suffers from integrity inconsistency.

We have a hardcoded list of AMIs mapped to a region and availability zone:

(lib/cloudprovider/aws/regions.go)
```go
// Regions defines a map of supported EC2 regions to various attributes
// like machine image and availability zone to use in any specific region.
var Regions = map[RegionName]RegionMapping{
	// TODO: map additional regions and AMIs
	NVirginia: {Image: "ami-366be821", AvailabilityZone: "us-east-1d"},
	Oregon:    {Image: "ami-14b07274", AvailabilityZone: "us-west-2a"},
}
```

Anytime you need to make an AMI available for installation in any specific
region (by creating a new AMI or copying an existing one to the new region)
make sure you update this list to make the AMI available to the installer.

### AMIs we have in use

**Northern Virginia** (us-east-1)
 - ami-366be821 (CentOS 7 (Enhanced Networking / lvm2))
 - ami-15673770 Debian Jessie (Source: 126027368216/Debian Jessie) (Copied ami-e41dfdd7 from us-west-2 Debian Jessie)
 - ami-61bbf104 CentOS 7 (Source: aws-marketplace/CentOS Linux 7 x86_64 HVM EBS 20150928_01-b7ee8a69-ee97-4a49-9e68-afaee216db2e-ami-69327e0c.2)

**Oregon** (us-west-2)
 - ami-14b07274 (CentOS 7 (Enhanced Networking / lvm2))
 - ami-ff4fbf9f CentOS 7 with [Enhanced Networking](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/enhanced-networking.html) (Name: CentOS 7 (Enhanced Networking)), based on ami-73619113
 - ami-73619113 CentOS 7 (Name: CentOS Linux 7 x86_64 HVM EBS 1602 28Apr), **no marketplace binding**
 - ami-2bc92c4b Debian Jessie (Name: debian), **no marketplace binding**
 - ami-b0e805d0 Other Linux (Name: jenkins-feb-26)
 - ami-b3332dd2 CentOS (Name: secure-centos)
 - ami-4fd6262f CentOS 7 (Name: CentOS Linux 7 x86_64 HVM EBS 1602-b7ee8a69-ee97-4a49-9e68-afaee216db2e-ami-d7e1d2bd.3)
 - ami-d440a6e7 CentOS 7 (Name: CentOS Linux 7 x86_64 HVM EBS 20150928_01-b7ee8a69-ee97-4a49-9e68-afaee216db2e-ami-69327e0c.2)

**Northern California** (us-west-1)
 - ami-1f51915b Debian Jessie (Copied ami-e41dfdd7 from us-west-2)
 - ami-f77fbeb3 CentOS 7 (Source: aws-marketplace/CentOS Linux 7 x86_64 HVM EBS 20150928_01-b7ee8a69-ee97-4a49-9e68-afaee216db2e-ami-69327e0c.2)

### Installing AWS-integrated applications

In order to install an application with AWS-integration, add
[kubernetes-deployer] policy to your user account if it has not already been
added (Our `developers` group already has this policy attached!). The policy
adds another set of permissions that are required to run terraform AWS
provisioning scripts.

### Upload artifacts to Amazon S3

We want to keep some of the build artifacts accessible and have an experimental
support for deploying them to Amazon S3. There's a private bucket used for
builds: s3://build.gravitational.io and makefiles are tailored to pull from
that bucket first before doing the build for a particular dependency.

To be able to deploy artifacts from local machine you need to set up aws CLI tools.
Download and install:

```shell
$ python -m pip install awscli
```

Then do upload:

```shell
$ make upload
```

### Monitoring

Refer to [monitoring](https://github.com/gravitational/monitoring-app) page for
details about application monitoring.


### Logging

Refer to [logging](https://github.com/gravitational/logging-app) page for details.


### Note on build versioning

With the introduction of build artifacts on Amazon S3 it becomes important to
consistently name the (git) tags. Since the artifacts deployment uses `git describe`
as a simple artifacts versioning scheme, it is no longer valid to tag a build
with a name that would break if used as a directory name.  For consistency,
name the tags to be [semver](http://semver.org/)-complaint.

Discouraged naming:

```bash
$ git tag
my/tag
fixes/this/and/that
```

Better naming:
```bash
$ git tag
v0.0.1
v0.0.2-alpha
```

# Telekube OpsCenter

The OpsCenter is the heart of Telekube. It manages user identities, vendors
with their applications and installed instances of said applications. Installed
applications run on remote sites and they "dial back home" to the OpsCenter who
initiated their installation.

In addition to being the centralized hub of many sites, a OpsCenter can also be
installed on-site for those use cases when a fully autonomous and disconnected
mode is desired.

OpsCenter is a web application but nearly all functions are accessible via CLI
as well.

## Staging Environment

Currently OpsCenter is deployed on https://portal.gravitational.io
To SSH into it:

```bash
> ssh -p 61822 admin@portal.gravitational.io
```

... assuming your SSH key is published in the [RSA S3 bucket](https://wiki.gravitational.io/hosting/our-aws/#ssh-keys)
and added to `deploy/provision.yaml`.

## Development Mode

To start OpsCenter in development mode, we need to have the following

* go version >= 1.5.4 < 1.6
* docker version >= 1.8.x
* gravity itself (use latest master)

**Setting up Go and Docker**

Check out these docs to install Go and docker:

* https://docs.docker.com/engine/installation/ubuntulinux/
* https://golang.org/doc/install

**IMPORTANT**: make sure you can run docker without sudo, otherwise it won't work

**Setting up dnsmasq**

Install dnsmasq and make sure it has the following in its configuration:

```
address=/opscenter.localhost.localdomain/127.0.0.1
```

You can create `/etc/dnsmasq.d/opscenter.conf` that contains the line above and
include it in the main config file `/etc/dnsmasq.conf`.

Also, make sure it is configured as your resolver in `/etc/resolv.conf`:

```
nameserver 127.0.0.1
```

**Building components**

`make` this will build gravity and all dependencies:

```shell
make production
sudo make install
```

**Accessing OpsCenter**

URL:  https://localhost:33009/
Use your Google @gravitational.com account to sign in in web UI.

When working with local OpsCenter, use the default admin account (admin/gravity+1) for login:
```sh
$ gravity ops connect https://localhost:33009
```

Local user accounts are configured in `assets/local/gravity.yaml`.

**Development mode**

Trust your localhost certs in chrome.Open this in browser and enable mode
"Allow invalid certificates for resources loaded from localhost."

```
chrome://flags/#allow-insecure-localhost
```

There is a pair of build targets specifically aimed at development: `dev` and `run`.

The `dev` target does not pull / recompile any dependencies - all it does is
build gravity executable.  You can place alternative versions of dependencies
(applications and other packages) directly inside `build` to make gravity use
them.

For example, simply copying a new version of planet (`planet-master.tar.gz` and
`planet-node.tar.gz`) will make it available in the next run of `make dev run`.

The `run` target starts a local gravity OpsCenter used to control various
gravity tasks, including but not limited to installing a new site or deleting
an active site.

```shell
make dev run
```

## Provisioning new Ops Centers

Check out https://github.com/gravitational/ops/tree/master/opscenter#provisioning-opscenter

## Generating installers

There are two ways to create a tarball with a self-sufficient application
installer. See "Creating sites" section below to learn how to use installers to
install applications.

### Via an Ops Center

The command below downloads a generated installer tarball from a remote Ops
Center. The application the installer is requested for should be present in the
Ops Center.

```shell
$ tele pull --ops-url=<opscenter address> <app package> <dir>
```

For example, to request a "telekube" app installer from the locally running Ops
Center and unpack it into "installer" directory:

```shell
$ tele login -o https://localhost:33009
$ tele pull --ops-url=https://localhost:33009 telekube
```

### Without an Ops Center

To build an application installer in a standalone mode, all that's required is
the application manifest:

```shell
$ tele build <path to manifest> -o <tarball name>
```

For example, to build a Mattermost installer from our Quick Start Guide:

```shell
$ tele build mattermost/resources/app.yaml -o installer.tar.gz
```

## Creating clusters

### Bare metal install via Ops Center

1. Start Ops Center
2. Start Vagrant machines (`vagrant up` from `assets/vagrant/3node`)
3. Log into the Ops Center, pick an app to install and select "Bare metal" mode
4. When done, destroy Vagrant machines (`vagrant destroy`)

_Hint: It is possible to start only select Vagrant machines instead of all 3 by
specifying their names when spinning them up: `vagrant up node-1` to start just
one, `vagrant up node-1 node-2` to start two, etc. This may be useful if you
want to install just 1-node cluster and/or short on disk space as each of the
machines requires 2 10GB disks._

### Using standalone installer

1. Generate a standalone installer, like described in the "Generating
   installers" section above
2. Start Vagrant machines (`vagrant up` from `assets/vagrant/3node`)
3. Upload installer to one of Vagrant machines (`vagrant scp` plugin may come
   in handy) and unpack it

#### Graphical installation

4. Launch `sudo ./install` from the unpacked installer directory and follow prompts

#### CLI installation

5. From the unpacked installer directory run:

```shell
$ ./gravity install --cluster=<cluster name> --advertise-addr=<node's address> --token=<join token>
```

6. To later add another node to the cluster, upload/unpack the same installer
   tarball to another node and run:

```shell $ ./gravity join --advertise-addr=<node's address> --token=<same join
token in install>
```

### Installing on Mac

You can deploy a gravity cluster on a Mac using vagrant.

Prerequisites:

 - Mac - tested with OSX 10.9.5
 - [vagrant] - tested with 1.8.1
 - [virtualbox] - whatever version vagrant installs automatically (tested with 5.0.16)
 - [Vagrantfile](assets/onprem/Vagrantfile)

Any application with an `onprem` installer can be used for installation.

Install vagrant if necessary, and use the provided
[Vagrantfile](assets/onprem/Vagrantfile) to start a cluster of three nodes:

```shell
$ vagrant up
```

The Vagrantfile assumes the OpsCenter is running locally - if this is not the
case, the easiest would be to route to OpsCenter via `localhost` - i.e.
override `localhost` to the address of the computer running the OpsCenter.

For example, assuming the OpsCenter is running on `192.168.178.32` (where it is
running as `localhost`) - temporarily update `/etc/hosts` on each node to point
to the OpsCenter:

```
192.168.178.32 localhost
```

Open the installer on `https://localhost:33009/web/login` and start the `bare
metal` installation. After the agent link has been generated, use it on each of
the instances to download and start an install agent.

Follow the instructions to configure the proper IPs and roles for the nodes and
continue the installation through to completion.


## Accessing installed clusters

### Prerequisites

To be able to access remote sites from your laptop, you will need `tsh` version
`>=1.2.5` (github.com/gravitational/teleport), `kubectl` and `tele` binary from
this repo.

### Logging into sites

To log into installed site, you can do:

```
✗ tele --insecure login -o <opscenter> <site>
```

This will bring up a browser OIDC login form.

If you don't know what sites are available, it is possible to log into OpsCenter first:

```
✗ tele --insecure login -o opscenter.localhost.localdomain:33009
```

then inspect available clusters using `tsh`:

```
✗ tsh clusters
Cluster Name                        Status
------------                        ------
opscenter.localhost.localdomain     online
example.com                         online
```

and then login into an installed site:

```
✗ tele --insecure login -o opscenter.localhost.localdomain:33009 example.com
```

After successful login, you should be able to use `tsh` and `kubectl` directly from your laptop.

For example, to list servers for the selected site:

```
✗ tsh ls
Node Name                                        Node ID                                          Address                  Labels
---------                                        -------                                          -------                  ------
abad58a0029c4c3cbb944e669203280c.example.com     abad58a0029c4c3cbb944e669203280c.example.com     192.168.122.176:3022     advertise-ip=192.168.122.176,role=node
```

To run a command on one of servers:

```
 tsh ssh root@192.168.122.176 gravity ls --namespace=kube-system
+----+---------------------+---------------+-----------------------------------------------------------------+-----------------+
| #  |         POD         |   CONTAINER   |                             LABELS                              |     HOST IP     |
+----+---------------------+---------------+-----------------------------------------------------------------+-----------------+
|  1 | bandwagon           | bandwagon     | app=bandwagon                                                   | 192.168.122.176 |
|  2 | grafana-tju1i       | grafana       | app=grafana                                                     | 192.168.122.176
...
```

Use use `kubectl` directly:

```
✗ kubectl get pods --namespace=kube-system
NAME                  READY     STATUS    RESTARTS   AGE
bandwagon             1/1       Running   0          12m
grafana-tju1i         1/1       Running   0          11m
gravity-site-gqea9    2/2       Running   0          11m
...
```

Note: For this to work, make sure you have dnsmasq installed and configured as
described in the "Setting up dnsmasq" section somewhere above, and that it is
used as your resolved in `/etc/resolv.conf`.

### Web development

Refer to [web/README.md](web/README.md) for instructions on how to build Web UI.

### Applications

In gravity, a concept of an application is built on top of a package concept
with a bit of metadata thrown into the mix.  The application metadata is
defined inside an [application manifest](#application-manifest).

While applications are, in fact, packages, the extra information specific to
them requires additional handling - therefore, gravity provides an additional
layer for managing applications, including a whole CLI subsystem - `app`:

```shell
$ gravity help app
usage: gravity app <command> [<args> ...]

operations on gravity applications
Flags:
  --help      Show help (also see --help-long and --help-man).
  --debug     Enable debug mode
  --insecure  Skip TLS verification
  --state-dir=STATE-DIR
              directory where gravity stores its state

Subcommands:
  app import [<flags>] <src> <pkg>
    Import k8s application into gravity

  app export [<flags>] <pkg>
    export gravity application

  app delete [<flags>] <pkg>
    delete gravity application

  app install [<flags>] <pkg>
    install gravity application

  app list [<flags>] [<repo>]
    list installed applications

  app uninstall [<flags>] <pkg>
    uninstall application

  app status [<flags>] <pkg>
    get app status

  app pull --ops-url=OPS-URL [<flags>] <pkg>
    pull an application package from remote OpsCenter

  app push --ops-url=OPS-URL <pkg>
    push an application package to remote OpsCenter

  app hook <pkg> <hook-name>
    run the specified application hook
```

Creating a gravity application is a two step process: creating the package (or
a package template) and importing it.  The format of an application package is
described [here](#application-package).  After the application package has been
created, it can be imported into gravity using the `import` command:

```shell
$ gravity app import package.tar.gz
```

Please note that the application metadata, like repository name, application
name and application version are taken from the application manifest file found
inside the package. It is possible, however, to override any of these values,
for example:

```shell
$ gravity app import package.tar.gz --repository=anotherrepo.io --name=anothername --version=4.5.6
```

Use help to get details on `gravity app import command`:

```shell
$ gravity help app import
```

Note, that applications are referenced as any other packages in gravity.

The import operation can either operate on a self-contained packages or vendor
from a package template (which can be a directory).

The import operation also checks that all application's dependencies are
present in the import destination (e.g. remote OpsCenter) and
will fail if any of the dependencies cannot be satisfied.

The self-contained application packages are tarballs with a copy of each
(docker) container image referenced in application resources.  As such, a
self-contained package can be imported offline (without access to docker and
internet).

When vendoring, import does the following:

  * Pulls all container references offline (containers will be stored in the
    project's `registry` directory)
  * Rewrites all container image references in resource files to point to the
    private docker registry

In either mode, the package is then imported into gravity using the standard
package management layer.

Here's an example of using vendoring mode:

```shell
$ gravity app import --vendor --registry-url=apiserver:5000 --glob="**/*.yaml" assets/mattermost --state-dir=/var/lib/gravity
```

By default, the vendoring mode uses `apiserver:5000` as private docker registry
address.  The `**/*.yaml` glob pattern is also a default but other patterns can
be provided to relax or narrow the resource filter.  Note, that `app install`
does not rewrite images anymore - so it is important that the imported
applications have their container image references pointing to proper private
docker registry. The `vendor` mode will probably be made default in future.

Vendoring mode rewrites image names in all found application resources,
including application manifest.


**Rewriting image names**

In addition to rewriting image references, vendoring mode can also rewrite
images which is useful if you want to use `latest` tag locally but import with
an appropriate version into remote OpsCenter:

```shell
$ gravity app import --set-image=postgres:9.3.5 k8s-aws.tar.gz
```

**Setting Dependencies**

If your application depends on some package and lists it in the `dependencies` section, e.g.:

```
dependencies:
   apps: gravitational.io/ntp-app:0.0.0
```

And you want to dynamically set the version during application build, you can use `set-dep`

```shell
$ gravity app import --set-dep=gravitational.io/ntp-app:1.2.1 k8s
```

This command works both for application and package dependencies

**Context**

When used w/o parameters - import creates a package in the implicitly local
context. You can override the context to use for the operation by specifying
the state folder to use with `--state-dir` parameter:

```shell
$ gravity app import k8s-aws.tar.gz --state-dir=/var/lib/gravity
```

Note, that specifying the state directory should be considered an
implementation detail and this workflow will be improved in future. This is
fine for a development workflow though.

An application can also be imported into remote OpsCenter by providing an
`--ops-url` parameter:

```shell
$ gravity app import k8s-aws.tar.gz --ops-url=https://opscenter.gravitational.io --vendor
```

### Application Manifest

Every gravity application needs a manifest that describes how to install and configure it.

A manifest starts with an application metadata section (similar to any
kubernetes resource definition):

```yaml
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: app
  resourceVersion: 0.0.1
```

There are 3 kinds of applications.

Bundles are user applications, the ones that can be installed via Ops Centers
or standalone installers:

```yaml
kind: Bundle
```

System applications provide essential services to user apps but cannot be
installed by themselves and are used as dependencies for user apps (think DNS,
NTP, logging, etc.):

```yaml
kind: SystemApplication
```

Runtimes are low level backbone applications (think Kubernetes):

```yaml
kind: Runtime
```

#### Runtimes

User applications are based on runtime apps and inherit certain configuration
from them. By default, an application has "kubernetes" app of the latest
available version as a runtime, but it can be overridden:

```yaml
systemOptions:
  runtime:
    version: "1.4.6"
```

Inheriting configuration is a simple process of adopting all attributes not
explicitly defined in the manifest.  If a configuration section is found in
both manifests, they are merged.

#### Providers

Providers section describes a set of supported infrastructure providers. The
following are supported currently:

 * **aws**: AWS EC2 provider for automatic provisioning of clusters in AWS EC2
 * **generic**: For bare metal provisioning

Each provider can be configured with a specific type of networking.  Currently
following networking types have been defined:

 * **aws-vpc**: Networking type specific to AWS (flannel CNI with AWS-VPC backend)
 * **calico**: Networking type using [calico] (BGP + direct iptables configuration)
 * **flannel**: Networking type using [flannel] (flannel with VXLAN backend)

Networking type can be defined either on the provider level:

```yaml
providers:
  aws:
    network: aws-vpc
```

Among other useful fields that user apps can specify in their "providers"
section are terraform scripts and supported regions:

```yaml
providers:
  aws:
    terraform:
      script: file://terraform.tf
      instanceScript: file://instance.tf
    regions:
      - us-east-1
      - us-west-2
```

All providers are enabled by default. To disable a provider:

```yaml
providers:
  azure:
    disabled: true
```

#### Installer

Installer section allows to customize installer behavior, for example enable
EULA and specify flavors:

```yaml
installer:
  eula:
    source: file://eula.txt
  flavors:
    prompt: "How many nodes do you want in your cluster?"
    items:
      - name: "3"
        description: ""
        nodes:
          - profile: node
            count: 3
```

When installing an application, a user will be presented with a choice of one
of the defined flavors to install.

#### Node profiles

This section allows to define specific node profiles and their hardware requirements.

```yaml
nodeProfiles:
  - name: node
    description: "Worker node"
    requirements:
      cpu:
        min: 4
      ram:
        min: "8GB"
    providers:
      aws:
        instanceTypes:
          - c3.2xlarge
```

The `instanceTypes` field lists instance types that satisfy the profile's
requirements for each cloud provider. When deploying on the cloud provider's
infrastructure, the installer will present a user with a choice of an instance
type to provision.

#### Dependencies

Another facet of application configuration is listing application dependencies
- packages and other applications this application depends on. The dependency
configuration is pretty straight-forward:

```yaml
dependencies:
  packages:
  - name: gravitational.io/teleport:0.1.10-alpha
    selector:
      role: teleport
  - name: gravitational.io/planet-master:1.2.3
    selector:
      role: planet-master
  - name: gravitational.io/planet-node:1.2.3
    selector:
      role: planet-node
  apps:
  - gravitational.io/dns-app:0.0.2
```

#### Docker

Docker storage driver can be overriden from the default "devicemapper":

```yaml
systemOptions:
  docker:
    storageDriver: overlay
```

### License

The application manifest can also have a licensing section:

```yaml
license:
  enabled: true
```

If licensing is enabled (by default it is disabled), the installer will ask a
user to provide a valid license before the installation.

Normally, a license is a certificate + private key that can be generated in
OpsCenter. This is the default type of license (`certificate`) and it can be
omitted from manifest. Some customers, though, use their own license format as
a JSON blob with a signature and in this case the `type` should be explicitly
set to `payload`. An application that accepts one type of license will not
accept another.

Licenses enforce two restrictions: expiration time (mandatory) and maximum
amount of servers (optional). If a license has a restriction on a maximum
number of servers, operations that will attempt to exceed this number will be
rejected. For instance, if a license allows a maximum of 3 servers, then an
attempt to install a 4-node cluster will be denied, as well as an attempt to
expand a 3-node cluster.

If license installed on site expires, the site goes into `degraded` state and
cannot perform any operations until the license has been updated. Optionally, a
license can be generated with "stop app on expiration" setting which will stop
the application running on site and start it back up once the license has been
updated.

### Application Package

The application package format has the following structure:

<pre>
 ├─registry
 │   └─docker
 │      └─registry
 │         └─...
 └─resources
     └─app.yaml
</pre>

Where,

  - `registry` contains the copies of each (docker) container image referenced
    by the application; the contents of this directory are in the format
    expected by the docker registry v2.x and should be considered opaque as far
    as package format goes
  - `resources` directory lists any application-specific resources - in files
    or sub-directories. The only mandatory requirement of this directory is a
    file named `app.yaml` - the application manifest. The other part of a
    configuration convention is that any file with extension .yaml or .json
    besides the application manifest in this directory is considered a
    kubernetes resource and is consumed during import. This directory is also
    mounted inside hook containers (described in the following section) so
    usually contains installation scripts and resources, etc.

### Hooks

*Hooks* manifest section is used for defining application-specific hooks that
are run on different stages of application (or rather, site) lifecycle. Gravity
supports the following hooks:

* install
* postInstall
* preUninstall
* uninstall
* update
* postUpdate
* rollback
* postRollback
* preNodeAdd
* postNodeAdd
* preNodeRemove
* postNodeRemove
* status
* info
* licenseUpdated
* start
* stop
* dump
* backup
* restore

Hooks are essentially Kubernetes jobs so in app manifest they are defined as
valid Kubernetes job specifications in the following format:

```yaml
hooks:
  install:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: k8s-install
      spec:
        template:
          metadata:
            name: k8s-install
          spec:
            restartPolicy: OnFailure
            containers:
              - name: hook
                image: quay.io/gravitational/debian-tall:0.0.1
                command: ["/usr/local/bin/kubectl", "create", "-f", "/var/lib/gravity/resources/e2e.yaml"]
```

The "resources" folder from the application's tarball gets mounted into all hooks containers under
`/var/lib/gravity/resources/` directory. In addition to that, the following is also mounted into hooks
containers:

* Planet's kubectl as `/usr/local/bin/kubectl`
* Host's `/etc/ssl/certs` directory

All hooks (= Kubernetes jobs) are run on the cluster's master node.

### Backup and Restore

Backup and restore functionality is provided by the two hooks `backup` and `restore`. Unlike the other hooks,
they can be fired on request with the gravity commands:
 - `gravity system backup <package> <archive>`
 - `gravity system restore <package> <archive>`

The implementation of the backup and restore hooks should work against the
directory `/var/lib/gravity/backup` -- when backing up, files put in this
directory will be combined into a tarball and given to the end user. When
restoring, this directory will contain the already-unzipped contents of the
archive (i.e. the files just as they were originally placed there).

Currently, these commands must be run from within planet. To avoid end-users
having to fully understand entering planet, the following commands work from
the node, outside of planet.

```sudo gravity planet enter -- --notty /usr/bin/gravity -- system backup gravitational.io/package:0.0.0+latest /ext/share/backup.tar.gz```

This will leave the backup archive in `/var/lib/planet/share/` on the host,
outside of planet.

This command will generate an archive of the system state:

 ```sudo gravity planet enter -- --notty /usr/bin/gravity -- system restore gravitational.io/package:0.0.0+latest /ext/share/restore.tar.gz```

This restore command will require the archive being restored from to be placed
in `/var/lib/planet/share` prior to running.

### Automatic vs onprem provisioning and state configuration

Recent changes simplified the provisioning of nodes for both automatic and
manual (onprem) cases: the state directory (`/var/lib/gravity` by default) will
be created (and mounted) automatically. This is possible due to several
changes:

  - we have currently only RHEL (7.x) and CentOS (7.x) images in use (both for
    local and AWS deployments)
  - as a result of the above, we now install and configure docker with
    `devicemapper` storage driver by default
  - agent download URL has been split into several variables for better control
    (more on this later)

#### Agent download URL format

The old agent download URL used to be a black box
`{{.variables.system.instructions}}` that contained a CURL command combined
with all required attributes and was generated by opscenter for each possible
scenario. Obviously this was not too flexible.

The new format exposes more of the parts making up a url and can be extended
for use in any script or in the application manifest:

```
curl --tlsv1.2 {{if .variables.system.devmode}}--insecure{{end}} {{.variables.system.ops_url}}/{{.variables.system.token}}/auto?system_device=vdb&docker_device=vdc&mount=data:/var/lib/app/data&mount=logs:/var/logs/app
```

Note the `devmode` variable exposed only to enable the insecure TLS communication in development - this is considered an implementation detail and should only be used for development.

 - `ops_url` contains the URL prefix with the opscenter address
 - `token` is the reference to the auto-generated access token. Might be merged
   with `ops_url` as it is always required and is not subject to customizations
 - `auto` defines the node role. In this case (for automatic provisioning), the
   role is irrelevant (as automatically inferred), so a special placeholder is
   used. Other possible values are `master`, `node` or `db` - names of server
   profiles from the manifest.
 - `system_device` is an optional configuration parameter that defines the name
   of the (unformatted) device to use with gravity. If specified, it will be
   formatted as (`xfs`/`ext4`) and mounted as `/var/lib/gravity`
 - `docker_device` is an optional device name to use for docker devicemapper
   storage configuration (`direct-lvm` mode).
 - `mount` is an optional mount specification in the format
   `mount-name`:`/path/on/host`. Can be specified multiple times. This can be
   used to override mount configuration from the manifest by providing new
   values for the `Source` directory.


### Deploying new versions of planet

Planet builds are the most time-consuming step of the gravity build so it makes
a lot of sense to have planet build artifacts available to bootstrap it.

Whenever a new version of planet is released - it is tagged and pushed to
Amazon S3 build bucket:

```shell
$ git tag -a v0.0.215 -m "Added new logging infrastructure and bumped k8s version to 2.3.4"
```

On jenkins, use the [Planet-deploy-artifacts] project to build and deploy new
version of planet to S3.

Update the gravity Makefile to use the new planet tag:

```make
PLANET_TAG := v0.0.215
```

### Schema migrations

Refer to [migrations] section for details.

### Publishing Telekube artifacts into the distribution Ops Center

There are two Makefile targets for publishing Telekube artifacts into get.gravitational.io:

* `make publish-telekube`  <-- builds tele/gravity/tsh binaries for both Linux and Mac and pushes them to the "get"
* `make publish-artifacts` <-- builds telekube and opscenter apps and pushes them to the "get" along with all dependencies

You don't have to run them manually as there are Jenkins jobs configured for them:

* https://jenkins.gravitational.io/job/TelekubePublishBinaries/
* https://jenkins.gravitational.io/job/TelekubePublishArtifacts/

### Troubleshooting

If there is a problem, simply wipe out the contents of /var/lib/gravity/opscenter and restart the OpsCenter. If it does not help, ping Sasha in slack.


[//]: # (Footnotes and references)

[kubernetes-master]: <assets/aws/kubernetes-master.json>
[kubernetes-node]: <assets/aws/kubernetes-node.json>
[kubernetes-basic]: <assets/aws/kubernetes-basic.json>
[kubernetes-deployer]: <assets/aws/kubernetes-deployer.json>
[libvirt]: <http://libvirt.org>
[vagrant]: <https://www.vagrantup.com/downloads.html>
[virtualbox]: <https://www.virtualbox.org/wiki/Downloads>
[terraform]: <https://www.terraform.io/>
[Planet-deploy-artifacts]: <https://jenkins.gravitational.io/job/Planet-deploy-artifacts/>
[onprem Vagrantfile]: <assets/onprem/Vagrantfile>
[flannel]: https://github.com/coreos/flannel
[calico]: https://www.projectcalico.org/
[migrations]: <assets/migrations/>
