/*
Copyright 2018 Gravitational, Inc.

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

package vacuum

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	libpack "github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/vacuum/internal/fsm"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns new garbage collector for the specified configuration
func New(config Config) (*Collector, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Collector{
		Config: config,
	}, nil
}

// Run runs the garbage collection.
func (r *Collector) Run(ctx context.Context) error {
	machine, err := r.init()
	if err != nil {
		return trace.Wrap(err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- r.executePlan(ctx, machine)
	}()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var lastProgress *ops.ProgressEntry

L:
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if progress := r.updateProgress(lastProgress); progress != nil {
				lastProgress = progress
			}
		case err = <-errCh:
			break L
		}
	}

	return trace.Wrap(err)
}

// RunPhase runs the specified garbage collection phase.
func (r *Collector) RunPhase(ctx context.Context, phase string, phaseTimeout time.Duration, force bool) error {
	if phase == libfsm.RootPhase {
		return trace.Wrap(r.Run(ctx))
	}

	machine, err := r.init()
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(ctx, phaseTimeout)
	defer cancel()

	progress := utils.NewProgress(ctx, fmt.Sprintf("Executing phase %q", phase), -1, false)
	defer progress.Stop()

	return trace.Wrap(machine.ExecutePhase(ctx, libfsm.Params{
		PhaseID:  phase,
		Progress: progress,
		Force:    force,
	}))
}

// SetPhase sets the specified phase state without executing it.
func (r *Collector) SetPhase(ctx context.Context, phase, state string) error {
	machine, err := r.init()
	if err != nil {
		return trace.Wrap(err)
	}
	return machine.ChangePhaseState(ctx, libfsm.StateChange{
		Phase: phase,
		State: state,
	})
}

// Create creates the garbage collection operation but does not start it.
func (r *Collector) Create(ctx context.Context) error {
	_, err := r.init()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r *Collector) init() (*libfsm.FSM, error) {
	_, err := r.getOrCreateOperationPlan()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	machine, err := fsm.New(fsm.Config{
		App:           r.App,
		RemoteApps:    r.RemoteApps,
		Apps:          r.Apps,
		Packages:      r.Packages,
		LocalPackages: r.LocalPackages,
		Operation:     r.Operation,
		Operator:      r.Operator,
		RuntimePath:   r.RuntimePath,
		Runner:        r.Runner,
		Silent:        r.Silent,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return machine, nil
}

func (r *Collector) executePlan(ctx context.Context, machine *libfsm.FSM) error {
	planErr := machine.ExecutePlan(ctx, nil)
	if planErr != nil {
		r.Warnf("Failed to execute plan: %v.", trace.DebugReport(planErr))
	}

	err := machine.Complete(ctx, planErr)
	if err == nil {
		err = planErr
	}

	var addrs []string
	for _, server := range r.Servers {
		addrs = append(addrs, server.AdvertiseIP)
	}

	// Keep the agents running as long as the operation can be resumed
	if planErr == nil {
		if errShutdown := rpc.ShutdownAgents(ctx, addrs, r.FieldLogger, r.Runner); errShutdown != nil {
			r.Warnf("Failed to shutdown agents: %v.", trace.DebugReport(errShutdown))
		}
	}
	return trace.Wrap(err)
}

func (r *Collector) updateProgress(lastProgress *ops.ProgressEntry) *ops.ProgressEntry {
	progress, err := r.Operator.GetSiteOperationProgress(r.Operation.Key())
	if err != nil {
		log.WithError(err).Warn("Failed to query operation progress.")
		return nil
	}
	if lastProgress == nil || !lastProgress.IsEqual(*progress) {
		r.Silent.Printf("%v\t%v\n", time.Now().UTC().Format(constants.HumanDateFormatSeconds),
			progress.Message)
	}
	return progress
}

func (r *Config) checkAndSetDefaults() error {
	if r.App == nil {
		return trace.BadParameter("application package is required")
	}
	if r.Apps == nil {
		return trace.BadParameter("application service is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("package service is required")
	}
	if r.LocalPackages == nil {
		return trace.BadParameter("local package service is required")
	}
	if r.Operator == nil {
		return trace.BadParameter("cluster operator service is required")
	}
	if r.Operation == nil {
		return trace.BadParameter("cluster operation is required")
	}
	if len(r.Servers) == 0 {
		return trace.BadParameter("at least a single server is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "gc:collector")
	}
	return nil
}

// Config describes configuration of the cluster garbage collector
type Config struct {
	// ClusterKey identifies the cluster
	ClusterKey ops.SiteKey
	// App specifies the cluster application
	App *storage.Application
	// RemoteApps lists optional applications from remote clusters
	RemoteApps []storage.Application
	// Apps is the cluster application service
	Apps app.Applications
	// Packages is the cluster package service
	Packages libpack.PackageService
	// LocalPackages is the service for packages local to the node
	LocalPackages libpack.PackageService
	// Operator is the cluster operator service
	Operator ops.Operator
	// Operation references the garbage collection operation to work with
	Operation *ops.SiteOperation
	// Servers is the list of cluster servers
	Servers []storage.Server
	// Runner specifies the runner for remote commands
	Runner rpc.AgentRepository
	// RuntimePath is the path to the runtime container's rootfs
	RuntimePath string
	// FieldLogger is the logger to use
	log.FieldLogger
	// Silent controls whether the process outputs messages to stdout
	localenv.Silent
}

type Collector struct {
	// Config is the collector's configuration
	Config
}
