---
title: SELinux and Kubernetes
description: How to run Kubernetes applications with SELinux in air-gapped and on-premise environments
---

# SELinux

## Host Preparation

Before installing Gravity, you have to ensure that the user performing the installation has the privilege
to load policy modules.

Check whether SELinux is enabled and is in enforcing mode:
```sh
$ sestatus
SELinux status:                 enabled
Current mode:                   enforcing
Policy from config file:        targeted
...
```

Next, the Linux user performing the installation needs to be mapped to the proper SELinux user/role.
Installer needs to run with an administrative role capable of loading SELinux policy modules - for example `sysadm_r`.

To check existing mappings, use the following:

```sh
$ semanage login -l
                Labeling   MLS/       MLS/
SELinux User    Prefix     MCS Level  MCS Range                      SELinux Roles
root            user       s0         s0-s0:c0.c1023                 staff_r sysadm_r system_r unconfined_r
staff_u         user       s0         s0-s0:c0.c1023                 staff_r sysadm_r system_r unconfined_r
sysadm_u        user       s0         s0-s0:c0.c1023                 sysadm_r
unconfined_u    user       s0         s0-s0:c0.c1023                 system_r unconfined_r
...
```

In order to map the Linux user `john` to the SELinux user `staff_u`, do the following:
```sh
$ semanage login -a -s staff_u john
```
or modify the mapping with:
```
$ semanage login -m -s staff_u john
```

Switch to the `sysadm_r` role:
```sh
$ sudo --role sysadm_r --login
```

Alternatively, directly run the installer using the role `sysadm_r` and type `sysadm_t`:
```sh
$ runcon --role sysadm_r --type sysadm_t ./gravity install --selinux ...
```

Gravity supports SELinux users `sysadm_u` and `unconfined_u` (and their corresponding roles) out of the box.
If you need to install and manage Gravity clusters using a different user/role, a custom user policy is required.


## Installation

