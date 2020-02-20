DOCBOX=docs-buildbox:latest
BUILDROOT:=$(if $(GRAVITY_BUILDDIR),$(GRAVITY_BUILDDIR),../build)
BUILDDIR:=$(BUILDROOT)/docs
DOCBOX_ID_FILE=$(BUILDROOT)/.docs-buildbox.id
HOME=/home
PORT=6601
DOCKERPORTS = -p $(PORT):$(PORT)
DOCKERFLAGS = --rm=true -v "$$(pwd)/../":$(HOME) -w $(HOME)/docs -h docs
NOROOT=-u $$(id -u):$$(id -g)
VERSIONS=4.x 5.x 6.x 7.x
LATEST=$(lastword $(VERSIONS))
CFGS=$(addsuffix .yaml, $(VERSIONS))
LATEST_CFG=$(lastword $(CFGS))
RUN_CFG?=$(LATEST_CFG)

# generate user-facing documentation
.PHONY:docs
docs: $(DOCBOX_ID_FILE)
	docker run $(NOROOT) $(DOCKERFLAGS) $(DOCBOX) make container-compile-docs

# run the docs in development mode
.PHONY:run
run: $(DOCBOX_ID_FILE) $(RUN_CFG)
	@echo "\n\nOpen http://localhost:$(PORT)/overview/ in your local browser\n\n"
	docker run  $(NOROOT) $(DOCKERFLAGS) $(DOCKERPORTS) $(DOCBOX) make container-run RUN_CFG=$(RUN_CFG)

# drops you into bash shell of mkdocs container
.PHONY:shell
shell: $(DOCBOX_ID_FILE)
	docker run -ti $(NOROOT) $(DOCKERFLAGS) $(DOCKERPORTS) $(DOCBOX) \
		/bin/bash

$(DOCBOX_ID_FILE): Dockerfile
	mkdir -p $(BUILDDIR)
	docker build \
		--build-arg UID=$$(id -u) \
		--build-arg GID=$$(id -g) \
		--build-arg HOME=$(HOME) \
		--build-arg PORT=$(PORT) \
	    --iidfile $(DOCBOX_ID_FILE) \
		--tag $(DOCBOX) .

# builds a docker container which is used for running `mkdocs`
.PHONY:bbox
bbox: $(DOCBOX_ID_FILE)

.PHONY:clean
clean:
	rm -rf $(DOCBOX_ID_FILE) $(BUILDDIR)


################################################################################
# Targets & variables for use inside the docs-buildbox container
################################################################################
LATEST_SYMLINK=$(BUILDDIR)/latest

.PHONY:container-run
container-run: $(RUN_VERSION)
	@echo "Starting mkdocs server..."
	mkdocs serve --livereload --config-file $(RUN_CFG) --dev-addr=0.0.0.0:$(PORT)

.PHONY:container-compile-docs
container-compile-docs: $(VERSIONS) $(LATEST_SYMLINK)

.PHONY:$(VERSIONS)
$(VERSIONS): %.x:
	mkdocs build --config-file $(*F).x.yaml

.PHONY:$(LATEST_SYMLINK)
$(LATEST_SYMLINK): $(LATEST)
	cd $(BUILDDIR) && rm -f latest && ln -s $(LATEST) latest
