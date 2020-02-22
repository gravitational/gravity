FROM quay.io/gravitational/debian-venti:go1.12.9-stretch

ARG PROTOC_VER
ARG PROTOC_PLATFORM
ARG GOGO_PROTO_TAG
ARG GRPC_GATEWAY_TAG
ARG VERSION_TAG

ENV TARBALL protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip
ENV GRPC_GATEWAY_ROOT /src/github.com/grpc-ecosystem/grpc-gateway
ENV GOGOPROTO_ROOT /src/github.com/gogo/protobuf
ENV PROTOC_URL https://github.com/google/protobuf/releases/download/v${PROTOC_VER}/protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip

ENV LANGUAGE="en_US.UTF-8" \
     LANG="en_US.UTF-8" \
     LC_ALL="en_US.UTF-8" \
     LC_CTYPE="en_US.UTF-8" \
     GOPATH="/gopath" \
     PATH="$PATH:/opt/protoc/bin:/opt/go/bin:/gopath/bin"

RUN (mkdir -p /gopath/src/github.com/gravitational && \
     cd /gopath/src/github.com/gravitational && \
     git clone https://github.com/gravitational/version.git && \
     cd /gopath/src/github.com/gravitational/version && \
     git checkout ${VERSION_TAG} && \
     go install github.com/gravitational/version/cmd/linkflags)

RUN (mkdir -p /opt/protoc && \
     wget --quiet -O /tmp/${TARBALL} ${PROTOC_URL} && \
     unzip -d /opt/protoc /tmp/${TARBALL} && \
     mkdir -p /src/github.com/gogo/ /src/github.com/grpc-ecosystem && \
     git clone https://github.com/gogo/protobuf --branch ${GOGO_PROTO_TAG} /src/github.com/gogo/protobuf && cd /src/github.com/gogo/protobuf && make install && \
     git clone https://github.com/grpc-ecosystem/grpc-gateway --branch ${GRPC_GATEWAY_TAG} /src/github.com/grpc-ecosystem/grpc-gateway && cd /src/github.com/grpc-ecosystem/grpc-gateway && pwd && go install ./protoc-gen-grpc-gateway)

ENV PROTO_INCLUDE "/usr/local/include":"/src":"${GRPC_GATEWAY_ROOT}/third_party/googleapis":"${GOGOPROTO_ROOT}/gogoproto"

# install some development libraries used when compiling fio
RUN apt -q -y update --fix-missing && apt -q -y install libaio-dev zlib1g-dev

ARG USER_ID=1000
RUN adduser jenkins --uid=$USER_ID --disabled-password --system
RUN (mkdir -p /src/github.com/gravitational/gravity && \
     chown -R jenkins /gopath && \
     chown -R jenkins /src && \ 
     mkdir -p /.cache && \
     chmod 777 /.cache && \
     chmod 777 /tmp)

VOLUME ["/src/github.com/gravitational/gravity"]
