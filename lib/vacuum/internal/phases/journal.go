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

package phases

import (
	"context"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/vacuum/prune"
	"github.com/gravitational/gravity/lib/vacuum/prune/journal"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewJournal returns a new executor to remove obsolete systemd journal
// directories inside the runtime container.
func NewJournal(params libfsm.ExecutorParams, runtimePath string, silent localenv.Silent, logger log.FieldLogger) (libfsm.PhaseExecutor, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logDir := state.LogDir(stateDir, "journal")
	machineIDFile := filepath.Join(runtimePath, constants.PlanetRootfs, defaults.SystemdMachineIDFile)
	pruner, err := journal.New(journal.Config{
		LogDir:        logDir,
		MachineIDFile: machineIDFile,
		Config: prune.Config{
			Silent:      silent,
			FieldLogger: logger,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &journalExecutor{
		FieldLogger: logger,
		Pruner:      pruner,
	}, nil
}

// Execute executes phase
func (r *journalExecutor) Execute(ctx context.Context) error {
	err := r.Prune(ctx)
	return trace.Wrap(err)
}

// PreCheck is a no-op
func (r *journalExecutor) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (r *journalExecutor) PostCheck(context.Context) error {
	return nil
}

// Rollback is a no-op
func (r *journalExecutor) Rollback(context.Context) error {
	return nil
}

type journalExecutor struct {
	// FieldLogger is the logger the executor uses
	log.FieldLogger
	// Pruner is the actual clean up implementation
	prune.Pruner
}
