package server

import (
	"bytes"
	"context"
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
func New(ctx context.Context, config Config) *Server {
	localCtx, cancel := context.WithCancel(ctx)
	grpcServer := grpc.NewServer()
	server := &Server{
		FieldLogger: log.WithField(trace.Component, "installer:service"),
		config:      config,
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

// Stop gracefully stops the server
func (r *Server) Stop(ctx context.Context) {
	r.stop(ctx)
	r.StopRPC()
}

// StopRPC gracefully stops the RPC server
func (r *Server) StopRPC() {
	r.rpc.GracefulStop()
}

// Interrupt aborts the server.
// This implements manual server interruption
func (r *Server) Interrupt(ctx context.Context) {
	r.Info("Interrupt.")
	r.abort(ctx)
	r.rpc.GracefulStop()
}

// Wait waits for server to finish the operation
func (r *Server) Wait() {
	r.wg.Wait()
}

// Abort aborts the operation and cleans up the state.
// Implements installpb.AgentServer
func (r *Server) Abort(ctx context.Context, req *installpb.AbortRequest) (*installpb.AbortResponse, error) {
	r.Info("Abort.")
	r.abort(ctx)
	return &installpb.AbortResponse{}, nil
}

// Shutdown shuts down the installer.
// Implements installpb.AgentServer
func (r *Server) Shutdown(ctx context.Context, req *installpb.ShutdownRequest) (*installpb.ShutdownResponse, error) {
	r.Info("Shutdown.")
	// The caller should be blocked at least as long as the wizard process is closing.
	// TODO(dmitri): find out how this returns to the caller and whether it would make sense
	// to split the shut down into several steps with wizard shutdown to be invoked as part of Shutdown
	// and the rest - from a goroutine so the caller is not receiving an error when the server stops
	// serving
	r.stop(ctx)
	return &installpb.ShutdownResponse{}, nil
}

// Execute executes the installation using the specified engine
// Implements installpb.AgentServer
func (r *Server) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	r.executeOnce.Do(r.execute)
	for {
		select {
		case event := <-r.eventsC:
			err := respondWithEvent(stream, event)
			if err != nil {
				r.WithError(err).Warn("Failed to stream event.")
				return trace.Wrap(err)
			}
		case <-r.abortC:
			err := stream.Send(&installpb.ProgressResponse{
				Status: installpb.ProgressResponse_Aborted,
			})
			r.WithError(err).Warn("Operation loop aborted.")
			return trace.Wrap(err)
		case <-r.ctx.Done():
			// Clean exit
			r.Info("Operation loop done.")
			return nil
		}
	}
	return nil
}

// // Run schedules f to run as server's internal process.
// // Use WaitForOperation to await completion of all processes
// // upon completion or abort
// func (r *Server) Run(f func()) {
// 	r.wg.Add(1)
// 	go func() {
// 		f()
// 		r.wg.Done()
// 	}()
// }

// Send streams the specified progress event to the client.
// The method is not blocking - event will be dropped if it cannot be published
func (r *Server) Send(event Event) error {
	select {
	case r.eventsC <- event:
		// Pushed the progress event
		return nil
	case <-r.ctx.Done():
		return nil
	default:
		r.WithField("event", event).Warn("Failed to publish event.")
		return trace.BadParameter("failed to publish event")
	}
}

// RunProgressLoop runs progress loop for the specified operation until the operation
// is completed.
// If the operation is aborted, the method exits with the corresponding error
func (r *Server) RunProgressLoop(operator ops.Operator, operationKey ops.SiteOperationKey) error {
	r.WithField("operation", operationKey.OperationID).Info("Start progress feedback loop.")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
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
				return nil
			}
		case <-r.abortC:
			// Unblock the executor.Execute but do not return an error
			// aborted exit status is handled on client when it exits from Execute
			// and in service, when it exits Runs
			return nil
		case <-r.ctx.Done():
			return nil
		}
	}
}

// Done returns the channel that is signalled when the server operation has completed
func (r *Server) Done() <-chan struct{} {
	return r.ctx.Done()
}

// Aborted returns the channel that is signalled when the server has been aborted
func (r *Server) Aborted() <-chan struct{} {
	return r.abortC
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
	config Config
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
	wg          sync.WaitGroup
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

// Config defines the server's configuration
type Config struct {
	// AbortHandler specifies the handler for aborting the installation
	AbortHandler func(context.Context) error
}

func (r *Server) execute() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		if err := r.executor.Execute(); err != nil {
			r.WithError(err).Info("Failed to executor.Execute.")
			if errSend := r.sendError(err); errSend != nil {
				r.WithError(errSend).Info("Failed to send error to client.")
			}
		}
		r.Info("Cancel server.")
		r.cancel()
	}()
}

func (r *Server) stop(ctx context.Context) {
	r.executor.Shutdown(ctx)
	r.cancel()
	r.wg.Wait()
}

func (r *Server) abort(ctx context.Context) {
	close(r.abortC)
	r.executor.AbortOperation(ctx)
	r.cancel()
	r.wg.Wait()
}

func (r *Server) sendError(err error) error {
	return trace.Wrap(r.Send(Event{Error: err}))
}

func respondWithEvent(stream installpb.Agent_ExecuteServer, event Event) error {
	resp := &installpb.ProgressResponse{}
	if event.Progress != nil {
		resp.Message = event.Progress.Message
		if event.Complete {
			resp.Status = installpb.ProgressResponse_Completed
		}
	} else if event.Error != nil {
		resp.Error = &installpb.Error{Message: event.Error.Error()}
	}
	return trace.Wrap(stream.Send(resp))
}
