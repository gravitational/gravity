TOP:=$(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))
DIR:=$(realpath $(TOP)/../../../)
OS_RELEASE?=centos
BUILDBOX?=gravity-selinux-build
CONTAINER_RUN_ARGS=
CONTAINER_RUNTIME?=$(shell command -v podman 2> /dev/null || echo docker)
ifneq (,$(findstring docker,$(CONTAINER_RUNTIME)))
	CONTAINER_RUN_ARGS=--user $$(id -u)
endif
GRAVITY_REPOSITORY?=github.com/gravitational/gravity
POLICY_OUT:=internal/policy/policy_embed.go
ASSETSDIR:=internal/policy/assets/$(OS_RELEASE)

all: buildbox $(POLICY_OUT)

$(POLICY_OUT): internal/policy/policy.go $(ASSETSDIR)/container.pp.bz2 $(ASSETSDIR)/gravity.pp.bz2 $(ASSETSDIR)/gravity.statedir.fc.template
	$(CONTAINER_RUNTIME) run \
		$(CONTAINER_RUN_ARGS) \
		--volume $(DIR):/go/src/$(GRAVITY_REPOSITORY) \
		--rm $(BUILDBOX) \
		/go/bin/generate

.PHONY: buildbox
buildbox: internal/generate/generate.go
	$(CONTAINER_RUNTIME) build \
		--tag $(BUILDBOX) \
		--file Dockerfile .
