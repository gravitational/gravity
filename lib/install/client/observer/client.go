package observer

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/fsm"
	installpb "github.com/gravitational/gravity/lib/install/proto"
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
func New(ctx context.Context, config Config) (*client, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c := &client{Config: config}
	err = restartService(config.ServiceName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if config.ConnectTimeout != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.ConnectTimeout)
		defer cancel()
	}
	cc, err := installpb.NewClient(ctx, config.StateDir, config.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.client = cc
	c.addTerminationHandler()
	return c, nil
}

// ExecutePhase executes the operation phase specified with phase
func (r *client) ExecutePhase(ctx context.Context, machine *fsm.FSM, config fsm.Params) error {
	if config.PhaseID == fsm.RootPhase {
		progress := utils.NewProgress(ctx, "Resuming operation", -1, false)
		defer progress.Stop()

		err := machine.ExecutePlan(ctx, progress)
		if err != nil {
			r.WithError(err).Warn("Failed to execute plan.")
		}
		return trace.Wrap(machine.Complete(err))
	}
	progress := utils.NewProgress(ctx, fmt.Sprintf("Executing phase %q", config.PhaseID), -1, false)
	defer progress.Stop()
	err := machine.ExecutePhase(ctx, fsm.Params{
		PhaseID:  config.PhaseID,
		Force:    config.Force,
		Progress: progress,
	})
	return trace.Wrap(err)
}

func (r *Config) checkAndSetDefaults() error {
	if r.StateDir == "" {
		return trace.BadParameter("StateDir is required")
	}
	if r.InterruptHandler == nil {
		return trace.BadParameter("InterruptHandler is required")
	}
	if r.Printer == nil {
		r.Printer = utils.DiscardPrinter
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "client:observer")
	}
	return nil
}

// Config specifies the configuration for the client
type Config struct {
	log.FieldLogger
	utils.Printer
	*signals.InterruptHandler
	// StateDir specifies the install state directory on local host
	StateDir string
	// ConnectTimeout specifies the maximum amount of time to wait for
	// installer service connection. Wait forever, if unspecified
	ConnectTimeout time.Duration
	// ServiceName specifies the name of the service unit
	ServiceName string
}

func (r *client) addTerminationHandler() {
	r.InterruptHandler.AddStopper(signals.StopperFunc(func(ctx context.Context) error {
		_, err := r.client.Shutdown(ctx, &installpb.ShutdownRequest{})
		return trace.Wrap(err)
	}))
}

type client struct {
	Config
	client installpb.AgentClient
}

// restartService restarts the installer's systemd unit
func restartService(name string) error {
	return trace.Wrap(service.Start(name))
}

func isDone(doneC <-chan struct{}) bool {
	select {
	case <-doneC:
		return true
	default:
		return false
	}
}
