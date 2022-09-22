# This Dockerfile makes the container used to build Gravity documentation
#
# A multi step build is used to keep the go toolchain outside the final container

# milv-builder, contains the whole go toolchain
FROM golang:1.19-buster AS milv-builder
RUN GO111MODULE=on go install github.com/magicmatatjahu/milv@v0.0.6

# docbox, contains everything for building gravity documentation
FROM quay.io/gravitational/mkdocs-base:1.0.3-1

ARG UID
ARG GID
ARG PORT

RUN getent group  $GID || groupadd builder --gid=$GID -o; \
    getent passwd $UID || useradd builder --uid=$UID --gid=$GID --create-home --shell=/bin/bash;

COPY --from=milv-builder /go/bin/milv /usr/bin/milv

EXPOSE $PORT
