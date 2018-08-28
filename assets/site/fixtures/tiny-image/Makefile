ITAG=tiny-binary:5.0.0
TARBALL=$(ITAG).tar

#
# Builds a docker image and saves it to samlpe-app:1.0.2.tar
#
build: tinybin
	docker build --rm=true --tag=$(ITAG) .
	docker save --output=$(TARBALL) $(ITAG) 
	docker rmi --force $(ITAG)

tinybin: *.c Makefile
	musl-gcc --static -o tinybin tiny.c
	strip tinybin
