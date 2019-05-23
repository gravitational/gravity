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
	"bytes"
	"context"
	"fmt"
	"net"
	"sync"

	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// New returns a new instance of the installer server.
// Use Serve to make server start listening
func New(ctx context.Context) *Server {
	localCtx, cancel := context.WithCancel(ctx)
	grpcServer := grpc.NewServer()
	server := &Server{
		FieldLogger: log.WithField(trace.Component, "installer:service"),
		ctx:         localCtx,
		cancel:      cancel,
		rpc:         grpcServer,
		respC:       make(chan *installpb.ProgressResponse),
		recvC:       make(chan []*installpb.ProgressResponse),
		// errC is the chan that receives error from either execute or
		// the operation aborted event
		errC:  make(chan error, 2),
		execC: make(chan *installpb.ExecuteRequest, 1),
	}
	installpb.RegisterAgentServer(grpcServer, server)
	return server
}

// Serve starts the server using the specified executor
func (r *Server) Serve(executor Executor, listener net.Listener) error {
	r.executor = executor
	r.startExecuteLoop()
	r.startMessageBufferLoop()
	return trace.Wrap(r.rpc.Serve(listener))
}

// Stop gracefully stops the server
func (r *Server) Stop(ctx context.Context) {
	r.Info("Stop.")
	r.stop(ctx)
	r.rpc.GracefulStop()
}

// Interrupt aborts the server.
// This implements manual server interruption
func (r *Server) Interrupt(ctx context.Context) error {
	r.Info("Interrupt.")
	r.signalAbort()
	return nil
}

// Abort aborts the operation and cleans up the state.
// Implements installpb.AgentServer
func (r *Server) Abort(ctx context.Context, req *installpb.AbortRequest) (*installpb.AbortResponse, error) {
	r.Info("Abort.")
	r.signalAbort()
	return &installpb.AbortResponse{}, nil
}

// Execute executes the installation using the specified engine
// Implements installpb.AgentServer
func (r *Server) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	r.submit(req)
	for {
		select {
		case batch, ok := <-r.recvC:
			if !ok {
				// Stop has been signaled
				return nil
			}
			for _, resp := range batch {
				err := stream.Send(resp)
				if err != nil {
					r.WithError(err).Warn("Failed to stream event.")
					return trace.Wrap(err)
				}
			}
		case <-stream.Context().Done():
			r.Info("Event loop done.")
			return nil
		}
	}
}

// Complete manually completes the operation given with req.
// Implements installpb.AgentServer
func (r *Server) Complete(ctx context.Context, req *installpb.CompleteRequest) (*types.Empty, error) {
	err := r.executor.Complete(installpb.KeyFromProto(req.Key))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return installpb.Empty, nil
}

// Send streams the specified progress event to the client.
// The method is not blocking - event will be dropped if it cannot be published
func (r *Server) Send(event Event) {
	r.send(eventToProgressResponse(event))
}

// Done is signalled when the server is done.
// The server is done when either happens:
//  - the execute finishes successfully or with an error
//  - the operation is aborted (in which case the chan will return a special
// abort error)
func (r *Server) Done() <-chan error {
	return r.errC
}

// SendAbort sends abort notification to client
func (r *Server) SendAbort() {
	r.send(&installpb.ProgressResponse{
		Status: installpb.ProgressResponse_Aborted,
	})
}

// Executor wraps a potentially failing operation
type Executor interface {
	// Execute executes an operation.
	Execute(*installpb.ExecuteRequest_Phase) error
	// Complete executes an operation.
	Complete(operationKey ops.SiteOperationKey) error
}

