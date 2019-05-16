package observer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system/cleanup"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns a new client to handle the observer case.
// The observer client starts the installer service if necessary
// and shuts it down when done.
//
// See docs/design/client/install-observer.puml for details
func New(ctx context.Context, config Config) (*Client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c := &Client{Config: config}
	err = c.restartService()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.ConnectTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.ConnectTimeout)
		defer cancel()
	}
	cc, err := installpb.NewClient(ctx, config.SocketPath, config.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.client = cc
	c.addTerminationHandler()
	return c, nil
}

// Execute executes the operation phase specified with phase
func (r *Client) Execute(ctx context.Context, machine *fsm.FSM, params fsm.Params) error {
	if !params.IsResume() {
		progress := utils.NewProgress(ctx, fmt.Sprintf("Executing phase %q", params.PhaseID), -1, false)
		defer progress.Stop()
		err := machine.ExecutePhase(ctx, fsm.Params{
			PhaseID:  params.PhaseID,
			Force:    params.Force,
			Progress: progress,
		})
		return trace.Wrap(err)
	}
	progress := utils.NewProgress(ctx, "Resuming operation", -1, false)
	defer progress.Stop()

	planErr := machine.ExecutePlan(ctx, progress)
	if planErr != nil {
		r.WithError(planErr).Warn("Failed to execute plan.")
	}
	if err := machine.Complete(planErr); err != nil {
		r.WithError(err).Warn("Failed to complete plan.")
	}
	if planErr != nil {
		return trace.Wrap(planErr)
	}
	r.complete()
	return nil
}

// Rollback rolls back the operation phase specified with phase
func (r *Client) Rollback(ctx context.Context, machine *fsm.FSM, params fsm.Params) error {
	progress := utils.NewProgress(ctx, fmt.Sprintf("Rolling back phase %q", params.PhaseID), -1, false)
	defer progress.Stop()
	err := machine.RollbackPhase(ctx, fsm.Params{
		PhaseID:  params.PhaseID,
		Force:    params.Force,
		Progress: progress,
	})
	return trace.Wrap(err)
}

// Completed returns true if the operation has already been completed
func (r *Client) Completed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.completed
}

func (r *Config) checkAndSetDefaults() error {
	if r.InterruptHandler == nil {
		return trace.BadParameter("InterruptHandler is required")
	}
	if r.Printer == nil {
		r.Printer = utils.DiscardPrinter
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:observer")
	}
	if r.ServicePath == "" {
		r.ServicePath = state.GravityInstallDir(defaults.GravityRPCInstallerServiceName)
	}
	if !filepath.IsAbs(r.ServicePath) {
		return trace.BadParameter("ServicePath needs to be absolute path")
	}
	if r.SocketPath == "" {
		r.SocketPath = installpb.SocketPath()
	}
	return nil
}

// Config specifies the configuration for the client
type Config struct {
	log.FieldLogger
	utils.Printer
	*signals.InterruptHandler
	// SocketPath specifies the path to the service socket file
	SocketPath string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Wait forever, if unspecified
	ConnectTimeout time.Duration
	// ServicePath specifies the absolute path to the service unit
	ServicePath string
}

func (r *Client) addTerminationHandler() {
	// FIXME: this client is used for remote 'plan execute' calls
	// during an operation, so it cannot shutdown the service
}

// complete uninstalls the service and cleans up the temporary state.
// Implements client.ProgressHandler
func (r *Client) complete() {
	r.mu.Lock()
	r.completed = true
	r.mu.Unlock()
	if err := cleanup.UninstallAgentServices(r.FieldLogger); err != nil {
		r.WithError(err).Warn("Failed to uninstall installer service.")
	}
	if err := os.RemoveAll(state.GravityInstallDir()); err != nil {
		r.WithError(err).Warn("Failed to remove installer state directory.")
	}
}

// shutdown signals the service to stop
func (r *Client) shutdown(ctx context.Context) error {
	_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
	return trace.Wrap(err)
}

// restartService restarts the installer's systemd unit
func (r *Client) restartService() error {
	return trace.Wrap(service.Start(r.ServicePath))
}

type Client struct {
	Config
	client installpb.AgentClient
	// mu guards fields below
	mu sync.Mutex
	// completed indicates whether the operation is complete
	completed bool
}
