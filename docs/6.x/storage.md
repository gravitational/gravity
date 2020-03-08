# Persistent Storage

Many applications deployed on Kubernetes require persistent storage. To provide
persistent storage capabilities, Gravity has an out-of-the-box integration with
[OpenEBS](https://openebs.io/), Kubernetes-native Container Attached Storage
framework.

OpenEBS provides the following benefits:

* Synchronous data replication.
* Snapshots and clones.
* Backup and restore.
* Monitoring via Prometheus.

OpenEBS is a [CNCF](https://www.cncf.io/) native project. See OpenEBS
[documentation](https://docs.openebs.io/docs/next/overview.html) for more
information.

!!! note "Supported version"
    OpenEBS integration is supported starting from Gravity 7.0.

## Enable OpenEBS

By default OpenEBS integration is disabled. It can be enabled by setting the
following field in a cluster image manifest file:

```yaml
storage:
  openebs:
    enabled: true
```

When OpenEBS is enabled, it will be packaged in the cluster image tarball
alongside other dependencies during the `tele build` process. During the
cluster installation, OpenEBS operator and other components will be installed
in the `openebs` namespace.

!!! note "Privileged containers"
    Certain OpenEBS components need to operate in a privileged mode so
    privileged containers will be implicitly allowed when integration with
    OpenEBS is enabled. See [their FAQ](https://docs.openebs.io/docs/next/faq.html#why-ndm-priviledged)
    for details.

### Enable OpenEBS During Upgrade

OpenEBS can be enabled for existing Gravity clusters when they are upgraded
to a new version that has OpenEBS integration turned on.

To enable it in the existing cluster:

* Update your cluster image manifest to enable OpenEBS integration like shown above.
* Build a new version of the cluster image using `tele build`. See [Building a Cluster Image](pack.md#building-a-cluster-image) for details.
* Upgrade the existing cluster to this new version. See [Upgrading a Cluster](cluster.md#updating-a-cluster) for details.

OpenEBS will be installed and configured during the upgrade operation.

## Configure OpenEBS

OpenEBS scans the cluster nodes to discover block devices attached to the nodes
and makes them available for use via persistent volumes. In the default configuration
it takes into account all devices it finds, excluding some well-known system
ones such as loop/devicemapper devices, RAM disks and so on. Sometimes it may
be necessary to only consider certain devices on the nodes, for example to
exclude some disks that are attached to the node but should not be allocated
for the in-cluster persistent storage use.

Gravity provides a way to configure OpenEBS devices via a `PersistentStorage`
resource:

```yaml
kind: PersistentStorage
version: v1
spec:
  # OpenEBS-specific configurations.
  openebs:
    # Filters for OpenEBS block device discovery.
    filters:
      # Include/exclude specific devices or device patterns.
      devices:
        include: []
        exclude: ["/dev/sdb", "/dev/sdc"]
      # Exclude devices mounted under specific paths.
      mountPoints:
        exclude: ["/", "/etc/hosts", "/boot"]
      # Include/exclude devices from specific vendors.
      vendors:
        include: []
        exclude: ["CLOUDBYT", "OpenEBS"]
```

The presence of include vs exclude filters affects which devices get discovered
by OpenEBS. The behavior is as follows:

* If `include` filter is present, only matching devices will be selected.
* If `include` filter is empty, all devices will be selected, except for those
  matching `exclude` filters.

Note that the `mountPoints` filter provides only the `exclude` rule. See
[Include filters](https://docs.openebs.io/docs/next/ugndm.html#Include-filters) and
[Exclude filters](https://docs.openebs.io/docs/next/ugndm.html#Exclude-filters)
OpenEBS documentation for more details.

The default OpenEBS filters will be merged with the filters configured via a
`PersistentStorage` resource. The default filters are:

* Exclude devices: `loop`, `/dev/fd0`, `/dev/sr0`, `/dev/ram`, `/dev/dm-`, `/dev/md`.
* Exclude vendors: `CLOUDBYT`, `OpenEBS`.
* Exclude mount points: `/`, `/etc/hosts`, `/boot`.

Suppose that we want to include only two specific devices, `/dev/sdd` and
`/dev/sde`, in the OpenEBS pool. Let's define the following resource in a
`storage.yaml` file:

```yaml
kind: PersistentStorage
version: v1
spec:
  openebs:
    filters:
      devices:
        include: ["/dev/sdd", "/dev/sde"]
```

To update the OpenEBS configuration:

```bash
$ gravity resource create storage.yaml
```

To view the current persistent storage configuration:

```bash
$ gravity resource get storage
```

To check which devices are currently being managed by OpenEBS, you can use
`kubectl` to view their respective custom resources:

```bash
$ kubectl get blockdevices --all-namespaces
```

The devices that are currently being managed by OpenEBS will appear in the
`Active` state. If a device was discovered by OpenEBS and later on excluded via
a filter, it will stay among the block devices list in the `Unknown` state and
won't be allocated for persistent storage anymore.

### Per-Node OpenEBS Configuration

At the moment OpenEBS does not support specifying filters on a per-node basis.
It is possible, however, to entirely exclude devices on some nodes from being
discovered by OpenEBS by setting a `gravitational.io/no-storage` label on those
nodes.

For example, a label can be set via a node profile in the manifest file:

```yaml
nodeProfiles:
- name: node
  labels:
    gravitational.io/no-storage: "true"
```

### Configure OpenEBS at Install Time

Persistent storage configuration can be provided during the initial cluster
installation to make sure that only appropriate devices are discovered when
OpenEBS comes up for the first time.

To supply the initial persistent storage configuration, pass it via a `--config`
flag to the install command:

```bash
$ sudo ./gravity install --config=storage.yaml
```

!!! note "Multiple resources"
    Configuration file passed on the command-line may contain multiple Kubernetes
    and Gravity resources, like described in a general [Cluster Configuration](config.md)
    section.

Once installed & configured, OpenEBS provides a few ways to consume persistent
storage in a cluster:

* Via a Local Volume Provisioner using either host volumes or block devices
  directly.
* Via a cStor storage engine that provides additional features such as
  replication, high resiliency, snapshotting and so on.

See OpenEBS documentation on [LocalPV](https://docs.openebs.io/docs/next/localpv.html)
and [cStor](https://docs.openebs.io/docs/next/cstor.html) for detailed information
about these storage engines.

## Local Provisioner / Host Volumes

OpenEBS supports using host volumes for persistent storage. In this mode of
operation it functions similarly to `hostPath` volume type but allows you to
request a persistent volume via a persistent volume claim of a certain storage
class.

Gravity clusters come with a pre-installed storage class called `openebs-hostpath`
that uses `/var/lib/gravity/openebs/local` directory to store the persistent
volumes data.

!!! note "Custom state directory"
    If cluster is using custom state directory location, say `/opt/gravity`,
    the local volumes data will be stored under `/opt/gravity/openebs/local`
    on host, but will still map to the `/var/lib/gravity/openebs/local` inside
    the master (planet) container.

To use the host volume storage, first define a persistent volume claim that
uses the appropriate storage class, for example:

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: claim-hostpath
spec:
  storageClassName: openebs-hostpath
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

And then consume the claim in a pod template:

```yaml
volumes:
- name: local-vol-from-hostpath
  persistentVolumeClaim:
    claimName: claim-hostpath
```

If you want to keep the local volumes data on a separate device, it is a common
practice to mount it under `/var/lib/gravity/openebs/local` before initial
cluster installation.

Alternatively, you can create another storage class that will use the
host volume provisioner as described in [OpenEBS documentation](https://docs.openebs.io/docs/next/localpv.html#how-to-use-openebs-local-pvs)
and use it in your persistent volume claims instead of the default one.

## Local Provisioner / Block Devices

OpenEBS local provisioner also supports using block devices directly.

To use a block device in a persistent volume, create a persistent volume claim
that uses `openebs-device` storage class which is pre-created during cluster
installation:

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: claim-device
spec:
  storageClassName: openebs-device
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

When persistent volume claim is attached to a pod, OpenEBS will find a matching
block device among its managed devices pool (which can be viewed using
`kubectl get blockdevices --all-namespaces`) and use it in a persistent volume:

```yaml
volumes:
- name: local-vol-from-device
  persistentVolumeClaim:
    claimName: claim-device
```

See [OpenEBS documentation](https://docs.openebs.io/docs/next/localpv.html#how-to-use-openebs-local-pvs)
on local provisioner to learn how to create another device-based storage class
and tweak its parameters, if needed.

## cStor Engine

cStor is the most sophisticated storage engine supported by OpenEBS that provides
features such as data replication, thin provisioning, snapshotting and so on.
cStor serves block storage to containers over iSCSI interface.

!!! note "iSCSI tools"
    iSCSI tools must be installed on the cluster nodes in order for cStor volumes
    to work. See [iSCSI install instructions](https://docs.openebs.io/docs/next/prerequisites.html)
    on OpenEBS website. In future Gravity versions, iSCSI tools may come
    preinstalled removing the need to have them on host.

In order to start using cStor, you will first need to create a cStor storage
pool claim. A storage pool claim contains one or more block devices from the list
of devices managed by OpenEBS:

```bash
$ kubectl get blockdevices --all-namespaces
NAMESPACE   NAME                                           NODENAME    SIZE          CLAIMSTATE   STATUS   AGE
openebs     blockdevice-6fad0b5501068297703de4b2c8553886   10.0.2.15   21474836480   Unclaimed    Active   21h
openebs     blockdevice-8064e98edba7e57141daab4800ecd792   10.0.2.15   10737418240   Unclaimed    Active   21h
```

Once you've figured out which devices you want to use for the pool, create a
storage pool claim, for example:

```yaml
apiVersion: openebs.io/v1alpha1
kind: StoragePoolClaim
metadata:
  name: cstor-pool
spec:
  name: cstor-pool
  type: disk
  poolSpec:
    poolType: striped
  blockDevices:
    blockDeviceList:
    - blockdevice-8064e98edba7e57141daab4800ecd792
```

!!! note "Storage pool devices"
    Devices selected for the cStor storage pool must be unclaimed, unformatted
    and unmounted on the node.

Once the storage pool claim (and its corresponding storage pool) has been created,
create a storage class that will use this pool:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: openebs-cstor
  annotations:
    openebs.io/cas-type: cstor
    cas.openebs.io/config: |
      - name: StoragePoolClaim
        value: "cstor-pool"
      - name: ReplicaCount
        value: "1"
provisioner: openebs.io/provisioner-iscsi
```

After that, you can create a persistent volume claim for this storage class:

```yaml
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: claim-cstor
spec:
  storageClassName: openebs-cstor
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
```

And consume it in a pod template:

```yaml
volumes:
- name: local-vol
  persistentVolumeClaim:
    claimName: claim-cstor
```

See [cStor documentation](https://docs.openebs.io/docs/next/ugcstor.html) on
OpenEBS website for more detailed information about creating and configuring
cStor pools and storage classes.
