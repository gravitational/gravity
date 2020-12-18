# See README.md for usage & implementation notes.
TOP := $(realpath $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))))
ROOT := $(realpath $(TOP)/../..)

# The following variables are typically set by Gravity's top level or e
# Makefile. They're redeclared here for direct invocation of this Makefile.

BUILDDIR ?= $(ROOT)/build
# Use := instead of ?= for GRAVITY_VERSION to avoid running the same shell
# command repeatedly upon recursive expansion.
ifeq ($(origin GRAVITY_VERSION), undefined)
GRAVITY_VERSION := $(shell cd $(ROOT) && ./version.sh)
endif
GRAVITY_BUILDDIR ?= $(BUILDDIR)/$(GRAVITY_VERSION)
TELE_OUT ?= $(GRAVITY_BUILDDIR)/tele
GRAVITY_OUT ?= $(GRAVITY_BUILDDIR)/gravity
OPSCENTER_OUT ?= $(GRAVITY_BUILDDIR)/opscenter.tar
TELEKUBE_OUT ?= $(GRAVITY_BUILDDIR)/telekube.tar
PACKAGES_DIR ?= $(GRAVITY_BUILDDIR)/packages
# ROBOTEST_DOWNLOAD_TELE_SCRIPT allows the e Makefile to override how tele binaries are sourced.
ROBOTEST_DOWNLOAD_TELE_SCRIPT ?= $(TOP)/download_tele.sh
# ROBOTEST_CACHE_FLAVOR is a cache subdirectory. It may be overridden to specify a non-compatible
# (e.g. enterprise) cache.
ROBOTEST_CACHE_FLAVOR ?= oss
# ROBOTEST_CONFIG is the desired robotest configuration (pr, nightly)
ROBOTEST_CONFIG ?= pr
# ROBOTEST_CONFIG_SCRIPT is full path to the scrip that will generate robotest configurations
ROBOTEST_CONFIG_SCRIPT = $(TOP)/config/$(ROBOTEST_CONFIG).sh

# End variables expected to be set outside this Makefile.
# Everything below is Robotest specific.

ROBOTEST_VERSION ?= 2.2.2
ROBOTEST_DOCKER_IMAGE ?= quay.io/gravitational/robotest-suite:$(ROBOTEST_VERSION)

# ROBOTEST_BUILDDIR is the root of all robotest build artifacts for this build
ROBOTEST_BUILDDIR ?= $(GRAVITY_BUILDDIR)/robotest
# ROBOTEST_STATEDIR is the directory where robotest will store terraform state and logs
ROBOTEST_STATEDIR ?= $(ROBOTEST_BUILDDIR)/state
# ROBOTEST_BUILD_TMP is a staging directory for building/copying/unpacking content
# before an atomic rename into the ROBOTEST_IMAGEDIR. Both must be on the same filesystem.
ROBOTEST_BUILD_TMP = $(ROBOTEST_BUILDDIR)/tmp

# Robotest cache configuration.

# ROBOTEST_CACHE_ROOT is a filesystem cache intended to be used across subsequent
# invocations of this Makefile against wildly disparate (5.5, 6.1, 7.0, 7.1+)
# gravity versions. Although the cache root defaults to within the builddir, it
# is expected to be overridden (to avoid wipe on `make clean`).
ROBOTEST_CACHE_ROOT ?= $(BUILDDIR)/cache
# ROBOTEST_CACHEDIR is the flavor specific cache directory.
# If the cache is in the builddir, it is already enterprise/oss specific. If not
# we need to disambiguate between the two.
ifeq ($(BUILDDIR), $(realpath $(dir $(ROBOTEST_CACHE_ROOT))))
# ROBOTEST_CACHEDIR is the flavor specific cache directory.
ROBOTEST_CACHEDIR ?= $(ROBOTEST_CACHE_ROOT)
else
ROBOTEST_CACHEDIR ?= $(ROBOTEST_CACHE_ROOT)/$(ROBOTEST_CACHE_FLAVOR)
endif
# ROBOTEST_GRAVITY_PKG_CACHE is a gravity state dir storing packages
# downloaded from the default distribution infrastructure.
# ROBOTEST_GRAVITY_PKG_CACHE may be shared between many parallel robotest builds
# in CI infra. A reasonable override for personal use is sharing with:
#   $(HOME)/.gravity
ROBOTEST_GRAVITY_PKG_CACHE ?=  $(ROBOTEST_CACHEDIR)/gravity-state
# ROBOTEST_CACHE_TMP is a staging directory for building/copying/unpacking content
# before an atomic rename into ROBOTEST_TELE_CACHE. Must be on the same filesystem
# as ROBOTEST_TELE_CACHE.
ROBOTEST_CACHE_TMP = $(ROBOTEST_CACHEDIR)/tmp
# ROBOTEST_TELE_CACHE is a filesytem cache storing prebuilt tele binaries for
# different gravity versions. ROBOTEST_TELE_CACHE may be shared between many
# parallel robotest builds in CI infra.
ROBOTEST_TELE_CACHE ?= $(ROBOTEST_CACHEDIR)/tele
# ROBOTEST_CACHES are all the directories on this machine used to speed up
# upgrade artifact construction by avoiding redownloading/rebuilding released
# artifacts.
ROBOTEST_CACHES = $(ROBOTEST_GRAVITY_PKG_CACHE) $(ROBOTEST_TELE_CACHE)

