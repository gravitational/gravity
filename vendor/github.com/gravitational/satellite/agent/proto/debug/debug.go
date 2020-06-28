/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package debug

import (
	"runtime/pprof"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewServer creates a new instance of the Debug service
func NewServer() *Server {
	return &Server{}
}

// Profile executes the specified debug profile and streams the results to the caller
func (r *Server) Profile(req *ProfileRequest, stream Debug_ProfileServer) error {
	profile := pprof.Lookup(req.Profile)
	if profile == nil {
		return status.Errorf(codes.NotFound, "invalid profile: %v", req.Profile)
	}
	var debug int
	switch req.Output {
	case OutputNormal:
		debug = 1
	case OutputBasic:
		debug = 0
	case OutputDebug:
		debug = 2
	}
	if err := profile.WriteTo(&byteWriter{stream: stream}, debug); err != nil {
		return status.Errorf(codes.Internal, "failed to stream profile: %v", err)
	}
	return nil
}

// Server encapsulates a Debug service
type Server struct {}

// Write writes the specified byte slice into the underlying stream
func (r *byteWriter) Write(p []byte) (n int, err error) {
	if err := r.stream.Send(&ProfileResponse{
		Output: p[:],
	}); err != nil {
		return 0, err
	}
	return len(p), err
}

type byteWriter struct {
	stream Debug_ProfileServer
}