// Server implements the installer gRPC server.
// The server itself does not do any work but merely relays requests to an executor.
// Once started, the server will signal the completion on its Done() chan with either a nil
// (no error, completed successfully) or a specific err value.
// If err value is a special kind of installpb.ErrAborted, the operation has been aborted.
type Server struct {
	log.FieldLogger
	// rpc is the internal gRPC server instance
	rpc      *grpc.Server
	executor Executor

	// ctx defines the local server context used to cancel internal operation
	ctx    context.Context
	cancel context.CancelFunc
	// respC accepts progress messages to dispatch to the client
	respC chan *installpb.ProgressResponse
	// recvC specifies the channel that is used to propagate progress messages
	// to the client. It is not an error if there's no receiver for the
	// channel (client disconnected) - in which case server will continue buffering
	// the messages until the receiver is reconnected.
	// Upon receiving the cancellation request, the buffer loop will try to submit
	// any pending messages and close the channel.
	recvC chan []*installpb.ProgressResponse
	// errC signals the error from either the execute or
	// operation being aborted
	errC chan error

	// execC channel accepts new execute requests
	execC chan *installpb.ExecuteRequest
	wg    sync.WaitGroup
}

// String formats this event as text
func (r Event) String() string {
	var buf bytes.Buffer
	fmt.Print(&buf, "event(")
	if r.Progress != nil {
		fmt.Fprintf(&buf, "progress(completed=%v, message=%v),",
			r.Progress.Completion, r.Progress.Message)
	}
	if r.Error != nil {
		fmt.Fprintf(&buf, "error(%v)", r.Error.Error())
	}
	fmt.Print(&buf, ")")
	return buf.String()
}

// Event describes the installer progress step
type Event struct {
	// Progress describes the operation progress
	Progress *ops.ProgressEntry
	// Error specifies the error if any
	Error error
	// Complete indicates whether this event is terminal
	Complete bool
}

func (r *Server) startExecuteLoop() {
	r.wg.Add(1)
	go func() {
		// No select on r.ctx since we're guaranteed to send on errC once
		r.errC <- r.execute()
		r.wg.Done()
	}()
}

func (r *Server) startMessageBufferLoop() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		// Pending accumulates the progress messages we could not send
		// to the receiver.
		// It is unbounded but the installer is not expected to have a large
		// number of progress messages so it is an acceptable compromise
		var pending []*installpb.ProgressResponse
		for {
			if len(pending) == 0 {
				// Receive at least one message
				select {
				case event := <-r.respC:
					pending = append(pending, event)
				case <-r.ctx.Done():
					r.Info("Buffer loop done.")
					// return
				}
			}
			select {
			case event := <-r.respC:
				pending = append(pending, event)
			case r.recvC <- pending:
				pending = nil
			case <-r.ctx.Done():
				if len(pending) != 0 {
					select {
					case r.recvC <- pending:
					default:
					}
				}
				close(r.recvC)
				r.Info("Buffer loop done.")
				return
			}
		}
	}()
}

func (r *Server) execute() error {
	for {
		select {
		case req := <-r.execC:
			err := r.executor.Execute(req.Phase)
			if err == nil {
				return nil
			}
			r.WithError(err).Warn("Failed to execute.")
			r.sendError(err)
			return trace.Wrap(err)
		case <-r.ctx.Done():
			return trace.Wrap(r.ctx.Err())
		}
	}
}

func (r *Server) submit(req *installpb.ExecuteRequest) {
	select {
	case r.execC <- req:
	default:
		// Drop the execute request if another request is already in flight.
		// Ideally, we sould be able to notify the client that the request has been ignored
	}
}

func (r *Server) stop(ctx context.Context) {
	r.cancel()
	r.wg.Wait()
}

func (r *Server) signalAbort() {
	// errC always has a slot for abort
	r.errC <- installpb.ErrAborted
}

func (r *Server) sendError(err error) {
	r.send(&installpb.ProgressResponse{
		Error: &installpb.Error{Message: err.Error()},
	})
}

// send streams the specified progress event to the client.
// The method is not blocking as writes to respC are always serviced by the buffer loop
func (r *Server) send(resp *installpb.ProgressResponse) {
	select {
	case r.respC <- resp:
	case <-r.ctx.Done():
	}
}

func eventToProgressResponse(event Event) *installpb.ProgressResponse {
	resp := &installpb.ProgressResponse{}
	if event.Progress != nil {
		resp.Message = event.Progress.Message
		if event.Complete {
			resp.Status = installpb.ProgressResponse_Completed
		}
	} else if event.Error != nil {
		resp.Error = &installpb.Error{Message: event.Error.Error()}
	}
	return resp
}
