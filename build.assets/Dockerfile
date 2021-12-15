FROM quay.io/gravitational/debian-venti:go1.17.5-stretch

ARG PROTOC_VER
ARG PROTOC_PLATFORM
ARG GOGO_PROTO_TAG
ARG GRPC_GATEWAY_TAG
ARG VERSION_TAG
ARG GOLANGCI_LINT_VER
ARG UID
ARG GID

ENV TARBALL protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip
ENV GRPC_GATEWAY_ROOT /gopath/src/github.com/grpc-ecosystem/grpc-gateway
ENV GOGOPROTO_ROOT /gopath/src/github.com/gogo/protobuf
ENV PROTOC_URL https://github.com/google/protobuf/releases/download/v${PROTOC_VER}/protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip

# install development libraries used when compiling fio
RUN apt-get -q -y update --fix-missing && apt-get -q -y install libaio-dev zlib1g-dev

RUN getent group  $GID || groupadd builder --gid=$GID -o; \
    getent passwd $UID || useradd builder --uid=$UID --gid=$GID --create-home --shell=/bin/sh;

RUN set -ex && (mkdir -p /opt/protoc && \
     mkdir -p /.cache && \
     chown -R $UID:$GID /gopath && \
     chown -R $UID:$GID /opt/protoc && \
     chmod 777 /.cache && \
     chmod 777 /tmp)

USER $UID:$GID

ENV LANGUAGE="en_US.UTF-8" \
     LANG="en_US.UTF-8" \
     LC_ALL="en_US.UTF-8" \
     LC_CTYPE="en_US.UTF-8" \
     GOPATH="/gopath" \
     PATH="$PATH:/opt/protoc/bin:/opt/go/bin:/gopath/bin"

RUN set -ex && (mkdir -p /gopath/src/github.com/gravitational && \
     cd /gopath/src/github.com/gravitational && \
     git clone https://github.com/gravitational/version.git && \
     cd /gopath/src/github.com/gravitational/version && \
     git checkout ${VERSION_TAG} && \
     go install github.com/gravitational/version/cmd/linkflags@0.0.2)

RUN set -ex && (wget --quiet -O /tmp/${TARBALL} ${PROTOC_URL} && \
     unzip -d /opt/protoc /tmp/${TARBALL} && \
     mkdir -p /gopath/src/github.com/gogo/ /gopath/src/github.com/grpc-ecosystem && \
     git clone https://github.com/gogo/protobuf --branch ${GOGO_PROTO_TAG} /gopath/src/github.com/gogo/protobuf && cd /gopath/src/github.com/gogo/protobuf && make install && \
     git clone https://github.com/grpc-ecosystem/grpc-gateway --branch ${GRPC_GATEWAY_TAG} /gopath/src/github.com/grpc-ecosystem/grpc-gateway && cd /gopath/src/github.com/grpc-ecosystem/grpc-gateway && pwd && go install ./protoc-gen-grpc-gateway)

ENV GOLANGCI_RELEASE_TARBALL https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VER}/golangci-lint-${GOLANGCI_LINT_VER}-linux-amd64.tar.gz

RUN set -ex && \
	curl -sSfL ${GOLANGCI_RELEASE_TARBALL} | tar xz --strip-components=1 -C $(go env GOPATH)/bin golangci-lint-${GOLANGCI_LINT_VER}-linux-amd64/golangci-lint

ENV PROTO_INCLUDE "/usr/local/include":"/gopath/src":"${GRPC_GATEWAY_ROOT}/third_party/googleapis":"${GOGOPROTO_ROOT}/gogoproto"


VOLUME ["/gopath/src/github.com/gravitational/gravity"]
