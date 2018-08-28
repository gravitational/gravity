ITAG=busy-bash:1.0.2

#
# Builds a Docker image and saves it to busy-bash:1.0.2.tar
#
build:
	docker build --rm=true --tag=$(ITAG) .
	docker save --output=$(ITAG).tar $(ITAG) 
	docker rmi --force $(ITAG)
