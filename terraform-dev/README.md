# Description

This is a single-node development based
on terraform-libvirt plugin.

Sets up a single node gravity cluster. It is faster than Vagrant
and does not have Vagrant's problems with networking setup and ruby
dependencies.

## Requirements

Install:

* `ansible==2.6.1`
* `Terraform v0.11.7`
* `terraform-libvirt==0.4`

* Build local telekube in the top dir:

`make production telekube`

* Fom this directory, run

`make`

## Wireguard

To patch existing telekube cluster to use CNI and Wireguard,
use `install-wireguard` and `install-wireguard-cni` make targets:

```
make install-wireguard install-wireguard-cni
```

This command will patch the existing cluster to use CNI bridge
plugin, and will set up routes and iptables rules
to forward inter-pod traffic via wireguard.

