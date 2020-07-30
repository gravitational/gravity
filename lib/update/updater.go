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

// This file implements a controller to run an update operation
package update

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewUpdater returns a new updater for the specified operation using the given machine
func NewUpdater(ctx context.Context, config Config, machine *fsm.FSM) (*Updater, error) {
	// TODO: avoid multiple CheckAndSetDefaults
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	plan, err := machine.GetPlan()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Updater{
		Config:  config,
		machine: machine,
		servers: plan.Servers,
	}, nil
}

// Run executes the operation plan to completion
func (r *Updater) Run(ctx context.Context) (err error) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- r.executePlan(ctx)
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

// RunPhase runs the specified phase.
func (r *Updater) RunPhase(ctx context.Context, phase string, phaseTimeout time.Duration, force bool) error {
	if phase == fsm.RootPhase {
		return trace.Wrap(r.Run(ctx))
	}

	ctx, cancel := context.WithTimeout(ctx, phaseTimeout)
	defer cancel()

	progress := utils.NewProgress(ctx, fmt.Sprintf("Executing phase %q", phase), -1, false)
	defer progress.Stop()

	return trace.Wrap(r.machine.ExecutePhase(ctx, fsm.Params{
		PhaseID:  phase,
		Progress: progress,
		Force:    force,
	}))
}

// SetPhase sets phase state without executing it.
func (r *Updater) SetPhase(ctx context.Context, phase, state string) error {
	return r.machine.ChangePhaseState(ctx, fsm.StateChange{
		Phase: phase,
		State: state,
	})
}

// Check validates the provided FSM parameters.
func (r *Updater) Check(params fsm.Params) error {
	if params.PhaseID != fsm.RootPhase {
		return nil
	}
	// Make sure resume is launched from the correct node.
	plan, err := r.GetPlan()
	if err != nil {
		return trace.Wrap(err)
	}
	err = fsm.CheckPlanCoordinator(plan)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// RollbackPhase rolls back the specified phase.
func (r *Updater) RollbackPhase(ctx context.Context, params fsm.Params, phaseTimeout time.Duration) error {
	if params.PhaseID == fsm.RootPhase {
		return r.rollbackPlan(ctx, params.DryRun)
	}

	ctx, cancel := context.WithTimeout(ctx, phaseTimeout)
	defer cancel()

	progress := utils.NewProgress(ctx, fmt.Sprintf("Rolling back phase %q", params.PhaseID), -1, false)
	defer progress.Stop()

	return trace.Wrap(r.machine.RollbackPhase(ctx, fsm.Params{
		PhaseID:  params.PhaseID,
		Progress: progress,
		Force:    params.Force,
	}))
}

// Complete completes the active operation
func (r *Updater) Complete(ctx context.Context, fsmErr error) error {
	if fsmErr == nil {
		fsmErr = trace.Errorf("completed manually")
	}
	if err := r.machine.Complete(ctx, fsmErr); err != nil {
		return trace.Wrap(err)
	}
	if err := r.emitAuditEvent(context.TODO()); err != nil {
		log.WithError(err).Warn("Failed to emit audit event.")
	}
	return nil
}

// Activate activates the cluster.
func (r *Updater) Activate() error {
	return r.Operator.ActivateSite(ops.ActivateSiteRequest{
		AccountID:  r.Operation.AccountID,
		SiteDomain: r.Operation.SiteDomain,
	})
}

func (r *Updater) emitAuditEvent(ctx context.Context) error {
	clusterOperator, err := localenv.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	operation, err := r.Operator.GetSiteOperation(r.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	err = events.EmitForOperation(ctx, clusterOperator, *operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetPlan returns the up-to-date operation plan
func (r *Updater) GetPlan() (*storage.OperationPlan, error) {
	return r.machine.GetPlan()
}

// Close closes the underlying FSM
func (r *Updater) Close() error {
	return r.machine.Close()
}

func (r *Updater) rollbackPlan(ctx context.Context, dryRun bool) error {
	progress := utils.NewProgress(ctx, formatOperation(*r.Operation), -1, false)
	defer progress.Stop()

	if err := r.machine.RollbackPlan(ctx, progress, dryRun); err != nil {
		return trace.Wrap(err)
	}

	if !dryRun {
		if err := r.machine.Complete(ctx, trace.BadParameter("rolled back")); err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

func (r *Updater) executePlan(ctx context.Context) error {
	progress := utils.NewProgress(ctx, formatOperation(*r.Operation), -1, false)
	defer progress.Stop()

	planErr := r.machine.ExecutePlan(ctx, progress)
	if planErr != nil {
		r.WithError(planErr).Warn("Failed to execute plan.")
	}

	err := r.machine.Complete(ctx, planErr)
	if err == nil {
		err = planErr
	}

	// Keep the agents running as long as the operation can be resumed
	if planErr != nil {
		return trace.Wrap(err)
	}

	var addrs []string
	for _, server := range r.servers {
		addrs = append(addrs, server.AdvertiseIP)
	}
	if errShutdown := rpc.ShutdownAgents(ctx, addrs, r.FieldLogger, r.Runner); errShutdown != nil {
		r.WithError(errShutdown).Warn("Failed to shutdown agents.")
	}
	return nil
}

func (r *Updater) updateProgress(lastProgress *ops.ProgressEntry) *ops.ProgressEntry {
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

// CheckAndSetDefaults validates this object and sets defaults where necessary
func (r *Config) CheckAndSetDefaults() error {
	if r.Operator == nil {
		return trace.BadParameter("cluster operator service is required")
	}
	if r.Operation == nil {
		return trace.BadParameter("cluster operation is required")
	}
	if r.Backend == nil {
		return trace.BadParameter("backend is required")
	}
	if r.LocalBackend == nil {
		return trace.BadParameter("local backend is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithFields(log.Fields{
			trace.Component: "update",
			"operation":     r.Operation,
		})
	}
	return nil
}

// Config describes configuration for executing an update operation
type Config struct {
	// Operation references the update operation
	Operation *ops.SiteOperation
	// Operator is the cluster operator service
	Operator ops.Operator
	// Backend specifies the cluster backend
	Backend storage.Backend
	// LocalBackend specifies the authoritative source for operation state
	LocalBackend storage.Backend
	// Runner specifies the runner for remote commands
	Runner rpc.AgentRepository
	// FieldLogger is the logger to use
	log.FieldLogger
	// Silent controls whether the process outputs messages to stdout
	localenv.Silent
}

// Updater manages the operation specified with machine
type Updater struct {
	// Config defines the updater configuration
	Config
	machine *fsm.FSM
	servers []storage.Server
}

// LocalPackageService defines a package service on local host
type LocalPackageService interface {
	pack.PackageService
	UnpackedPath(loc.Locator) (path string, err error)
	Unpack(loc loc.Locator, targetDir string) error
	GetPackageManifest(loc loc.Locator) (*pack.Manifest, error)
}
