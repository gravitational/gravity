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
#  - If any content in ROBOTEST_IMG_SRC changes, old images in ROBOTEST_UPGRADE_IMAGE_CACHE
#    must be ruled out. These shouldn't necessarily be removed if the cache is
#    shared between branches, as other branches may use the older images. In this
#    scenario, ROBOTEST_TELE_CACHE and ROBOTEST_GRAVITY_PKG_CACHE remain valid.
#  - ROBOTEST_IMAGEDIR can be heavy (up to 30GB for 7.0.x). This can
#    aggravate CI workers that have many active builds or don't clean up.
TOP := $(realpath $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))))
ROOT := $(realpath $(TOP)/../..)

# The following variables are typically set by Gravity's top level or e
# Makefile. They're redeclared here for direct invocation of this Makefile.
ASSETSDIR ?= $(ROOT)/assets
BUILDDIR ?= $(ROOT)/build
GRAVITY_VERSION := $(shell cd ../.. && ./version.sh)
GRAVITY_BUILDDIR ?= $(BUILDDIR)/$(GRAVITY_VERSION)
TELE_OUT ?= $(GRAVITY_BUILDDIR)/tele
GRAVITY_OUT ?= $(GRAVITY_BUILDDIR)/gravity
OPSCENTER_OUT ?= $(GRAVITY_BUILDDIR)/opscenter.tar
PACKAGES_DIR ?= $(GRAVITY_BUILDDIR)/packages


# Robotest specific variables.
# These are consumed by the targets in this Makefile.

ROBOTEST_VERSION ?= 2.1.0
ROBOTEST_DOCKER_UPSTREAM_IMAGE ?= quay.io/gravitational/robotest-suite:$(ROBOTEST_VERSION)
# ROBOTEST_DOCKER_IMAGE has the local UID & GID injected into it, to avoid
# permissions issues with the logs.
ROBOTEST_DOCKER_IMAGE ?= robotest-suite:$(ROBOTEST_VERSION)-local

# ROBOTEST_BUILDDIR is the root of all robotest build artifacts for this build
ROBOTEST_BUILDDIR ?= $(GRAVITY_BUILDDIR)/robotest
# ROBOTEST_DOCKER_IMAGE_IIDFILE tracks if the local image has been built
ROBOTEST_DOCKER_IMAGE_IIDFILE = $(ROBOTEST_BUILDDIR)/docker-image.iid
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
# we need to disambiguate between the two
ifeq ($(BUILDDIR), $(realpath $(dir $(ROBOTEST_CACHE_ROOT))))
# ROBOTEST_CACHEDIR is the flavor specific cache directory.
ROBOTEST_CACHEDIR ?= $(ROBOTEST_CACHE_ROOT)
else
# ROBOTEST_CACHE_FLAVOR keeps cached enterprise teles, packages, tarballs seperate
# from oss caches. It is underneath ROBOTEST_CACHE_ROOT disambiguate caches when
# outside the build directory.
ifdef ENTERPRISE
ROBOTEST_CACHE_FLAVOR ?= ent
else
ROBOTEST_CACHE_FLAVOR ?= oss
endif
ROBOTEST_CACHEDIR ?= $(ROBOTEST_CACHE_ROOT)/$(ROBOTEST_CACHE_FLAVOR)
endif
# ROBOTEST_GRAVITY_PKG_CACHE is a gravity state dir storing packages
# downloaded from the default distribution infrastructure.
# ROBOTEST_GRAVITY_PKG_CACHE may be shared between many parallel robotest builds
# in CI infra. A reasonable override for personal use is sharing with:
#   $(HOME)/.gravity
ROBOTEST_GRAVITY_PKG_CACHE ?=  $(ROBOTEST_CACHEDIR)/gravity-state
# ROBOTEST_CACHE_TMP is a staging directory for building/copying/unpacking content
# before an atomic rename into the. Must be on the same filesystem
# as ROBOTEST_TELE_CACHE and ROBOTEST_UPGRADE_IMAGE_CACHE
ROBOTEST_CACHE_TMP = $(ROBOTEST_CACHEDIR)/tmp
# ROBOTEST_TELE_CACHE is a filesytem cache storing prebuilt tele binaries for
# different gravity versions. ROBOTEST_TELE_CACHE may be shared between many
# parallel robotest builds in CI infra.
ROBOTEST_TELE_CACHE ?= $(ROBOTEST_CACHEDIR)/tele
# ROBOTEST_UPGRADE_IMAGE_CACHE is a filesytem cache storing prebuilt robotest
# images for different gravity versions. ROBOTEST_UPGRADE_IMAGE_CACHE may be
# shared between many parallel robotest invocations in production infra)
ROBOTEST_UPGRADE_IMAGE_CACHE = $(ROBOTEST_CACHEDIR)/upgrade-images
# ROBOTEST_CACHES are all the directories on this machine used to speed up
# upgrade artifact contruction by avoiding redownloading/rebuilding released
# artifacts.
ROBOTEST_CACHES = $(ROBOTEST_GRAVITY_PKG_CACHE) $(ROBOTEST_TELE_CACHE) $(ROBOTEST_UPGRADE_IMAGE_CACHE)

