# syntax=docker/dockerfile:1.2

ARG GO_VERSION=1.17.5
FROM golang:${GO_VERSION}-alpine AS gobase
RUN apk add --no-cache git build-base

FROM gobase AS builder
WORKDIR /go/src/github.com/gravitational/gravity
RUN --mount=target=.,rw --mount=target=/root/.cache,type=cache --mount=target=/go/pkg/mod,type=cache \
	set -ex && \
	go run -mod=vendor ./mage.go -debug -goos=linux -goarch=amd64 \
		-ldflags='-linkmode external -w -s -extldflags "-static"' \
		-compile /builder

FROM scratch AS releaser
COPY --from=builder /builder /

FROM releaser
