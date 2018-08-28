ITAG=sample-app:1.0.2

#
# Builds a docker image and saves it to samlpe-app:1.0.2.tar
#
build:
	go build -o sample
	docker build --rm=true --tag=$(ITAG) .
	docker save --output=$(ITAG).tar $(ITAG) 
	docker rmi --force $(ITAG)
	rm -rf sample
