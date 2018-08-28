OPS_URL := https://opscenter.localhost.localdomain:33009
REPOSITORY := gravitational.io
NAME := test-app
VER := 0.0.1

.PHONY: import
import:
	-gravity app delete --ops-url=$(OPS_URL) $(REPOSITORY)/$(NAME):$(VER) --force --insecure
	gravity app import --vendor --ops-url=$(OPS_URL) --insecure --repository=$(REPOSITORY) --name=$(NAME) --version=$(VER) --glob=**/*.yaml --registry-url=apiserver:5000 --debug .
