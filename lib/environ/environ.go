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

package environ

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/environ/internal/fsm"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns new updater for the specified configuration
func New(config Config) (*Updater, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Updater{
		Config: config,
	}, nil
}

// Run updates the environment variables in the cluster
func (r *Updater) Run(ctx context.Context, force bool) (err error) {
	machine, err := r.init()
	if err != nil {
		return trace.Wrap(err)
	}

	errCh := make(chan error, 1)
	updateCh := make(chan ops.ProgressEntry)
	go func() {
		errCh <- r.executePlan(ctx, machine, force)
	}()
	go pollProgress(ctx, updateCh, r.Operation.Key(), r.Operator)

L:
	for {
		select {
		case <-ctx.Done():
			return nil
		case progress := <-updateCh:
			r.Emitter.PrintStep(progress.Message)
		case err = <-errCh:
			break L
		}
	}

	return trace.Wrap(err)
}

// RunPhase runs the specified phase.
func (r *Updater) RunPhase(ctx context.Context, phase string, phaseTimeout time.Duration, force bool) error {
	if phase == libfsm.RootPhase {
		return trace.Wrap(r.Run(ctx, force))
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

// Rollbackhase rolls back the specified phase.
func (r *Updater) RollbackPhase(ctx context.Context, phase string, phaseTimeout time.Duration, force bool) error {
	machine, err := r.init()
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(ctx, phaseTimeout)
	defer cancel()

	progress := utils.NewProgress(ctx, fmt.Sprintf("Rolling back phase %q", phase), -1, false)
	defer progress.Stop()

	return trace.Wrap(machine.RollbackPhase(ctx, libfsm.Params{
		PhaseID:  phase,
		Progress: progress,
		Force:    force,
	}))
}

// Create creates the update operation but does not start it.
func (r *Updater) Create(ctx context.Context) error {
	_, err := r.init()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r *Updater) init() (*libfsm.FSM, error) {
	_, err := r.getOrCreateOperationPlan()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	machine, err := fsm.New(fsm.Config{
		Operation: r.Operation,
		Operator:  r.Operator,
		Runner:    r.Runner,
		Silent:    r.Silent,
		Emitter:   r.Emitter,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return machine, nil
}

func (r *Updater) executePlan(ctx context.Context, machine *libfsm.FSM, force bool) error {
	planErr := machine.ExecutePlan(ctx, nil, force)
	if planErr != nil {
		r.Warnf("Failed to execute plan: %v.", trace.DebugReport(planErr))
	}

	// FIXME: wrap this (or inners) in a retry loop, as cluster controller
	// might be temporarily unavailable (connection refused)
	err := machine.Complete(planErr)
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

func (r *Config) checkAndSetDefaults() error {
	if r.Operator == nil {
		return trace.BadParameter("cluster operator service is required")
	}
	if len(r.Servers) == 0 {
		return trace.BadParameter("at least a single server is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "envars:updater")
	}
	if r.Emitter == nil {
		r.Emitter = utils.NopEmitter()
	}
	return nil
}

// Config describes configuration for updating cluster environment variables
type Config struct {
	// ClusterKey identifies the cluster
	ClusterKey ops.SiteKey
	// Operator is the cluster operator service
	Operator ops.Operator
	// Operation references a potentially active garbage collection operation
	Operation *ops.SiteOperation
	// Servers is the list of cluster servers
	Servers []storage.Server
	// Runner specifies the runner for remote commands
	Runner libfsm.AgentRepository
	// FieldLogger is the logger to use
	log.FieldLogger
	// Silent controls whether the process outputs messages to stdout
	localenv.Silent
	// Emitter outputs progress messages to stdout
	utils.Emitter
}

// Updater executes a cobntrolled update of cluster environment variables
type Updater struct {
	// Config defines the updater configuration
	Config
}

func pollProgress(ctx context.Context, updateCh chan<- ops.ProgressEntry, opKey ops.SiteOperationKey, operator ops.Operator) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var lastProgress *ops.ProgressEntry
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			progress, err := operator.GetSiteOperationProgress(opKey)
			if err != nil {
				log.WithError(err).Warn("Failed to query operation progress.")
				continue
			}
			if lastProgress == nil || !lastProgress.IsEqual(*progress) {
				select {
				case <-ctx.Done():
					return
				case updateCh <- *progress:
				}
			}
			if progress.IsCompleted() {
				return
			}
			lastProgress = progress
		}
	}
}