# Image configuration.

# ROBOTEST_IMAGEDIR is a store of robotest images used only for the current
# robotest run. Expected to be populated on demand from the caches and mounted
# as a volume into the robotest container. This exists to prevent any
# concurrency issues stemming from using cached images directly (e.g. if the
# cache is edited/cleared during a test run).
ROBOTEST_IMAGEDIR = $(ROBOTEST_BUILDDIR)/images
# ROBOTEST_IMAGE_SRC are the files that affect the robotest application.  If any
# of these change, all robotest images must be rebuilt.
ROBOTEST_IMAGE_SRC := $(shell find $(ASSETSDIR)/robotest/current/ -type f)
# ROBOTEST_APP_YAML is the gravity application manifest to build into robotest images.
ROBOTEST_APP_YAML = $(ASSETSDIR)/robotest/current/app.yaml
# ROBOTEST_APP_YAML is the gravity application manifest to build into robotest images.
ROBOTEST_UPGRADE_BASE_IMAGE_SRC := $(shell find $(ASSETSDIR)/robotest/upgrade-base/ -type f)
# ROBOTEST_APP_YAML is the gravity application manifest to build into robotest images.
ROBOTEST_UPGRADE_BASE_APP_YAML = $(ASSETSDIR)/robotest/upgrade-base/app.yaml
# ROBOTEST_IMAGE is the robotest image for the gravity version
# checked out in the working copy.
ROBOTEST_IMAGE = $(ROBOTEST_IMAGEDIR)/robotest-$(GRAVITY_VERSION).tar
# ROBOTEST_OPSCENTER_IMAGE is robotest's copy of the opscenter image, used for install testing.
ROBOTEST_OPSCENTER_IMAGE = $(ROBOTEST_IMAGEDIR)/opscenter-$(GRAVITY_VERSION).tar
# ROBOTEST_TEST_CONFIG is the desired robotest configuration (pr, nightly)
ROBOTEST_TEST_CONFIG = $(TOP)/pr_config.sh
# ROBOTEST_UPGRADE_VERSIONS is a list of gravity semvers that will be
# used in upgrade testing.
ROBOTEST_UPGRADE_VERSIONS = $(shell ./scrape_upgrade_from_versions.sh $(ROBOTEST_TEST_CONFIG))
# ROBOTEST_UPGRADE_CACHE_IMAGES is a list of potentially cached robotest tarballs
# to be used in upgrade testing.
ROBOTEST_UPGRADE_CACHE_IMAGES = $(addsuffix .tar, $(addprefix $(ROBOTEST_UPGRADE_IMAGE_CACHE)/robotest-, $(ROBOTEST_UPGRADE_VERSIONS)))
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
ROBOTEST_GOOGLE_APPLICATION_CREDENTIALS ?= $(ROOT)/robotest-gcp-credentials.json

.PHONY: help
# kudos to https://gist.github.com/prwhite/8168133 for inspiration
help: ## Show this message.
	@echo 'Usage: make [options] [target] ...'
	@echo
	@echo 'Options: run `make --help` for options'
	@echo
	@echo 'Targets:'
	@egrep '^(.+)\:\ ##\ (.+)' ${MAKEFILE_LIST} | column -t -c 2 -s ':#' | sort | sed 's/^/  /'

