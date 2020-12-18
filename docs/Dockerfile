# This Dockerfile makes the "build box": the container used to build
# Gravity documentation
FROM debian:jessie

ARG UID
ARG GID

ENV DEBIAN_FRONTEND noninteractive

RUN (apt-key update \
	&& apt-get -q -y update --fix-missing \
    && apt-get -q -y update \
	&& apt-get install -q -y apt-utils \
	&& apt-get install -q -y less \
	&& apt-get install -q -y locales) ;

RUN apt-get install -q -y \
         curl \
         make \
         git \
         gcc \
         tar \
         gzip \
         python \
         python-pip

RUN (pip install Jinja2==2.9.6 Markdown==2.6.8 docutils==0.13.1 click==4.1 \
     recommonmark==0.4.0 mkdocs==0.16.3  markdown-include==0.5.1 ;\
     apt-get -y autoclean; apt-get -y clean)

RUN getent group  $GID || groupadd builder --gid=$GID -o; \
    getent passwd $UID || useradd builder --uid=$UID --gid=$GID --create-home --shell=/bin/bash;

RUN (mkdir -p /var/lib/teleport && chown -R $UID:$GID /var/lib/teleport)

# Install SASS
RUN  apt-get install -q -y ruby-sass

ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    GOPATH="/gopath" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/gopath/bin"

ARG HOME
ARG PORT

VOLUME [$HOME]
EXPOSE $PORT
