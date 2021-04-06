# Prerequisites:
# - Linux-based OS
# - golang 1.10+
# - git
# - Docker 1.9+
#
# Userful targets:
# - make          : default containerized build. The output goes into build/<version>/
# - make install  : build via `go install`. The output goes into GOPATH/bin/
# - make clean    : remove the build output and artifacts
#
TOP := $(realpath $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))))

OPS_URL ?=

GRAVITY_PKG_PATH ?= github.com/gravitational/gravity

ASSETSDIR=$(TOP)/assets
BINDIR ?= /usr/bin

# Current Kubernetes version
K8S_VER := 1.19.8
# Kubernetes version suffix for the planet package, constructed by concatenating
# major + minor padded to 2 chars with 0 + patch also padded to 2 chars, e.g.
# 1.13.5 -> 11305, 1.13.12 -> 11312, 2.0.0 -> 20000 and so on
K8S_VER_SUFFIX := $(shell printf "%d%02d%02d" $(shell echo $(K8S_VER) | sed "s/\./ /g"))
GOLFLAGS ?= -w -s

ETCD_VER := v2.3.7
# Version of the version tool
VERSION_TAG := 0.0.2

FIO_VER ?= 3.15
FIO_TAG := fio-$(FIO_VER)
FIO_PKG_TAG := $(FIO_VER).0

# Current versions of the dependencies
CURRENT_TAG ?= $(shell ./version.sh)
GRAVITY_TAG := $(CURRENT_TAG)
# Abbreviated gravity version to use as a build ID
GRAVITY_VERSION := $(CURRENT_TAG)

RELEASE_TARBALL_NAME ?=
RELEASE_OUT ?=

TELEPORT_TAG = 3.2.17
# TELEPORT_REPOTAG adapts TELEPORT_TAG to the teleport tagging scheme
TELEPORT_REPOTAG := v$(TELEPORT_TAG)
PLANET_TAG := 7.1.25-$(K8S_VER_SUFFIX)
PLANET_BRANCH := $(PLANET_TAG)
K8S_APP_TAG := $(GRAVITY_TAG)
TELEKUBE_APP_TAG := $(GRAVITY_TAG)
WORMHOLE_APP_TAG := $(GRAVITY_TAG)
INGRESS_APP_TAG ?= 0.0.1
STORAGE_APP_TAG ?= 0.0.4
LOGGING_APP_TAG ?= 7.1.0
MONITORING_APP_TAG ?= 7.1.4
DNS_APP_TAG = 7.1.2
BANDWAGON_TAG ?= 7.1.0
RBAC_APP_TAG := $(GRAVITY_TAG)
TILLER_VERSION = 2.16.12
TILLER_APP_TAG = 7.1.0
SELINUX_VERSION ?= 6.0.0
# URI of Wormhole container for default install
WORMHOLE_IMG ?= quay.io/gravitational/wormhole:0.3.3
# set this to true if you want to use locally built planet packages
DEV_PLANET ?=
OS := $(shell uname | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)

# Image Vulnerability Scanning
# The following variables are used to copy all docker images from a cluster image to a docker repository
# that is able to scan and report on those images
TELE_COPY_TO_REGISTRY ?= quay.io/gravitational
TELE_COPY_TO_REPOSITORY ?= gravitational/gravity-scan
TELE_COPY_TO_PREFIX ?= $(GRAVITY_VERSION)
TELE_COPY_TO_USER ?=
TELE_COPY_TO_PASS ?=

CURRENT_COMMIT := $(shell git rev-parse HEAD)
VERSION_FLAGS := -X github.com/gravitational/version.gitCommit=$(CURRENT_COMMIT) \
	-X github.com/gravitational/version.version=$(GRAVITY_VERSION) \
	-X github.com/gravitational/gravity/lib/defaults.WormholeImg=$(WORMHOLE_IMG) \
	-X github.com/gravitational/gravity/lib/defaults.TeleportVersionString=$(TELEPORT_TAG)
GRAVITY_LINKFLAGS = "$(VERSION_FLAGS) $(GOLFLAGS)"
ifeq ($(OS),linux)
GRAVITY_BUILDTAGS = selinux selinux_embed
endif

