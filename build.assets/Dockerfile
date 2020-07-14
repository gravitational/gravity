# syntax=docker/dockerfile:1.1.7-experimental

ARG GOLANG_VER
FROM quay.io/gravitational/debian-venti:go${GOLANG_VER}

ARG PROTOC_VER
ARG PROTOC_PLATFORM
ARG GOGO_PROTO_TAG
ARG GRPC_GATEWAY_TAG
ARG GOLANGCI_VER
ARG UID
ARG GID

ENV TARBALL protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip
ENV GRPC_GATEWAY_ROOT /gopath/src/github.com/grpc-ecosystem/grpc-gateway
ENV GOGOPROTO_ROOT /gopath/src/github.com/gogo/protobuf
ENV PROTOC_URL https://github.com/google/protobuf/releases/download/v${PROTOC_VER}/protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip

RUN  groupadd jenkins --gid=$GID -o && \
     useradd jenkins --uid=$UID --gid=$GID --create-home --shell=/bin/sh

RUN curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $GOPATH/bin ${GOLANGCI_VER}

     # install development libraries used when compiling fio
RUN --mount=type=cache,sharing=locked,id=gravity-aptlib,target=/var/lib/apt \
    --mount=type=cache,sharing=locked,id=gravity-aptcache,target=/var/cache/apt \
          DEBIAN_FRONTEND=noninteractive apt-get -q -y update --fix-missing && apt-get -q -y install libaio-dev zlib1g-dev

RUN (mkdir -p /opt/protoc && \
     mkdir -p /.cache && \
     chown -R jenkins:jenkins /gopath && \
     chown -R jenkins:jenkins /opt/protoc && \
     chmod 777 /.cache && \
     chmod 777 /tmp)

USER jenkins
ENV LANGUAGE="en_US.UTF-8" \
     LANG="en_US.UTF-8" \
     LC_ALL="en_US.UTF-8" \
     LC_CTYPE="en_US.UTF-8" \
     GOPATH="/gopath" \
     PATH="$PATH:/opt/protoc/bin:/opt/go/bin:/gopath/bin"

RUN (wget --quiet -O /tmp/${TARBALL} ${PROTOC_URL} && \
     unzip -d /opt/protoc /tmp/${TARBALL} && \
     mkdir -p /gopath/src/github.com/gogo/ /gopath/src/github.com/grpc-ecosystem && \
     git clone https://github.com/gogo/protobuf --branch ${GOGO_PROTO_TAG} /gopath/src/github.com/gogo/protobuf && cd /gopath/src/github.com/gogo/protobuf && make install && \
     git clone https://github.com/grpc-ecosystem/grpc-gateway --branch ${GRPC_GATEWAY_TAG} /gopath/src/github.com/grpc-ecosystem/grpc-gateway && cd /gopath/src/github.com/grpc-ecosystem/grpc-gateway && pwd && go install ./protoc-gen-grpc-gateway)

ENV PROTO_INCLUDE "/usr/local/include":"/gopath/src":"${GRPC_GATEWAY_ROOT}/third_party/googleapis":"${GOGOPROTO_ROOT}/gogoproto"


VOLUME ["/gopath/src/github.com/gravitational/gravity"]
