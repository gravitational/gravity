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

VOLUME [$WORKDIR]
EXPOSE $PORT
