DOCBOX=docs-buildbox:latest
BUILDROOT:=../build
BUILDDIR:=$(BUILDROOT)/docs
DOCBOX_ID_FILE=$(BUILDROOT)/.docs-buildbox.id
WORKDIR=/home
PORT=6601
DOCKERPORTS = -p $(PORT):$(PORT)
NOROOT=-u $$(id -u):$$(id -g)
DOCKERFLAGS = --rm=true $(NOROOT) -v "$$(pwd)/../":$(WORKDIR) -w $(WORKDIR)/docs -h docs
DOCKERMAKE=docker run $(DOCKERFLAGS) $(DOCBOX) make
VERSIONS=4.x 5.x 6.x 7.x 8.x
LATEST=$(lastword $(VERSIONS))
CFGS=$(addsuffix .yaml, $(VERSIONS))
LATEST_CFG=$(lastword $(CFGS))
RUN_CFG?=$(LATEST_CFG)

# generate user-facing documentation
.PHONY:docs
docs: $(DOCBOX_ID_FILE)
	$(DOCKERMAKE) container-compile-docs

# run static analysis against the docs
.PHONY: lint
lint: $(DOCBOX_ID_FILE)
	$(DOCKERMAKE) container-lint

# run the docs in development mode
#
# requires DOCKERPORTS, which should not be shared with other targets due to
# port collision. E.g. If other targets use DOCKERPORTS, multiple `make docs`
# cannot run at the same time on the same server, which CI doesn't like --2020-03 walt
.PHONY:run
run: $(DOCBOX_ID_FILE) $(RUN_CFG)
	@echo "\n\nOpen http://localhost:$(PORT)/ in your local browser\n\n"
	docker run $(DOCKERFLAGS) $(DOCKERPORTS) $(DOCBOX) make BUILDROOT=$(BUILDROOT) container-run RUN_CFG=$(RUN_CFG)

# drops you into bash shell of mkdocs container
.PHONY:shell
shell: $(DOCBOX_ID_FILE)
	docker run -ti $(DOCKERFLAGS) $(DOCBOX) /bin/bash

$(DOCBOX_ID_FILE): Dockerfile
	mkdir -p $(BUILDDIR)
	docker build \
		--build-arg UID=$$(id -u) \
		--build-arg GID=$$(id -g) \
		--build-arg WORKDIR=$(WORKDIR) \
		--build-arg PORT=$(PORT) \
		--tag $(DOCBOX) .
	# prefer --iidfile to touch, but jenkin's docker is too old. See ops issue #141
	touch $(DOCBOX_ID_FILE)

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
	mkdocs serve --strict --livereload --config-file $(RUN_CFG) --dev-addr=0.0.0.0:$(PORT)


.PHONY:container-compile-docs
container-compile-docs: $(VERSIONS) $(LATEST_SYMLINK)

.PHONY:$(VERSIONS)
$(VERSIONS): %.x:
	mkdocs build --strict --config-file $(*F).x.yaml --site-dir $(BUILDDIR)/$(*F).x

LINTS=$(addprefix lint-, $(VERSIONS))

.PHONY:container-lint
container-lint: $(LINTS)

$(LINTS): lint-%:
	cd $(*F) && milv -ignore-external

.PHONY:$(LATEST_SYMLINK)
$(LATEST_SYMLINK): $(LATEST)
	cd $(BUILDDIR) && rm -f latest && ln -s $(LATEST) latest
