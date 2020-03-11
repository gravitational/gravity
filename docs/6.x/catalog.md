# Application Catalog

!!! note 
    The Application Catalog feature is currently under active development and
    is available starting from 5.4.0 alpha releases.

Gravity supports packaging Helm charts as self-contained application images
that can be installed in a Gravity cluster or a generic Kubernetes cluster
(provided it has an access to a Docker registry).

An application image is a tarball that contains:

* A Helm chart (possibly with chart dependencies).
* Vendored Docker images for the chart's Kubernetes resources.

!!! tip
    The `tele` and `gravity` binaries have all required Helm functionality
    built-in so the `helm` binary isn't required to be installed on the
    server when building applications, or inside a deployed cluster. The
    `tiller` server (Helm's server component) does need to be deployed to
    the cluster.

Both  `tele version` and `gravity version` commands report the embedded Helm
version:

```bsh
$ tele version
Edition:      open-source
Version:      5.4.0
Git Commit:   971d93b06da08c3b277fc033c1c4fdca55a0ec6b
Helm Version: v2.12
```

### Build an Application Image

For this example we will be using a [sample Helm chart](https://github.com/helm/helm/tree/master/cmd/helm/testdata/testcharts/alpine).

This chart spins up a single pod of Alpine Linux:

```bsh
$ tree alpine
alpine
├── Chart.yaml
├── README.md
├── templates
│   ├── alpine-pod.yaml
│   └── _helpers.tpl
└── values.yaml
```

Before building an application image, we need to make sure that the pod's
image reference includes a registry template variable which can be set to
an appropriate registry during installation:

```bsh
image: "{% raw %}{{ .Values.image.registry }}{{ .Values.image.repository }}:{{ .Values.image.tag }}{% endraw %}"
```

We can now use `tele` to build an application image from this chart:

```bsh
$ tele build alpine
```

The result is a tarball `alpine-0.1.0.tar` which includes a packaged Helm
chart and the Alpine image layers.

### Publish an Application Image

!!! note 
    Publishing applications requires a [Gravity Hub](hub.md) and is
    available only in the Enterprise edition of Gravity.

A built application image can be published to an Gravity Hub. This allows the
Gravity Hub to perform the role of a distribution endpoint for application images.

To upload an application image to an Gravity Hub, first log into it:

```bsh
$ tele login -o hub.example.com
```

Then push the application image tarball:

```bsh
$ tele push alpine-0.1.0.tar
```

To view all currently published applications:

```bsh
$ tele ls
Type        Name:Version    Created (UTC)
----        ------------    -------------
App image   alpine:0.1.0    01/16/19 23:31
```

#### Interacting with Docker Registry and Helm Chart Repository

A Gravity Hub (and any Gravity cluster for that matter) acts as a Docker
registry and a Helm chart repository so it is possible to pull Docker images
and fetch Helm charts for published applications directly from the Gravity Hub.
Note that you have to be logged into the Gravity Hub with `tele login` for this
to work.

First let's try to pull a Docker image for the alpine application:

```bsh
$ docker pull hub.example.com/alpine:3.3
3.3: Pulling from alpine
da1f53af4030: Pull complete
Digest: sha256:014c089bd8b453b6870d2994eb4240ee69555fc5d760ffe515ef3079f5bcdad8
Status: Downloaded newer image for hub.example.com/alpine:3.3
```

!!! note 
    Pushing Docker images directly to the Gravity Hub registry with `docker push`
    is not supported. Use `tele push` to publish application image along with
    its charts and layers to the Gravity Hub.

Next make sure that the Gravity Hub was configured as a Helm charts repository:

```bsh
$ helm repo list
NAME                URL
hub.example.com     https://hub.example.com:443/charts
```

To search for a particular chart the standard Helm command can be used:

```bsh
$ helm search alpine
NAME                        CHART VERSION   APP VERSION DESCRIPTION
hub.example.com/alpine      0.1.0           3.3         Deploy a basic Alpine Linux pod
```

Helm can also be used to retrieve a Helm chart package archive, in the standard
Helm format:

```bsh
$ helm fetch hub.example.com/alpine --version 0.1.0  # will produce alpine-0.1.0.tgz
```

Execute `tele logout` to clear login information for the Gravity Hub, including
Docker registry and Helm chart repository credentials.

### Search Application Images

For the purpose of discovering applications to install in a deployed cluster,
Gravity provides a search command:

```bsh
$ gravity app search [-r|--remote] [-a|--all] [pattern]
```

By default the command will show application images that are available in
the local cluster. Provide `-r` flag to the command to display
applications from remote application catalog in the search results.

Note that the meaning of the "remote application catalog" differs
between the Open-Source and Enterprise versions of Gravity:

* For the Open-Source clusters, it is the Gravitational default
distribution portal (`get.gravitational.io`).
* For the Enterprise clusters, it is the Gravity Hub a cluster
is connected to. See [Configuring Trusted Clusters](config.md#trusted-clusters-enterprise)
for details on how to connect a cluster to an Gravity Hub.

The `-a` flag makes the command to display both local and remote applications.

Here's an example search result in a cluster that is connected to the Ops
Center `hub.example.com`:

```bsh
$ gravity app search -r
Name                    Version Description                      Created
----                    ------- -----------                      -------
hub.example.com/alpine  0.1.0   Deploy a basic Alpine Linux pod  Wed Jan 16 23:31 UTC
hub.example.com/nginx   0.1.0   A basic NGINX HTTP server        Tue Jan 15 20:58 UTC
```

The search command also accepts an optional application name pattern:

```bsh
$ gravity app search -r alpine
Name                    Version Description                      Created
----                    ------- -----------                      -------
hub.example.com/alpine  0.1.0   Deploy a basic Alpine Linux pod  Wed Jan 16 23:31 UTC
```

### Install a Release

To deploy an application image from a tarball, transfer it onto a
cluster node and execute the install command:

```bsh
$ gravity app install alpine-0.1.0.tar
```

The install command can also download the specified application from a remote
application catalog. Following up on the search example above, the alpine
application can be installed directly from the remote Gravity Hub:

```bsh
$ gravity app install hub.example.com/alpine:0.1.0
```

When executed inside a Gravity cluster, it will automatically push the
application to the local cluster controller which will keep the images
synchronized with the local Docker registries.

When deploying into a generic Kubernetes cluster, the install command needs
to know where to push the images and where the chart resources should pull
them from:

```bsh
$ gravity app install alpine-0.1.0.tar \
    --registry 10.103.27.132:5000 \
    --set image.registry=10.103.27.132:5000/
```

Gravity manages application lifecycle using the embedded Helm which
means that it creates a "release" for each deployed application image. The
same application image can be installed into a cluster multiple times,
and a new release will be created to track each installation.

To view all currently deployed releases, run:

```bsh
$ gravity app ls
Release         Status      Chart          Revision  Namespace  Updated
test-release    DEPLOYED    alpine-0.1.0   1         default    Thu Dec  6 21:13:14 UTC
```

!!! tip
    The `gravity app` set of sub-commands support many of the same flags of
    the respective `helm` commands such as `--set`, `--values`, `--namespace`
    and so on. Check `--help` for each command to see which are supported.

### Upgrade a Release

To upgrade a release, build a new application image (`alpine-0.2.0.tar`),
upload it to a cluster node and execute the upgrade command:

```bsh
$ gravity app upgrade test-release alpine-0.2.0.tar \
    --registry 10.103.27.132:5000 \
    --set image.registry=10.103.27.132:5000/
```

Or, download and install the upgrade application image from the connected
Gravity Hub:

```bsh
$ gravity app upgrade test-release hub.example.com/alpine:0.2.0
```

Similar to app install, registry flags need to be provided only when installing
in a generic Kubernetes cluster. When running in a Gravity cluster, application
will be synced with the local cluster registries automatically.

### Rollback a Release

Each release has an incrementing version number which is bumped every time
a release is upgraded. To rollback a release, first find out the revision
number to roll back to:

```bsh
$ gravity app history test-release
Revision    Chart           Status      Updated                  Description
2           alpine-0.2.0    DEPLOYED    Thu Dec  6 22:53:10 UTC  Upgrade complete
1           alpine-0.1.0    SUPERSEDED  Thu Dec  6 21:13:14 UTC  Install complete
```

Then rollback to a given revision:

```bsh
$ gravity app rollback test-release 1
```

This command will rollback the specified release `test-release` to the
revision number `1`.

### Uninstall a Release

To uninstall a release, execute the following command:

```bsh
$ gravity app uninstall test-release
```