# Image configuration.

# ROBOTEST_IMAGEDIR is a store of robotest images used only for the current
# robotest run. Expected to be populated on demand from the caches and mounted
# as a volume into the robotest container. This exists to prevent any
# concurrency issues stemming from using cached images directly (e.g. if the
# cache is edited/cleared during a test run).
ROBOTEST_IMAGEDIR = $(ROBOTEST_BUILDDIR)/images

# ROBOTEST_IMAGE_SRCDIR is the directory containing all sources to build the robotest application.
# It is mounted into a build container.
ROBOTEST_IMAGE_SRCDIR = $(TOP)/current
# ROBOTEST_IMAGE_SRC are the files that affect the robotest application.  If any
# of these change, the robotest image must be rebuilt.
ROBOTEST_IMAGE_SRC := $(shell find $(ROBOTEST_IMAGE_SRCDIR) -type f)
# ROBOTEST_IMAGE_MANIFEST is the gravity application manifest to build into robotest images.
ROBOTEST_IMAGE_MANIFEST = $(ROBOTEST_IMAGE_SRCDIR)/app.yaml

# ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR is the directory containing all sources to build the
# robotest upgrade base application. It is mounted into a build container.
ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR = $(TOP)/upgrade-base
# ROBOTEST_UPGRADE_BASE_IMAGE_SRC are the files that affect the robotest application.  If any
# of these change, all robotest upgrade base images must be rebuilt.
ROBOTEST_UPGRADE_BASE_IMAGE_SRC := $(shell find $(ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR) -type f)
# ROBOTEST_UPGRADE_BASE_IMAGE_MANIFEST is the gravity application manifest to build into robotest images.
ROBOTEST_UPGRADE_BASE_IMAGE_MANIFEST = $(ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR)/app.yaml

# ROBOTEST_IMAGE is the robotest image for the gravity version checked out in the working copy.
ROBOTEST_IMAGE = $(ROBOTEST_IMAGEDIR)/robotest-$(GRAVITY_VERSION).tar
# ROBOTEST_OPSCENTER_IMAGE is robotest's copy of the opscenter image, used for install testing.
ROBOTEST_OPSCENTER_IMAGE = $(ROBOTEST_IMAGEDIR)/opscenter-$(GRAVITY_VERSION).tar
# ROBOTEST_TELEKUBE_IMAGE is robotest's copy of the telekube image, used for install testing.
ROBOTEST_TELEKUBE_IMAGE = $(ROBOTEST_IMAGEDIR)/telekube-$(GRAVITY_VERSION).tar
# GRAVITY_VERSION_INTERMEDIATE is a gravity semver used for --upgrade-via testing
GRAVITY_VERSION_INTERMEDIATE := $(shell cd $(TOP)/config && $(ROBOTEST_CONFIG_SCRIPT) intermediateversion)
# ROBOTEST_IMAGE_INTERMEDIATE is a robotest image that includes an older intermediate runtime.
# Used to test upgrades from releases that are too old to support direct upgrade
ROBOTEST_IMAGE_INTERMEDIATE = $(ROBOTEST_IMAGEDIR)/robotest-$(GRAVITY_VERSION)-via-$(GRAVITY_VERSION_INTERMEDIATE).tar
# ROBOTEST_UPGRADE_VERSIONS is a list of gravity semvers that will be used in upgrade testing.
ROBOTEST_UPGRADE_VERSIONS := $(shell cd $(TOP)/config && $(ROBOTEST_CONFIG_SCRIPT) upgradeversions)
# ROBOTEST_TELE_BINARIES is the list of build specific tele binaries needed to
# build all robotest upgrade images.
ROBOTEST_TELE_BINARIES := $(addprefix $(ROBOTEST_TELE_CACHE)/tele-, $(ROBOTEST_UPGRADE_VERSIONS))
# ROBOTEST_UPGRADE_IMAGES is the list of build specific upgrade images, suitable
# for mounting into the robotest docker container. These are not shared state.
ROBOTEST_UPGRADE_IMAGES := $(addsuffix .tar, $(addprefix $(ROBOTEST_IMAGEDIR)/robotest-, $(ROBOTEST_UPGRADE_VERSIONS)))
# ROBOTEST_IMAGES is all build specific cluster images/targets needed to run robotest.
ROBOTEST_IMAGES = $(ROBOTEST_TELEKUBE_IMAGE) $(ROBOTEST_OPSCENTER_IMAGE) $(ROBOTEST_IMAGE) $(ROBOTEST_UPGRADE_IMAGES)

