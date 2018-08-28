PWD = $(shell pwd)
REPODIR = $(abspath $(PWD)/../)
export

TAG := dns-app-hooks:$(DNS_APP_TAG)
CONTAINERS := $(TAG)
REPO_URL := leader.telekube.local:5000

.PHONY: all
all: hooks

.PHONY: hooks
hooks:
	$(eval CHANGESET = $(shell echo $$DNS_APP_TAG | sed -e 's/[\.]//g'))
	if [ -z "$(CHANGESET)" ]; then \
	  echo "DNS_APP_TAG is not set"; exit 1; \
	fi;
	docker build --build-arg CHANGESET=dns-$(CHANGESET) -t $(TAG) .

.PHONY: deploy
deploy:
	$(foreach c,$(CONTAINERS), \
		docker tag $(c) $(REPO_URL)/$(c) ; \
		docker push $(REPO_URL)/$(c) ; )
