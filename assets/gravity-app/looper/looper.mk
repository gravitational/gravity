.PHONY: build run bash
BINARY=build/looper

#
# builds looper container
#
build: $(BINARY)
	docker build --tag=$(LOOPERTAG) -f looper.dockerfile .

#
# builds looper binary
#
$(BINARY): $(shell find -type f -name "*.go" -print) 
	@mkdir -p build
	go build -o $(BINARY) looper/main.go

#
# run looper on local docker (for development)
#
run: build
	@docker run -h looper \
		--volume=/bin:/bin \
		--volume=/lib:/lib \
		--volume=/usr:/usr \
		--volume=/lib64:/lib64 \
		--volume=/etc:/etc \
		$(LOOPERTAG) 

#
# run bash inside a local looper container (for development)
#
bash: build
	@docker run -ti -h looper \
		--volume=/bin:/bin \
		--volume=/lib:/lib \
		--volume=/usr:/usr \
		--volume=/lib64:/lib64 \
		--volume=/etc:/etc \
		--volume=/home/ekontsevoy:/data \
		$(LOOPERTAG) /bin/bash

#
# removes the binary+container
#
clean:
	rm -rf build/looper
	-docker rmi -f $(LOOPERTAG) 2>/dev/null || true
