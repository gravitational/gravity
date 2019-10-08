IDL = $(wildcard *.proto)
google_deps = Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types
agent_deps = Magentpb/agent.proto=github.com/gravitational/satellite/agent/proto/agentpb
deps = $(google_deps),$(agent_deps)

.PHONY: all
all: $(IDL)
	protoc -I=. -I=$(PROTO_INCLUDE) \
		-I=/gopath/src/github.com/gravitational/gravity/vendor/github.com/gravitational/satellite/agent/proto \
		-I=/gopath/src/github.com/gravitational/gravity/vendor/github.com/gravitational/satellite/agent/proto/agentpb \
		$^ \
		--gogo_out=plugins=grpc,$(deps):.
