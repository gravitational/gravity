PWD = $(shell pwd)
REPODIR = $(abspath $(PWD)/../)
export

SITE_TAG := gravity-site:$(VERSION)
HOOK_TAG := site-app-hook:$(VERSION)
CONTAINERS := $(SITE_TAG) $(HOOK_TAG)
REPO_URL := leader.telekube.local:5000
DOCKER_ARGS ?= --pull

.PHONY: all
all: site hook

.PHONY: hook
hook:
	$(eval CHANGESET = $(shell echo $$VERSION | sed -e 's/[\.]//g'))
	if [ -z "$(CHANGESET)" ]; then \
	  echo "VERSION is not set"; exit 1; \
	fi;
	docker build --build-arg CHANGESET=site-$(CHANGESET) $(DOCKER_ARGS) -t $(HOOK_TAG) hook

.PHONY: site
site:
	docker build -t $(SITE_TAG) site

.PHONY: deploy
deploy:
	$(foreach c,$(CONTAINERS), \
		docker tag $(c) $(REPO_URL)/$(c) ; \
		docker push $(REPO_URL)/$(c) ; )
