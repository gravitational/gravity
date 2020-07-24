# Mage
This is a highly experimental build framework that writes the complicated build logic in golang. This allows to write
the build system in the same language as the project itself, take advantage of any libraries and share code between
projects, and more aggresively take advantage of caching and parralelization to get out of devs way.

Due to the complexity of the current Makefile I'm not confident I've fully replicated them in go, however, the outcome
of the build can be installed.

## Getting Started

### OSS

`go run mage.go cluster:gravity`

This is roughly equivelant to `make production telekube` within the OSS source directory. 

### Enterprise

`ENTERPRISE=true go run mage.go cluster:gravity`

This is roughly equivelant to `make production telekube` within the enterprise source directory. 

## Configuration
Mage doesn't support build arguments, so all customization of the build process is done via environment variables. For
a complete listing of environment variables, run the help:envs target. This will give a listing of all env variables
and any current settings.

`go run mage.go help:envs`

## Notes and Tips

1. There isn't really a good way to rebuild specific packages bypassing caching, besides changing the version that
would be assigned. So for example to force rebuild RBAC would require changing the version by env variable.

For Example: `RBAC_APP_TAG=7.0.0-rbac.1 go run mage.go cluster:gravity`

2. The automatic version assignment this uses is a bit different than the version.sh script currently being used. I'm 
not really sure what's better, but version.sh will generate something like `7.1.0-alpha.1.149` and this will use
`7.1.0-alpha.1-149-g86435b2c-dirty`. The downside is this can generate more packages to rebuild when getting marked as
dirty.

3. To better interact with gopls, dependent apps are no longer extracted to the build/ directory, and instead generate
temporary directory for extraction and build.

4. For now I've set caching to default to the build directory. So build/cache will be used for golang's caches, and 
build/apps get's used for internal applications. The cache directory can be overriden by `XDG_CACHE_HOME`.

5. To get a full list of targets run `go run mage.go -l`. In general the targets are organized as:
    1. build - Building golang assets
    2. cluster - Building cluster images
    3. help - Help
    4. package - System packages that will be shipped with the installer
    5. test - Tests such as running go tests, linting, etc.