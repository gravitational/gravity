# SELinux

Starting with version 7, Gravity comes with SELinux support.
It is not a requirement to have SELinux enabled, but whenever the installer detects that
it runs on a SELinux-enabled host, it will automatically turn on SELinux support.

When operating with SELinux support on, the following changes:

 * Installer process automatically loads the policy and configures the local paths and ports necessary
for its operation. After bootstrapping, the installer will run confined in its own domain.

 * Planet container runs its services and all Kubernetes workloads confined - this means Docker will also
be configured to run with SELinux support on.

## Host Preparation

Before installing Gravity, you have to ensure that the user performing the installation has the privilege
to load policy modules - otherwise the installer will fail to bootstrap.

To check the SELinux status, run the following:
```sh
$ sestatus
SELinux status:                 enabled
Current mode:                   enforcing
Policy from config file:        targeted
...
```

Next, the Linux user performing the installation needs to be mapped to the proper SELinux user/role tuple.
Installer needs to run with an administrative role capable of loading the SELinux policies - for example `sysadm_r`.

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

To map the Linux user `john` to a SELinux user `staff_u`, do the following:
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
$ runcon --role sysadm_r --type sysadm_t ./gravity install ...
```

Gravity supports SELinux users `sysadm_u` and `unconfined_u` (and their corresponding roles) out of the box.
If you need to install and manage Gravity clusters using a different user/role, a custom user policy is required.

TODO: user policy module template


## Installation

The install will automatically use SELinux if a) the host has SELinux enabled and b) installer has SELinux support for the host OS distribution.

Installer does the following as the first step when running on a host with the above conditions met:

  * loads the Gravity SELinux policy module
  * creates local port bindings for Gravity and Kubernetes-specific ports
  * creates local file contexts for paths used during the operation
  * creates local file contexts for custom state directory (if it has been overridden with `--state-dir`)

Additional SELinux configuration might happen later as part of execution of the operation plan.

To start the installation, use the `gravity install` command as usual:

```sh
$ gravity install ...
 Bootstrapping installer for SELinux
 ...
```

Likewise, on the joining node:

```sh
$ gravity join ...
 Bootstrapping installer for SELinux
 ...
```

SELinux support can be turned off individually by specifying `--no-selinux` for either command:

```sh
$ gravity install --no-selinux ...
```

or join:

```sh
$ gravity join --no-selinux ...
```

SELinux support is managed per-node.

## Upgrades

The upgrade will automatically determine whether SELinux support is on on the cluster nodes and will install and configure
the new policy individually on each node.


## Custom SELinux policies

It is not yet possible to bundle a custom SELinux policy in the cluster image. If you have custom SELinux policy and want
to use it for your Kubernetes workloads, you'll need to make sure to load the policy on each node where necessary prior to
installing the cluster image.

It is planned to add support for custom policies in a future version.


## OS Distribution Support

Currently the following distributions are supported:

| Distribution | Version |
|--------------|----------------|
| CentOS       | 7+            |
| RedHat       | 7+            |


## Troubleshooting

If the installer fails, pay attention to the errors about denied permissions which might be the indicator of an SELinux issue.

Unfortunately, it might not be obvious whether the particular denied permission is an SELinux denial.
Before SELinux gets a chance to validate permissions for a particular resource, the access first passes through the
Linux DAC (Discretionary Access Control) validation.

Absence of the logged denial in the SELinux audit log might be an indication that the access has failed DAC validation.

The basic approach to determining whether a permission has been denied due to SELinux is as following:

Enable the logging of `dontaudit` rules which are usually suppressed by default:

  ```sh
  $ semodule --disable_dontaudit --build
  ```

Search for recent denials using a catch-all message type:

  ```sh
  $ ausearch --message all --start recent --interpret --success no
  ```

If the logs still don't show a denial, it is likely a failed DAC check. In this case, check that the user in fact has permissions
to access the resources mentioned in the error message.


### Searching For Denials

To check for all relevant recent SELinux denials, use the following:

```sh
$ ausearch --message all --start recent --interpret --success no
```

To search for all SELinux denials logged today for the gravity binary context, use the following:

```sh
$ ausearch --message all --start today --context gravity_t --interpret --success no
```

To search for SELinux denials and have them automatically converted to SELinux rules:

```sh
$ ausearch --message all --start recent | audit2allow
```

Although the primary use for this is creating a custom policy to fix the denials, it also provides a succinct overview of the denials.


### Additional Tools

#### setroubleshoot

setroubleshoot ecosystem provides additional tools to analyze and provide user-friendly descriptions of SELinux denials.
The package consists of these additional tools:

 * setroubleshootd is a DBus service that automatically provides user-friendly explanations for the SELinux AVC denials by receiving
 the denials from the audit daemon, analyzing them using a set of plugins and pushing the notifications to the clients.

 * sealert is the UI client to the setroubleshootd DBus daemon. It listens and displays the notifications that the daemon generates.

See [setroubleshootd](https://linux.die.net/man/8/setroubleshootd) and [sealert](https://linux.die.net/man/8/sealert) for more details.
