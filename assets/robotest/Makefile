# This Makefile & related scripts exist to:
#
#  - Determine appropriate robotest configuration for the branch
#  - Fetch or build the necessary artifacts for robotest testing
#  - Gather necessary configuration & parameters robotest
#  - Using all the above, run robotest!
#
# With the advent of custom robotest images in upgrade testing (#1915)
# constructing all the necessary cluster images became complex. As such
# the logic has been quarantined in this file to reduce complexity in Gravity's
# Makefile, e/Makefile, and build.assets/Makefile.
#
# Robotest image construction uses the following strategy:
#
#  - For the release under test, build using TELE_OUT and PACKAGES_DIR from
#    the Gravity Makefiles.  This must occur after a successful `make telekube`
#    (which populates PACKAGES_DIR with a base image). This Makefile does not
#    know how to generate these and will fail if they're absent.
#
#  - For upgrade to/from versions:
#
#    1) Download & cache appropriate tele from our release infrastructure.
#    2) Build & cache the robotest image using the current application manifest
#       with the old tele. Build with a shared --state-dir caching released
#       packages locally.
#    3) Populate the current build directory from the image cache.
#
# Caching is critical because at the time of writing:
#
#  - Jenkins is pushing 20TB/mo of network data, primarily for robotest
#    image construction & distribution.
#  - Downloading one gravity base image is O(minutes). Multiply this by 3-10
#    different versions used in upgrade testing and the build times become tedious.
#
# Misc additional concerns:
#
#  - All CACHE population must be atomic (file renames on the same filesystem). Otherwise
#    a race between builds could result in a half copied artifact being used.
#  - CACHES should not be assumed to be on the same filesystem as the build directories.
#  - Enterprise tele and OSS tele cannot share caches, nor should their outputs
#    be confused.
#  - ROBOTEST_IMAGEDIR can be heavy (up to 30GB for 7.0.x). This can
#    aggravate CI workers that have many active builds or don't clean up.
TOP := $(realpath $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))))
ROOT := $(realpath $(TOP)/../..)

# The following variables are typically set by Gravity's top level or e
# Makefile. They're redeclared here for direct invocation of this Makefile.
BUILDDIR ?= $(ROOT)/build
GRAVITY_VERSION ?= $(shell cd $(ROOT) && ./version.sh)
GRAVITY_BUILDDIR ?= $(BUILDDIR)/$(GRAVITY_VERSION)
TELE_OUT ?= $(GRAVITY_BUILDDIR)/tele
GRAVITY_OUT ?= $(GRAVITY_BUILDDIR)/gravity
OPSCENTER_OUT ?= $(GRAVITY_BUILDDIR)/opscenter.tar
PACKAGES_DIR ?= $(GRAVITY_BUILDDIR)/packages
# ROBOTEST_DOWNLOAD_TELE_SCRIPT allows the e Makefile to override how tele binaries are sourced.
ROBOTEST_DOWNLOAD_TELE_SCRIPT ?= $(TOP)/download_tele.sh
# ROBOTEST_CACHE_FLAVOR may be overriden to specify a non-compatible cache.
ROBOTEST_CACHE_FLAVOR ?= oss
# ROBOTEST_CONFIG is the desired robotest configuration (pr, nightly)
ROBOTEST_CONFIG = pr
# ROBOTEST_CONFIG_SCRIPT is full path to the scrip that will generate robotest configurations
ROBOTEST_CONFIG_SCRIPT = $(TOP)/config/$(ROBOTEST_CONFIG).sh


# Robotest specific variables.
# These are local to the targets in this Makefile.

ROBOTEST_VERSION ?= 2.1.0
ROBOTEST_DOCKER_IMAGE ?= quay.io/gravitational/robotest-suite:$(ROBOTEST_VERSION)
# ROBOTEST_TELE_BUILD_DOCKER_IMAGE is an image used to sandbox tele builds. It could
# be almost anything.
ROBOTEST_TELE_BUILD_DOCKER_IMAGE ?= quay.io/gravitational/debian-tall:buster-20200916

