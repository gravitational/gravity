# Common Issues

## Conflicting Programs

Certain programs such as `dnsmasq`, `dockerd` and `lxd` interfere with Telekube operation and need be uninstalled.

## IPv4 Forwarding

IPv4 forwarding on servers is required for internal Kubernetes load balancing and must be turned on.

```bash
sysctl -w net.ipv4.ip_forward=1
echo net.ipv4.ip_forward=1 >> /etc/sysconf.d/10-ipv4-forwarding-on.conf
```

## Network Bridge Driver

Network bridge driver should be loaded, which could be ensured with `modprobe br_netfilter` or `modprobe bridge` command depending on the distribution.
Distributions based on kernels prior to 3.18 might have this compiled into the kernel.
In CentOS 7.2, the module is called `bridge` instead of `br_netfilter`.

Additionally, it should be configured to pass IPv4 traffic to `iptables` chains:

```
sysctl -w net.bridge.bridge-nf-call-iptables=1
echo net.bridge.bridge-nf-call-iptables=1 >> /etc/sysconf.d/10-bridge-nf-call-iptables.conf
```

For more information see the relevant [System Requirements](requirements.md#br_netfilter-module)
section.

## Overlay Driver

If the application uses overlay or overlay2 Docker storage driver, the
`overlay` kernel module should be loaded. See the relevant
[System Requirements](requirements.md#overlay-module) section for details.

## D_TYPE Support in Filesystem

If the application uses overlay or overlay2 Docker storage driver, the backing
filesystem needs to support `d_type` for overlay to work properly.
For example, XFS does not support `d_type` if it has been formatted with the
`-n ftype=0` option.

!!! warning
    Starting with v1.13.0, Docker deprecates support for file systems without `d_type`
    support for overlay/overlay2 storage drivers.

Running on file systems without `d_type` support causes Docker to skip the attempt
to use the overlay or overlay2 driver. Existing installs will continue to run,
but produce an error. This is to allow users to migrate their data.
In a future version, this will be a fatal error, which will prevent Docker from
starting.

## System Time Sync

Kubernetes master nodes are sensitive to system time differences between nodes they're running on. It is recommended to have the system time synchronized against an NTP time source via tools like `ntpd`.

##  Firewalld

If `firewalld` is enabled in the system, Kubernetes services may not be able to communicate under default configuration.

```bash
firewall-cmd --zone=trusted --add-source=10.244.0.0/16 --permanent # pod subnet
firewall-cmd --zone=trusted --add-source=10.100.0.0/16 --permanent # service subnet
firewall-cmd --zone=trusted --add-interface=eth0 --permanent       # enable eth0 in trusted zone so nodes can communicate
firewall-cmd --zone=trusted --add-masquerade --permanent           # masquerading so packets can be routed back
firewall-cmd --reload
systemctl restart firewalld
```

Note that pod and service subnet addresses may be [customized at install time](installation.md#standalone-offline-cli-installation).`

## Azure Hyper-V Clock Sync

Azure is adjusting VM clock from host, which may result in time drift between nodes.
Currently a recommended workaround is installing NTP client such as `chrony`, and disabling Hyper-V time sync:

```bash
function get_vmbus_attr {
  local dev_path=$1
  local attr=$2

  cat $dev_path/$attr | head -n1
}

function get_timesync_bus_name {
  local timesync_bus_id='{9527e630-d0ae-497b-adce-e80ab0175caf}'
  local vmbus_sys_path='/sys/bus/vmbus/devices'

  for device in $vmbus_sys_path/*; do
    local id=$(get_vmbus_attr $device "id")
    local class_id=$(get_vmbus_attr $device "class_id")
    if [ "$class_id" == "$timesync_bus_id" ]; then
      echo vmbus_$id; exit 0
    fi
  done
}

timesync_bus_name=$(get_timesync_bus_name)
if [ ! -z "$timesync_bus_name" ]; then
  # disable Hyper-V host time sync
  echo $timesync_bus_name > /sys/bus/vmbus/drivers/hv_util/unbind
fi
```

## Kubernetes Pods Stuck in Terminating State

Some linux distributions have included the kernel setting fs.may_detach_mounts with a default value of 0. This can cause conflicts with the docker daemon, where docker is then unable to unmount mount points within the container. Kubernetes will show pods as stuck in the terminating state if docker is unable to clean up one of the underlying containers.

If the installed kernel exposes the option fs.may_detach_mounts we recommend always setting this value to 1, or you may experience issues terminating pods in your cluster.

```
sysctl -w fs.may_detach_mounts=1
echo "fs.may_detach_mounts = 1" >> /etc/sysctl.d/10-may_detach_mounts.conf
```
