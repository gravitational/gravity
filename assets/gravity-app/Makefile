.PHONY:build

export LOOPERTAG=looper:1.0.0
export WEBHEADTAG=webhead:1.0.0

CONTAINERS=$(LOOPERTAG) $(WEBHEADTAG)
REGISTRYIMAGE=registry:2.1.1
REGISTRYPORT=5055
RUNNINGREG=$$(docker ps -q --filter=ancestor=$(REGISTRYIMAGE))
REGADDRESS=127.0.0.1:$(REGISTRYPORT)
OUT=build/gravity-app.tar.gz
APPDIR=build/app

#
# makes a Gravity App with 'looper' and 'webhead' pods
#
app: containers
	$(MAKE) start-registry
	$(MAKE) push-layers-to-registry
	$(MAKE) export-layers-from-registry
	$(MAKE) make-tarball
	$(MAKE) stop-registry


make-tarball:
	@echo "Making a tarball...."
	cp -r resources $(APPDIR)/
	tar -cvzf $(OUT) -C build/app .
	@echo "\n\nDONE -------> $(OUT)\n\n"


# pushes images from local Docker to temporary registry. THIS TAKES A LOT OF TIME.
push-layers-to-registry:
	@mkdir -p $(APPDIR)
	for container in $(CONTAINERS); do \
		echo "docker tag $$container $(REGADDRESS)/$$container" ;\
		docker tag $$container $(REGADDRESS)/$$container ;\
		docker push $(REGADDRESS)/$$container ;\
		docker rmi $(REGADDRESS)/$$container ;\
		echo "\n" ;\
	done


# exports /var/lib/registry from temporary registry container into 'registry' folder
export-layers-from-registry:
	@echo "Copying layres from temporary registry into registry folder..."
	@rm -rf $(APPDIR)/registry
	@mkdir -p $(APPDIR)/registry
	@docker cp $(RUNNINGREG):/var/lib/registry $(APPDIR)/registry


# builds two containers for the app
containers:
	$(MAKE) -f looper/looper.mk
	$(MAKE) -f webhead/webhead.mk


clean:
	$(MAKE) -f looper/looper.mk clean
	$(MAKE) -f webhead/webhead.mk clean
	rm -rf build


# starts the temporary docker registry
start-registry:
	$(MAKE) stop-registry
	@if [ -z "$(RUNNINGREG)" ]; then \
		docker run -d -p $(REGISTRYPORT):5000 $(REGISTRYIMAGE) ;\
		echo "Started temporary Docker registry on port $(REGISTRYPORT)\n" ;\
		sleep 2 ;\
	else \
		echo "Temporary Docker registry is already listening on port $(REGISTRYPORT)\n" ;\
	fi


# stops the temporary docker registry
stop-registry:
	@if [ ! -z "$(RUNNINGREG)" ]; then \
		docker kill $(RUNNINGREG) ;\
	else \
		echo registry is not running ;\
	fi


run-loooper:
	$(MAKE) -f looper/looper.mk run


bash-loooper:
	$(MAKE) -f looper/looper.mk bash


run-webhead:
	$(MAKE) -f webhead/webhead.mk run


bash-webhead:
	$(MAKE) -f webhead/webhead.mk bash