# ROBOTEST_BUILDDIR is the root of all robotest build artifacts for this build
ROBOTEST_BUILDDIR ?= $(GRAVITY_BUILDDIR)/robotest
# ROBOTEST_STATEDIR is the directory where robotest will store terraform state and logs
ROBOTEST_STATEDIR ?= $(ROBOTEST_BUILDDIR)/state
# ROBOTEST_BUILD_TMP is a staging directory for building/copying/unpacking content
# # before an atomic rename into the build directory. Must be on the same filesystem
# # as ROBOTEST_BUILDDIR.
ROBOTEST_BUILD_TMP = $(ROBOTEST_BUILDDIR)/tmp

# Robotest cache configuration.

# ROBOTEST_CACHE_ROOT is a filesystem cache intended to be shared between parallel
# invocations of this Makefile against wildly disparate (5.5, 6.1, 7.0, 7.1+)
# gravity versions. Although this defaults to within the builddir, it is expected
# to be overridden (to avoid wipe on `make clean`).
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
# before an atomic rename into the. Must be on the same filesystem as ROBOTEST_TELE_CACHE
ROBOTEST_CACHE_TMP = $(ROBOTEST_CACHEDIR)/tmp
# ROBOTEST_TELE_CACHE is a filesytem cache storing prebuilt tele binaries for
# different gravity versions. ROBOTEST_TELE_CACHE may be shared between many
# parallel robotest builds in CI infra.
ROBOTEST_TELE_CACHE ?= $(ROBOTEST_CACHEDIR)/tele
# ROBOTEST_CACHES are all the directories on this machine used to speed up
# upgrade artifact contruction by avoiding redownloading/rebuilding released
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
# of these change, all robotest images must be rebuilt.
ROBOTEST_IMAGE_SRC := $(shell find $(ROBOTEST_IMAGE_SRCDIR) -type f)
# ROBOTEST_IMAGE_MANIFEST is the gravity application manifest to build into robotest images.
ROBOTEST_IMAGE_MANIFEST = $(ROBOTEST_IMAGE_SRCDIR)/app.yaml

# ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR is the directory containing all sources to build the
# robotest upgrade base application. It is mounted into a build container.
ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR = $(TOP)/upgrade-base
# ROBOTEST_UPGRADE_BASE_IMAGE_SRC is the gravity application manifest to build into robotest images.
ROBOTEST_UPGRADE_BASE_IMAGE_SRC := $(shell find $(ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR) -type f)
# ROBOTEST_UPGRADE_BASE_IMAGE_MANIFEST is the gravity application manifest to build into robotest images.
ROBOTEST_UPGRADE_BASE_IMAGE_MANIFEST = $(ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR)/app.yaml

# ROBOTEST_IMAGE is the robotest image for the gravity version checked out in the working copy.
ROBOTEST_IMAGE = $(ROBOTEST_IMAGEDIR)/robotest-$(GRAVITY_VERSION).tar
# ROBOTEST_OPSCENTER_IMAGE is robotest's copy of the opscenter image, used for install testing.
ROBOTEST_OPSCENTER_IMAGE = $(ROBOTEST_IMAGEDIR)/opscenter-$(GRAVITY_VERSION).tar
# ROBOTEST_UPGRADE_VERSIONS is a list of gravity semvers that will be used in upgrade testing.
ROBOTEST_UPGRADE_VERSIONS = $(shell cd $(TOP)/config && $(ROBOTEST_CONFIG_SCRIPT) upgradeversions)
# ROBOTEST_TELE_BINARIES is the list of build specific tele binaries needed to
# build all robotest upgrade images.
ROBOTEST_TELE_BINARIES = $(addprefix $(ROBOTEST_TELE_CACHE)/tele-, $(ROBOTEST_UPGRADE_VERSIONS))
# ROBOTEST_UPGRADE_IMAGES is the list of build specific upgrade images, suitable
# for mounting into the robotest docker container. These are not shared stated.
ROBOTEST_UPGRADE_IMAGES = $(addsuffix .tar, $(addprefix $(ROBOTEST_IMAGEDIR)/robotest-, $(ROBOTEST_UPGRADE_VERSIONS)))
# ROBOTEST_IMAGES is all build specific cluster images/targets needed to run robotest.
ROBOTEST_IMAGES = $(ROBOTEST_IMAGE) $(ROBOTEST_OPSCENTER_IMAGE) $(ROBOTEST_UPGRADE_IMAGES)

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
clean-caches: ## Remove cached tele binaries, gravity packages, and robotest upgrade images.
	rm -rf $(ROBOTEST_CACHES) $(ROBOTEST_CACHE_TMP)

