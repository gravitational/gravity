# Robotest Branch Specific Test Configurations

This directory contains shell scripts that generate robotest configurations
specific to the branch. Each script must accept a single parameter that is
either:

 * upgradeversions
 * configuration

`./script.sh upgradeversions` must return a list of gravity versions the
configuration depends on.  For instance:

```
$ ./script.sh upgradeversions
6.1.38 7.0.7 7.0.12 7.0.16
```


`./script.sh configuration` will have several environment variables set to
allow the test configuration to refer to installer tars:
 * OPSCENTER_URL
 * INSTALLER_URL

`./script.sh configuration` must return a robotest suite configuration as described in
https://github.com/gravitational/robotest/blob/master/suite/README.md.  For instance:

```
$ ./script.sh configuration
```
