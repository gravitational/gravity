# Robotest CI Configurations

This directory contains scripts to generate and execute recommended suites of
[robotest](https://github.com/gravitational/robotest) integration tests. To get
started, find a list targets from the Makefile in this directory:

```
make -C assets/robotest help
```

## Developing & Running Locally

Although the primary use of robotest is via automated CI runs, it is possible
to generate configurations, build images, and run the suites from a development machine.

### Building Images

Before robotest images can be built, Gravity, must be built. From the root of
the Gravity repo, run:

```
make production telekube opscenter
```

Once this completes, build robotest images with:

```
make -C assets/robotest images
```

This may take a while. When complete `build/<gravity>/robotest/images` will be
populated with cluster images needed to run robotest.

### Running Robotest

Before runnning robotest, set the following environment variables needed to
provision & connect to infrastructure.

```bash
# ssh keys must not have a password. Generating unique dedicated keys is recommended.
export ROBOTEST_SSH_KEY=~/.ssh/robotest
export ROBOTEST_SSH_PUB=~/.ssh/robotest.pub
# See https://cloud.google.com/docs/authentication/production
export ROBOTEST_GOOGLE_APPLICATION_CREDENTIALS=~/.config/gcloud/legacy_credentials/robotest.json
```

Once these are set, robotest with:

```
make -C assets/robotest run
```

## Tips & Tricks

Building robotest images and running robotest is time consuming, 10-20 of minutes
for a clean build and an hour for a build & PR run. To improve the development process
try the following:

### Using Alternative Configurations

By default, all targets use the `pr` configuration found in the `config/`
directory. A `noop` configuration this does not rely on any upgrade images is
provided to quickly test robotest build & execution.  To use alternative configs:

```
make -C assets/robotest ROBOTEST_CONFIG=noop run
```

For example:

```
$ make -C assets/robotest ROBOTEST_CONFIG=noop get-config
noop={"fail":false,"sleep":3}
$ make -C assets/robotest ROBOTEST_CONFIG=pr get-config
install={"flavor":"three","nodes":3,"role":"node","os":"centos:7","storage_driver":"overlay2"}
install={"installer_url":"/images/opscenter-7.1.0-alpha.1.233.tar","nodes":1,"flavor":"standalone","role":"node","os":"ubuntu:18","ops_advertise_addr":"example.com:443"}
resize={"to":3,"flavor":"one","nodes":1,"role":"node","state_dir":"/var/lib/telekube","os":"ubuntu:18","storage_driver":"overlay2"}
shrink={"nodes":3,"flavor":"three","role":"node","os":"redhat:7"}
upgrade={"flavor":"three","nodes":3,"role":"node","os":"ubuntu:16","from":"/images/robotest-7.0.7.tar","installer_url":"/images/robotest-7.1.0-alpha.1.233.tar","service_uid":997,"service_gid":994,"storage_driver":"overlay2"}
upgrade={"flavor":"three","nodes":3,"role":"node","os":"centos:7","from":"/images/robotest-7.0.13.tar","installer_url":"/images/robotest-7.1.0-alpha.1.233.tar","service_uid":997,"service_gid":994,"storage_driver":"overlay2"}
upgrade={"flavor":"three","nodes":3,"role":"node","os":"ubuntu:18","from":"/images/robotest-7.0.12.tar","installer_url":"/images/robotest-7.1.0-alpha.1.233.tar","service_uid":997,"service_gid":994,"storage_driver":"overlay2"}
upgrade={"flavor":"three","nodes":3,"role":"node","os":"centos:7","from":"/images/robotest-7.0.16.tar","installer_url":"/images/robotest-7.1.0-alpha.1.233.tar","service_uid":997,"service_gid":994,"storage_driver":"overlay2"}
```

### Local Caching

When building locally, keep robotests build cache outside the build directory such
that `tele` binaries and packages are not deleted by `make clean` and can be shared
across subsequent builds:

```bash
export ROBOTEST_CACHE_ROOT=~/.cache/robotest
```

This will considerably speed up subsequent upgrade image builds.

### Recycling Gravity Builds

While iterating on robotest changes, frequently it is unnecessary to rebuild Gravity
binaries and packages every time a commit is made. To reuse the last Gravity build
(including `tele`, `gravity`, and the package cache):

```
make -C assets/robotest GRAVITY_BUILDDIR=build/current images
```

## File Structure

 * `Makefile` is the entrypoint. Run `make help` to see targets.
 * `config/` contain branch specific suite configurations. See the README.md therein.
 * `current/` includes the version of the robotest cluster image to be used
   with the gravity version at this commit.
 * `upgrade-base/` includes an "older" version of the robotest cluster image
   compatible with gravity versions as far back as we upgrade test from.

The files are structured this way for two major reasons:

 * To consolidate expected differences between branches in `configs/`. This
   reduces the chance of merge conflicts on changes that don't affect branch
   specific code (e.g. upgrade configurations, or supported distros).
 * To allow generation of test configuration, without downloading heavy artifacts
   or running the tests. This provides a quick way for maintainers to validate
   config changes without executing them (which can be an hour long process). To
   check configs, use:

```
make -C assets/robotest ROBOTEST_CONFIG=nightly get-config
```

## Implementation Notes

[#1908](https://github.com/gravitational/gravity/issues/1908) and
[#1915](https://github.com/gravitational/gravity/issues/1915) introduced
custom robotest images, in addition to exercising the raw artifacts
generated by the Gravity build. These images allow robotest to test customizable
Gravity features that are important to our users.

Robotest image construction uses the following strategy:

For the release under test, build using `TELE_OUT` and `PACKAGES_DIR` from
the Gravity Makefiles.  This *must* occur after a successful `make telekube`
(which populates `PACKAGES_DIR` with a base image).

For upgrade to/from versions:

 1. Download & cache appropriate tele from the gravity release infrastructure.
 2. Build the the robotest image using the `upgrade-base` manifest
    with the downloaded tele.

Caching is critical because (as of 2020-09):

 - Jenkins is pushing 20TB/mo of network data, primarily for robotest
   image construction & distribution.
 - Downloading one gravity base image is O(minutes). Multiply this by 3-10
   different versions used in upgrade testing and the build times become tedious.

Misc additional concerns:

 - All shared cache population must be atomic (file renames on the same filesystem).
   Otherwise a race between builds could result in a half copied artifact being used.
 - Caches should not be assumed to be on the same filesystem as the build directories.
 - Enterprise tele and OSS tele cannot share caches, nor should their outputs
   be confused.
 - ROBOTEST_IMAGEDIR can be heavy (up to 30GB for 7.0.x). This can
   aggravate CI workers that have many active builds or don't clean up.
