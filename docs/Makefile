DOCBOX=docs-buildbox:latest
HOME=/home
PORT=6600
DOCKERPORTS = -p $(PORT):$(PORT)
DOCKERFLAGS = --rm=true -v "$$(pwd)/../":$(HOME) -w $(HOME) -h docs
NOROOT=-u $$(id -u):$$(id -g)

# generate user-facing documentation
.PHONY:docs
docs: bbox
	@docker run $(NOROOT) $(DOCKERFLAGS) $(DOCBOX) docs/build.sh


# 'make run' starts running the docs
.PHONY:run
run: bbox
	@echo "\n\nYou have to open http://localhost:$(PORT)/overview/ in your local browser\n\n"
	docker run  $(NOROOT) $(DOCKERFLAGS) $(DOCKERPORTS) $(DOCBOX) \
		 docs/build.sh run

# install mkdocs on this machine 
.PHONY:mkdocs
mkdocs:
	pip install -q -U mkdocs

# builds a docker container which is used for running `mkdocs`
.PHONY:bbox
bbox:
	docker build \
		--build-arg UID=$$(id -u) \
		--build-arg GID=$$(id -g) \
		--build-arg HOME=$(HOME) \
		--build-arg PORT=$(PORT) \
		--tag $(DOCBOX) .


# make shell drops you into bash shell of mkdocs container
.PHONY:shell
shell:bbox
	docker run -ti $(NOROOT) $(DOCKERFLAGS) $(DOCKERPORTS) $(DOCBOX) \
		/bin/bash
