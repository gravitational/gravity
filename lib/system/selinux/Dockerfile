FROM golang:1.12.9-buster

RUN set -x && \
	go get -u github.com/gravitational/vfsgen && \
	cd /go/src/github.com/gravitational/vfsgen && git checkout dmitri/encode-bytes && \
	go install -tags generate_policy github.com/gravitational/gravity/lib/system/selinux/internal/generate

WORKDIR "/go/src/github.com/gravitational/gravity/lib/system/selinux/"
