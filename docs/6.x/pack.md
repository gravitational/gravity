# Building Cluster Images

## Introduction

This section covers how to build a Gravity Cluster Image.
There are two use cases we'll cover here:

1. Building an "empty" Kubernetes Cluster Image. You can use these Cluster
   Images for quickly creating a large number of identical, production-ready
   Kubernetes Clusters within an organization.

2. Building a Cluster Image that also includes Kubernetes applications.  You
   can these to distribute Kubernetes applications to 3rd parties.

If you wish to package a cloud-native application into a Cluster Image, the
application must run on Kubernetes. This means:

* The application is packaged into Docker containers.
* You have Kubernetes resource definitions for application services, pods, etc.

You can optionally use Helm charts for your application(s).

!!! tip
        For easy Kubernetes development while porting applications to
        Kubernetes, we recommend [minikube](https://github.com/kubernetes/minikube),
        a Kubernetes distribution optimized to run on a developer's machine.

## Getting the Tools

Any Linux machine can be used to package a Kubernetes applications into a Cluster
Image. To get started, you need to [download Gravity](https://gravitational.com/gravity/download/).

!!! tip "Gravity Versions"
    For new users who are just exploring Gravity, we recommend the latest "pre-release" build. Make sure to select "Show pre-releases" selector. Production users must use the latest stable release.

To create a Cluster Image, you will be using `tele`, the Gravity build tool.
Below is the list of `tele` commands:

| Command        | Description |
|----------------|-------------|
| `tele build`   | Builds a new Cluster Image.
| `tele push`    | Pushes a Cluster Image to an image repository called Gravity Hub (enterprise version only).
| `tele pull`    | Downloads a Cluster Image from a Gravity Hub.
| `tele ls`      | Lists all available Cluster images.

## Building a Cluster Image

Before creating a Cluster Image, you must create an _Image Manifest_ file. An
Image Manifest uses the YAML file format to describe the image build and installation
process and the system requirements for the Cluster. See the [Image Manifest](pack.md#image-manifest) section below for more details.

After an Image Manifest is created, execute the `tele build` command to build
a Cluster Image.

```bsh
tele build [options] [cluster-manifest.yaml]

Options:
  -o           The name of the produced tarball, for example "-o cluster-image.tar".
               By default the name of the current directory will be used.
  --state-dir  Hello
```

The `build` command will read the `manifest.yaml` file and will make sure that
all of the dependencies are available locally on the build machine. If the
dependencies are not available locally, it will download them from the external
sources.

There are two kinds of external sources for Cluster dependencies:

1. **Kubernetes binaries** like `kube-apiserver`, `kubelet` and others, plus their
   dependencies like `etcd`. The build tool will download them from the Gravity Hub
   you are connected to. Users of the open source version of Gravity are always connected to the public Hub hosted on `get.gravitational.io`.
2. **Application Containers**. If your Cluster must have pre-loaded
   applications, their container images will be downloaded from the external
   container registry.

You can follow the [Quick Start](quickstart.md) to build a Cluster Image from a
sample Image Manifest.

#### Building with Docker

You can execute `tele build` from inside a Docker container. Using Linux
containers is a good strategy to introduce reproducible builds that do not
depend on the host OS. Containerized builds are also easier to automate by
plugging them into a CI/CD pipeline.

```bsh
# This Dockerfile builds a Docker image called `tele-buildbox`.  The
# resulting container image will contain `tele` tool and can be used
# to create Cluster images from within a container.

FROM quay.io/gravitational/debian-grande:buster

ARG TELE_VERSION
RUN apt-get update && \
    apt-get -y install curl make git apt-transport-https ca-certificates gnupg software-properties-common
RUN curl -fsSL https://download.docker.com/linux/debian/gpg | sudo apt-key add - && \
    add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/debian buster stable"
RUN apt-get update && \
    apt-get -y install docker-ce-cli
RUN curl https://get.gravitational.io/telekube/bin/${TELE_VERSION}/linux/x86_64/tele -o /usr/bin/tele && chmod 755 /usr/bin/tele
```

Set the `TELE_VERSION` argument to the desired Gravity version, then build the docker image:

```bsh
docker build . -t tele-buildbox:latest
```

Now you should have `tele-buildbox` container on the build machine. Next, do the following:

* Use host networking.
* Expose the Docker socket into the container to allow `tele` to pull container
  images referenced in the Image Manifest.
* Expose the working directory with the Image Manifest to the container as `/mnt/cluster`

The command below assumes that the Image Manifest `app.yaml` is located in the current directory:

```bsh
docker run \
       -v /tmp/tele-cache:/mnt/tele-cache \
       -v /var/run/docker.sock:/var/run/docker.sock \
       -v $(pwd):/mnt/cluster \
       -w /mnt/cluster \
       --net=host \
       tele-buildbox:latest \
       dumb-init tele --state-dir=/mnt/tele-cache build app.yaml -o cluster.tar
```

!!! note
    Notice that we are reusing tele loaded cache directory in between builds
    by setting `--state-dir`. You can use unique temporary directory
    to avoid sharing state between builds, or use parallel builds instead.

## Vendoring

When you execute the `tele build` command, `tele` discovers all Docker images
referenced by your cluster or application image resources (Helm charts or plain
Kubernetes spec files) and packages them inside the resulting cluster or
application image tarball. This process is called "vendoring" and it gives the
Gravity images their "self-sufficiency" property: all vendored images become
available in the in-cluster private Docker registry when the cluster is
installed and Kubernetes pulls them from this local registry when creating the
pods.

#### Private Docker Registries

To vendor a Docker image, it needs to be available locally. If an image being
vendored is not available via the local Docker client, `tele` will attempt to
pull it from the remote registry specified by the image, or the default [Docker
Hub](https://hub.docker.com) if the registry is not specified.

In case the image belongs to a private Docker registry, your local Docker client
must be configured with proper credentials for it (e.g. via `docker login` or
[TLS certificates](https://docs.docker.com/engine/security/certificates/)) in
order for `tele` to be able to pull it.

#### Image References Discovery

The `tele` tool can extract image references from all core Kubernetes objects
such as pods, deployments, replica and daemon sets and so on.

When building cluster or application image out of a Helm chart, `tele` will
render the Helm templates before extracting the image references. You can
use `--set` and `--values` flags to provide custom Helm values to `tele build`.

A special `ImageSet` custom resource allows to list additional images to vendor,
which `tele` would otherwise not be able to extract (for example, from custom
resource types).

!!! note 
    The `ImageSet` resource support will be available starting from Gravity 7.0.

```yaml
# Note that the ImageSet resource resides in the "cluster.gravitational.io" group.
apiVersion: cluster.gravitational.io/v1beta1
kind: ImageSet
metadata:
  name: extra-images
spec:
  images:
  - image: nginx:1.11.0
  - image: quay.io/bitnami/redis:5.0
```

## Image Manifest

The Image Manifest is a YAML file that is passed as an input to `tele build`
command.

The manifest concept was partially inspired by Dockerfiles and one can think of
Image Manifest as a "dockerfile" for an entire Kubernetes Cluster, and the
resulting Cluster Image can be seen as a "container" for an entire Cluster.

Below is an incomplete list of configurable settings. We have also included a sample Image Manifest further below with all the configurable settings.

* **Base Image** - A base image usually contains pre-packaged
  Kubernetes binaries and their optimal configuration. Currently, only the base
  images provided by Gravitational are supported. Use the _base image_ setting
  to select a Kubernetes version for your Cluster Image. Run `tele ls --all` to
  see the list of available base images.
* **Metadata** - The name, version and the author of the Cluster Image.
* **Network configuration** - The type of Cluster networking
  to use for Cluster instances created from a resulting Cluster Image.
* **System requirements** - Define and enforce the minimal hardware or
  infrastructure requirements such as RAM, CPU cores, network, etc.
* **Installer behavior** - Customize the process of installing a
  Cluster Image, i.e. creating a new Kubernetes Cluster instance. for example
  you can allow end users to select one of the "Cluster flavors" based on
  custom criteria, ask to accept EULA, etc.
* **System options** - Customize the runtime behavior of Kubernetes
  Clusters, for example you can set command line arguments for system daemons
  like `etcd` or `kubelet`, force a certain Docker configuration and so on.

#### Manifest Design Goals

Gravity was designed with the goal of being compatible with existing, standard
Kubernetes applications and to reuse as much of functionality provided by Kubernetes
and other widely available tools. Therefore the Image Manifest's purpose is to
be _the only Gravity-specific artifact_ you will have to create and maintain.

The file format was designed to mimic a Kubernetes resource as much as possible
and several Kubernetes concepts are used for efficiency, for example:

1. Use standard Kubernetes [ConfigMaps](http://kubernetes.io/docs/user-guide/configmap/)
   to manage application configuration. The Image Manifest should not be used
   for this purpose.

2. To customize the installation process, create regular [Kubernetes Services](http://kubernetes.io/docs/user-guide/services/)
   and tell Gravity to invoke them with the Image Manifest.

3. You can define custom Cluster life cycle hooks like _install_, _uninstall_ or _update_
   using the Image Manifest, but the hooks themselves should be implemented as a regular
   [Kubernetes Job](http://kubernetes.io/docs/user-guide/jobs/).

The Image Manifest is designed to be as small as possible in an
effort to promote open standards. As Kubernetes itself matures and promising
proposals like the "Cluster API" or "application API" become standards, certain
manifest capabilities will be deprecated.

#### Image Manifest Format

Several Image Manifest fields, in addition to allowing literal strings
for values, can also have their values populate from URIs during the build
process. For this to work, they must begin with a URI schema, i.e. `file://`
or `https://`. The following fields can fetch their values from URIs: `.releaseNotes`,
`.logo`, `.installer.eula.source`, `.installer.flavors.description`,
`.providers.aws.terraform.script`, `.providers.aws.terraform.instanceScript`,
`.hooks.*.job`.

Below is the full Image Manifest format. The only mandatory data is the `metadata:` section. The other fields can be omitted and `tele build` will try to use
sensible defaults.


```yaml
#
# The header of the application manifest uses the same signature as a Kubernetes
# resource.
#
apiVersion: cluster.gravitational.io/v2
kind: Cluster
metadata:
  # The Cluster name as shown to the end user, must be a single alphanumeric word
  name: MyCluster

  # Cluster version, must be in SemVer format (http://semver.org/)
  resourceVersion: 1.2.3-alpha.1

  # Free-form verbose description of the Cluster Image
  description: |
    Description of the Cluster Image

  author: Alice <alice@example.com>

# The base image to use. To see the list of available base images, execute:
# $ tele ls --all
# If not specified, the latest version of Kubernetes will be used.
baseImage: "gravity:6.0.0"

# Release notes is a freestyle HTML field which will be shown as part of the
# install/upgrade of the Cluster.
#
# In this case "tele build" will look for "notes.html" file in the same directory as
# manifest. To specify an absolute path: "file:///home/user/notes.html"
releaseNotes: file://notes.html

# You can add your logo in order to white-label the installer. You can either reference the URL
# of a hosted image or you can inline-encode it using url(data:image/png;base64,xxxx..) format.
logo: http://example.com/logo.png

# Endpoints are used to define exposed Kubernetes services. This can be your application
# URL or an API endpoint.
#
# Endpoints are shown to the Cluster user at the end of the installation.
endpoints:
  - name: "Control Panel"
    description: "The admin interface of the application"
    # Kubernetes selector to locate the matching service.
    selector:
      app: nginx
    protocol: https

  # This endpoint will be used as a custom post-install step, see below
  - name: "Setup"
    # Name of Kubernetes service that serves the web page with this install step
    serviceName: setup-helper
    # This endpoint will be hidden from the list of endpoints generally shown to a user
    hidden: true

# Providers allow you to override certain aspects of cloud and generic providers configuration
providers:
  # Name of the cloud provider integration to use by default, can be one of
  # "aws", "gce" or "generic" (no cloud integration).
  #
  # If the default cloud provider is not set, it will be auto-detected at
  # install time unless specified explicitly via `--cloud-provider` flag.
  #
  # Not set by default.
  default: ""

  generic:
    # Network section allows to specify networking type;
    # vxlan - (Default) use flannel for overlay network
    # wireguard - use wireguard for overlay network
    network:
      type: vxlan

#
# Use the section below to customize the Cluster installer behavior
#
installer:
  # The end user license agreement (EULA). When set, a user will be presented with
  # the EULA text before the start of the installation and forced to accept it.
  # This capability is often used when Cluster images are used to distribute downloadable
  # enterprise software.
  eula:
    source: file://eula.txt

  # If the installation flavors are defined, a Cluster user will be presented with a
  # prompt and the Cluster flavor will be selecte based on their answer.
  #
  # A "Cluster flavor" consists of a name and a set of server profiles, along with the
  # number of servers for every profile.
  #
  # The example below declares two flavors: "small" and "large", based on how many page
  # views per second the Cluster user wants to serve.
  #
  flavors:
    prompt: "How many requests per second will you need?"

    # This text will appear on the right-hand side during "capacity" step
    description: file://flavors-help.txt

    # The default flavor
    default: small

    # List of flavors:
    items:
      - name: "small"
        # UI label which installer will use to label this selection
        description: "Up to 250 requests/sec"

        # This section describes the minimum required quantity of each server type (profile)
        # for this flavor:
        nodes:
          - profile: worker
            count: 3
          - profile: db
            count: 2

      - name: "large"
        description: "More than 250 requests/sec"
        nodes:
          - profile: worker
            count: 5
          - profile: db
            count: 3

  # If additional installation UI screens are needed, they can be packaged as Kubernetes
  # services and you can list their Kubernetes endpoint names below:
  setupEndpoints:
    - "Setup"

# The node profiles section describes the system requirements of the application. The
# requirements are expressed as 'server profiles'.
#
# Gravity will ensure that the provisioned machines match the system requirements
# for each profile.
#
# This example uses two profiles: 'db' and 'node'. For example it might make sense to
# restrict 'db' profile to have at least 8 CPU and 32GB of RAM.
nodeProfiles:
  - name: db
    description: "Cassandra Node"

    # These labels will be applied to all nodes of this type in Kubernetes
    labels:
      role: "db"

    # Requirements allow you to specify the requirements servers of this profile should
    # satisfy, all of these are optional
    requirements:
      cpu:
        min: 8

      ram:
        # Other supported units are "B" (bytes), "kB" (kilobytes) and "MB" (megabytes)
        min: "32GB"

      # Supported operating systems, name should match "ID" from /etc/os-release
      os:
        - name: centos
          versions: ["7"]

        - name: rhel
          versions: ["7.2", "7.3"]

      # This section allows to run custom pre-flight checks on a node before
      # allowing Cluster installation to continue (scripts must return 0 for success)
      # Stdout/stderr output from pre-flight check scripts will be mirrored in
      # the installation log.
      customChecks:
        - description: Custom check
          script: |
              #!/bin/bash
              # inline script goes here

        - description: Custom checks defined in external file
          script: file://checks.sh

      volumes:
        # This setting tells the installer to ensure that /var/lib/logs directory
        # exists and offers at least 512GB of space:
        - path: /var/lib/logs
          capacity: "512GB"

        # A volume defined like this allows to address variations of different
        # filesystem layouts in Linux distributions (note skipIfMissing attribute)
        - path: /path/to/centos/file
          name: centos-specific-library
          targetPath: /path/to/container
          skipIfMissing: true

        # This example shows how to mount volumes into containers using a shell
        # file pattern:
        - name: wildcard-volume
          path: /path/to/dir-???
          targetPath: /path/inside/container/dir-???

        # This setting tells the installer to request an external mount for /var/lib/data
        # and mount it as /var/lib/data into containers as well
        - name: app-data
          path: /var/lib/data
          targetPath: /var/lib/data
          capacity: "512GB"
          filesystems: ["ext4", "xfs"]
          minTransferRate: "50MB/s"
          # Create the directory on host if it doesn't exist (default is 'true')
          createIfMissing: true
          # UID and GID set linux UID and GID on the directory if specified
          uid: 114
          gid: 114
          # Unix file permissions mode to set on the directory
          mode: "0755"
          # Recursive defines a recursive mount, i.e. all submounts under specified path
          # are also mounted at the corresponding location in the targetPath subtree
          recursive: false

      # This setting makes sure specified devices from host are made available
      # inside Gravity container
      devices:
          # Device(-s) path, treated as a glob
        - path: /dev/nvidia*
          # Device permissions as a composition of 'r' (read), 'w' (write) and
          # 'm' (mknod), default is 'rw'
          permissions: rw
          # Device file mode in octal form, default is '0666'
          fileMode: "0666"
          # Device user ID, default is '0'
          uid: 0
          # Device group ID, default is '0'
          gid: 0

      network:
        minTransferRate: "50MB/s"
        # Request these ports to be available
        ports:
          - protocol: tcp
            ranges:
              - "8080"
              - "10000-10005"

  - name: worker
    description: "General Purpose Worker Node"
    labels:
      role: "worker"
    requirements:
      cpu:
        min: 4
      ram:
        min: "4GB"

# If license is enabled, a user will be asked to enter a correct license to be able
# to create a Cluster from this image
license:
  enabled: true

# This section allows to tweak persistent storage in a Cluster
storage:
  # Parameters specific to OpenEBS
  openebs:
    # Set this to true to install OpenEBS in a Cluster - it is disabled by default
    # Note that setting this to true will also implicitly enable privileged containers
    enabled: false

#
# This section allows to configure the runtime behavior of a Kubernetes Cluster
#
systemOptions:
  docker:
    # Storage backend used, supported: "overlay", "overlay2" (default)
    storageDriver: overlay
    # List of additional command line args to provide to docker daemon
    args: ["--log-level=DEBUG"]

  # Etcd section allows to customize etcd
  etcd:
    # List of additional command line args to provide to etcd daemon
    args: ["-debug"]

  # Kubelet section allows to customize kubelet
  kubelet:
    # List of additional command line args to provide to kubelet daemon
    args: ["--system-reserved=memory=500Mi"]
    hairpinMode: "promiscuous-bridge"

  # When set to true, allows running privileged containers in the deployed
  # clusters, defaults to false
  allowPrivileged: false

#
# This section allows to disable pre-packaged system extensions that
# Gravitational includes into the base images by default (see below for more information).
#
extensions:
  # This setting will not install system logging service and hide "logs tab"
  # in the Cluster UI
  logs:
    disabled: false

  # This setting will not install system monitoring application and hide
  # Monitoring tab in the Cluster UI
  monitoring:
    disabled: false

  # This setting will hide Kubernetes tab in the Cluster UI
  kubernetes:
    disabled: false

  # This setting will not install the Tiller application
  catalog:
    disabled: false

# This section specifies the Cluster lifecycle hooks, i.e. the ability to execute
# custom code in response to lifecycle events.
#
# Every hook is a name of a Kubernetes job.
#
hooks:
  # install hook is called right after the application is installed for the first time.
  install:
    # Job directive defines a Kubernetes job which can be declared inline here in the manifest
    # It will be created and executed:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: db-seed
        namespace: default
      spec:
        template:
          spec:
            restartPolicy: OnFailure
            containers:
              - name: dbseed
                image: installer-hooks:latest

  # called after the application has been installed
  postInstall:
    # A Kubernetes job can also be specified via a separate YAML file
    # `post-install-hook.yaml` file located in the same directory as
    # this application bundle manifest
    job: file://post-install-hook.yaml

  # called when uninstalling the application
  uninstall:

  # called before uninstall is launched
  preUninstall:

  # called before adding a new node to the Cluster
  preNodeAdd:

  # called after a new node has need added to the Cluster
  postNodeAdd:

  # called before a node is removed from the Cluster
  preNodeRemove:

  # called after a node has been removed from the Cluster
  postNodeRemove:

  # called when updating the application
  update:

  # called after successful application update
  postUpdate:

  # called when rolling back after an unsuccessful update
  rollback:

  # called after successful rollback
  postRollback:

  # called every minute to check the application status (visible in Control Panel)
  status:

  # called after the application license has been updated
  licenseUpdated:

  # used to start the application
  start:

  # used to stop the application
  stop:

  # used to retrieve application specific dump for debug reports
  dump:

  # triggers application data backup
  backup:

  # restores application state from backup
  restore:

  # install a custom CNI network plugin during Cluster installation
  networkInstall:

  # update a custom CNI network plugin during Cluster upgrade
  networkUpdate:

  # rollback a custom CNI network plugin during Cluster rollback
  networkRollback:
```

See [here](requirements.md#identifying-os-distributions-in-manifest) for a version
matrix to help with specifying OS distribution requirements for a node profile.

## Cluster Hooks

"Cluster Hooks" are Kubernetes jobs that run at different points in the Cluster
life cycle or in response to certain events happening in the Cluster.

Each hook job has access to the "Cluster Resources" which are mounted under
`/var/lib/gravity/resources` directory in each of the job's containers. The
Cluster resources include the Cluster Image manifest and everything else that
was in the same directory with the Image Manifest at the moment of `tele build`
execution. For example, if during the build process the directory with the
Cluster resources looked like:

```text
myapp/
  ├── app.yaml
  ├── install-hook.yaml
  ├── logo.svg
  └── resources.yaml
```

... then all these files will be made available to the Cluster hooks mounted inside
a hook job container as:

```text
/var/lib/gravity/resources/
  ├── app.yaml
  ├── install-hook.yaml
  ├── logo.svg
  └── resources.yaml
```

Every hook container gets `kubectl` and `helm` binaries mounted under `/usr/local/bin/`
which it can use to create Kubernetes resources in the Cluster.

Below is an example of a simple `install` hook that creates Kubernetes resources
from "resources.yaml":

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: install-hook
spec:
  template:
    metadata:
      name: install-hook
    spec:
      restartPolicy: OnFailure
      containers:
        - name: debian-tall
          image: quay.io/gravitational/debian-tall:buster
          command:
            - /usr/local/bin/kubectl
            - create
            - -f
            - /var/lib/gravity/resources/resources.yaml
```

which can then be included in the Image Manifest:

```yaml
hooks:
  install:
    job: file://install-hook.yaml
```

To see more examples of specific hooks, please refer to the following documentation sections:

* [Cluster Status](cluster.md#cluster-status) for `status` hook
* [Backup & Restore](cluster.md#backup-and-restore) for `backup` and `restore` hooks

!!! tip
    The `quay.io/gravitational/debian-tall:buster` image is a lightweight (~11MB)
    distribution of Debian Linux that is a good fit for running Go or statically
    linked binaries.

## Helm Integration

Gravity has a first-class [Helm](https://docs.helm.sh/) support and lets you use Helm
charts as a way to package and install applications.

!!! note "Helm version"
    Gravity 6 works with Helm 2. We are currently working on Helm 3 integration.

Suppose you have the application resources directory with the following layout:

```
example/
   ├── app.yaml     # Gravity application manifest
   └── charts/      # Directory with all Helm charts
       └── example/ # An application chart
           ├── Chart.yaml
           ├── templates/
           │   └── example.yaml
           └── values.yaml
```

When building the Cluster Image, the `tele build` command will find
directories with Helm charts (determined by the presence of `Chart.yaml` file)
and vendor all Docker images they reference into the resulting image tarball.

The `tele build` command also allows to override Helm chart values at build time
via `--values` and `--set` flags. These values will be taken into account when
rendering Helm templates which is useful if you need to vendor a specific version
of a certain Docker image or pull it from a specific registry.

The flags have the same meaning and syntax as the Helm flags of the same names:
`--values` specifies a YAML file with custom values and `--set` sets values directly
on the command-line. Both can be provided multiple times:

```bash
$ tele build example/app.yaml --values=custom-values.yaml --set=nginx.image=1.9.1 --set=postgres.registry=internal.registry.io
```

During the installation, the vendored images will be pushed to the Cluster's local
Docker registry which is available inside the Cluster at `registry.local:5000`.
Helm templating engine can be used to tag images with an appropriate registry.

For example, `example.yaml` may contain the following image reference:

```yaml
image: {% raw %}{{.Values.registry}}postgres:9.4.4{% endraw %}
```

And `values.yaml` may define the `registry` templating variable that can be set
during application installation:

```
registry: ""
```

An install hook can then use the `helm` binary (which gets mounted into every hook
container under `/usr/local/bin`) to install these resources:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: install
spec:
  template:
    metadata:
      name: install
    spec:
      restartPolicy: OnFailure
      containers:
        - name: install
          image: quay.io/gravitational/debian-tall:buster
          command: ["/usr/local/bin/helm", "install", "/var/lib/gravity/resources/charts/example", "--set", "registry=leader.telekube.local:5000/"]
```

The hook command sets the registry variable to point to the Cluster's
local Docker registry so that when Helm renders resource templates, they contain
correct image references.

!!! tip
    There is a sample application available on [GitHub](https://github.com/gravitational/quickstart/tree/master/mattermost)
    that demonstrates this workflow.

### Customizing Helm values

!!! note "Version support"
    The ability to customize Helm values during install is available starting with
    Gravity 7.0.

It is possible to customize values of your Helm charts when installing or
upgrading the application. To provide custom Helm values at install time,
pass them via `--values` and `--set` flags to `gravity install` command:

```bash
unpacked-image$ ./gravity install --values=custom-values.yaml --set=nginx.image=1.9.1 --set=postgres.registry=internal.registry.io
```

The provided values are merged into a single values file that is mounted
into install and post-install hooks under `/var/lib/gravity/helm/values.yaml`
so the install hook can use it in the `helm install` command:

```yaml
...
command: ["/usr/local/bin/helm", "install", "/var/lib/gravity/resources/charts/example", "--values", "/var/lib/gravity/helm/values.yaml"]
```

The same is true for upgrades: both the `./upgrade` script included with the cluster
image and `gravity upgrade` commands support providing custom Helm values which
will get mounted at the same location in the upgrade/post-upgrade hooks:

```bash
unpacked-image$ ./upgrade --values=custom-values.yaml --set=nginx.image=1.11.0
```

## Custom Installation Screen

The Gravity graphical installer supports plugging in custom screens after the
main installation phase (such as installing Kubernetes and system dependencies)
has successfully completed.

A "Custom Installation Screen" is just a web application running inside the
deployed Kubernetes Cluster and reachable through a Kubernetes service. Enabling a
Custom Installation Screen allows the user to perform actions specific to an
Gravity Cluster upon successful install (for example, configuring an
application or launch a database migration).

The standard Cluster Images come with a sample Custom Installation Screen
called "bandwagon".  It is a Kubernetes web application, i.e. it exposes a
Kubernetes endpoint. The installer can be configured to transfer the user to
that endpoint after the installation. Bandwagon presents users with a form
where they can enter login and password to provision a local Gravity Cluster
user and choose whether to enable or disable remote support.

Bandwagon is [open sourced on GitHub](https://github.com/gravitational/bandwagon) and can be used as an example of how to implement your own custom installer screen.

To enable Bandwagon, add this to your Image Manifest:

```yaml
# define an endpoint for bandwagon service
endpoints:
  - name: "Bandwagon"
    hidden: true # hide this endpoint from the Cluster control page
    serviceName: bandwagon # Kubernetes service name specified in bandwagon app resources

# refer to the endpoint defined above
installer:
  setupEndpoints:
    - "Bandwagon"
```

!!! note 
	Currently, only one setup endpoint per application is supported.

## System Extensions

By default, a Cluster Image contains several system services to provide
Cluster logging, monitoring and application catalog (via Tiller) functionality.
You may want to disable any of these components if you prefer to replace them with a solution of your choice. To do that, define the following section in the
Image Manifest:

```yaml
extensions:
  # This setting will not install system logging service and hide "logs tab"
  # in the Cluster Control Panel UI
  logs:
    disabled: true

  # This setting will not install system monitoring application and hide
  # Monitoring tab in the Cluster Control Panel UI
  monitoring:
    disabled: true

  # This setting will hide Kubernetes tab in the Cluster UI
  kubernetes:
    disabled: true

  # This setting will not install the Tiller application
  catalog:
    disabled: true
```

!!! note 
    Disabling the system logging component will result in inability
    to view operation logs via the Cluster UI.

## Service User

When Gravity creates a Cluster from a Cluster Image, it installs a
special system container on each host, visible as the `gravity` daemon. It contains all of Kubernetes services, performs automatic management and isolates them from other pre-existing daemons running on Cluster hosts.

All system services inside the container run under a special system user
called `planet` with a UID of `1000`.

Starting with LTS 4.54, Gravity allows the system user to be configured during
installation. The same service user with the same UID will be created on all
nodes of a Cluster.

In order to configure the `planet` service user, you have the following options:

  * Create a user with the same UID on all Cluster nodes upfront.
  * Specify a user ID on the installer's command line and a user named `planet`
    (and a group with the same name) will automatically be created with the given
    ID during installation.

Here's an example of using a custom system user/group before starting the installation:

```bash
# before installing a new Cluster:
# create a group named mygroup (repeat on all Cluster nodes)
$ groupadd mygroup -g 1001

# create a user named myuser in group mygroup (repeate on all Cluster nodes)
$ useradd --no-create-home -u 1001 -g mygroup myuser

# specify the service user during installation
$ ./gravity install <options> --service-uid=1001
```

The service user can also be used for running unprivileged services inside the
Kubernetes Cluster.  To run a specific Pod (or just a container) under the
service user, use `-1` as a user ID which will be translated to the
corresponding service user ID:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  labels:
    name: nginx
spec:
  securityContext:
    runAsUser: -1   # to use for all containers in a Pod
  containers:
  - name: nginx
    image: nginx
    ports:
    - containerPort: 80
    securityContext:
      runAsUser: -1   # to use for a single container
```

Only resources stored as YAML files are subject to automatic translation.  If
a Cluster life cycle hook uses custom resource provisioning, it might need to perform
the conversion manually.

The value of the effective service user ID is stored in the
`GRAVITY_SERVICE_USER` environment variable which is available to each
hook.

## Custom System Container

When Gravity creates a Kubernetes Cluster from a Cluster Image, it installs a
special system container or "Master Container" on each host. It is called "planet"
and visible as the `gravity` daemon.

The Master Container contains all of Kubernetes services, performs automatic management and
isolates them from other pre-existing daemons running on Cluster hosts.

`planet` is a Docker image maintained by Gravitational. At this moment `planet`
image is based on Debian 9. `planet` base image is published to a public Docker registry at
`quay.io/gravitational/planet` so you can customize `planet` environment for
your Clusters by using Gravitational's image as a base. Here's an example of a
Dockerfile of a custom `planet` image that installs an additional package:

```
FROM quay.io/gravitational/planet:5.1.1-1906
RUN chmod 777 /tmp && \
    mkdir -p /var/cache/apt/archives/partial && \
    apt-get update && \
    apt-get install -y emacs
```

Now let's build the Docker image:

```bsh
$ docker build . -t custom-planet:1.0.0
```

!!! tip "Versioning"
    The image version must be a valid [semver](https://semver.org/).

Once the custom `planet` image has been built, it can be referenced in the
application manifest as a user-defined base image for a specific node profile:

```yaml
nodeProfiles:
  - name: worker
    description: "Worker Node"
    systemOptions:
      baseImage: custom-planet:1.0.0
```

When building a Cluster Image, `tele build` will discover `custom-planet:1.0.0`
Docker image and vendor it along with other dependencies. During the Cluster
installation all nodes with the role `worker` will use the custom "planet" Docker
image instead of the default one.
