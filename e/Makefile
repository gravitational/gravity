TOP := $(realpath $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))))
CURRENT_TAG ?= $(shell cd .. && ./version.sh)
GRAVITY_TAG := $(CURRENT_TAG)
# Abbreviated gravity version to use as a build ID
GRAVITY_VERSION := $(CURRENT_TAG)
CURRENT_COMMIT := $(shell git rev-parse HEAD)
VERSION_FLAGS := -X github.com/gravitational/gravity/vendor/github.com/gravitational/version.gitCommit=$(CURRENT_COMMIT) \
	-X github.com/gravitational/gravity/vendor/github.com/gravitational/version.version=$(GRAVITY_VERSION)
LINKFLAGS := "$(VERSION_FLAGS) -w -s"

GRAVITY_WEB_APP_DIR := $(TOP)/web
GRAVITY_PKG_PATH=github.com/gravitational/gravity/e
ifdef LOCAL
BUILDDIR=$(TOP)/../build
GRAVITY_BASE_PKG_PATH=github.com/gravitational/gravity
LOCAL_BUILDDIR=/gopath/src/$(GRAVITY_BASE_PKG_PATH)/build
LOCAL_GRAVITY_BUILDDIR=/gopath/src/$(GRAVITY_BASE_PKG_PATH)/build/$(GRAVITY_VERSION)
else
BUILDDIR=$(TOP)/build
LOCAL_BUILDDIR=/gopath/src/$(GRAVITY_PKG_PATH)/build
LOCAL_GRAVITY_BUILDDIR=/gopath/src/$(GRAVITY_PKG_PATH)/build/$(GRAVITY_VERSION)
endif

BINARIES ?= gravity tele terraform-provider-gravity terraform-provider-gravityenterprise

GRAVITY_BUILDDIR ?= $(BUILDDIR)/$(GRAVITY_VERSION)
TEST_PACKAGES ?= \
	 $(GRAVITY_PKG_PATH)/lib/... \
	 $(GRAVITY_PKG_PATH)/tool/...

export BUILDDIR \
	GRAVITY_PKG_PATH \
	LOCAL_BUILDDIR \
	LOCAL_GRAVITY_BUILDDIR \
	GRAVITY_WEB_APP_DIR \
	BINARIES

.PHONY: build
build:
	$(MAKE) -C .. build

.PHONY: version
version:
	@echo $(GRAVITY_VERSION)

.PHONY: dev
dev:
	$(MAKE) -C .. dev

.PHONY: $(BINARIES)
$(BINARIES):
	$(MAKE) -C .. $@

.PHONY: install
install:
	$(MAKE) -C .. install

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)

.PHONY: production
production:
	$(MAKE) -C .. production

.PHONY: telekube
telekube:
	$(MAKE) -C .. telekube

.PHONY: wormhole
wormhole:
	$(MAKE) -C .. wormhole

.PHONY: opscenter
opscenter:
	$(MAKE) -C .. opscenter

.PHONY: site-app
site-app:
	$(MAKE) -C .. site-app

.PHONY: telekube-app
telekube-app:
	$(MAKE) -C .. telekube-app

.PHONY: test
test:
	$(MAKE) TEST_PACKAGES="$(TEST_PACKAGES)" -C .. test

.PHONY: test-package
test-package:
	go test -v ./$(p)

.PHONY: publish-telekube
publish-telekube:
	$(MAKE) -C .. publish-telekube

.PHONY: publish-artifacts
publish-artifacts:
	$(MAKE) -C .. publish-artifacts

.PHONY: publish
publish:
	$(MAKE) -C .. publish

.PHONY: robotest-run-suite
robotest-run-suite:
	$(MAKE) -C .. robotest-run-suite

.PHONY: robotest-run-nightly
robotest-run-nightly:
	$(MAKE) -C .. robotest-run-nightly

.PHONY: goinstall
goinstall:
	$(MAKE) -C .. goinstall
