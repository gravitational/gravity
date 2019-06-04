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
func New() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	grpcServer := grpc.NewServer()
	server := &Server{
		FieldLogger: log.WithField(trace.Component, "installer:service"),
		ctx:         ctx,
		cancel:      cancel,
		rpc:         grpcServer,
		respC:       make(chan *installpb.ProgressResponse),
		recvC:       make(chan []*installpb.ProgressResponse),
		// errC is signalled when the server is done.
		// The server is done when either happens:
		//  - the execute finishes successfully or with an error
		//  - the operation is aborted (in which case the chan will return a special abort error)
		errC:  make(chan error, 2),
		execC: make(chan *installpb.ExecuteRequest, 1),
	}
	installpb.RegisterAgentServer(grpcServer, server)
	return server
}

// Run starts the server using the specified executor and blocks until
// either the executor completes or the operation is aborted.
// To properly stop all server internal processes, use Stop
func (r *Server) Run(executor Executor, listener net.Listener) error {
	r.executor = executor
	r.startMessageBufferLoop()
	errC := make(chan error, 1)
	go func() {
		errC <- r.rpc.Serve(listener)
	}()
	select {
	case err := <-errC:
		return trace.Wrap(err)
	case err := <-r.errC:
		if installpb.IsAbortedErr(err) {
			r.sendAbort()
			return trace.Wrap(err)
		}
		return trace.Wrap(err)
	}
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

// Execute executes the operation specified with req.
// After the operation has been started, it dispatches the progress messages
// to the client until the progress channel is closed or client exits.
//
// Implements installpb.AgentServer
func (r *Server) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	if err := r.submit(req); err != nil {
		return trace.Wrap(err)
	}
	for msg := range r.recvC {
		// FIXME: handle completion differently if required.
		err := stream.Send(msg)
		if err != nil {
			r.WithError(err).Warn("Failed to stream event.")
			return trace.Wrap(err)
		}
	}
}

// ExecuteAndWait executes the operation specified with req.
// After the operation has been started, it dispatches the progress messages
// to the client until the progress channel is closed or client exits.
// If the client exits before the operation is complete, the operation is cancelled.
//
// Implements installpb.AgentServer
func (r *Server) ExecuteAndWait(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	dispatcher, recvC := newEventDispatcher()
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()
	r.submitWithDispatcher(ctx, req, dispatcher)
	for msg := range recvC {
		err := stream.Send(msg)
		if err != nil {
			r.WithError(err).Warn("Failed to stream event.")
			return trace.Wrap(err)
		}
	}
	return nil
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
// FIXME: this is the default implementation of the EventDispatcher
func (r *Server) Send(event Event) {
	r.send(eventToProgressResponse(event))
}

// Executor wraps a potentially failing operation
type Executor interface {
	// Execute executes an operation.
	Execute(context.Context, *installpb.ExecuteRequest_Phase, EventDispatcher) error
	// Complete manually completes the operation given with operationKey.
	Complete(operationKey ops.SiteOperationKey) error
}

// EventDispatcher dispatches progress events to clients
type EventDispatcher interface {
	Send(Event)
}

// Server implements the installer gRPC server.
// The server itself does not do any work but merely relays requests to an executor.
//
// Server has an internal progress message bus that dispatches installer events to
// the connected client.
// If the client drops the connection, the messages are buffered internally and will be
// resent once the client reconnects.
//
// Executor is responsible for detecting end-of-operation condition and stop and shutdown
// the server appropriately.
type Server struct {
	log.FieldLogger
	// rpc is the internal gRPC server instance
	rpc      *grpc.Server
	executor Executor

	// ctx defines the local server context used to cancel internal operation
	ctx context.Context
	// cancel cancels internal server processes
	cancel context.CancelFunc

	// respC accepts progress messages to dispatch to the client
	respC chan *installpb.ProgressResponse
	// recvC specifies the default channel to propagate progress messages
	// to the client. If the client disconnects (i.e. receiver is not available),
	// the server will continue buffering messages until the client is reconnected.
	// Upon receiving the cancellation request, the buffer loop will try to submit
	// any pending messages and close the channel.
	recvC chan []*installpb.ProgressResponse
	// FIXME: rename to abortC as it's only signaled when the service is aborted
	// errC signals the error from either the execute or
	// operation being aborted
	errC chan error
	// execC channel accepts new execute requests
	execC chan *installpb.ExecuteRequest

	wg sync.WaitGroup
}

// IsCompleted determines if this event indicates a completed operation event
func (r Event) IsCompleted() bool {
	return r.Status == StatusCompleted ||
		r.Status == StatusCompletedPending
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
		fmt.Fprintf(&buf, "error(%v),", r.Error.Error())
	}
	fmt.Fprintf(&buf, "status(%v)", r.Status)
	fmt.Print(&buf, ")")
	return buf.String()
}

// Event describes the installer progress step
type Event struct {
	// Progress describes the operation progress
	Progress *ops.ProgressEntry
	// Error specifies the error if any
	Error error
	// Completed indicates whether this event is terminal
	Status Status
}

// Status defines the progress status
type Status byte

const (
	// StatusUnknown indicates an unknown progress status
	StatusUnknown Status = 0
	// StatusCompleted indicates a completed operation
	StatusCompleted Status = iota
	// StatusCompletedPending indicates a completed operation
	// but with installer still active
	StatusCompletedPending Status = iota
)

// startMessageBufferLoop starts the message buffering loop for the default message
// handler to account for client dropping and reconnecting later.
func (r *Server) startMessageBufferLoop() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		var recvC chan *installpb.ProgressResponse
		// Pending accumulates the progress messages we could not send
		// to the receiver.
		// It is unbounded but the installer is not expected to have a large
		// number of progress messages so it is an acceptable compromise
		var pending []*installpb.ProgressResponse
		for {
			select {
			case event := <-r.respC:
				pending = append(pending, event)
				recvC = r.recvC
			case recvC <- pending[0]:
				pending = pending[1:]
				if len(pending) == 0 {
					recvC = nil
				}
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
			err := r.executor.Execute(r.ctx, req.Phase, r)
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

func (r *Server) submit(ctx context.Context, req *installpb.ExecuteRequest) error {
	select {
	case r.execC <- req:
		// Successfully submitted execute request
		return nil
	default:
		return trace.AlreadyExists("operation is already active")
	}
}

func (r *Server) submitWithDispatcher(ctx context.Context, req *installpb.ExecuteRequest, dispatcher EventDispatcher) {
	// TODO: create an event dispatcher with a dedicated progress piping loop
	go func() {
		if err := r.executor.Execute(ctx, req.Phase, dispatcher); err != nil {
			r.sendError(err)
		}
	}()
}

func (r *Server) stop(ctx context.Context) {
	r.cancel()
	r.wg.Wait()
}

func (r *Server) signalAbort() {
	// errC always has a slot for abort
	r.errC <- installpb.ErrAborted
}

func (r *Server) sendAbort() {
	r.send(&installpb.ProgressResponse{
		Status: installpb.StatusAborted,
	})
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
		switch event.Status {
		case StatusCompleted:
			resp.Status = installpb.StatusCompleted
		case StatusCompletedPending:
			resp.Status = installpb.StatusCompletedPending
		}
	} else if event.Error != nil {
		resp.Error = &installpb.Error{Message: event.Error.Error()}
	}
	return resp
}
