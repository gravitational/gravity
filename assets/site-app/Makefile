REPODIR ?= $(shell realpath ../../)
SITEDIR ?= $(shell realpath ./images/site)

GRAVITY ?= gravity
REPOSITORY := gravitational.io
NAME := site
OPS_URL ?= https://opscenter.localhost.localdomain:33009
UPDATE_METADATA_OPTS := --repository=$(REPOSITORY) --name=$(NAME) --version=$(VERSION)
export VERSION ?=


.PHONY: all
all: images

.PHONY: images
images:
	$(MAKE) -C images all

.PHONY: import
import: images
	-$(GRAVITY) app delete --ops-url=$(OPS_URL) $(REPOSITORY)/$(NAME):$(VERSION) \
		--force --insecure
	$(GRAVITY) app import --insecure --vendor \
		--ops-url=$(OPS_URL) \
		--set-image=site-app-hook:$(VERSION) \
		--set-image=gravity-site:$(VERSION) \
		$(UPDATE_METADATA_OPTS) \
		--include=resources --include=registry .

# development targets below

.PHONY: login
login:
	kubectl config set-cluster local --server=http://leader.telekube.local:8080
	kubectl config set-context local --cluster=local
	kubectl config use-context local


.PHONY: deploy
deploy:
	$(MAKE) -C images deploy

.PHONY: reinstall
reinstall:
	$(MAKE) -C ../.. goinstall
	$(MAKE) uninstall images install

.PHONY: uninstall
uninstall:
	-kubectl delete --namespace=kube-system deployments/gravity-site
	-kubectl delete --namespace=kube-system configmaps/gravity-site

.PHONY: install
install:
	kubectl create configmap gravity-site --namespace=kube-system --from-file=./resources/config
	kubectl create -f resources/deployment.yaml
