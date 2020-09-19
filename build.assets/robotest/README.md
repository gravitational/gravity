# Robotest CI Configurations

This directory contains scripts to generate and execute recommended
suites of robotest integration tests.

* `run.sh` provides the glue between the top level Makefile, what robotest expects,
and the dynamically generated configurations.
* `config/` contain branch specific suite configurations. See the README.md therein.
* `utils.sh` contains functions common to both pull request (pr) and nightly
configurations.
* `test.sh` contains unit tests for the interesting logic in `utils.sh`

The files are structured this way for two major reasons:

* To consolidate expected differences between branches in `configs/`. This
reduces the chance of merge conflicts on changes that don't affect branch
specific code (e.g. upgrade configurations, or supported distros).
* To allow generation of test configuration, without downloading heavy artifacts
or running the tests. This provides a quick way for maintainers to validate
config changes without executing them (which can be an hours long process).
