# This Dockerfile makes the "build box": the container used to build
# Gravity documentation
FROM quay.io/gravitational/mkdocs-base:1.0.3-1

ARG UID
ARG GID
ARG WORKDIR
ARG PORT

RUN groupadd jenkins --gid=$GID -o && useradd jenkins --uid=$UID --gid=$GID --create-home --shell=/bin/sh

# Install SASS
RUN apt-get update -y && apt-get install -q -y ruby-sass

# Install Go & milv, go version doesn't particularly matter as it isn't used in the product
RUN mkdir -p /opt /gopath
ENV GOROOT="/opt/go" \
    GOPATH="/gopath" \
    PATH="$PATH:/opt/go/bin:/gopath/bin"
RUN cd /opt && curl https://dl.google.com/go/go1.14.linux-amd64.tar.gz | tar xz
RUN GO111MODULE=on go get -u -v github.com/magicmatatjahu/milv@v0.0.6

VOLUME [$WORKDIR]
EXPOSE $PORT
