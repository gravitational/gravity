# site.mk defines how to build and test 'site api' component of Gravity
#
# Examples:
#
#     make site
#     make test-site
#     make clean-site
#
# NOTES:
# 	Tests need Vagrant-based k8s cluster to be up and running.
# 	'make site-test' will try to bring it up for you.
#
.PHONY: site test clear-docker prepare-vagrant clean-site site-clean sample-apps

# These variables point to Kubernetes-master server running on 
# Vagrant (assets/site/Vagrantfile). 
#
# WARNING: they're used outside of this make file : tests call os.Getenv()
K8S_IP=10.0.10.100
KUBEAPI=$(K8S_IP):8080
ETCDAPI=$(K8S_IP):2379
REGISTRY=$(K8S_IP):5000
ASSETSDIR=assets/site

SITEPACKAGES=$$(find lib/site -type d)

#
# 'make site' builds the Gravity site and its sample app
#
site: build/sitectl
build/sitectl: $(shell find tool lib -type f -name *.go) 
	@mkdir -p build
	go install github.com/gravitational/gravity/tool/sitectl
	cp -f $(GOPATH)/bin/sitectl build/
	@echo "done --> ./build/sitectl"


# runs 'site' tests
test-site: site-test
site-test: site sample-apps clear-docker 
	@for package in $(SITEPACKAGES) ; do \
		echo "\nTesting $$package ..." ;\
		go test -v $(REPO)/$$package ;\
	done

# builds a sample applicatin tarbal. this includes building Docker images,
# exporting them into .tar files and creating a resulting tarball
sample-apps: build/sample-app.tar.gz $(ASSETSDIR)/fixtures/tiny-image/tiny-binary\:5.0.0.tar
build/sample-app.tar.gz:
	@mkdir -p build
	cd $(ASSETSDIR)/fixtures/sample-app/images/bash-app; make
	cd $(ASSETSDIR)/fixtures/sample-app/images/sample-app; make
	cd $(ASSETSDIR)/fixtures/sample-app; tar -czf ../../../../build/sample-app.tar.gz *

$(ASSETSDIR)/fixtures/tiny-image/tiny-binary\:5.0.0.tar:
	make -C $(ASSETSDIR)/fixtures/tiny-image

# checks if vagrant-based k8s cluster is up and running and starts it 
# if it's not
prepare-vagrant: 
	@if [ ! $$(which ruby) ] ; then \
		echo "ERROR:\nRuby is not found. Ruby is needed to launch Vagrant-based Kubernetes cluster to run tests\n" ;\
		exit 1 ;\
	fi
	@ruby $(ASSETSDIR)/test_helper.rb 

clear-docker: prepare-vagrant
	@docker rmi $$(docker images -f "dangling=true" -q) 2>/dev/null || true

# deletes temporary data created by site building/testing
site-clean: clean-site
clean-site:
	@mkdir -p build
	rm -rf build/sample-app.tar.gz
	find assets/site/fixtures -name "*tar" -delete
	rm -rf build/sitectl

# Ev's "dev shortcuts"
z: sample-apps
	go test -v -short github.com/gravitational/gravity/lib/site/docker