`gravity install` will use SELinux if all of these conditions are met:

  * the host has SELinux enabled
  * the installer has SELinux support for the host OS distribution (see [Supported OS Distributions](#os-distribution-support) below)
  * SELinux support has been turned on with `--selinux`

The installer does the following as the first step when running on a host with the above conditions met:

  * loads the Gravity SELinux policy module
  * creates local port bindings for Gravity and Kubernetes-specific ports
  * creates local file contexts for paths used during the operation
  * creates local file contexts for custom state directory (if it has been overridden with `--state-dir`)

Additionally, the operation plan will contain a dedicated step:

  * to add SELinux mapping for the custom VXLAN port if one was provided
  * to configure local file contexts for all volumes mounted into the container

To start the installation, use the `gravity install` command as usual:

```sh
$ gravity install --selinux ...
 Bootstrapping installer for SELinux
 ...
```

Likewise, on the joining node:

```sh
$ gravity join --selinux ...
 Bootstrapping installer for SELinux
 ...
```

SELinux support is managed per-node.

## Upgrades

The upgrade will automatically determine whether SELinux support is on on the cluster nodes and will install and configure
the new policy individually on each node.


## Kubernetes

Kubernetes supports SELinux with [seLinuxOptions](https://v1-17.docs.kubernetes.io/docs/tasks/configure-pod-container/security-context/#assign-selinux-labels-to-a-container) inside the [securityContext](https://v1-17.docs.kubernetes.io/docs/tasks/configure-pod-container/security-context/). The options can be configured either for the whole Pod or per-container:

```yaml
...
securityContext:
  seLinuxOptions:
    type: "my_type_t"
    level: "s0:c123,c456"
```

Level specifies the SELinux MCS/MLS security level the container (or containers if specified for the Pod) is run with.
If left unspecified, container runtime (Docker in this case) will generate a unique level.

The security context will also apply to Pod's volumes (where applicable). The volumes that support SELinux labeling,
are automatically relabeled according to the specified level:

  * If the `level` is left unspecified, the container runtime will use the same level generated for the container to label the volume
  * Otherwise, the volume is labeled with the specified level

If multiple Pods run with the same `level`, they will be able to share volumes. If you need inter-Pod protection, consider running
the Pods with unique MCS `level` or leave the `level` unspecified so it gets automatically generated to be unique.

Kubernetes groups volume storage types into groups and differentiates between Unshared and Shared. SELinux labeling will differ depending on the storage type group.
`Secret` (and consequently all `EmptyDir`-derived volume types) are examples of Unshared storage and support SELinux labeling, while `hostPath` (as a Shared storage) does not support labeling at the time of this writing.

If you don't specify SELinux domain (`seLinuxOptions.type`) or leave `seLinuxOptions` unspecified, containers will run in the default container process domain.

One implication of the SELinux labeling concerns the volumes bind-mounted from host inside the Planet container.
In order for external volumes to be accessible to Kubernetes workloads they need to be labeled with a label accessible to the process domain the
containers are run with.
By default, each volume will automatically get relabeled at install time to the default container file label unless overridden.
The label can be customized in the application manifest:

```yaml
nodeProfiles:
  - name: node
    ...
    volumes:
      - path: /var/data
        targetPath: /var/data
        seLinuxLabel: "system_u:object_r:my_file_type_t:s0"
```

If the directory has been labeled prior to the installation and relabeling is undesired, the labeling can be turned off with:

```yaml
nodeProfiles:
  - name: node
    ...
    volumes:
      - path: /var/data
        targetPath: /var/data
        seLinuxLabel: none
```


!!! warning "Performance"
  Relabeling of directories with a large number of files/sub-directories can be time-consuming.


## Custom SELinux policies

It is not yet possible to bundle a custom SELinux policy in the cluster image. If you have custom SELinux policy and want
to use it for your Kubernetes workloads, you'll need to make sure to load the policy on each node where necessary prior to
installing the cluster image.


## OS Distribution Support

Currently the following distributions are supported:

| Distribution | Version |
|--------------|----------------|
| CentOS       | 7+            |
| RedHat       | 7+            |


## Troubleshooting

If the installer fails, it is important to determine whether the problem is related to SELinux.

If the reported error is a permission denied error, it might be related to SELinux but it also might be a
Linux DAC (Discretionary Access Control) violation.
Before SELinux gets a chance to validate permissions for a particular resource, the access first passes through Linux DAC.
So, an absence of denials in the SELinux audit log might be an indication that the access is due to a DAC violation.

As a first step, if the permission denied error has been generated for a specific file system location, verify that the location
has proper access mode for the current user before turning to SELinux.

Gravity installer captures the audit log messages relevant for the operation as part of the automatically generated crash report file.

Extract the contents of the crashreport.tgz into a directory of your choice:

```sh
$ tar xf crashreport.tgz -C /path/to/dir
```

The contents of the tarball might vary depending on the operation step and will either contain the captures from the current host only
or the captures from all cluster nodes. In the latter case, the contents of the crashreport.tgz will be similar to:

```sh
$ tar tvf crashreport.tgz
<hostname>-debug-logs.tar.gz
<hostname>-k8s-logs.tar.gz
<hostname>-etcd-backup.json.tar.gz
cluster.json
...
```

The audit log captures will be then available inside the `<hostname>-debug-logs.tar` where `hostname` is the name of the current host.

Issue the following command to see the list of denials as SELinux allow rules:

```sh
$ tar xf <node>-debug-logs.tar --to-stdout audit.log.tz | gunzip -c | audit2allow
```

In order to see more detailed (but also more complex) output, use the following:

```sh
$ tar xf <node>-debug-logs.tar --to-stdout audit.log.tz | gunzip -c | ausearch --interpret
```

Sometimes, the lack of denials in the log is due to suppressed `dontaudit` rules.
Force them to be logged with:

```sh
$ semodule --disable_dontaudit --build
```

and retry the operation.


### Searching For Denials In Host Auditlog

To check for all relevant recent SELinux denials, use the following:

```sh
$ ausearch --message all --start recent --interpret --success no
```

To search for all SELinux denials logged today for the gravity process domain, use the following:

```sh
$ ausearch --message all --start today --context gravity_t --interpret --success no
```

### Additional Tools

See [autrace](http://man7.org/linux/man-pages/man8/autrace.8.html) and [ausearch](http://man7.org/linux/man-pages/man8/ausearch.8.html) for more details
about the kernel audit system.

#### setroubleshoot

setroubleshoot ecosystem provides additional tools to analyze and provide user-friendly descriptions of SELinux denials.
The package consists of these additional tools:

 * setroubleshootd is a DBus service that automatically provides user-friendly explanations for the SELinux AVC denials by receiving
 the denials from the audit daemon, analyzing them using a set of plugins and pushing the notifications to the clients.

 * sealert is the UI client to the setroubleshootd DBus daemon. It listens and displays the notifications that the daemon generates.

See [setroubleshootd](https://linux.die.net/man/8/setroubleshootd) and [sealert](https://linux.die.net/man/8/sealert) for more details.


