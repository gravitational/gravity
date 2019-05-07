package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/ops"

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
		parentCtx:   ctx,
		ctx:         localCtx,
		cancel:      cancel,
		rpc:         grpcServer,
		// TODO(dmitri): arbitrary channel buffer size
		eventsC: make(chan Event, 100),
		abortC:  make(chan struct{}, 1),
	}
	installpb.RegisterAgentServer(grpcServer, server)
	return server
}

// Serve starts the server using the specified executor
func (r *Server) Serve(executor Executor, listener net.Listener) error {
	r.executor = executor
	return trace.Wrap(r.rpc.Serve(listener))
}

// Stop stops the server gracefully
func (r *Server) Stop(ctx context.Context) {
	r.stop(ctx)
	r.rpc.GracefulStop()
}

// Interrupt aborts the server
func (r *Server) Interrupt(ctx context.Context) {
	r.abort(ctx)
	r.rpc.GracefulStop()
}

// WaitForOperation waits for executor to finish the operation
func (r *Server) WaitForOperation() {
	r.execWG.Wait()
}

// Abort aborts the operation and cleans up the state
// Implements installpb.AgentServer
func (r *Server) Abort(ctx context.Context, req *installpb.AbortRequest) (*installpb.AbortResponse, error) {
	r.Info("Abort.")
	r.abort(ctx)
	// Do not block Stop from the server's connection as it waits for all connections
	// to complete
	go r.rpc.GracefulStop()
	return &installpb.AbortResponse{}, nil
}

// Shutdown shuts down the installer.
// Implements installpb.AgentServer
func (r *Server) Shutdown(ctx context.Context, req *installpb.ShutdownRequest) (*installpb.ShutdownResponse, error) {
	// The caller should be blocked at least as long as the wizard process is closing.
	// TODO(dmitri): find out how this returns to the caller and whether it would make sense
	// to split the shut down into several steps with wizard shutdown to be invoked as part of Shutdown
	// and the rest - from a goroutine so the caller is not receiving an error when the server stops
	// serving
	r.stop(ctx)
	go r.Stop(ctx)
	return &installpb.ShutdownResponse{}, nil
}

// Execute executes the installation using the specified engine
// Implements installpb.AgentServer
func (r *Server) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	r.executeOnce.Do(r.execute)
	for {
		select {
		case event := <-r.eventsC:
			resp := &installpb.ProgressResponse{}
			if event.Progress != nil {
				resp.Message = event.Progress.Message
				if event.Complete {
					resp.Status = installpb.ProgressResponse_Completed
				}
			} else if event.Error != nil {
				resp.Errors = append(resp.Errors, &installpb.Error{Message: event.Error.Error()})
				if event.Error == errAborted {
					resp.Status = installpb.ProgressResponse_Aborted
				}
			}
			err := stream.Send(resp)
			if err != nil {
				return trace.Wrap(err)
			}
		case <-stream.Context().Done():
			return trace.Wrap(stream.Context().Err())
		case <-r.parentCtx.Done():
			return trace.Wrap(r.parentCtx.Err())
		case <-r.ctx.Done():
			// Clean exit
			r.Info("Operation loop done.")
			return nil
		}
	}
	return nil
}

// Run schedules f to run as server's internal process.
// Use WaitForOperation to await completion of all processes
// upon completion or abort
func (r *Server) Run(f func()) {
	r.execWG.Add(1)
	go func() {
		f()
		r.execWG.Done()
	}()
}

// Send streams the specified progress event to the client.
// The method is not blocking - event will be dropped if it cannot be published
func (r *Server) Send(event Event) error {
	select {
	case r.eventsC <- event:
		// Pushed the progress event
		return nil
	case <-r.parentCtx.Done():
		return nil
	case <-r.ctx.Done():
		return nil
	default:
		r.WithField("event", event).Warn("Failed to publish event.")
		return trace.BadParameter("failed to publish event")
	}
}

// RunProgressLoop starts progress loop for the specified operation
func (r *Server) RunProgressLoop(operator ops.Operator, operationKey ops.SiteOperationKey, doneC chan struct{}) {
	r.serveWG.Add(1)
	go func() {
		r.WithField("operation", operationKey.OperationID).Info("Start progress feedback loop.")
		ticker := time.NewTicker(1 * time.Second)
		defer func() {
			ticker.Stop()
			r.serveWG.Done()
		}()
		var lastProgress *ops.ProgressEntry
		for {
			select {
			case <-ticker.C:
				progress, err := operator.GetSiteOperationProgress(operationKey)
				if err != nil {
					r.WithError(err).Warn("Failed to query operation progress.")
					continue
				}
				if lastProgress != nil && lastProgress.IsEqual(*progress) {
					continue
				}
				r.Send(Event{Progress: progress})
				lastProgress = progress
				if progress.IsCompleted() {
					select {
					case doneC <- struct{}{}:
					case <-r.parentCtx.Done():
						return
					case <-r.ctx.Done():
						return
					}
					return
				}
			case <-r.parentCtx.Done():
				return
			case <-r.ctx.Done():
				return
			}
		}
	}()
}

// Executor wraps a potentially failing operation
type Executor interface {
	// Execute executes an operation.
	Execute() error
	// AbortOperation gracefully aborts the operation and cleans up the operation state
	AbortOperation(context.Context) error
	// Shutdown gracefully stops the operation
	Shutdown(context.Context) error
}

// Server implements the installer gRPC server
type Server struct {
	log.FieldLogger
	// parentCtx specifies the external context.
	// If cancelled, all external operations abort with the corresponding error
	parentCtx context.Context
	// ctx defines the local server context used to cancel internal operation
	ctx    context.Context
	cancel context.CancelFunc

	executor Executor
	eventsC  chan Event
	// abortC is signaled when the operation is aborted
	abortC chan struct{}
	// rpc is the internal gRPC server instance
	rpc *grpc.Server

	executeOnce sync.Once
	// serveWG is a wait group for internal processes
	serveWG sync.WaitGroup
	// execWG is a wait group for executor-specific workloads.
	// Use WaitForOperation to await completion of scheduled processes
	// after cancelling the operation.
	execWG sync.WaitGroup
}

// String formats this event for readability
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
	// Complete indicates that this is the last event sent
	Complete bool
}

func (r *Server) execute() {
	r.execWG.Add(2)
	execC := make(chan error, 1)
	go func() {
		var err error
		select {
		case <-r.abortC:
			err = errAborted
		case err = <-execC:
		}
		if err != nil {
			if errSend := r.sendError(err); errSend != nil {
				r.WithError(errSend).Info("Failed to send error to client.")
			}
		}
		// No explicit stop in case of error
		r.execWG.Done()
		if err == nil {
			r.stop(r.parentCtx)
		}
	}()
	go func() {
		execC <- r.executor.Execute()
		r.execWG.Done()
	}()
}

func (r *Server) stop(ctx context.Context) {
	r.executor.Shutdown(ctx)
	r.cancel()
	r.serveWG.Wait()
}

func (r *Server) abort(ctx context.Context) {
	select {
	case r.abortC <- struct{}{}:
		// Notify that the operation has been aborted
	case <-ctx.Done():
	}
	r.executor.AbortOperation(ctx)
	r.cancel()
	r.serveWG.Wait()
}

func (r *Server) sendError(err error) error {
	return trace.Wrap(r.Send(Event{Error: err}))
}

var errAborted = errors.New("operation aborted")