TELEKUBE_GRAVITY_PKG := gravitational.io/gravity_$(OS)_$(ARCH):$(GRAVITY_TAG)
TELEKUBE_TELE_PKG := gravitational.io/tele_$(OS)_$(ARCH):$(GRAVITY_TAG)

TELEPORT_PKG := gravitational.io/teleport:$(TELEPORT_TAG)
PLANET_PKG := gravitational.io/planet:$(PLANET_TAG)
WEB_ASSETS_PKG := gravitational.io/web-assets:$(GRAVITY_TAG)
GRAVITY_PKG := gravitational.io/gravity:$(GRAVITY_TAG)
DNS_APP_PKG := gravitational.io/dns-app:$(DNS_APP_TAG)
INGRESS_APP_PKG := gravitational.io/ingress-app:$(INGRESS_APP_TAG)
STORAGE_APP_PKG := gravitational.io/storage-app:$(STORAGE_APP_TAG)
MONITORING_APP_PKG := gravitational.io/monitoring-app:$(MONITORING_APP_TAG)
LOGGING_APP_PKG := gravitational.io/logging-app:$(LOGGING_APP_TAG)
SITE_APP_PKG := gravitational.io/site:$(GRAVITY_TAG)
K8S_APP_PKG := gravitational.io/kubernetes:$(K8S_APP_TAG)
TELEKUBE_APP_PKG := gravitational.io/telekube:$(TELEKUBE_APP_TAG)
BANDWAGON_PKG := gravitational.io/bandwagon:$(BANDWAGON_TAG)
RBAC_APP_PKG := gravitational.io/rbac-app:$(RBAC_APP_TAG)
TILLER_APP_PKG := gravitational.io/tiller-app:$(TILLER_APP_TAG)
FIO_PKG := gravitational.io/fio:$(FIO_PKG_TAG)
SELINUX_POLICY_PKG := gravitational.io/selinux:$(SELINUX_VERSION)

# Output directory that stores all of the build artifacts.
# Artifacts from the gravity build (the binary and any internal packages)
# are collected into a directory named after the current gravity version suffix.
# All static (external) dependencies are version by appending a corresponding version
# suffix to the tarball.
# planet/teleport binaries are stored in separate versioned directories to be compatible with
# `aws s3 sync` command (which only works on directories)
BUILDDIR ?= $(TOP)/build
GRAVITY_BUILDDIR := $(BUILDDIR)/$(GRAVITY_VERSION)
GRAVITY_CURRENT_BUILDDIR := $(BUILDDIR)/current
PLANET_DIR := $(BUILDDIR)/planet
PLANET_SRCDIR := $(PLANET_DIR)/src
PLANET_BUILDDIR := $(PLANET_DIR)/$(PLANET_TAG)
PLANET_BINDIR := $(PLANET_BUILDDIR)/bin
TELEPORT_BUILDDIR := $(BUILDDIR)/teleport
TELEPORT_SRCDIR := $(TELEPORT_BUILDDIR)/src
TELEPORT_BINDIR := $(TELEPORT_BUILDDIR)/bin/$(TELEPORT_TAG)
FIO_BUILDDIR := $(BUILDDIR)/fio-$(FIO_VER)
HACK_DIR := $(TOP)/hack
GENERATED_DIR := $(TOP)/lib/client

LOCAL_BUILDDIR ?= /gopath/src/github.com/gravitational/gravity/build
LOCAL_GRAVITY_BUILDDIR ?= /gopath/src/github.com/gravitational/gravity/build/$(GRAVITY_VERSION)

# Directory used as a state dir with all packages when building an application
# with tele build (e.g. opscenter, robotest, telekube, or wormhole)
PACKAGES_DIR ?= $(GRAVITY_BUILDDIR)/packages

