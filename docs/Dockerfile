# This Dockerfile makes the "build box": the container used to build
# Gravity documentation
FROM quay.io/gravitational/mkdocs-base:1.0.3-1

ARG UID
ARG GID
ARG WORKDIR
ARG PORT

ENV DEBIAN_FRONTEND noninteractive

RUN groupadd jenkins --gid=$GID -o && useradd jenkins --uid=$UID --gid=$GID --create-home --shell=/bin/sh
RUN (mkdir -p /var/lib/teleport && chown -R jenkins /var/lib/teleport)

# Install SASS
RUN apt-get update -y && apt-get install -q -y ruby-sass parallel
             
ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    GOPATH="/gopath" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/gopath/bin"

VOLUME [$WORKDIR]
EXPOSE $PORT
