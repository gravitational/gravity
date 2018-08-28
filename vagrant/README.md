# Yet another Vagrant environment

## Requirements

* Vagrant
* ansible

IP forwarding must be enabled on host machine:

```
sudo sysctl net.ipv4.ip_forward
> net.ipv4.ip_forward = 1
```

Enable SNAT for Internet access interface and allow forwarding packets on host machine:

```
iptables -w -t nat -A POSTROUTING -o wlan+ -j MASQUERADE
iptables -w -t nat -A POSTROUTING -o eth+ -j MASQUERADE
iptables -w -t filter -A INPUT -i virbr+ -j ACCEPT
...
```

## Install Telekube on Vagrant

Change working directory to `vagrant` folder and execute:

```sh
$ cd vagrant
$ vagrant up
```

This will bring 3 VMs properly configured for development

Build telekube

```sh
$ make production telekube
```

Install telekube

```sh
$ cd vagrant
$ make ansible-install
```

## Redeploy just Gravity

To update binaries on all hosts (e.g. during development), do the following:

```sh
$ cd vagrant
$ make ansible-update-gravity
```

## Redeploy Gravity to remote hosts

If you have deployed somewhere else, it is handy to update binaries on remote hosts.

You can use the same targets with remote hosts as well using `remote-` prefixes
that will generate remote inventories instead.

```sh
REMOTE_USER=centos REMOTE_KEY=~/.ssh/ops.pem REMOTE_HOSTS="34.211.109.240" make remote-update-gravity
```

## Test full upgrade

```sh
make production telekube
make ansible-upgrade
```

## Using Sandbox

Sandbox is handy when troubleshooting specific operation such as upgrade with frequent need to rollback to original state. 

Install required plugins

```sh
vagrant plugin install fog-libvirt sahara
```

Save current state

```sh
vagrant suspend && vagrant sandbox on && vagrant resume
```

Rollback to original state

```sh
vagrant suspend && vagrant sandbox rollback && vagrant resume
```

Note that it will also rollback localtime on nodes. 