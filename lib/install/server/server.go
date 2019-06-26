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

// Interrupted executes abort handler on the executor.
// This cannot block or invoke blocking APIs since it might be invoked
// by the RPC agent during shutdown
func (r *Server) Interrupted(ctx context.Context) error {
	r.Info("Interrupted.")
	r.aborted(ctx)
	return nil
}

// Stopped executes the stop handler on the executor.
// completed indicates whether this is the result of a successfully completed operation.
// This cannot block or invoke blocking APIs since it might be invoked
// by the RPC agent during shutdown
func (r *Server) Stopped(ctx context.Context, completed bool) error {
	r.Info("Stopped.")
	if completed {
		r.completed(ctx)
	} else {
		r.done(ctx)
	}
	return nil
}

// Execute executes the operation specified with req.
// After the operation has been started, it dispatches the progress messages
// to the client until the progress channel is closed or client exits.
//
// Implements installpb.AgentServer
func (r *Server) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	r.WithField("req", req).Info("Execute.")
	return r.executor.Execute(req, stream)
}

// Complete manually completes the operation given with req.
// Implements installpb.AgentServer
func (r *Server) Complete(ctx context.Context, req *installpb.CompleteRequest) (*types.Empty, error) {
	r.WithField("req", req).Info("Complete.")
	err := r.executor.Complete(installpb.KeyFromProto(req.Key))
	if err != nil {
		// Not wrapping err as it passes the gRPC boundary
		return nil, err
	}
	return installpb.Empty, nil
}

// Abort aborts the operation and cleans up the state.
// Implements installpb.AgentServer
func (r *Server) Abort(ctx context.Context, req *installpb.AbortRequest) (*types.Empty, error) {
	r.Info("Abort.")
	r.aborted(ctx)
	return installpb.Empty, nil
}

// Shutdown closes the server gracefully.
// Implements installpb.AgentServer
func (r *Server) Shutdown(ctx context.Context, req *installpb.ShutdownRequest) (*types.Empty, error) {
	r.WithField("req", req).Info("Shutdown.")
	if req.Completed {
		r.completed(ctx)
	} else {
		r.done(ctx)
	}
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
	Completer
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

// Completer describes completion outcomes
type Completer interface {
	// HandleAborted indicates that the operation has been aborted and completion steps
	// specific to abort should be executed
	HandleAborted(context.Context) error
	// HandleStopped indicates that the operation is still is progress but the service
	// is stopping
	HandleStopped(context.Context) error
	// HandleCompleted indicates that the operation has been successfully completed
	// and executes steps specific to this event
	HandleCompleted(context.Context) error
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

func (r *Server) done(ctx context.Context) {
	r.executor.HandleStopped(ctx)
	r.errC <- nil
}

func (r *Server) aborted(ctx context.Context) {
	r.executor.HandleAborted(ctx)
	r.errC <- installpb.ErrAborted
}

func (r *Server) completed(ctx context.Context) {
	r.executor.HandleCompleted(ctx)
	r.errC <- installpb.ErrCompleted
}
