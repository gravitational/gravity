FROM golang:1.12.9-buster

RUN set -x && \
	go get -u github.com/shurcooL/vfsgen && \
	go install -tags generate_policy github.com/gravitational/gravity/lib/system/selinux/generate

WORKDIR "/go/src/github.com/gravitational/gravity/lib/system/selinux/"
