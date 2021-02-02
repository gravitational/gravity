FROM quay.io/gravitational/debian-venti:go1.12.9-stretch

ARG PROTOC_VER
ARG PROTOC_PLATFORM
ARG GOGO_PROTO_TAG
ARG GRPC_GATEWAY_TAG
ARG GODEP_TAG
ARG VERSION_TAG
ARG UID
ARG GID

ENV TARBALL protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip
ENV GRPC_GATEWAY_ROOT /src/github.com/grpc-ecosystem/grpc-gateway
ENV GOGOPROTO_ROOT /src/github.com/gogo/protobuf
ENV PROTOC_URL https://github.com/google/protobuf/releases/download/v${PROTOC_VER}/protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip

RUN getent group  $GID || groupadd builder --gid=$GID -o; \
    getent passwd $UID || useradd builder --uid=$UID --gid=$GID --create-home --shell=/bin/sh;

RUN (set -ex && mkdir -p /gopath/src/github.com/gravitational/gravity && \
     chown -R $UID:$GID /gopath && \
     mkdir -p /.cache && \
     chmod 777 /.cache && \
     chmod 777 /tmp)

USER $UID:$GID

ENV LANGUAGE="en_US.UTF-8" \
     LANG="en_US.UTF-8" \
     LC_ALL="en_US.UTF-8" \
     LC_CTYPE="en_US.UTF-8" \
     GOPATH="/gopath" \
     PATH="$PATH:/opt/protoc/bin:/opt/go/bin:/gopath/bin"

RUN (set -ex && mkdir -p /gopath/src/github.com/gravitational && \
     cd /gopath/src/github.com/gravitational && \
     git clone https://github.com/gravitational/version.git && \
     cd /gopath/src/github.com/gravitational/version && \
     git checkout ${VERSION_TAG} && \
     go install github.com/gravitational/version/cmd/linkflags)

RUN (set -ex && mkdir -p /opt/protoc && \
     wget --quiet -O /tmp/${TARBALL} ${PROTOC_URL} && \
     unzip -d /opt/protoc /tmp/${TARBALL} && \
     mkdir -p /src/github.com/gogo/ /src/github.com/grpc-ecosystem && \
     git clone https://github.com/gogo/protobuf --branch ${GOGO_PROTO_TAG} /src/github.com/gogo/protobuf && cd /src/github.com/gogo/protobuf && make install && \
     git clone https://github.com/grpc-ecosystem/grpc-gateway --branch ${GRPC_GATEWAY_TAG} /src/github.com/grpc-ecosystem/grpc-gateway && cd /src/github.com/grpc-ecosystem/grpc-gateway && pwd && go install ./protoc-gen-grpc-gateway)

ENV PROTO_INCLUDE "/usr/local/include":"/src":"${GRPC_GATEWAY_ROOT}/third_party/googleapis":"${GOGOPROTO_ROOT}/gogoproto"

# install DEP tool
RUN set -ex && wget --quiet -O /usr/bin/dep https://github.com/golang/dep/releases/download/${GODEP_TAG}/dep-linux-amd64 && chmod +x /usr/bin/dep
RUN set -ex && chmod -R a+rw /gopath

# install some development libraries used when compiling fio
RUN set -ex && apt -q -y update --fix-missing && apt -q -y install libaio-dev zlib1g-dev

VOLUME ["/gopath/src/github.com/gravitational/gravity"]
