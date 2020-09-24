# Packaging And Deployment

This section covers how to prepare an application for distribution with Telekube.

Telekube works with Kubernetes applications. This means the following prerequisites exist in order to use Telekube:

* The application is packaged into Docker containers.
* You have Kubernetes resource definitions for application services, pods, etc. Kubernetes resources should be stored in the resources directory.

!!! tip
		For easy development while porting applications to Kubernetes, we recommend
		[minikube](https://github.com/kubernetes/minikube), a Kubernetes distribution
		optimized to run on a developer's machine. Once your application runs on
		Kubernetes, it's trivial to package it for distribution using Telekube tools.

## Getting Started

Any Linux or macOS laptop can be used to package and publish Kubernetes
applications using Telekube. To get started, you need to download and
install the Telekube SDK tools:

```bash
$ curl https://get.gravitational.io/telekube/install | bash
```

You will be using `tele`, the Telekube CLI client. By using `tele` on your laptop you can:

* Package Kubernetes applications into self-installing tarballs, ("Application Bundles" or "Applications")
* Publish Application Bundles into the Ops center.
* Deploy Application Bundles to server clusters ("Telekube Clusters" or "Clusters")
* Manage Telekube Clusters in the Ops Center.

Here's the full list of `tele` commands:

| Command  | Description |
|----------|-------------|
| login    | Log in to an Ops Center and makes that Ops Center active for other commands like `tsh`.
| status   | Shows the status of Telekube SDK and the current Ops Center you're connected to.
| build    | Packages a Kubernetes application into a self-deployable tarball ("Application Bundle").
| push     | Pushes a Kubernetes application into the Ops Center for publishing.
| pull     | Downloads an application from the Ops Center.
| rm       | Removes an application in the Ops Center.
| ls       | Lists published aplications in the Ops Center.

## Ops Center Login

Telekube CLI tools require that a user be first logged into an Ops Center account.

`tele login` is used to log into an Ops center. You can optionally specify a `cluster` parameter to
log into a specific remote application instance.

```bash
tele login [options] [cluster]

Options:
  -o       Ops center to connect to
  --auth   Authentication method
  --key    API key

Arguments:
  cluster  The name of the remote cluster to connect to.
```

If the Ops Center is configured to use password-based authentication, it will require
a password and (optionally) for a 2nd factor token in the
command line.

Example command:

```bash
$ tele login -o opscenter.example.com
```

Example Response:

```bash
If browser window does not open automatically, open it by clicking on the link:
 https://accounts.google.com/o/oauth2/v2/auth?client_id=281182034774-5opnsdim9rsdfemaljphdqg5a7tpc1lqb.apps.googleusercontent.com&prompt=select_account&redirect_uri=https%3A%2F%2Fdemo.gravitational.io%2Fportalapi%2Fv1%2Foidc%2Fcallback&response_type=code&scope=openid+email&state=fdaa63a0dd83755e267e6e3a422be22e
Ops Center:	opscenter.example.com
Username:	meta@example.com
Cluster:	remote.cluster.1234
Expires:	Fri Feb 17 15:46 UTC (19 hours from now)
```

!!! note 
    The `tele login` command needs to
    be executed from a machine with a browser by default.

Further information about the Ops Center will then be displayed by executing
`tele status`.

Example Response:

```bash
Ops Center:	demo.gravitational.io
Username:	user@gravitational.com
Cluster:	demo.gravitational.io
Expires:	Wed Oct 11 16:57 UTC (19 hours from now)
```

## Packaging Applications

Telekube can package any Kubernetes application (along with any dependencies) into a self-deploying,
tarball ("Application Bundle").

An Application Manifest is required to create an Application Bundle. An Application Manifest is
a YAML file which describes the build and installation process and requirements. The
[Application Manifest](#application-manifest) section has further details about it.

`tele build` command will read an Application Manifest and will make sure that
all of the dependencies are available locally on the build machine. If the
dependencies are not available locally, it will download them from
the Ops Center.

```bash
tele build [options] [app-manifest.yaml]

Options:
  -o   The name of the produced tarball, for example "-o myapp-v3.tar".
       By default the name of the current directory will be used to name the tarball.
```


### Building with Docker

`tele build` can be used inside a Docker container. Using Linux containers is a good strategy to introduce reproducible builds
that do not depend on the host OS. Containerized builds are also easier to automate by plugging them into a CI/CD pipeline.

The example below builds a Docker image called `tele-buildbox`. This image will contain `tele` tool and can be used to create Telekube packages.

#### Build Docker image with Tele

First, build docker image `tele-buildbox` with `tele` inside:

```Docker
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

```bash
docker build . -t tele-buildbox:latest
```

#### Build script

The example script below uses `tele` to login into ops center (optional step),
build a local application and publish it (optional step):

```bash
# optional step: if you are using private ops center
tele login -o ${OPS_URL} --token=${OPS_TOKEN}
# start tele build
tele ${TELE_FLAGS} build app.yaml
# optional step: push the app to the ops center
tele push ${OPS_URL}
```

#### Start build

To run this build under Docker:

* Expose a OPS_TOKEN and TELE_FLAGS environment variables
* Use host networking
* Expose the Docker socket into the container to allow tele to pull container images referenced in the manifest

The script below assumes that `build.sh` is located in the same working directory as the application:

```bash
docker run -e OPS_URL=<opscenter url> \
       -e OPS_TOKEN=<token> \
       -e TELE_FLAGS="--state-dir=/mnt/tele-cache" \
       -v /tmp/tele-cache:/mnt/tele-cache \
       -v /var/run/docker.sock:/var/run/docker.sock \
       -v $(pwd):/mnt/app \
        --net=host \
        tele-buildbox:latest \
        bash -c "cd /mnt/app && build.sh"
```

!!! note
    Notice that we are reusing tele loaded cache directory in between builds
    by setting `--state-dir`. You can use unique temporary directory
    to avoid sharing state between builds, or use parallel builds instead.


## Publishing Applications

After packaging an application into an Application Bundle, it can be deployed and
installed by publishing it into the Ops Center. The commands below are used to manage the
publishing process.

!!! note 
		The commands below will only work if a user is first
		logged into an Ops Center by using `tele login`.


`tele push` is used to upload a Kubernetes Application Bundle to the Ops Center.

```html
tele push [options] tarball.tar

Options:
  --force, -f  Forces to overwrite the already-published application if it exists.
```

`tele pull` will download the Application Bundle from the Ops Center:

```html
tele [options] pull [application]

Options:
  -o   Name of the output tarball.
```

`tele rm app` deletes an Application Bundle from the Ops Center.

```html
tele rm app [options] [application]

Options:
  --force  Do not return error if the application cannot be found or removed.
```

`tele ls` lists the Application Bundles currently published in the Ops Center.

```html
tele [options] ls
```

## Application Manifest

The Application Manifest is a YAML file that is used to describe the packaging and
installation process and requirements for an Application Bundle.

### Manifest Design Goals

Telekube was designed with the goal of being compatible with existing, standard
Kubernetes applications. The Application Manifest is _the only Telekube-specific artifact_ you
will have to create and maintain.

The file format was designed to mimic a Kubernete
resource as much as possible and several Kubernetes concepts are used
for efficiency:

1. Kubernetes [Config Maps](http://kubernetes.io/docs/user-guide/configmap/)
   are used to manage the application configuration.

2. The custom Installation Wizard steps are implemented as regular
   [Kubernetes Services](http://kubernetes.io/docs/user-guide/services/).

3. Application lifecycle hooks like _install_, _uninstall_ or _update_ are implemented as
   [Kubernetes Jobs](http://kubernetes.io/docs/user-guide/jobs/).

Additionally, the Application Manifest is designed to be as small as possible in an effort to promote
open standards as the project matures.

The Application Manifest shown here covers the basic capabilities of Telekube.
It can be extended with additional Kubernetes plug-ins. Examples of pluggable features
include PostgreSQL, streaming replication, cluster-wide state snapshots or in-cluster encryption.

!!! note
    The following manifest fields, in addition to having literal string values,
    can read their values from files (via file://) or the Internet (via http(s)://):
    `.releaseNotes`, `.logo`, `.installer.eula.source`, `.installer.flavors.description`,
    `.hooks.*.job`. These values are vendored into the Application Manifest during "tele build".

### Sample Application Manifest

```bash
#
# The header of the application manifest uses the same signature as a Kubernetes
# resource.
#
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  # Application name as shown to the end user, must be a single alphanumeric word
  name: ApplicationName

  # Application version, must be in SemVer format (http://semver.org/)
  resourceVersion: 0.0.1-alpha.1

  # Free-form verbose description of the application
  description: |
    Description of the application

  # Free-form author of the application
  author: Alice <alice@example.com>

# Release notes is a freestyle HTML field which will be shown as part of the install/upgrade
# of the application.
#
# In this case "tele build" will look for "notes.html" file in the same directory as
# manifest. To specify an absolute path: "file:///home/user/notes.html"
releaseNotes: file://notes.html

# You can add your logo in order to white-label the installer. You can either reference the URL
# of a hosted image or you can inline-encode it using url(data:image/png;base64,xxxx..) format.
logo: http://example.com/logo.png

# Endpoints are used to define exposed Kubernetes services. This can be your application
# URL, or (if your application is a database) it's API endpoint.
#
# Endpoints are shown to the end user at the end of the installation.
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
  aws:
    # Supported AWS regions, defaults to all regions
    regions:
      - us-east-1
      - us-west-2

  # Generic provider is used for on-premise installations
  generic:
    # Network section allows to specify networking type; default is "vxlan", also supports "calico"
    network:
      type: vxlan

#
# Installer section is used to customzie the installer behavior
#
installer:
  # Optional end user license agreement; if specified, a user will be presented with EULA
  # text before the start of the installation and prompted to agree with it
  eula:
    source: file://eula.txt

  # Installation flavors define the initial cluster sizes
  #
  # Each flavor has a name and a set of server profiles, along with the number of servers for
  # every profile.
  #
  # This manifest declares two flavors: "small" and "large", based on how many page views the
  # end user desires to serve.
  flavors:
    # This question will be shown during "capacity" installation step
    prompt: "How many requests per second will you need?"

    # This text will appear on the right-hand side during "capacity" step
    description: file://flavors-help.txt

    # The default flavor will be pre-selected on the "capacity" step
    default: small

    items:
      # "small" flavor: 250 requests/second with 2 DB nodes and 3 regular nodes
      - name: "small"
        # UI label which installer will use to label this selection
        description: "0-250 requests/sec"
        # This section describes the minimum required quantity of each server type (profile)
        # for this flavor:
        nodes:
          - profile: worker
            count: 3
          - profile: db
            count: 2

      # "large" flavor: 250+ requests/second with 3 DB nodes and 5 regular nodes
      - name: "large"
        description: "250+ requests/sec"
        nodes:
          - profile: worker
            count: 5
          - profile: db
            count: 3

  # This directive allows the application vendor to supply custom installer steps (screens)
  # An installer screen is a regular web page backed by a Kubernetes service.
  #
  # In this case, after the installation, the installer will redirect user to the "Setup"
  # endpoint defined above.
  setupEndpoints:
    - "Setup"

# Node profiles section describes the system requirements of the application. The
# requirements are expressed as 'server profiles'.
#
# Telekube will ensure that the provisioned machines match the system requirements
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
          versions:
            - "7"

        - name: rhel
          versions:
            - "7.2"
            - "7.3"

      volumes:
        # This directive tells the installer to ensure that /var/lib/logs directory
        # exists created with 512GB of space:
        - path: /var/lib/logs
          capacity: "512GB"

        # This directive tells the installer to request an external mount for /var/lib/data
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
          # mode is a linux mode to set on the directory
          mode: "0755"

      # This directive makes sure specified devices from host are made available
      # inside Telekube container
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

    # Fixed expand policy prevents adding more nodes of this type on an installed cluster
    #
    # Another supported policy is "fixed-instance" which only allows adding more nodes
    # of this type of the same instance type (e.g. on AWS)
    expandPolicy: fixed

    # Instance types directive allows application vendors to further restrict the
    # server flavor to the specific AWS (or other cloud) instance types.
    providers:
      aws:
        instanceTypes:
          - c3.2xlarge
          - m3.2xlarge

  - name: worker
    description: "General Purpose Worker Node"
    labels:
      role: "worker"
    requirements:
      cpu:
        min: 4
      ram:
        min: "4GB"

# If license mode is enabled, a user will be asked to enter a correct license to be able
# to install an application
license:
  enabled: true

systemOptions:
  # Runtime allows you to override the version of the Kubernetes runtime that is used
  # (defaults to the latest available)
  runtime:
    version: "1.5.0"

  # Docker section allows to customize docker
  docker:
    # Storage backend used, supported: "devicemapper" (default), "overlay", "overlay2"
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

# This section specifies application lifecycle hooks, i.e. the events that application
# may want to react to.
# Every hook is just a name of a Kubernetes job.
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

  # called to provision the cluster via Ops Center using custom job
  clusterProvision:

  # called to deprovision the cluster via Ops Center with custom job
  clusterDeprovision:

  # called when uninstalling the application
  uninstall:

  # called before uninstall is launched
  preUninstall:

  # called before adding a new node to the cluster
  preNodeAdd:

  # called to provision one or several nodes
  nodesProvision:

  # called after a new node has need added to the cluster
  postNodeAdd:

  # called before a node is removed from the cluster
  preNodeRemove:

  # called to deprovision one or several nodes
  nodesDeprovision:

  # called after a node has been removed from the cluster
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
```

See [here](requirements.md#identifying-os-distributions-in-manifest) for version matrix to help with
specifying OS distribution requirements for a node profile.

## Application Hooks

"Application Hooks" are Kubernetes jobs that run at different points in the application lifecycle or in
response to certain events happening in the cluster.

Each hook job has access to the "Application Resources" which are mounted under
`/var/lib/gravity/resources` directory in each of the job's containers. The Application's
Resources include the Application Manifest and everything else that was in the same directory
with the Application Manifest when building the Application Bundle. For example, if
during the build the directory with the Application Resources looked like:

```
myapp/
  ├── app.yaml
  ├── install-hook.yaml
  ├── logo.svg
  └── resources.yaml
```

then all these files will be made available to the Application Hooks under:

```
/var/lib/gravity/resources/
  ├── app.yaml
  ├── install-hook.yaml
  ├── logo.svg
  └── resources.yaml
```

Every hook container gets `kubectl` and `helm` binaries mounted under `/usr/local/bin/`
which it can use to create Kubernetes resources in the cluster.

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
          image: quay.io/gravitational/debian-tall:0.0.1
          command:
            - /usr/local/bin/kubectl
            - create
            - -f
            - /var/lib/gravity/resources/resources.yaml
```

which can then be included in the Application Manifest:

```yaml
hooks:
  install:
    job: file://install-hook.yaml
```

To see more examples of specific hooks, please refer to the following documentation sections:

* [Application Status](cluster.md#application-status) for `status` hook
* [Backup & Restore](cluster.md#backup-and-restore) for `backup` and `restore` hooks

!!! tip
    The `quay.io/gravitational/debian-tall:0.0.1` image is a lightweight (~11MB)
    distribution of Debian Linux that is a good fit for running Go or statically
    linked binaries.

## Helm Integration

!!! note
    Support for Helm charts is available starting from version `5.0.0-alpha.10`.

It is possible to use [Helm](https://docs.helm.sh/) charts as a way to package
and install applications as every Telekube cluster comes with a preconfigured
Tiller server and its client, Helm.

Suppose you have the application resources directory with the following layout:

```
example/
   ├── app.yaml     # Telekube application manifest
   └── charts/      # Directory with all Helm charts
       └── example/ # An application chart
           ├── Chart.yaml
           ├── templates/
           │   └── example.yaml
           └── values.yaml
```

When building the application installer, the `tele build` command will find
directories with Helm charts (determined by the presence of `Chart.yaml` file)
and vendor all Docker images they reference into the resulting installer
tarball.

!!! note 
    The machine running `tele build` must have Helm binary [installed](https://docs.helm.sh/using_helm/#installing-helm)
    and available in PATH as well as its [template plugin](https://docs.helm.sh/using_helm/#installing-a-plugin).

During the installation the vendored images will be pushed to the cluster's local
Docker registry which is available inside the cluster at `leader.telekube.local:5000`.
Helm templating engine can be used to tag images with an appropriate registry.
For example, `example.yaml` may contain the following image reference:

```yaml
image: {% raw %}{{.Values.registry}}{% endraw %}postgres:9.4.4
```

And `values.yaml` may define the `registry` templating variable that can be set
during application installation:


```
registry: ""
```


An install hook can then use `helm` binary (which gets mounted into every hook
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
          image: quay.io/gravitational/debian-tall:0.0.1
          command: ["/usr/local/bin/helm", "install", "/var/lib/gravity/resources/charts/example", "--set", "registry=leader.telekube.local:5000/"]
```

Note how the hook command sets the registry variable to point to the cluster's
local Docker registry so that when Helm renders resource templates, they contain
correct image references.

!!! tip
    There is a sample application available on [GitHub](https://github.com/gravitational/quickstart/tree/master/mattermost)
    that demonstrates this workflow.

## Custom Installation Screen

The Telekube graphical installer supports plugging in custom screens after the main
installation phase (such as installing Kubernetes and system dependencies) has successfully completed.

A "Custom Installation Screen" is just a web application running inside the deployed Kuberneted cluster
and reachable via a Kubernetes service. Enabling a Custom Installation Screen allows the user to perform
actions specific to an Telekube Cluster upon successful install (for example, configuring an application or
launch a database migration).

Telekube comes with a sample Custom Installation Screen called "bandwagon". It is a web application that itself
runs on Kubernetes and exposes a Kubernetes endpoint. The installer can be configured to transfer
the user to that endpoint after the installation. Bandwagon presents users with a form where they
can enter login and password to provision a local Telekube Cluster user and choose whether to enable or disable
remote support.

Bandwagon is [open source](https://github.com/gravitational/bandwagon) on Github and can be used as an
example of how to implement your own custom installer screen.

To enable Bandwagon, add this to your Application Manifest:

```yaml
# define an endpoint for bandwagon service
endpoints:
  - name: "Bandwagon"
    hidden: true # hide this endpoint from the cluster Admin page
    serviceName: bandwagon # Kubernetes service name specified in bandwagon app resources

# refer to the endpoint defined above
installer:
  setupEndpoints:
    - "Bandwagon"
```

!!! note 
	  Currently, only one setup endpoint per application is supported.



## Service User
Telekube uses a special user for running system services inside the environment container called `planet`.
Historically, this user has had a hardcoded UID `1000` on host hence rendering user management
inflexible and cumbersome.

Starting with LTS 4.54, Telekube allows this user to be configured in offline installation.
A single service user is configured for the whole cluster. This means you cannot use different
user IDs on multiple nodes.

In order to configure the service user, you have the following options:

  * Create users with the same ID on all nodes upfront.
  * Specify a user ID on installer's command line and a user named `planet` (and a group with the same name)
    will automatically be created with the given ID during installation.

Here's an example of creating a user/group and starting the installation with service user override:

```shell
# create a group named mygroup
root$ groupadd mygroup -g 1001
# create a user named myuser in group mygroup
root$ useradd --no-create-home -u 1001 -g mygroup myuser
# override the service user for installation
root$ ./gravity install <options> --service-uid=1001
```

Then agents connecting from every other node in the cluster will use (and create if not
existing) the same user ID.

Service user can also be used for running unprivileged services inside the kubernetes cluster.
To run a specific Pod (or just a container) under the service user, use `-1` as a user ID
which will be translated to the corresponding service user ID:

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

Only resources stored as .yaml files are subject to automatic translation.
If an application hook uses custom resource provisioning, it might need to perform conversion manually.

The value of the effective service user ID is stored in the `GRAVITY_SERVICE_USER` environment variable which is made
available to each hook.


## User-Defined Base Image

!!! note 
    Ability to override default base image is currently only supported in
    the `5.1.x` line of releases starting from `5.1.0-alpha.4`.

To ensure consistency across various supported OS distributions and versions,
Telekube clusters are deployed on top of a containerized Kubernetes environment
called `planet`. The `planet` is a Docker image maintained by Gravitational. At
this moment `planet` image is based on Debian 9.

The `planet` base image is published to a public Docker registry at
`quay.io/gravitational/planet` so you can customize `planet` environment for
your bundle by using Gravitational's image as a base. Here's an example of a
Dockerfile of a custom `planet` image that installs an additional package:

```
FROM quay.io/gravitational/planet:5.1.1-1906
RUN chmod 777 /tmp && \
    mkdir -p /var/cache/apt/archives/partial && \
    apt-get update && \
    apt-get install -y emacs
```

Now let's build the Docker image:

```bash
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

When packaging the application, `tele build` will discover `custom-planet:1.0.0` image
and vendor it in along with other application dependencies. During cluster
installation all nodes with the role `worker` will use the specified base image
instead of the default one.


### Application Manifest Changes

The 5.1.x release introduces a couple of changes to the application manifest to support the planet as
a docker image use-case.

New volume definition flag `skipIfMissing` controls whether a particular directory will be mounted inside
the container. The main use-case for this is simplifying OS-specific mount configuration:

```yaml
  nodeProfiles:
    requirements:
      volumes:
        # This directory is only found on CentOS
        - path: /path/to/dir/on/centos
          targetPath: /path/to/dir/in/container
          # This attribute tells the installer to mount the directory only if it exists
          # on host. With this set, createIfMissing is ignored.
          skipIfMissing: true
          name: centos-library

        # This directory is only found on Ubuntu
        - path: /path/to/dir/on/ubuntu
          targetPath: /path/to/dir/in/container
          skipIfMissing: true
          name: ubuntu-library

        # Path can also accept a shell file pattern. In this case,
        # it will be mounted in the telekube container under the
        # same path as matched on host.
        - path: /path/to/dir-???
          targetPath: /path/to/dir-???  # targetPath is required even though
                                        # it will be automatically set to the actual match
          skipIfMissing: true
```

In the example above, when we install on `CentOS`, only the `centos-library` directory is mounted
inside the container, while on `Ubuntu` only the directory named `ubuntu-library` will be mounted.

The values specified in `path` can contain shell file name patterns.
See the description of the [Match](https://golang.org/pkg/path/filepath/#Match) API for details
of the supported syntax.
If a mount specifies a file pattern in `path`, `targetPath` will be automatically set to the
actual match as found on host.

!!! note 
    When working with mounts, it is important to always specify the `targetPath` to
    differentiate a mount from a volume requirement.
    Leaving the `targetPath` empty does not automatically set it equal to `path`
    inside the container.

Additionally, it is possible to define custom preflight checks.
A custom check is a shell script that can either be placed in manifest inline or read from a URL:

```yaml
nodeProfiles:
 - name: custom-profile
   requirements:
     cpu:
       min: 1
     ram:
       min: "8GB"
     customChecks:
      - description: custom check
        script: |
          #!/bin/bash

          # script goes here

      - description: another custom check
        script: file://checks.sh
```
During the build process, the script will be rendered in-place inside the manifest.

To report a failure from a script, exit with a code other than `0` (`0` denotes a success outcome).

Stdout/stderr output from the script will be mirrored in the installation log in case
of a failure.