$(ROBOTEST_BUILDDIR):
	mkdir -p $(ROBOTEST_BUILDDIR)

$(ROBOTEST_IMAGEDIR):
	mkdir -p $(ROBOTEST_IMAGEDIR)

$(ROBOTEST_TELE_CACHE):
	mkdir -p $(ROBOTEST_TELE_CACHE)

$(ROBOTEST_GRAVITY_PKG_CACHE):
	mkdir -p $(ROBOTEST_GRAVITY_PKG_CACHE)

$(ROBOTEST_BUILD_TMP):
	mkdir -p $(ROBOTEST_BUILD_TMP)

$(ROBOTEST_CACHE_TMP):
	mkdir -p $(ROBOTEST_CACHE_TMP)

.PHONY: images
images: ## Create all images necessary to run robotest CI tests.
images: image-robotest images-upgrade image-opscenter

.PHONY: image-opscenter
image-opscenter: ## Move opscenter.tar cluster image into place for test execution.
image-opscenter: $(ROBOTEST_OPSCENTER_IMAGE)

$(ROBOTEST_OPSCENTER_IMAGE): $(OPSCENTER_OUT) | $(ROBOTEST_IMAGEDIR)
	cp $(OPSCENTER_OUT) $(ROBOTEST_OPSCENTER_IMAGE)

.PHONY: image-robotest
image-robotest: ## Create robotest cluster image using the current tele.
image-robotest: $(ROBOTEST_IMAGE)

$(ROBOTEST_IMAGE): export TARGET = $(ROBOTEST_IMAGE)
$(ROBOTEST_IMAGE): export IMAGE = $(ROBOTEST_TELE_BUILD_DOCKER_IMAGE)
$(ROBOTEST_IMAGE): export BUILD_TMP = $(ROBOTEST_BUILD_TMP)
$(ROBOTEST_IMAGE): export APP_MANIFEST = $(ROBOTEST_IMAGE_MANIFEST)
$(ROBOTEST_IMAGE): export APP_SRCDIR = $(ROBOTEST_IMAGE_SRCDIR)
$(ROBOTEST_IMAGE): export VERSION = $(GRAVITY_VERSION)
$(ROBOTEST_IMAGE): export TELE = $(TELE_OUT)
$(ROBOTEST_IMAGE): export STATE_DIR = $(PACKAGES_DIR)
$(ROBOTEST_IMAGE): $(TELE_OUT) $(ROBOTEST_IMAGE_SRC) $(TOP)/tele_build.sh | $(ROBOTEST_IMAGEDIR) $(ROBOTEST_BUILD_TMP)
	$(TOP)/tele_build.sh

