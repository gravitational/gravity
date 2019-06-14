/*
Copyright 2019 Gravitational, Inc.

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
package server

import (
	"context"
	"net"
	"sync"

	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// New returns a new instance of the installer server.
// Use Serve to make server start listening
func New() *Server {
	grpcServer := grpc.NewServer()
	server := &Server{
		FieldLogger: log.WithField(trace.Component, "service:installer"),
		rpc:         grpcServer,
		errC:        make(chan error, 2),
	}
	installpb.RegisterAgentServer(grpcServer, server)
	return server
}

// Run starts the server using the specified executor and blocks until
// either the executor completes or the operation is aborted.
// To properly stop all server internal processes, use Stop
func (r *Server) Run(executor Executor, listener net.Listener) error {
	r.executor = executor
	errC := make(chan error, 1)
	go func() {
		errC <- r.rpc.Serve(listener)
	}()
	select {
	case err := <-errC:
		return trace.Wrap(err)
	case err := <-r.errC:
		return trace.Wrap(err)
	}
}

// Stop gracefully stops the server
func (r *Server) Stop(ctx context.Context) {
	r.Info("Stop.")
	r.rpc.GracefulStop()
}

// Interrupt aborts the server.
// This implements manual server interruption
func (r *Server) Interrupt(ctx context.Context) error {
	r.Info("Interrupt.")
	r.signalAbort()
	return nil
}

// Execute executes the operation specified with req.
// After the operation has been started, it dispatches the progress messages
// to the client until the progress channel is closed or client exits.
//
// Implements installpb.AgentServer
func (r *Server) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	return r.executor.Execute(req, stream)
}

// Complete manually completes the operation given with req.
// Implements installpb.AgentServer
func (r *Server) Complete(ctx context.Context, req *installpb.CompleteRequest) (*types.Empty, error) {
	err := r.executor.Complete(installpb.KeyFromProto(req.Key))
	if err != nil {
		// Not wrapping err as it passes the gRPC boundary
		return nil, err
	}
	return installpb.Empty, nil
}

// Abort aborts the operation and cleans up the state.
// Implements installpb.AgentServer
func (r *Server) Abort(context.Context, *installpb.AbortRequest) (*types.Empty, error) {
	r.Info("Abort.")
	r.signalAbort()
	return installpb.Empty, nil
}

// Shutdown closes the server gracefully.
// Implements installpb.AgentServer
func (r *Server) Shutdown(context.Context, *installpb.ShutdownRequest) (*types.Empty, error) {
	r.Info("Shutdown.")
	r.signalDone()
	return installpb.Empty, nil
}

// GenerateDebugReport requests that the installer generates the debug report.
// Implements installpb.AgentServer
func (r *Server) GenerateDebugReport(ctx context.Context, req *installpb.DebugReportRequest) (*types.Empty, error) {
	r.WithField("req", req).Info("Generate debug report.")
	if reporter, ok := r.executor.(DebugReporter); ok {
		err := reporter.GenerateDebugReport(req.Path)
		if err != nil {
			// Not wrapping err as it passes the gRPC boundary
			return nil, err
		}
		return installpb.Empty, nil
	}
	return nil, status.Error(codes.Unimplemented, "not implemented")
}

// Executor wraps a potentially failing operation
type Executor interface {
	// Execute executes an operation specified with req.
	Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error
	// Complete manually completes the operation given with operationKey.
	Complete(operationKey ops.SiteOperationKey) error
}

// DebugReporter allows to capture the operation state
type DebugReporter interface {
	// GenerateDebugReport captures the state of the operation state for debugging
	GenerateDebugReport(path string) error
}

// Server implements the installer gRPC server.
// The server itself does not do any work and merely relays requests to an executor.
//
// Executor is responsible for detecting end-of-operation condition and stopping and
// shutting down the server appropriately.
type Server struct {
	log.FieldLogger
	// rpc is the internal gRPC server instance
	rpc      *grpc.Server
	executor Executor

	doneOnce sync.Once

	// errC signals the error from either the execute or
	// operation being aborted
	errC chan error
}

func (r *Server) signalDone() {
	r.doneOnce.Do(func() {
		r.errC <- nil
	})
}

func (r *Server) signalAbort() {
	r.doneOnce.Do(func() {
		r.errC <- installpb.ErrAborted
	})
}
