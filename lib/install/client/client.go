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
	"time"

	"github.com/gravitational/gravity/lib/fsm"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	"github.com/gravitational/trace/trail"
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
		Phase: &installpb.Phase{
			Key:   installpb.KeyToProto(phase.Key),
			ID:    phase.ID,
			Force: phase.Force,
		},
	})
}

// SetPhase sets the specified phase state without executing it
func (r *Client) SetPhase(ctx context.Context, phase Phase, state string) error {
	r.WithField("phase", phase).WithField("state", state).Info("Set.")
	_, err := r.client.SetState(ctx, &installpb.SetStateRequest{
		Phase: &installpb.Phase{
			Key: installpb.KeyToProto(phase.Key),
			ID:  phase.ID,
		},
		State: state,
	})
	return trace.Wrap(err)
}

// RollbackPhase rolls back the specified phase
func (r *Client) RollbackPhase(ctx context.Context, phase Phase) error {
	r.WithField("phase", phase).Info("Rollback.")
	return r.execute(ctx, &installpb.ExecuteRequest{
		Phase: &installpb.Phase{
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
		return trail.FromGRPC(err)
	}
	return trace.Wrap(r.Lifecycle.Complete(ctx, r, installpb.StatusCompleted))
}

// Stop signals the service to stop and invokes the abort handler
// Implements signals.Stopper
func (r *Client) Stop(ctx context.Context) error {
	r.Info("Abort.")
	_, err := r.client.Abort(ctx, &installpb.AbortRequest{})
	errAbort := r.Lifecycle.Abort(ctx, r)
	if err == nil {
		err = errAbort
	}
	return trace.Wrap(err)
}

func (r *Config) checkAndSetDefaults() (err error) {
	if r.Lifecycle == nil {
		r.Lifecycle = &NoopLifecycle{}
	}
	if err := r.Lifecycle.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.ConnectStrategy == nil {
		return trace.BadParameter("ConnectStrategy is required")
	}
	if err := r.ConnectStrategy.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.InterruptHandler == nil {
		return trace.BadParameter("InterruptHandler is required")
	}
	if r.Printer == nil {
		r.Printer = utils.DiscardPrinter
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:installer")
	}
	return nil
}

// ConnectStrategy abstracts a way to connect to the installer service
type ConnectStrategy interface {
	// connect connects to the service and returns a client
	connect(context.Context) (installpb.AgentClient, error)
	checkAndSetDefaults() error
	serviceName() string
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
	// Lifecycle specifies the implementation of exit strategies after operation
	// completion
	Lifecycle Lifecycle
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
		return trace.Wrap(r.Lifecycle.HandleStatus(ctx, r, result.status, result.err))
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	case <-r.InterruptHandler.Done():
		return installpb.ErrAborted
	}
}

func (r *Client) startProgressLoop(stream installpb.Agent_ExecuteClient) <-chan result {
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
		if resp.IsCompleted() || resp.IsAborted() {
			r.WithField("resp", resp).Info("Received completed response.")
			return resp.Status, nil
		}
	}
}

func (r *Client) addTerminationHandler() {
	r.InterruptHandler.AddStopper(r)
}

func (r *Client) complete(ctx context.Context) error {
	_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{
		Completed: true,
	})
	if err := r.waitForServiceStopped(ctx); err != nil {
		log.WithError(err).Warn("Failed to wait for installer service to shut down.")
	}
	return trace.Wrap(err)
}

func (r *Client) shutdown(ctx context.Context) error {
	_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
	return trace.Wrap(err)
}

func (r *Client) shutdownWithExitCode(ctx context.Context, code int) error {
	_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{
		ExitCode: int32(code),
	})
	if isNoRestartExitCode(code) {
		if err := r.waitForServiceStopped(ctx); err != nil {
			log.WithError(err).Warn("Failed to wait for installer service to shut down.")
		}
	}
	return trace.Wrap(err)
}

func (r *Client) generateDebugReport(ctx context.Context, path string) error {
	_, err := r.client.GenerateDebugReport(ctx, &installpb.DebugReportRequest{
		Path: path,
	})
	return trace.Wrap(err)
}

// Client implements the client to the installer service
type Client struct {
	Config
	client installpb.AgentClient
}

// Lifecycle handles different exit strategies for an operation after
// completion.
type Lifecycle interface {
	checkAndSetDefaults() error
	// HandleStatus executes status-specific tasks after an operation is completed
	HandleStatus(context.Context, *Client, installpb.ProgressResponse_Status, error) error
	// Complete executes tasks after the operation has been completed successfully
	Complete(context.Context, *Client, installpb.ProgressResponse_Status) error
	// Abort handles clean up of state files and directories
	// the installer maintains throughout the operation
	Abort(context.Context, *Client) error
}

func (r *Client) waitForServiceStopped(ctx context.Context) error {
	boff := backoff.NewExponentialBackOff()
	boff.MaxElapsedTime = 5 * time.Minute
	return utils.RetryWithInterval(ctx, boff, func() error {
		return service.IsStatus(
			r.ConnectStrategy.serviceName(),
			systemservice.ServiceStatusInactive,
			systemservice.ServiceStatusFailed,
		)
	})
}

var _ signals.Stopper = (*Client)(nil)

type result struct {
	status installpb.ProgressResponse_Status
	err    error
}
