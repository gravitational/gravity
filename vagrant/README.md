# Yet another Vagrant environment

## Requirements

* Vagrant
* Vagrant plugins:
  - vagrant-disksize
* ansible


## Preparing for MacOS

1. Install brew on macOS https://brew.sh/
1. Install vagrant: `brew install --cask vagrant`
1. Install VirtualBox: `brew install --cask virtualbox`
1. Install ansible: `brew install ansible`
1. Install vagrant plugins: `vagrant plugin install vagrant-disksize`

## Install Telekube on Vagrant

Change working directory to `vagrant` folder and execute:

```sh
$ cd vagrant
$ vagrant up
```

This will bring 3 VMs properly configured for development

Build gravity and prepare tarball
```sh
$ make production telekube
```

Install telekube

```sh
$ cd vagrant
$ make ansible-install
```

Install gravity from custom tarball
```shell
$ cd vagrant
$ ANSIBLE_FLAGS='--extra-vars "gravity_archive_url=https://get.gravitational.com/gravity-7.0.28-linux-x86_64-bin.tar.gz tarball_path=/full/path/to/tarball.tar"'
$ ANSIBLE_FLAGS=$ANSIBLE_FLAGS make ansible-install
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