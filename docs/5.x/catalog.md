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
    built-in so you don't have to have the `helm` binary installed on the
    machine where application images are built, or in a cluster. A cluser
    needs to have `tiller` server (Helm's server component) running though.

To check the embedded Helm version, run `tele version` or `gravity version`
command:

```bsh
$ tele version
Edition:      open-source
Version:      5.4.0
Git Commit:   971d93b06da08c3b277fc033c1c4fdca55a0ec6b
Helm Version: v2.12
```

### Build an Application Image

For this example we will be using a [sample Helm chart](https://github.com/helm/helm/tree/master/docs/examples/alpine)
found among Helm examples. This chart spins up a single pod of Alpine Linux:

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

To make it work as an application image, we need to make sure that the pod's
image reference includes a registry template variable which can be set to
an appropriate registry during installation:

```bsh
image: "{{ .Values.image.registry }}{{ .Values.image.repository }}:{{ .Values.image.tag }}"
```

We can now use `tele` to build an application image from this chart:

```bsh
$ tele build alpine
```

The output is a tarball `alpine-0.1.0.tar` which includes a packaged Helm
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
to know where to push the images to and where the chart resources will pull
them from:

```bsh
$ gravity app install alpine-0.1.0.tar \
    --registry 10.103.27.132:5000 \
    --set image.registry=10.103.27.132:5000/
```

Gravity uses embedded Helm for all application lifecycle management which
means that it creates a "release" for each deployed application image. The
same application image can be installed into a cluster multiple times,
and a new release will be created to track each installation.

To view all currently deployed releases, run:

```bsh
$ gravity app ls
Release         Status      Chart          Revision  Namespace  Updated
falling-horse   DEPLOYED    alpine-0.1.0   1         default    Thu Dec  6 21:13:14 UTC
```

!!! tip:
    The `gravity app` set of sub-commands support many of the same flags of
    the respective `helm` commands such as `--set`, `--values`, `--namespace`
    and so on. Check `--help` for each command to see which are supported.

### Upgrade a Release

To upgrade a release, build a new application image (say, `alpine-0.2.0.tar`),
upload it to a cluster node and execute the upgrade command:

```bsh
$ gravity app upgrade falling-horse alpine-0.2.0.tar \
    --registry 10.103.27.132:5000 \
    --set image.registry=10.103.27.132:5000/
```

Similar to the install, registry address will be detected automatically
when running in a Gravity cluster.

### Rollback a Release

Each release has an incrementing version number which is bumped every time
a release is upgraded. To rollback a release, first find out the revision
number it should be rolled back to:

```bsh
$ gravity app history falling-horse
Revision    Chart           Status      Updated                  Description
2           alpine-0.2.0    DEPLOYED    Thu Dec  6 22:53:10 UTC  Upgrade complete
1           alpine-0.1.0    SUPERSEDED  Thu Dec  6 21:13:14 UTC  Install complete
```

After determining the version, perform a rollback:

```bsh
$ gravity app rollback falling-horse 1
```

### Uninstall a Release

To uninstall a release, execute the uninstall command:

```bsh
$ gravity app uninstall falling-horse
```
