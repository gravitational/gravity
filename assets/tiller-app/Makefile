REPOSITORY := gravitational.io
NAME := tiller-app
TILLER_VERSION ?= 1.0.0
VERSION ?= 2.2.0
OPS_URL ?= https://opscenter.localhost.localdomain:33009
GRAVITY ?= gravity
UPDATE_METADATA_OPTS := --repository=$(REPOSITORY) --name=$(NAME) --version=$(VERSION)

TILLER_IMAGE ?= ghcr.io/helm/tiller:v$(TILLER_VERSION)

.PHONY: import
import:
	-$(GRAVITY) app delete --ops-url=$(OPS_URL) $(REPOSITORY)/$(NAME):$(VERSION) \
		--force --insecure
	$(GRAVITY) app import --insecure --vendor \
		--ops-url=$(OPS_URL) \
		$(UPDATE_METADATA_OPTS) \
		--set-image=$(TILLER_IMAGE) \
		--include=resources --include=registry .

