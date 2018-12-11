# Application Catalog

!!! note:
    The Application Catalog feature is currently under active development and
    is available starting from 5.4.0 alpha releases.

Gravity supports packaging Helm charts as self-contained application images
that can be installed in a Gravity cluster or a generic Kubernetes cluster
(provided it has an access to a Docker registry).

An application image is a tarball that contains:

* A Helm chart (possibly with chart dependencies).
* Vendored Docker images for the chart's Kubernetes resources.

!!! tip:
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

For this example we will be using a [sample Helm chart](https://github.com/helm/helm/tree/master/docs/examples/alpine).
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
image: "{{ .Values.image.registry }}{{ .Values.image.repository }}:{{ .Values.image.tag }}"
```

We can now use `tele` to build an application image from this chart:

```bsh
$ tele build alpine
```

The result is a tarball `alpine-0.1.0.tar` which includes a packaged Helm
chart and the Alpine image layers.

### Install a Release

To deploy an application image, transfer it onto a cluster node and execute
the install command:

```bsh
$ gravity app install alpine-0.1.0.tar
```

When executed inside a Gravity cluster, it will automatically push the
application to the local cluster controller which will keep the images
synchonized with the local Docker registries.

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

!!! tip:
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