.PHONY: clean
clean: ## Remove build artifacts.
	rm -rf $(ROBOTEST_BUILDDIR)

.PHONY: clean-caches
clean-caches: ## Remove cached tele binaries, gravity packages, and robotest upgrade images.
	rm -rf $(ROBOTEST_CACHES) $(ROBOTEST_CACHE_TMP)

$(ROBOTEST_BUILDDIR):
	mkdir -p $(ROBOTEST_BUILDDIR)

$(ROBOTEST_UPGRADE_IMAGE_CACHE):
	mkdir -p $(ROBOTEST_UPGRADE_IMAGE_CACHE)

$(ROBOTEST_IMAGEDIR):
	mkdir -p $(ROBOTEST_IMAGEDIR)

$(ROBOTEST_TELE_CACHE):
	mkdir -p $(ROBOTEST_TELE_CACHE)

$(ROBOTEST_BUILD_TMP):
	mkdir -p $(ROBOTEST_BUILD_TMP)

$(ROBOTEST_CACHE_TMP):
	mkdir -p $(ROBOTEST_CACHE_TMP)

.PHONY:docker-image
docker-image: ## Create robotest docker image with the current user/group baked in.
docker-image: $(ROBOTEST_DOCKER_IMAGE_IIDFILE)

$(ROBOTEST_DOCKER_IMAGE_IIDFILE): Dockerfile Makefile
	docker build \
		--build-arg BASE=$(ROBOTEST_DOCKER_UPSTREAM_IMAGE) \
		--build-arg UID=$(shell id -u) \
		--build-arg GID=$(shell id -g) \
		--iidfile $(ROBOTEST_DOCKER_IMAGE_IIDFILE) \
		--tag $(ROBOTEST_DOCKER_IMAGE) .

.PHONY: images
images: ## Create all images necessary to run robotest CI tests.
images: image-robotest images-upgrade image-opscenter

.PHONY: image-robotest
image-robotest: ## Create robotest cluster image using the current tele.
image-robotest: $(ROBOTEST_IMAGE)

$(ROBOTEST_IMAGE): $(TELE_OUT) $(ROBOTEST_IMAGE_SRC) | $(ROBOTEST_IMAGEDIR) $(ROBOTEST_BUILD_TMP)
	$(eval TMP := $(shell mktemp -d -p $(ROBOTEST_BUILD_TMP)))
	$(eval TGT := $(TMP)/robotest-$(GRAVITY_VERSION).tar)
	$(TELE_OUT) build \
		$(ROBOTEST_APP_YAML) \
		--state-dir=$(PACKAGES_DIR) \
		--version $(GRAVITY_VERSION) \
		--output=$(TGT)
	mv $(TGT) $(ROBOTEST_IMAGE)
	rm -rf $(TMP)

.PHONY: images-upgrade
images-upgrade: ## Create/fetch robotest cluster images needed for upgrade testing.
images-upgrade:
	$(MAKE) -j $(ROBOTEST_UPGRADE_IMAGES)

.PHONY: image-opscenter
image-opscenter: ## Move opscenter.tar cluster image into place for test execution.
image-opscenter: $(ROBOTEST_OPSCENTER_IMAGE)

$(ROBOTEST_OPSCENTER_IMAGE): $(OPSCENTER_OUT) | $(ROBOTEST_IMAGEDIR)
	cp $(OPSCENTER_OUT) $(ROBOTEST_OPSCENTER_IMAGE)

# The following rule populates the build specific docker image mount
# from cached images. Fulfills the $(ROBOTEST_UPGRADE_IMAGES) targets.
# .PRECIOUS because these need to be present through the run target.
.PRECIOUS: $(ROBOTEST_IMAGEDIR)/robotest-%.tar
$(ROBOTEST_IMAGEDIR)/robotest-%.tar: $(ROBOTEST_UPGRADE_IMAGE_CACHE)/robotest-%.tar | $(ROBOTEST_IMAGEDIR) $(ROBOTEST_BUILD_TMP)
	$(eval TMP := $(shell mktemp -d -p $(ROBOTEST_BUILD_TMP)))
	$(eval ATOMIC := $(TMP)/robotest-$*.tar)
	cp $< $(ATOMIC)
	mv $(ATOMIC) $@
	rm -rf $(TMP)

