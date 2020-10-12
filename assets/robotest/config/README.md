# Robotest Branch Specific Test Configurations

This directory contains scripts that generate robotest configurations
specific to the branch. Each script must accept a single parameter that is
either:

 * upgradeversions
 * intermediateversion
 * configuration

`./script.sh upgradeversions` must return a list of gravity versions the
configuration depends on.  For instance:

```
$ ./script.sh upgradeversions
6.1.38 7.0.7 7.0.12 7.0.16
```

`./script.sh intermediateversion` must return a single gravity LTS release
that can upgrade to the current branch and serve as an intermediate hop for
upgrading for older versions. See the 
[intermediate upgrade design document](https://docs.google.com/document/d/1Dv87BQMY__MGJYy45eG5aRZQXbZRtVcmLPXo7Vseuz0/edit)
For instance:

```
$ ./script.sh intermediateversion
6.1.38
```

`6.1.38` could be used to upgrade to 7.0.x from 5.5.x.

`./script.sh configuration` will have several environment variables set to
allow the test configuration to refer to installer tars:
 * OPSCENTER_URL
 * INSTALLER_URL

`./script.sh configuration` must return a robotest suite configuration as described in
https://github.com/gravitational/robotest/blob/v2.1.0/suite/README.md.  For instance:

```
$ ./script.sh configuration
install={"flavor":"three","nodes":3,"role":"node","os":"centos:7","storage_driver":"overlay2"}
install={"installer_url":"/images/opscenter-7.1.0-alpha.1.234.tar","nodes":1,"flavor":"standalone","role":"node","os":"ubuntu:18","ops_advertise_addr":"example.com:443"}
resize={"to":3,"flavor":"one","nodes":1,"role":"node","state_dir":"/var/lib/telekube","os":"ubuntu:18","storage_driver":"overlay2"}
shrink={"nodes":3,"flavor":"three","role":"node","os":"redhat:7"}
upgrade={"flavor":"three","nodes":3,"role":"node","os":"ubuntu:16","from":"/images/robotest-7.0.7.tar","installer_url":"/images/robotest-7.1.0-alpha.1.234.tar","service_uid":997,"service_gid":994,"storage_driver":"overlay2"}
upgrade={"flavor":"three","nodes":3,"role":"node","os":"centos:7","from":"/images/robotest-7.0.13.tar","installer_url":"/images/robotest-7.1.0-alpha.1.234.tar","service_uid":997,"service_gid":994,"storage_driver":"overlay2"}
upgrade={"flavor":"three","nodes":3,"role":"node","os":"ubuntu:18","from":"/images/robotest-7.0.12.tar","installer_url":"/images/robotest-7.1.0-alpha.1.234.tar","service_uid":997,"service_gid":994,"storage_driver":"overlay2"}
upgrade={"flavor":"three","nodes":3,"role":"node","os":"centos:7","from":"/images/robotest-7.0.16.tar","installer_url":"/images/robotest-7.1.0-alpha.1.234.tar","service_uid":997,"service_gid":994,"storage_driver":"overlay2"}
```

These separate functionalities are consolidated in one file for ease of
maintenance.

Common functionality found in the `lib/` subdirectory.
