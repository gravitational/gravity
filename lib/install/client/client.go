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
package client

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// New returns a new client for the installer/agent service.
// The client installs the service and starts the operation.
// If restarted, the client will either attempt to connect to a running
// installer service or set up a new one (subject to connection strategy).
func New(ctx context.Context, config Config) (*Client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c := &Client{Config: config}
	client, err := c.connect(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.client = client
	c.addTerminationHandler()
	return c, nil
}

// Run starts the service operation and runs the loop to fetch and display
// operation progress
func (r *Client) Run(ctx context.Context) error {
	return r.execute(ctx, &installpb.ExecuteRequest{})
}

// ExecutePhase executes the specified phase
func (r *Client) ExecutePhase(ctx context.Context, phase Phase) error {
	r.WithField("phase", phase).Info("Execute.")
	return r.execute(ctx, &installpb.ExecuteRequest{
		Phase: &installpb.ExecuteRequest_Phase{
			Key:   installpb.KeyToProto(phase.Key),
			ID:    phase.ID,
			Force: phase.Force,
		},
	})
}

// RollbackPhase rolls back the specified phase
func (r *Client) RollbackPhase(ctx context.Context, phase Phase) error {
	r.WithField("phase", phase).Info("Rollback.")
	return r.execute(ctx, &installpb.ExecuteRequest{
		Phase: &installpb.ExecuteRequest_Phase{
			Key:      installpb.KeyToProto(phase.Key),
			ID:       phase.ID,
			Force:    phase.Force,
			Rollback: true,
		},
	})
}

// Complete manually completes the active operation
func (r *Client) Complete(ctx context.Context, key ops.SiteOperationKey) error {
	r.WithField("key", key).Info("Complete.")
	_, err := r.client.Complete(ctx, &installpb.CompleteRequest{
		Key: installpb.KeyToProto(key),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.complete(ctx, installpb.StatusCompleted))
}

// Stop signals the service to stop and invokes the abort handler
// Implements signals.Stopper
func (r *Client) Stop(ctx context.Context) error {
	r.Info("Abort.")
	_, err := r.client.Abort(ctx, &installpb.AbortRequest{})
	if errAbort := r.abort(ctx); errAbort != nil {
		r.WithError(errAbort).Warn("Failed to abort.")
		if err == nil {
			err = errAbort
		}
	}
	return trace.Wrap(err)
}

func (r *Config) checkAndSetDefaults() (err error) {
	if r.ConnectStrategy == nil {
		return trace.BadParameter("ConnectStrategy is required")
	}
	if err := r.ConnectStrategy.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.InterruptHandler == nil {
		return trace.BadParameter("InterruptHandler is required")
	}
	if r.ServicePath == "" {
		r.ServicePath, err = state.GravityInstallDir(defaults.GravityRPCInstallerServiceName)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if !filepath.IsAbs(r.ServicePath) {
		return trace.BadParameter("ServicePath needs to be absolute path")
	}
	if r.Aborter == nil {
		r.Aborter = emptyAborter
	}
	if r.Completer == nil {
		r.Completer = emptyCompleter
	}
	if r.Printer == nil {
		r.Printer = utils.DiscardPrinter
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:installer")
	}
	if r.SocketPath == "" {
		r.SocketPath, err = installpb.SocketPath()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ConnectStrategy abstracts a way to connect to the installer service
type ConnectStrategy interface {
	// connect connects to the service and returns a client
	connect(context.Context) (installpb.AgentClient, error)
	checkAndSetDefaults() error
}

// Config describes the configuration of the installer client
type Config struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// Printer specifies the message output sink for progress messages
	utils.Printer
	// InterruptHandler specifies the interruption handler to register with
	*signals.InterruptHandler
	// ConnectStrategy specifies the connection to setup/connect to the service
	ConnectStrategy
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
	// Aborter specifies the completion handler for when the operation is aborted
	Aborter func(context.Context) error
	// Completer specifies the optional completion handler for when the operation
	// is completed successfully
	Completer CompletionHandler
}

// IsResume returns true if this is a resume operation
func (r Phase) IsResume() bool {
	return r.ID == fsm.RootPhase
}

// Phase groups parameters for executing/rolling back a phase
type Phase struct {
	// ID specifies the phase ID
	ID string
	// Force defines whether the phase execution is forced regardless
	// of its state
	Force bool
	// Key identifies the active operation
	Key ops.SiteOperationKey
}

func (r *Client) execute(ctx context.Context, req *installpb.ExecuteRequest) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := r.client.Execute(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	resultC := r.startProgressLoop(stream)
	select {
	case result := <-resultC:
		return trace.Wrap(r.handleProgressStatus(ctx, cancel, result.status, result.err))
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-r.InterruptHandler.Done():
		return installpb.ErrAborted
	}
}

func (r *Client) startProgressLoop(stream installpb.Agent_ExecuteClient) chan result {
	resultC := make(chan result, 1)
	go func() {
		status, err := r.progressLoop(stream)
		resultC <- result{status: status, err: err}
	}()
	return resultC
}

func (r *Client) progressLoop(stream installpb.Agent_ExecuteClient) (status installpb.ProgressResponse_Status, err error) {
	for {
		resp, err := stream.Recv()
		if err != nil {
			if s, ok := grpcstatus.FromError(err); ok && s.Code() == codes.Canceled {
				return installpb.StatusUnknown, nil
			}
			if trace.IsEOF(err) {
				// Stream closed by the server
				return installpb.StatusUnknown, nil
			}
			r.WithError(err).Warn("Failed to fetch progress.")
			return installpb.StatusUnknown, trace.Wrap(err)
		}
		// Exit upon first error
		if resp.Error != nil {
			return resp.Status, trace.BadParameter(resp.Error.Message)
		}
		r.PrintStep(resp.Message)
		if resp.IsCompleted() {
			return resp.Status, nil
		}
	}
}

func (r *Client) handleProgressStatus(ctx context.Context, cancel context.CancelFunc, status installpb.ProgressResponse_Status, err error) error {
	switch {
	case err == nil:
		if status == installpb.StatusUnknown {
			return nil
		}
		// We received completion status
		err = r.complete(ctx, status)
		cancel()
		return trace.Wrap(err)
	case trace.IsEOF(err):
		// Stream done but no completion event
		return nil
	}
	return trace.Wrap(err)
}

func (r *Client) addTerminationHandler() {
	r.InterruptHandler.AddStopper(r)
}

func (r *Client) abort(ctx context.Context) error {
	if err := r.Aborter(ctx); err != nil {
		r.WithError(err).Warn("Failed to abort operation.")
	}
	return installpb.ErrAborted
}

func (r *Client) complete(ctx context.Context, status installpb.ProgressResponse_Status) error {
	_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.Completer(ctx, r.InterruptHandler, status))
}

// Client implements the client to the installer service
type Client struct {
	Config
	client installpb.AgentClient
}

// CompletionHandler describes a functional handler for tasks to run after
// operation is complete
type CompletionHandler func(context.Context, *signals.InterruptHandler, installpb.ProgressResponse_Status) error

func removeSocketFileCommand(socketPath string) (cmd string) {
	return fmt.Sprintf("-/usr/bin/rm -f %v", socketPath)
}

func emptyAborter(context.Context) error {
	return nil
}

func emptyCompleter(context.Context, *signals.InterruptHandler, installpb.ProgressResponse_Status) error {
	return nil
}

var _ signals.Stopper = (*Client)(nil)

type result struct {
	status installpb.ProgressResponse_Status
	err    error
}
