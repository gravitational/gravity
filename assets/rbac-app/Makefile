export VERSION ?= $(shell git describe --tags)
REPOSITORY := gravitational.io
NAME := rbac-app
OPS_URL ?= https://opscenter.localhost.localdomain:33009
OUT ?= $(NAME).tar.gz
GRAVITY ?= gravity
UPDATE_METADATA_OPTS := --repository=$(REPOSITORY) --name=$(NAME) --version=$(VERSION)
export

.PHONY: all
all:

.PHONY: import
import:
	-$(GRAVITY) app delete --ops-url=$(OPS_URL) $(REPOSITORY)/$(NAME):$(VERSION) \
		--force --insecure
	$(GRAVITY) app import --insecure --vendor \
		--ops-url=$(OPS_URL) \
		$(UPDATE_METADATA_OPTS) \
		--include=resources --include=registry .

