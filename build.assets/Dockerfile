FROM quay.io/gravitational/debian-venti:go1.12.9-stretch

ARG PROTOC_VER
ARG PROTOC_PLATFORM
ARG GOGO_PROTO_TAG
ARG GRPC_GATEWAY_TAG
ARG VERSION_TAG

ENV TARBALL protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip
ENV GOGOPROTO_ROOT ${GOPATH}/src/github.com/gogo/protobuf
ENV PROTOC_URL https://github.com/google/protobuf/releases/download/v${PROTOC_VER}/protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip

RUN adduser jenkins --uid=995 --disabled-password --system
RUN (mkdir -p /gopath/src/github.com/gravitational/gravity && \
     chown -R jenkins /gopath && \
     mkdir -p /.cache && \
     chmod 777 /.cache && \
     chmod 777 /tmp)

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

# TODO: go get (git checkout) versions from Gopkg.toml
RUN (mkdir -p /opt/protoc && \
     wget --quiet -O /tmp/${TARBALL} ${PROTOC_URL} && \
     unzip -d /opt/protoc /tmp/${TARBALL} && \
     go get -u github.com/gogo/protobuf/proto github.com/gogo/protobuf/protoc-gen-gogo github.com/gogo/protobuf/gogoproto \
     github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway \
     github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger)

RUN cd ${GOPATH}/src/github.com/gogo/protobuf && git reset --hard ${GOGO_PROTO_TAG} && make install
RUN cd ${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway && git reset --hard ${GRPC_GATEWAY_TAG} && go install ./protoc-gen-grpc-gateway

ENV PROTO_INCLUDE "/usr/local/include":"${GOPATH}/src":"${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis":"${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis":"${GOGOPROTO_ROOT}/protobuf":"${GOGOPROTO_ROOT}/gogoproto"

# install DEP tool
RUN wget --quiet -O /usr/bin/dep https://github.com/golang/dep/releases/download/v0.4.1/dep-linux-amd64 && chmod +x /usr/bin/dep
RUN chmod -R a+rw /gopath

VOLUME ["/gopath/src/github.com/gravitational/gravity"]
