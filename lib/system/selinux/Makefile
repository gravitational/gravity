TOP:=$(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))
DIR:=$(realpath $(TOP)/../../../)
OS_RELEASE?=centos
BUILDBOX_INSTANCE?=gravity-selinux-build
BUILDBOX?=gravity-selinux-build
CONTAINER_RUNTIME?=$(shell command -v podman 2> /dev/null || echo docker)
GRAVITY_REPOSITORY?=github.com/gravitational/gravity
POLICY_OUT:=policy_embed.go
ASSETSDIR:=assets/$(OS_RELEASE)

all: buildbox $(POLICY_OUT)

$(POLICY_OUT): policy.go $(ASSETSDIR)/container.pp.bz2 $(ASSETSDIR)/gravity.pp.bz2
	$(CONTAINER_RUNTIME) run \
		--name=$(BUILDBOX_INSTANCE) \
		-v ${DIR}:/go/src/$(GRAVITY_REPOSITORY) \
		--rm $(BUILDBOX) \
		/go/bin/generate

.PHONY: buildbox
buildbox: generate/generate.go
	$(CONTAINER_RUNTIME) build \
		-t $(BUILDBOX) \
		-f Dockerfile \
		-v $(DIR):/go/src/$(GRAVITY_REPOSITORY) .
