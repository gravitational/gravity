FROM golang:1.12.9-buster

RUN set -ex && \
	mkdir -p /go/src/github.com/gravitational/gravity/lib/system/selinux/internal/generate && \
	mkdir -p /go/src/github.com/gravitational/gravity/lib/system/selinux/internal/policy/assets/centos

COPY ./internal/policy/policy.go /go/src/github.com/gravitational/gravity/lib/system/selinux/internal/policy/policy.go
COPY ./internal/generate /go/src/github.com/gravitational/gravity/lib/system/selinux/internal/generate/

RUN set -ex && \
	go get -u github.com/gravitational/vfsgen && \
	cd /go/src/github.com/gravitational/vfsgen && \
	go install -tags generate_policy github.com/gravitational/gravity/lib/system/selinux/internal/generate

WORKDIR "/go/src/github.com/gravitational/gravity/lib/system/selinux/internal/policy/"
