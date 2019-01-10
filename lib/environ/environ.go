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

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/environ/internal/fsm"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// New returns new updater for the specified configuration
func New(ctx context.Context, config Config) (*Updater, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	machine, err := newMachine(ctx, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Updater{
		Config:  config,
		machine: machine,
	}, nil
}

// Run updates the environment variables in the cluster
func (r *Updater) Run(ctx context.Context, force bool) (err error) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- r.executePlan(ctx, r.machine, force)
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
	if phase == libfsm.RootPhase {
		return trace.Wrap(r.Run(ctx, force))
	}

	ctx, cancel := context.WithTimeout(ctx, phaseTimeout)
	defer cancel()

	progress := utils.NewProgress(ctx, fmt.Sprintf("Executing phase %q", phase), -1, false)
	defer progress.Stop()

	return trace.Wrap(r.machine.ExecutePhase(ctx, libfsm.Params{
		PhaseID:  phase,
		Progress: progress,
		Force:    force,
	}))
}

// RollbackPhase rolls back the specified phase.
func (r *Updater) RollbackPhase(ctx context.Context, phase string, phaseTimeout time.Duration, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, phaseTimeout)
	defer cancel()

	progress := utils.NewProgress(ctx, fmt.Sprintf("Rolling back phase %q", phase), -1, false)
	defer progress.Stop()

	return trace.Wrap(r.machine.RollbackPhase(ctx, libfsm.Params{
		PhaseID:  phase,
		Progress: progress,
		Force:    force,
	}))
}

// Complete completes the active operation
func (r *Updater) Complete() error {
	err := r.machine.Complete(trace.Errorf("completed manually"))
	if err != nil {
		return trace.Wrap(err)
	}
	err = r.Operator.ActivateSite(ops.ActivateSiteRequest{
		AccountID:  r.ClusterKey.AccountID,
		SiteDomain: r.ClusterKey.SiteDomain,
	})
	return trace.Wrap(err)
}

// GetPlan returns the up-to-date operation plan
func (r *Updater) GetPlan() (*storage.OperationPlan, error) {
	return r.machine.GetPlan()
}

func (r *Updater) executePlan(ctx context.Context, machine *libfsm.FSM, force bool) error {
	progress := utils.NewProgress(ctx, "Updating envars", -1, false)
	defer progress.Stop()

	planErr := machine.ExecutePlan(ctx, progress, force)
	if planErr != nil {
		r.Warnf("Failed to execute plan: %v.", trace.DebugReport(planErr))
	}

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

func (r *Config) checkAndSetDefaults() error {
	if r.Operator == nil {
		return trace.BadParameter("cluster operator service is required")
	}
	if r.Apps == nil {
		return trace.BadParameter("cluster application service is required")
	}
	if len(r.Servers) == 0 {
		return trace.BadParameter("at least a single server is required")
	}
	if r.Backend == nil {
		return trace.BadParameter("primary backend is required")
	}
	if r.LocalBackend == nil {
		return trace.BadParameter("local backend is required")
	}
	if r.ClusterPackages == nil {
		return trace.BadParameter("cluster package service is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "envars:updater")
	}
	return nil
}

// Config describes configuration for updating cluster environment variables
type Config struct {
	// ClusterKey identifies the cluster
	ClusterKey ops.SiteKey
	// Operator is the cluster operator service
	Operator ops.Operator
	// Apps is the cluster application service
	Apps app.Applications
	// Operation references a potentially active garbage collection operation
	Operation *ops.SiteOperation
	// When available, used to sync state between nodes
	Backend storage.Backend
	// LocalBackend specifies the authoritative source for operation state
	LocalBackend storage.Backend
	// ClusterPackages specifies the cluster package service
	ClusterPackages pack.PackageService
	// Client specifies the optional kubernetes client
	Client *kubernetes.Clientset
	// Servers is the list of cluster servers
	Servers []storage.Server
	// Runner specifies the runner for remote commands
	Runner libfsm.AgentRepository
	// FieldLogger is the logger to use
	log.FieldLogger
	// Silent controls whether the process outputs messages to stdout
	localenv.Silent
}

// Updater executes a controlled update of cluster environment variables
type Updater struct {
	// Config defines the updater configuration
	Config
	machine *libfsm.FSM
}

func newMachine(ctx context.Context, config Config) (*libfsm.FSM, error) {
	plan, err := getOrCreateOperationPlan(config.Operator, *config.Operation, config.Servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	machine, err := fsm.New(ctx, fsm.Config{
		Operation:       config.Operation,
		Operator:        config.Operator,
		Apps:            config.Apps,
		Backend:         config.Backend,
		LocalBackend:    config.LocalBackend,
		ClusterPackages: config.ClusterPackages,
		Client:          config.Client,
		Runner:          config.Runner,
		Silent:          config.Silent,
		Plan:            *plan,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return machine, nil
}
