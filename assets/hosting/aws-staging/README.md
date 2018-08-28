### Staging Area

Some helper scrits Ev used to create a small staging area on AWS.
The result is 4 instances:

* `master.s.gravitational.io`
* `worker-a.s.gravitational.io`
* `worker-b.s.gravitational.io`
* `db.s.gravitational.io`

### Configuration

* Based on `Centos 7.2` with `centos` user instead of `root`
* They are all tagged with `env:staging` so you can create a "resource group" for yourself to conveniently access them.
* They all use 'staging' security group which opens `80`, `22`, `61822` and `443` ports.
* SSH is configured for port `61822`

### Storage

Each box has a `/mnt` directory mounted to a local 30GB SSD volume on ext3 filesystem representing a
conservative CentOS setup. We have a symlink `/var/lib/gravity` owned by `centos:centos` (1000:1000) pointing
to `/mnt/gravity`.

### Access

Example:

```
$ ssh -p 61822 centos@master.s.gravitational.io
```

Typing `-p 61822` can be annoying. You can create a bash alias. Edit your `~/.bashrc` file and add:

```
alias zzh="ssh -p 61822"
```

... then you can do:

```
$ zzh centos@master.s.gravitational.io
```

### Purpose of this

Have a place to quickly play. There aren't supposed to be any important data on those
instances. 

**this staging area must not be used for customer demos**

