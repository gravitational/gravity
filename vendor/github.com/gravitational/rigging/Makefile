# all goinstalls everything
.PHONY: all
all: install

# install installs binary
.PHONY: install
install:
	go install github.com/gravitational/rigging/tool/rig

BUILDDIR ?= $(abspath build)

.PHONY: build
build:
	go build -ldflags '-w' -o $(BUILDDIR)/rig github.com/gravitational/rigging/tool/rig

.PHONY: test
test:
	go test -v ./ ./tool/...

BUILDBOX := quay.io/gravitational/debian-venti:go1.16.2-buster

# Directory with sources inside the container
DST := /gopath/src/github.com/gravitational/rigging

# This directory
SRC := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))

VERSION ?= $(shell git describe --tags --abbrev=8)
IMAGE := quay.io/gravitational/rig:$(VERSION)

# docker target starts build inside the container
.PHONY: docker-build
docker-build:
	docker run -i --rm=true \
		   -u $$(id -u):$$(id -g) -e GOCACHE=/tmp/.cache \
		   -v $(SRC):$(DST) \
		   -v $(BUILDDIR):$(DST)/build \
		   $(BUILDBOX) \
		   /bin/bash -c "make -C $(DST) build"

# docker target starts tests inside the container
.PHONY: docker-test
docker-test:
	docker run -i --rm=true \
		   -v $(SRC):$(DST) \
		   $(BUILDBOX) \
		   /bin/bash -c "make -C $(DST) test"

.PHONY: docker-image
docker-image: docker-build
	$(eval TEMPDIR = "$(shell mktemp -d $(BUILDDIR)/tmp.XXXXXX)")
	if [ -z "$(TEMPDIR)" ]; then \
	  echo "TEMPDIR is not set"; exit 1; \
	fi;
	mkdir -p $(TEMPDIR)/build
	cp build/rig $(TEMPDIR)/build/rig
	cp -r docker/rig.dockerfile $(TEMPDIR)/Dockerfile
	cd $(TEMPDIR) && docker build --pull -t $(IMAGE) .
	rm -rf $(TEMPDIR)

.PHONY: publish-docker-image
publish-docker-image:
	docker push $(IMAGE)


.PHONY: print-image
print-image:
	echo $(IMAGE)