# Robotest run configuration.
# These are only used in the run target.

ROBOTEST_GRAVITY_BINARY ?= $(GRAVITY_OUT)
ROBOTEST_SSH_KEY ?= $(HOME)/.ssh/id_rsa
ROBOTEST_SSH_PUB ?= $(HOME)/.ssh/id_rsa.pub
ROBOTEST_GOOGLE_APPLICATION_CREDENTIALS ?= $(TOP)/.gcp-credentials.json

# kudos to https://gist.github.com/prwhite/8168133 for inspiration
.PHONY: help
help: ## Show this message.
	@echo 'Usage: make [options] [target] ...'
	@echo
	@echo 'Options: run `make --help` for options'
	@echo
	@echo 'Targets:'
	@egrep --no-filename '^(.+)\:\ ##\ (.+)' ${MAKEFILE_LIST} | column -t -c 2 -s ':#' | sort | sed 's/^/  /'

.PHONY: clean
clean: ## Remove build artifacts.
	rm -rf $(ROBOTEST_BUILDDIR)

.PHONY: clean-caches
clean-caches: ## Remove cached tele binaries and gravity packages.
	rm -rf $(ROBOTEST_CACHES) $(ROBOTEST_CACHE_TMP)

$(ROBOTEST_BUILDDIR):
	mkdir -p $(ROBOTEST_BUILDDIR)

$(ROBOTEST_IMAGEDIR):
	mkdir -p $(ROBOTEST_IMAGEDIR)

$(ROBOTEST_TELE_CACHE):
	mkdir -p $(ROBOTEST_TELE_CACHE)

$(ROBOTEST_GRAVITY_PKG_CACHE):
	mkdir -p $(ROBOTEST_GRAVITY_PKG_CACHE)

# .PRECIOUS because this is a pattern rule. As such these caches would otherwise be marked intermediate
# and make would try to delete them.
.PRECIOUS: $(ROBOTEST_GRAVITY_PKG_CACHE)/%
$(ROBOTEST_GRAVITY_PKG_CACHE)/%:
	mkdir -p $@

$(ROBOTEST_BUILD_TMP):
	mkdir -p $(ROBOTEST_BUILD_TMP)

$(ROBOTEST_CACHE_TMP):
	mkdir -p $(ROBOTEST_CACHE_TMP)

.PHONY: images
images: ## Build all images necessary to run robotest CI tests.
images: image-telekube image-opscenter image-robotest image-robotest-intermediate image-robotest-upgrade

.PHONY: image-telekube
image-telekube: ## Move telekube.tar cluster image into place for test execution.
image-telekube: $(ROBOTEST_TELEKUBE_IMAGE)

$(ROBOTEST_TELEKUBE_IMAGE): $(TELEKUBE_OUT) | $(ROBOTEST_IMAGEDIR)
	cp $(TELEKUBE_OUT) $(ROBOTEST_TELEKUBE_IMAGE)

.PHONY: image-opscenter
image-opscenter: ## Move opscenter.tar cluster image into place for test execution.
image-opscenter: $(ROBOTEST_OPSCENTER_IMAGE)

$(ROBOTEST_OPSCENTER_IMAGE): $(OPSCENTER_OUT) | $(ROBOTEST_IMAGEDIR)
	cp $(OPSCENTER_OUT) $(ROBOTEST_OPSCENTER_IMAGE)

.PHONY: image-robotest
image-robotest: ## Build robotest cluster image using the current tele.
image-robotest: $(ROBOTEST_IMAGE)

$(ROBOTEST_IMAGE): export TARGET = $(ROBOTEST_IMAGE)
$(ROBOTEST_IMAGE): export BUILD_TMP = $(ROBOTEST_BUILD_TMP)
$(ROBOTEST_IMAGE): export APP_MANIFEST = $(ROBOTEST_IMAGE_MANIFEST)
$(ROBOTEST_IMAGE): export VERSION = $(GRAVITY_VERSION)
$(ROBOTEST_IMAGE): export TELE = $(TELE_OUT)
$(ROBOTEST_IMAGE): export STATE_DIR = $(PACKAGES_DIR)
$(ROBOTEST_IMAGE): $(TELE_OUT) $(ROBOTEST_IMAGE_SRC) $(TOP)/tele_build.sh | $(ROBOTEST_IMAGEDIR) $(ROBOTEST_BUILD_TMP)
	$(TOP)/tele_build.sh

