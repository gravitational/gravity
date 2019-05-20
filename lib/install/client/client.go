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
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/gravitational/gravity/lib/defaults"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system/environ"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	return trace.Wrap(err)
}

// Stop signals the service to stop
// Implements signals.Stopper
func (r *Client) Stop(ctx context.Context) error {
	if r.Completed() {
		return nil
	}
	return r.Shutdown(ctx)
}

// Shutdown signals the service to stop
func (r *Client) Shutdown(ctx context.Context) error {
	r.Info("Shutdown.")
	_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
	return trace.Wrap(err)
}

// Abort signals that the server cleans up the state and shuts down.
// Implements signals.Aborter
func (r *Client) Abort(ctx context.Context) error {
	r.Info("Abort.")
	_, err := r.client.Abort(ctx, &installpb.AbortRequest{})
	return trace.Wrap(err)
}

// Completed returns true if the operation has already been completed
func (r *Client) Completed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.completed
}

func (r *Config) checkAndSetDefaults() error {
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
		r.ServicePath = state.GravityInstallDir(defaults.GravityRPCInstallerServiceName)
	}
	if !filepath.IsAbs(r.ServicePath) {
		return trace.BadParameter("ServicePath needs to be absolute path")
	}
	if r.Printer == nil {
		r.Printer = utils.DiscardPrinter
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:installer")
	}
	if r.SocketPath == "" {
		r.SocketPath = installpb.SocketPath()
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
	log.FieldLogger
	utils.Printer
	*signals.InterruptHandler
	ConnectStrategy
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
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
	stream, err := r.client.Execute(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	err = r.progressLoop(stream)
	if err != errAborted {
		r.Shutdown(ctx)
	}
	return trace.Wrap(err)
}

func (r *Client) progressLoop(stream installpb.Agent_ExecuteClient) (err error) {
	for {
		resp, err := stream.Recv()
		if err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.Canceled {
				return nil
			}
			if trace.Unwrap(err) == io.EOF {
				// Stream done
				return nil
			}
			r.WithError(err).Warn("Failed to fetch progress.")
			return trace.Wrap(err)
		}
		if resp.Status == installpb.ProgressResponse_Aborted {
			return errAborted
		}
		// Exit upon first error
		if resp.Error != nil {
			return trace.BadParameter(resp.Error.Message)
		}
		r.PrintStep(resp.Message)
		if resp.Status == installpb.ProgressResponse_Completed {
			r.complete()
			// Do not explicitly exit the loop - wait for service to exit
		}
	}
}

func (r *Client) addTerminationHandler() {
	r.InterruptHandler.AddStopper(r)
}

// complete marks the operation completed in this client
// and uninstalls the installer service.
func (r *Client) complete() {
	r.mu.Lock()
	r.completed = true
	r.mu.Unlock()
	if err := environ.UninstallAgentServices(r.FieldLogger); err != nil {
		r.WithError(err).Warn("Failed to uninstall installer service.")
	}
}

// Client implements the client to the installer service
type Client struct {
	Config
	client installpb.AgentClient

	// mu guards fields below
	mu sync.Mutex
	// completed indicates whether the operation is complete
	completed bool
}

func removeSocketFileCommand(socketPath string) (cmd string) {
	return fmt.Sprintf("/usr/bin/rm -f %v", socketPath)
}

var _ signals.Stopper = (*Client)(nil)
var _ signals.Aborter = (*Client)(nil)

// FIXME: use the lib/install#ErrAborted
var errAborted = errors.New("operation aborted")