# When using a shared state dir 7.0.x package syncing is prone to races
# resulting in failures like:
#
#   [ERROR]: failed to sync packages for runtime version 7.0.16
#       package(gravitational.io/bandwagon:6.0.1) already exists
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
#  c) parallel builds with common seeded state dir (~2 mins)
# *d) serial builds with common unseeded state dir (5-7 mins)
#  e) serial builds with independent unseeded state dirs (didn't bother testing)
#  f) serial builds with common seeded state dir (3-4 mins)
#
# *   Recommended strategies
#
# Benchmarked on GCP e2-standard-8 with 30MB/s & 200 sustained random IOPS disks.
# Code at PR #2133, with 4 7.0.x upgrade base images.
#
# For now, I've implemented: d) serial builds with common unseeded state dir.  This has
# two nice properties:
#  1) The build output is linear and easy to read
#  2) Jenkins PR builds & local builds effectively have a semaphore, allowing f) seeded state
#     dir to kick in and bring the 2+ build time lower than b) parallel independent unseeded.
#
# Lastly, some test configs don't require upgrades, so the sub-make invocation can be skipped.
#
# - 2020-09 - walt
.PHONY: images-upgrade
images-upgrade: ## Create robotest cluster images needed for upgrade testing.
ifneq ($(ROBOTEST_TELE_BINARIES),)
	$(MAKE) $(ROBOTEST_UPGRADE_IMAGES)
endif

# The following rule builds all upgrade base images. It fulfills the $(ROBOTEST_UPGRADE_IMAGES) targets.
# .PRECIOUS because these need to be present through the run target.
.PRECIOUS: $(ROBOTEST_IMAGEDIR)/robotest-%.tar
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export TARGET = $@
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export IMAGE = $(ROBOTEST_TELE_BUILD_DOCKER_IMAGE)
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export BUILD_TMP = $(ROBOTEST_BUILD_TMP)
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export APP_MANIFEST = $(ROBOTEST_UPGRADE_BASE_IMAGE_MANIFEST)
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export APP_SRCDIR = $(ROBOTEST_UPGRADE_BASE_IMAGE_SRCDIR)
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export VERSION = $*
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export TELE = $<
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: export STATE_DIR = $(ROBOTEST_GRAVITY_PKG_CACHE)
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: $(ROBOTEST_TELE_CACHE)/tele-% $(ROBOTEST_UPGRADE_BASE_IMAGE_SRC) $(TOP)/tele_build.sh | $(ROBOTEST_IMAGEDIR) $(ROBOTEST_BUILD_TMP) $(ROBOTEST_GRAVITY_PKG_CACHE)
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
populate-caches: download-teles images-upgrade

.PHONY: get-upgrade-versions
get-upgrade-versions: ## Show Gravity versions used in upgrade testing.
	@echo $(ROBOTEST_UPGRADE_VERSIONS)

.PHONY: get-config
get-config: export INSTALLER_URL ?= /images/$(notdir $(ROBOTEST_IMAGE))
get-config: export OPSCENTER_URL ?= /images/$(notdir $(ROBOTEST_OPSCENTER_IMAGE))
get-config: export IMAGEDIR_MOUNTPOINT ?= /images
get-config: ## Show robotest configuration.
	$(ROBOTEST_CONFIG_SCRIPT) configuration | sed -e 's/^[[:space:]]*//' | tr ' ' '\n'

.PHONY: run
run: ## Run robotest.
run: export DOCKER_IMAGE = $(ROBOTEST_DOCKER_IMAGE)
run: export GRAVITY_URL = $(ROBOTEST_GRAVITY_BINARY)
run: export INSTALLER_URL ?= /images/$(notdir $(ROBOTEST_IMAGE))
run: export OPSCENTER_URL ?= /images/$(notdir $(ROBOTEST_OPSCENTER_IMAGE))
run: export IMAGEDIR = $(ROBOTEST_IMAGEDIR)
run: export STATEDIR ?= $(ROBOTEST_STATEDIR)
run: export SSH_KEY ?= $(ROBOTEST_SSH_KEY)
run: export SSH_PUB ?= $(ROBOTEST_SSH_PUB)
run: export GOOGLE_APPLICATION_CREDENTIALS ?= $(ROBOTEST_GOOGLE_APPLICATION_CREDENTIALS)
run: $(ROBOTEST_CONFIG_SCRIPT) images $(ROBOTEST_GRAVITY_BINARY)
	cd $(TOP) && ./run.sh $(ROBOTEST_CONFIG_SCRIPT)
