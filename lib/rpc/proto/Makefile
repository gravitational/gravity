IDL = $(wildcard *.proto)
google_deps = Mgoogle/protobuf/empty.proto=github.com/gogo/protobuf/types,Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types
deps = $(google_deps)

.PHONY: all
all: $(IDL)
	protoc -I=. -I=$(PROTO_INCLUDE) \
		$^ \
		--gogo_out=plugins=grpc,$(deps):.
