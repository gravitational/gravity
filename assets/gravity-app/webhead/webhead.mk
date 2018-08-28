.PHONY: build run bash
BINARY=build/webhead

#
# builds looper container
#
build: $(BINARY)
	docker build --tag=$(WEBHEADTAG) -f webhead.dockerfile .

#
# builds ze binary
#
$(BINARY): $(shell find -type f -name "*.go" -print) 
	@mkdir -p build
	go build -o $(BINARY) webhead/main.go


#
# runs webhead inside a docker container
#
run: webhead
	@docker run -h webhead \
		--volume=/bin:/bin \
		--volume=/lib:/lib \
		--volume=/usr:/usr \
		--volume=/lib64:/lib64 \
		--volume=/etc:/etc \
		$(TAG) 

#
# runs bash inside a webhead container
#
bash: webhead
	@docker run -ti -h webhead \
		--volume=/bin:/bin \
		--volume=/lib:/lib \
		--volume=/usr:/usr \
		--volume=/lib64:/lib64 \
		--volume=/etc:/etc \
		$(WEBHEADTAG) /bin/bash

#
# removes the binary+container
#
clean:
	rm -rf build/webhead
	-docker rmi -f $(WEBHEADTAG) 2>/dev/null || true