# Outputs
#
# External assets
TELEPORT_TARBALL := teleport-$(TELEPORT_TAG).tar.gz
TELEPORT_OUT := $(BUILDDIR)/$(TELEPORT_TARBALL)
PLANET_OUT := $(PLANET_BINDIR)/planet.tar.gz
LOGGING_APP_OUT := $(BUILDDIR)/logging-app-$(LOGGING_APP_TAG).tar.gz
MONITORING_APP_OUT := $(BUILDDIR)/monitoring-app-$(MONITORING_APP_TAG).tar.gz
INGRESS_APP_OUT := $(BUILDDIR)/ingress-app-$(INGRESS_APP_TAG).tar.gz
STORAGE_APP_OUT := $(BUILDDIR)/storage-app-$(STORAGE_APP_TAG).tar.gz
BANDWAGON_OUT := $(BUILDDIR)/bandwagon-$(BANDWAGON_TAG).tar.gz
FIO_OUT := $(FIO_BUILDDIR)/fio
#
# Assets resulting from building gravity
GRAVITY_OUT := $(GRAVITY_BUILDDIR)/gravity
TELE_OUT := $(GRAVITY_BUILDDIR)/tele
TSH_OUT := $(GRAVITY_BUILDDIR)/tsh
WEB_ASSETS_TARBALL = web-assets.tar.gz
WEB_ASSETS_OUT := $(GRAVITY_BUILDDIR)/$(WEB_ASSETS_TARBALL)
SITE_APP_OUT := $(GRAVITY_BUILDDIR)/site-app.tar.gz
DNS_APP_OUT := $(GRAVITY_BUILDDIR)/dns-app.tar.gz
K8S_APP_OUT := $(GRAVITY_BUILDDIR)/kubernetes-app.tar.gz
RBAC_APP_OUT := $(GRAVITY_BUILDDIR)/rbac-app.tar.gz
TELEKUBE_APP_OUT := $(GRAVITY_BUILDDIR)/telekube-app.tar.gz
TILLER_APP_OUT := $(GRAVITY_BUILDDIR)/tiller-app.tar.gz
TELEKUBE_OUT := $(GRAVITY_BUILDDIR)/telekube.tar
OPSCENTER_OUT := $(GRAVITY_BUILDDIR)/opscenter.tar
SELINUX_ASSETSDIR := $(TOP)/lib/system/selinux/internal/policy/assets/centos
SELINUX_ASSETS := $(SELINUX_ASSETSDIR)/gravity.pp.bz2 \
		$(SELINUX_ASSETSDIR)/container.pp.bz2 \
		$(SELINUX_ASSETSDIR)/gravity.statedir.fc.template
SELINUX_OUT := $(GRAVITY_BUILDDIR)/selinux-policy.tgz

LOCAL_OPSCENTER_HOST ?= opscenter.localhost.localdomain
LOCAL_OPS_URL := https://$(LOCAL_OPSCENTER_HOST):33009

LOCAL_STATE_DIR ?= $(GRAVITY_BUILDDIR)/state

# Build artifacts published to S3
GRAVITY_PUBLISH_TARGETS := $(GRAVITY_OUT) \
	$(TELE_OUT) \
	$(WEB_ASSETS_OUT) \
	$(SITE_APP_OUT) \
	$(DNS_APP_OUT) \
	$(K8S_APP_OUT) \
	$(RBAC_APP_OUT) \
	$(TELEKUBE_APP_OUT) \
	$(TILLER_APP_OUT)

GRAVITY_EXTRA_OPTIONS ?=

# Address of OpsCenter to publish telekube binaries and artifacts to
DISTRIBUTION_OPSCENTER ?= https://get.gravitational.io

# Command line of the current gravity binary
GRAVITY ?= $(GRAVITY_OUT) --state-dir=$(LOCAL_STATE_DIR) $(GRAVITY_EXTRA_OPTIONS)

DELETE_OPTS := --force \
		--ops-url=$(OPS_URL)
IMPORT_OPTS := --repository=gravitational.io \
		--ops-url=$(OPS_URL)
VENDOR_OPTS := --vendor $(IMPORT_OPTS)

USER := $(shell echo $${SUDO_USER:-$$USER})

TEST_ETCD ?= false
TEST_K8S ?= false