.PHONY: image-robotest-intermediate
image-robotest-intermediate: ## Build robotest cluster image with a prior LTS baked in through --upgrade-via.
image-robotest-intermediate: $(ROBOTEST_IMAGE_INTERMEDIATE)

$(ROBOTEST_IMAGE_INTERMEDIATE): export EXTRA_TELE_BUILD_OPTIONS = --upgrade-via=$(GRAVITY_VERSION_INTERMEDIATE)
$(ROBOTEST_IMAGE_INTERMEDIATE): export TARGET = $(ROBOTEST_IMAGE_INTERMEDIATE)
$(ROBOTEST_IMAGE_INTERMEDIATE): export IMAGE = $(TELE_BUILD_DOCKER_IMAGE)
$(ROBOTEST_IMAGE_INTERMEDIATE): export BUILD_TMP = $(ROBOTEST_BUILD_TMP)
$(ROBOTEST_IMAGE_INTERMEDIATE): export APP_MANIFEST = $(ROBOTEST_IMAGE_MANIFEST)
$(ROBOTEST_IMAGE_INTERMEDIATE): export APP_SRCDIR = $(ROBOTEST_IMAGE_SRCDIR)
$(ROBOTEST_IMAGE_INTERMEDIATE): export VERSION = $(GRAVITY_VERSION)
$(ROBOTEST_IMAGE_INTERMEDIATE): export TELE = $(TELE_OUT)
$(ROBOTEST_IMAGE_INTERMEDIATE): export STATE_DIR = $(PACKAGES_DIR)
$(ROBOTEST_IMAGE_INTERMEDIATE): $(TELE_OUT) $(ROBOTEST_IMAGE_SRC) $(TOP)/tele_build.sh $(TELE_BUILD_DOCKER_IIDFILE) | $(ROBOTEST_IMAGEDIR) $(ROBOTEST_BUILD_TMP)
	$(TOP)/tele_build.sh

# When using a shared state dir 7.0.x package syncing is prone to races
# resulting in failures like:
#
#   [ERROR]: failed to sync packages for runtime version 7.0.16
#       package(gravitational.io/bandwagon:6.0.1) already exists
#
# Possibly due to https://github.com/gravitational/gravity/issues/2060
#
# Because we must support old tele binaries to build old image versions, these races seem
# unavoidable.
#
# Interestingly, I've never seen sync races happen with oss tele and the s3Syncer
# I've only observed them with enterprise teles syncing from the distribution opscenter.
#
# Seeded state dir package caches eliminate the race but can only happen if:
#  - We're on build 2+ with a semaphore on concurrent builds, or
#  - We split state dir package cache population from `tele build` (and do so in a non-racy way)
#
# Benchmarks:
#  a) parallel builds with common unseeded state dirs (races & fails often)
# *b) parallel builds with independent unseeded state dirs (4-5 mins)
#  c) parallel builds with common seeded state dir (~2 mins, determinism suffers, see #2229)
# *d) parallel builds with independent seeded state dirs (~2 mins)
# *e) serial builds with common unseeded state dir (5-7 mins)
#  f) serial builds with independent unseeded state dirs (didn't bother testing)
#  g) serial builds with common seeded state dir (3-4 mins)
#
# *   Recommended strategies
#
# Benchmarked on GCP e2-standard-8 with 30MB/s & 200 sustained random IOPS disks.
# Code at PR #2133, with 4 7.0.x upgrade base images.
#
# I've implemented: d) parallel builds with independent seeded state dirs. This is fast and
# benefits from caching on subsequent runs but the parallel build output can get confusing.
#
# Lastly, some test configs don't require upgrades, so the sub-make invocation can be skipped.
#
# - 2020-09 - walt
.PHONY: image-robotest-upgrade
image-robotest-upgrade: ## Build robotest cluster images needed for upgrade testing.
ifneq ($(ROBOTEST_TELE_BINARIES),)
	$(MAKE) -j $(ROBOTEST_UPGRADE_IMAGES)
endif

