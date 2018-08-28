REPOSITORY := gravitational.io
NAME := dns-app
DNS_APP_TAG ?= 0.0.4
OPS_URL ?= https://opscenter.localhost.localdomain:33009
GRAVITY ?= gravity
UPDATE_METADATA_OPTS := --repository=$(REPOSITORY) --name=$(NAME) --version=$(DNS_APP_TAG)

.PHONY: hooks
hooks:
	$(MAKE) -C hooks all

.PHONY: import
import: hooks
	-$(GRAVITY) app delete --ops-url=$(OPS_URL) $(REPOSITORY)/$(NAME):$(DNS_APP_TAG) \
		--force --insecure
	$(GRAVITY) app import --insecure --vendor \
		--ops-url=$(OPS_URL) \
		$(UPDATE_METADATA_OPTS) \
		--set-image=dns-app-hooks:$(DNS_APP_TAG) \
		--include=resources --include=registry .