# grpc
PROTOC_VER ?= 3.10.0
PROTOC_PLATFORM := linux-x86_64
GOGO_PROTO_TAG ?= v1.3.0
GRPC_GATEWAY_TAG ?= v1.11.3

BINARIES ?= tele gravity

export

# the default target is a containerized CI/CD build
.PHONY: build
build:
	$(MAKE) -C build.assets build

# 'install' uses the host's Golang to place output into $GOPATH/bin
.PHONY: install
install:
	GO111MODULE=on go install -mod=vendor -ldflags $(GRAVITY_LINKFLAGS) -tags "$(GRAVITY_BUILDTAGS)" ./tool/tele ./tool/gravity

# 'clean' removes the build artifacts
.PHONY: clean
clean:
	$(MAKE) -C build.assets clean
	@rm -rf $(BUILDDIR)
	@rm -f $(GOPATH)/bin/tele $(GOPATH)/bin/gravity


.PHONY: production
production: TMP := $(shell mktemp -d)
production:
	GRAVITY="$(GRAVITY_OUT) --state-dir=$(TMP)" $(MAKE) -C build.assets production
	rm -rf $(TMP)

#
# generate GRPC files
#
.PHONY: grpc
grpc:
	PROTOC_VER=$(PROTOC_VER) PROTOC_PLATFORM=$(PROTOC_PLATFORM) \
	GOGO_PROTO_TAG=$(GOGO_PROTO_TAG) GRPC_GATEWAY_TAG=$(GRPC_GATEWAY_TAG) VERSION_TAG=$(VERSION_TAG) \
	$(MAKE) -C build.assets grpc

#
# build tsh binary
#
.PHONY: build-tsh
build-tsh:
	$(MAKE) -C build.assets build-tsh

.PHONY: fio
fio:
	$(MAKE) -C build.assets fio

#
# reimport site app and refresh tarball
#
.PHONY: site-app
site-app:
	$(MAKE) -C build.assets site-app

#
# reimport rbac-app and refresh tarball
#
.PHONY: rbac-app
rbac-app:
	$(MAKE) -C build.assets rbac-app

#
# reimport dns-app and refresh tarball
#
.PHONY: dns-app
dns-app:
	$(MAKE) -C build.assets dns-app

.PHONY: tiller-app
tiller-app:
	make -C build.assets tiller-app

#
# reimport k8s app and refresh tarball
#
.PHONY: k8s-app
k8s-app:
	$(MAKE) -C build.assets k8s-app

.PHONY: web-app
web-app:
	$(MAKE) -C build.assets web-app

#
# reimport telekube app and refresh tarball
#
.PHONY: telekube-app
telekube-app: dev
	$(MAKE) -C build.assets telekube-app

.PHONY: monitoring-app
monitoring-app: dev
	$(MAKE) -C build.assets monitoring-app

.PHONY: logging-app
logging-app: dev
	$(MAKE) -C build.assets logging-app

.PHONY: ingress-app
ingress-app: dev
	$(MAKE) -C build.assets ingress-app

.PHONY: storage-app
storage-app: dev
	$(MAKE) -C build.assets storage-app

.PHONY: bandwagon-app
bandwagon-app: dev
	$(MAKE) -C build.assets bandwagon

#
# publish dependencies (planet and teleport) to Amazon S3
#
.PHONY: publish
publish: scan-artifacts
	$(MAKE) -C build.assets publish

#
# prepare ansible variables for publishing to the hub
#
.PHONY: hub-vars
hub-vars:
	$(MAKE) -C build.assets hub-vars

#
# produce release tarball with binaries
#
.PHONY: release
release:
	$(MAKE) -C build.assets release

#
# publish telekube binaries (gravity, tele and tsh) to the distribution OpsCenter
#
.PHONY: publish-telekube
publish-telekube:
	$(MAKE) -C build.assets publish-telekube

.PHONY: publish-telekube-s3
publish-telekube-s3:
	$(MAKE) -C build.assets publish-telekube-s3

#
# test packages: called by Jenkins
#
.PHONY: test
test:
	$(MAKE) -C build.assets test