# The following rule builds all upgrade base images. It fulfills the $(ROBOTEST_UPGRADE_IMAGES) targets.
# .PRECIOUS because these need to be present through the run target.
.PRECIOUS: $(ROBOTEST_IMAGEDIR)/robotest-%.tar
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export TARGET = $@
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export BUILD_TMP = $(ROBOTEST_BUILD_TMP)
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export APP_MANIFEST = $(ROBOTEST_UPGRADE_BASE_IMAGE_MANIFEST)
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export VERSION = $*
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export TELE = $<
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export STATE_DIR = $(ROBOTEST_GRAVITY_PKG_CACHE)/$*
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: $(ROBOTEST_TELE_CACHE)/tele-% $(ROBOTEST_UPGRADE_BASE_IMAGE_SRC) $(TOP)/tele_build.sh | $(ROBOTEST_IMAGEDIR) $(ROBOTEST_BUILD_TMP) $(ROBOTEST_GRAVITY_PKG_CACHE)/%
	$(TOP)/tele_build.sh

# The following target is a pattern rule for populating the tele binary cache.
# Fulfills the $(ROBOTEST_TELE_BINARIES) targets.
# .PRECIOUS because it is a cache shared across branches/builds.
.PRECIOUS: $(ROBOTEST_TELE_CACHE)/tele-%
$(ROBOTEST_TELE_CACHE)/tele-%: export TARGET = $@
$(ROBOTEST_TELE_CACHE)/tele-%: export VERSION = $*
$(ROBOTEST_TELE_CACHE)/tele-%: export BUILD_TMP = $(ROBOTEST_CACHE_TMP)
$(ROBOTEST_TELE_CACHE)/tele-%: $(ROBOTEST_DOWNLOAD_TELE_SCRIPT) | $(ROBOTEST_CACHE_TMP) $(ROBOTEST_TELE_CACHE)
	$(ROBOTEST_DOWNLOAD_TELE_SCRIPT)

.PHONY: download-teles
download-teles: ## Download tele binaries needed to build robotest images.
ifneq ($(ROBOTEST_TELE_BINARIES),)
	$(MAKE) -j $(ROBOTEST_TELE_BINARIES)
endif

.PHONY: populate-caches
populate-caches: ## Populate caches with tele binaries and gravity packages.
populate-caches: download-teles image-robotest-upgrade

.PHONY: get-upgrade-versions
get-upgrade-versions: ## Show Gravity versions used in upgrade testing.
	@echo $(ROBOTEST_UPGRADE_VERSIONS)

.PHONY: get-config
get-config: export IMAGEDIR_MOUNTPOINT ?= /images
get-config: export INSTALLER_URL ?= /images/$(notdir $(ROBOTEST_IMAGE))
get-config: export OPSCENTER_URL ?= /images/$(notdir $(ROBOTEST_OPSCENTER_IMAGE))
get-config: export TELEKUBE_URL  ?= /images/$(notdir $(ROBOTEST_TELEKUBE_IMAGE))
get-config: export INTERMEDIATE_INSTALLER_URL ?= /images/$(notdir $(ROBOTEST_IMAGE_INTERMEDIATE))
get-config: ## Show robotest configuration.
	@cd $(TOP) && $(ROBOTEST_CONFIG_SCRIPT) configuration | sed -e 's/^[[:space:]]*//' | tr ' ' '\n'

.PHONY: run
run: ## Run robotest.
run: export DOCKER_IMAGE = $(ROBOTEST_DOCKER_IMAGE)
run: export GRAVITY_URL = $(ROBOTEST_GRAVITY_BINARY)
run: export IMAGEDIR_MOUNTPOINT ?= /images
run: export INSTALLER_URL ?= /images/$(notdir $(ROBOTEST_IMAGE))
run: export OPSCENTER_URL ?= /images/$(notdir $(ROBOTEST_OPSCENTER_IMAGE))
run: export TELEKUBE_URL  ?= /images/$(notdir $(ROBOTEST_TELEKUBE_IMAGE))
run: export INTERMEDIATE_INSTALLER_URL ?= /images/$(notdir $(ROBOTEST_IMAGE_INTERMEDIATE))
run: export IMAGEDIR = $(ROBOTEST_IMAGEDIR)
run: export STATEDIR ?= $(ROBOTEST_STATEDIR)
run: export SSH_KEY ?= $(ROBOTEST_SSH_KEY)
run: export SSH_PUB ?= $(ROBOTEST_SSH_PUB)
run: export GOOGLE_APPLICATION_CREDENTIALS ?= $(ROBOTEST_GOOGLE_APPLICATION_CREDENTIALS)
run: $(ROBOTEST_CONFIG_SCRIPT) images $(ROBOTEST_GRAVITY_BINARY)
	cd $(TOP) && ./run.sh $(ROBOTEST_CONFIG_SCRIPT)