# The following target is a pattern rule for populating the upgrade image cache
# Fulfills the $(ROBOTEST_UPGRADE_CACHE_IMAGES) targets.
# .PRECIOUS because it is a cache shared across versions.
.PRECIOUS: $(ROBOTEST_UPGRADE_IMAGE_CACHE)/robotest-%.tar
$(ROBOTEST_UPGRADE_IMAGE_CACHE)/robotest-%.tar: $(ROBOTEST_TELE_CACHE)/tele-% $(ROBOTEST_UPGRADE_BASE_IMAGE_SRC) | $(ROBOTEST_UPGRADE_IMAGE_CACHE) $(ROBOTEST_CACHE_TMP)
	$(eval TMP := $(shell mktemp -d -p $(ROBOTEST_CACHE_TMP)))
	$(eval TGT := $(TMP)/robotest-$*.tar)
	$< build \
		$(ROBOTEST_UPGRADE_BASE_APP_YAML) \
		--state-dir $(ROBOTEST_GRAVITY_PKG_CACHE) \
		--version $* \
		--output=$(TGT)
	mv $(TGT) $@
	rm -rf $(TMP)


# The following target is a pattern rule for populating the tele binary cache.
# Fulfills the $(ROBOTEST_TELE_BINARIES) targets.
# .PRECIOUS because it is a cache shared across branches/builds.
.PRECIOUS: $(ROBOTEST_TELE_CACHE)/tele-%
$(ROBOTEST_TELE_CACHE)/tele-%: | $(ROBOTEST_CACHE_TMP) $(ROBOTEST_TELE_CACHE)
	$(eval TMP := $(shell mktemp -d -p $(ROBOTEST_CACHE_TMP)))
ifdef ENTERPRISE
	wget --quiet https://get.gravitational.io/telekube/bin/$*/linux/x86_64/tele --output-document $(TMP)/tele
	chmod u+x $(TMP)/tele
else
	wget --quiet https://get.gravitational.com/gravity-$*-linux-x86_64-bin.tar.gz --directory-prefix $(TMP)
	tar -xzf $(TMP)/gravity-$*-linux-x86_64-bin.tar.gz -C $(TMP)
endif
	mv $(TMP)/tele $@
	rm -rf $(TMP)

.PHONY: download-teles
download-teles: ## Download tele binaries needed to build robotest images.
download-teles:
	$(MAKE) -j $(ROBOTEST_TELE_BINARIES)

.PHONY: populate-caches
populate-caches: ## Populate caches with tele binaries, gravity packages, and robotest upgrade images.
	$(MAKE) -j $(ROBOTEST_TELE_BINARIES)
	$(MAKE) -j $(ROBOTEST_UPGRADE_CACHE_IMAGES)

.PHONY: get-upgrade-from-versions
get-upgrade-from-versions: ## Show versions used in upgrade from testing.
get-upgrade-from-versions:
	@echo $(ROBOTEST_UPGRADE_VERSIONS)
export

.PHONY: run
run: ## Run robotest.
run: export GRAVITY_URL = $(ROBOTEST_GRAVITY_BINARY)
run: export INSTALLER_URL ?= /images/$(notdir $(ROBOTEST_IMAGE))
run: export OPSCENTER_URL ?= /images/$(notdir $(ROBOTEST_OPSCENTER_IMAGE))
run: export ROBOTEST_IMAGES_DIR = $(ROBOTEST_IMAGEDIR)
run: export STATEDIR ?= $(ROBOTEST_STATEDIR)
run: export SSH_KEY ?= $(ROBOTEST_SSH_KEY)
run: export SSH_PUB ?= $(ROBOTEST_SSH_PUB)
run: export GOOGLE_APPLICATION_CREDENTIALS ?= $(ROBOTEST_GOOGLE_APPLICATION_CREDENTIALS)
run: $(ROBOTEST_TEST_CONFIG) images docker-image $(ROBOTEST_GRAVITY_BINARY)
	bash run.sh $(ROBOTEST_TEST_CONFIG)