#
# '$(MAKE) packages' builds and imports all dependency packages
#
.PHONY: packages
packages: planet-packages binary-packages teleport-package gravity-packages dns-packages\
	rbac-app-package bandwagon-package tiller-package monitoring-package \
	ingress-package storage-package log-package k8s-packages telekube-packages \
	selinux-policy-package

.PHONY: teleport-package
teleport-package:
# teleport - access and identity layer
	-$(GRAVITY) package delete $(TELEPORT_PKG) $(DELETE_OPTS)
	$(GRAVITY) package import $(TELEPORT_OUT) $(TELEPORT_PKG) --ops-url=$(OPS_URL)

.PHONY: bandwagon-package
bandwagon-package:
# Bandwagon - installer extension
	-$(GRAVITY) app delete $(BANDWAGON_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(BANDWAGON_OUT) $(VENDOR_OPTS)

.PHONY: tiller-package
tiller-package:
# Tiller server
	-$(GRAVITY) app delete $(TILLER_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(TILLER_APP_OUT) $(VENDOR_OPTS)

.PHONY: ingress-package
ingress-package:
	-$(GRAVITY) app delete $(INGRESS_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(INGRESS_APP_OUT) $(VENDOR_OPTS)

.PHONY: storage-package
storage-package:
	-$(GRAVITY) app delete $(STORAGE_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(STORAGE_APP_OUT) $(VENDOR_OPTS)

.PHONY: monitoring-package
monitoring-package:
	-$(GRAVITY) app delete $(MONITORING_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(MONITORING_APP_OUT) $(VENDOR_OPTS)

.PHONY: log-package
log-package:
# Logging - log forwarding and storage
	-$(GRAVITY) app delete $(LOGGING_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(LOGGING_APP_OUT) $(VENDOR_OPTS)

.PHONY: binary-packages
binary-packages:
	-$(GRAVITY_OUT) package delete --state-dir=$(LOCAL_STATE_DIR) --force $(TELEKUBE_GRAVITY_PKG)
	$(GRAVITY_OUT) package import --state-dir=$(LOCAL_STATE_DIR) $(GRAVITY_OUT) $(TELEKUBE_GRAVITY_PKG)

	-$(GRAVITY_OUT) package delete --state-dir=$(LOCAL_STATE_DIR) --force $(TELEKUBE_TELE_PKG)
	$(GRAVITY_OUT) package import --state-dir=$(LOCAL_STATE_DIR) $(TELE_OUT) $(TELEKUBE_TELE_PKG)

.PHONY: rbac-app-package
rbac-app-package:
	-$(GRAVITY) app delete $(RBAC_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(RBAC_APP_OUT) $(VENDOR_OPTS)

.PHONY: gravity-packages
gravity-packages:
# gravity - k8s automation
	-$(GRAVITY) package delete $(GRAVITY_PKG) $(DELETE_OPTS)
	$(GRAVITY) package import $(GRAVITY_OUT) $(GRAVITY_PKG) --ops-url=$(OPS_URL)

# site app - local site controller running inside k8s
	-$(GRAVITY) app delete $(SITE_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(SITE_APP_OUT) --version=$(GRAVITY_TAG) $(VENDOR_OPTS)

.PHONY: k8s-packages
k8s-packages: fio-package web-assets
	-$(GRAVITY) app delete $(K8S_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(K8S_APP_OUT) --version=$(K8S_APP_TAG) $(VENDOR_OPTS)

$(TELEKUBE_APP_OUT):
	make -C build.assets telekube-app

.PHONY: telekube-packages
telekube-packages: $(TELEKUBE_APP_OUT)
	-$(GRAVITY) app delete $(TELEKUBE_APP_PKG) $(DELETE_OPTS)
	$(GRAVITY) app import $(TELEKUBE_APP_OUT) --version=$(TELEKUBE_APP_TAG) $(VENDOR_OPTS)

.PHONY: fio-package
fio-package:
	-$(GRAVITY) package delete $(FIO_PKG) $(DELETE_OPTS)
	$(GRAVITY) package import $(FIO_OUT) $(FIO_PKG) --ops-url=$(OPS_URL)

.PHONY: planet-packages
ifndef DEV_PLANET
planet-packages: planet-package
else
planet-packages: dev-planet-package
endif

.PHONY: planet-package
planet-package:
# planet master - RUNC container with k8s master
	$(GRAVITY) package delete $(PLANET_PKG) $(DELETE_OPTS) && \
	$(GRAVITY) package import $(PLANET_OUT) $(PLANET_PKG) \
		--labels=purpose:runtime \
		--ops-url=$(OPS_URL)

.PHONY: dev-planet-package
dev-planet-package: PLANET_OUT := $(GOPATH)/src/github.com/gravitational/planet/build/planet.tar.gz
dev-planet-package: planet-package

.PHONY: dns-packages
dns-packages:
# DNS - k8s KubeDNS app
	- $(GRAVITY) app delete $(DNS_APP_PKG) $(DELETE_OPTS) && \
	  $(GRAVITY) app import $(DNS_APP_OUT) $(VENDOR_OPTS)

.PHONY: web-assets
web-assets:
	$(GRAVITY) package delete $(WEB_ASSETS_PKG) $(DELETE_OPTS) && \
	$(GRAVITY) package import $(WEB_ASSETS_OUT) $(WEB_ASSETS_PKG) --ops-url=$(OPS_URL)

.PHONY: selinux-policy-package
selinux-policy-package:
	$(GRAVITY) package delete $(SELINUX_POLICY_PKG) $(DELETE_OPTS) && \
	$(GRAVITY) package import $(SELINUX_OUT) $(SELINUX_POLICY_PKG) --ops-url=$(OPS_URL)

#
# publish-artifacts uploads build artifacts to the distribution Ops Center
#
.PHONY: publish-artifacts
publish-artifacts: $(OPSCENTER_OUT) $(TELEKUBE_OUT)
	if [ -z "$(TELE_KEY)" ] || [ -z "$(DISTRIBUTION_OPSCENTER)" ]; then \
	   echo "TELE_KEY or DISTRIBUTION_OPSCENTER are not set"; exit 1; \
	fi;
	$(GRAVITY_BUILDDIR)/tele push $(TELEKUBE_OUT)  --hub=$(DISTRIBUTION_OPSCENTER) --token=$(TELE_KEY)
	$(GRAVITY_BUILDDIR)/tele push $(OPSCENTER_OUT) --hub=$(DISTRIBUTION_OPSCENTER) --token=$(TELE_KEY)

#
# scan-artifacts uploads a copy of all vendored containers to a docker registry for scanning and vulnerability reporting
#
.PHONY: scan-artifacts
scan-artifacts: $(TELEKUBE_OUT)
	$(GRAVITY) app sync \
		--registry=$(TELE_COPY_TO_REGISTRY) \
		--registry-username=$(TELE_COPY_TO_USER) \
		--registry-password=$(TELE_COPY_TO_PASS) \
		--scan-repository=$(TELE_COPY_TO_REPOSITORY) \
		--scan-prefix=$(TELE_COPY_TO_PREFIX) \
		$(TELEKUBE_OUT)

#
# builds telekube installer
#
.PHONY: telekube
telekube: $(TELEKUBE_OUT)

$(TELEKUBE_OUT): GRAVITY=$(GRAVITY_OUT) --state-dir=$(PACKAGES_DIR)
$(TELEKUBE_OUT): packages
	GRAVITY_K8S_VERSION=$(K8S_VER) $(GRAVITY_BUILDDIR)/tele build \
		$(ASSETSDIR)/telekube/resources/app.yaml -f \
		--version=$(TELEKUBE_APP_TAG) \
		--state-dir=$(PACKAGES_DIR) \
		--skip-version-check \
		-o $(TELEKUBE_OUT)

#
# runs robotest integration tests
#
.PHONY: robotest-run
robotest-run: # depends on: telekube opscenter $(TELE_OUT) $(GRAVITY_OUT)
	$(MAKE) -C assets/robotest run

#
# builds wormhole installer
#
.PHONY: wormhole
wormhole: GRAVITY=$(GRAVITY_OUT) --state-dir=$(PACKAGES_DIR)
wormhole: $(GRAVITY_BUILDDIR)/wormhole.tar

$(GRAVITY_BUILDDIR)/wormhole.tar: packages
	$(GRAVITY_BUILDDIR)/tele build $(ASSETSDIR)/wormhole/resources/app.yaml -f \
		--version=$(WORMHOLE_APP_TAG) \
		--state-dir=$(PACKAGES_DIR) \
		--skip-version-check \
		-o $(GRAVITY_BUILDDIR)/wormhole.tar

#
# Uploads opscenter to S3 is used to test custom releases of the ops center
#
.PHONY: upload-opscenter
upload-opscenter: $(OPSCENTER_OUT)
	aws s3 cp $(OPSCENTER_OUT) s3://testreleases.gravitational.io/$(GRAVITY_TAG)/opscenter.tar

#
# Uploads gravity to test builds
#
.PHONY: upload-binaries
upload-binaries:
	aws s3 cp $(GRAVITY_BUILDDIR)/gravity s3://testreleases.gravitational.io/$(GRAVITY_TAG)/gravity
	aws s3 cp $(GRAVITY_BUILDDIR)/tele s3://testreleases.gravitational.io/$(GRAVITY_TAG)/tele

#
# builds opscenter installer
#
.PHONY: opscenter
opscenter: $(OPSCENTER_OUT)

$(OPSCENTER_OUT): GRAVITY=$(GRAVITY_OUT) --state-dir=$(PACKAGES_DIR)
$(OPSCENTER_OUT): packages
	mkdir -p $(BUILDDIR)
# this is for Jenkins pipeline integration
	@echo env.GRAVITY_BUILDDIR=\"$(GRAVITY_BUILDDIR)\" > $(BUILDDIR)/properties.groovy
	if [ -z "$(GRAVITY_TAG)" ]; then \
	  echo "GRAVITY_TAG is not set"; exit 1; \
	fi;
	$(eval RIG_CHANGESET = ops-$(shell echo $(GRAVITY_TAG) | sed -e 's/[\.]//g'))
	if [ -z "$(RIG_CHANGESET)" ]; then \
	  echo "RIG_CHANGESET is not set"; exit 1; \
	fi;
	echo $(GRAVITY_TAG)
	echo $(RIG_CHANGESET)
	$(eval TEMPDIR = "$(shell mktemp -d)")
	if [ -z "$(TEMPDIR)" ]; then \
	  echo "TEMPDIR is not set - failed to create temporary directory"; exit 1; \
	fi;
	cp -r assets/opscenter/resources $(TEMPDIR)
	sed -i 's/GRAVITY_VERSION/$(GRAVITY_TAG)/g' $(TEMPDIR)/resources/app.yaml
	sed -i 's/RIG_CHANGESET_VAL/$(RIG_CHANGESET)/g' $(TEMPDIR)/resources/app.yaml
	cat $(TEMPDIR)/resources/app.yaml
	$(GRAVITY_BUILDDIR)/tele build $(TEMPDIR)/resources/app.yaml -f \
		--state-dir=$(PACKAGES_DIR) \
		-o $(OPSCENTER_OUT)
	rm -rf $(TEMPDIR)

#
# opscenter-apps imports additional apps into deployed OpsCenter
#
.PHONY: opscenter-apps
opscenter-apps: $(TELEKUBE_APP_OUT)
	- $(GRAVITY_OUT) --state-dir=$(LOCAL_STATE_DIR) app delete $(TELEKUBE_APP_PKG) $(DELETE_OPTS) && \
	  $(GRAVITY_OUT) --state-dir=$(LOCAL_STATE_DIR) app import $(TELEKUBE_APP_OUT) $(VENDOR_OPTS)

#
# current-build will print current build
#
.PHONY: current-build
current-build:
	@echo $(GRAVITY_BUILDDIR)

.PHONY: compile
compile:
	$(MAKE) -j $(BINARIES)

.PHONY: tele-mac
tele-mac: flags
	go install -ldflags $(GRAVITY_LINKFLAGS) github.com/gravitational/gravity/tool/tele


#
# goinstall builds and installs gravity locally
#
.PHONY: goinstall
goinstall: remove-temp-files compile | $(GRAVITY_BUILDDIR)
	for bin in ${BINARIES} ; do \
		cp $(GOPATH)/bin/$${bin} $(GRAVITY_BUILDDIR)/$${bin} ; \
	done
	$(GRAVITY) package delete $(GRAVITY_PKG) $(DELETE_OPTS) && \
		$(GRAVITY) package import $(GRAVITY_OUT) $(GRAVITY_PKG)
	$(MAKE) binary-packages

$(GRAVITY_BUILDDIR):
	mkdir -p $@

.PHONY: $(BINARIES)
$(BINARIES): selinux grpc
	GO111MODULE=on go install -mod=vendor -ldflags $(GRAVITY_LINKFLAGS) -tags "$(GRAVITY_BUILDTAGS)" $(GRAVITY_PKG_PATH)/tool/$@

.PHONY: dev
dev: goinstall

# Clean up development environment:
#  + remove development directories
#  + stop etcd container
#  + destroy development virsh guests
#  + remove development virsh images
.PHONY: dev-clean
dev-clean:
	bash scripts/cleanup.sh

.PHONY: remove-temp-files
remove-temp-files:
	@if [ $$USER != vagrant ] ; then \
		find . -name flymake_* -delete ; \
	fi

.PHONY: fakedevice
# fake device creates 1MB loopback device for testing purposes
fakedevice:
	dd if=/dev/urandom of=/tmp/dev0 bs=1M count=1
	sudo losetup /dev/loop0 /tmp/dev0

.PHONY: sloccount
sloccount:
	find . -path ./vendor -prune -o -name "*.go" -print0 | xargs -0 wc -l

.PHONY: test-package
test-package: remove-temp-files
	TEST_ETCD=$(TEST_ETCD) TEST_ETCD_CONFIG=$(TEST_ETCD_CONFIG) TEST_K8S=$(TEST_K8S) go test -v ./$(p)

.PHONY: test-grep-package
test-grep-package: remove-temp-files
	TEST_ETCD=$(TEST_ETCD) TEST_ETCD_CONFIG=$(TEST_ETCD_CONFIG) TEST_K8S=$(TEST_K8S) go test -v ./$(p) -check.f=$(e)

.PHONY: cover-package
cover-package: remove-temp-files
	TEST_ETCD=$(TEST_ETCD) TEST_ETCD_CONFIG=$(TEST_ETCD_CONFIG) TEST_K8S=$(TEST_K8S)  go test -v ./$(p) -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

# Dump abbreviated gravity version as used by the build
.PHONY: get-version
get-version:
	@echo $(GRAVITY_VERSION)

# Dump abbreviated planet version as used by the build
.PHONY: get-planet-tag
get-planet-tag:
	@echo $(PLANET_TAG)

# Dump abbreviated planet version as used by the build
.PHONY: get-teleport-tag
get-teleport-tag:
	@echo $(TELEPORT_TAG)

# Dump current gravity tag as a package suffix
.PHONY: get-tag
get-tag:
	@echo $(GRAVITY_TAG)

# Generate user-facing documentation
.PHONY: docs
docs:
	$(MAKE) -C docs

.PHONY: run-docs
run-docs:
	$(MAKE) -C docs run

.PHONY: docs-lint
docs-lint:
	$(MAKE) -C docs lint

# Dump current full k8s app tag
.PHONY: get-k8s-tag
get-k8s-tag:
	@echo $(K8S_APP_TAG)

.PHONY: update-codegen
update-codegen: clean-codegen
	bash $(HACK_DIR)/update-codegen.sh

.PHONY: verify-codegen
verify-codegen:
	bash $(HACK_DIR)/verify-codegen.sh

.PHONY: clean-codegen
clean-codegen:
	rm -r $(GENERATED_DIR)

.PHONY: selinux
selinux:
	$(MAKE) -C build.assets	selinux

include build.assets/etcd.mk
