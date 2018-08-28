package phases

import (
	"context"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/vacuum/prune"
	"github.com/gravitational/gravity/lib/vacuum/prune/journal"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewJournal returns a new executor to remove obsolete systemd journal
// directories inside the runtime container.
func NewJournal(params libfsm.ExecutorParams, runtimePath string, emitter utils.Emitter) (*journalExecutor, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log := log.WithField(trace.Component, "gc:journal")
	logDir := state.LogDir(stateDir, "journal")
	machineIDFile := filepath.Join(runtimePath, constants.PlanetRootfs, defaults.SystemdMachineIDFile)
	pruner, err := journal.New(journal.Config{
		LogDir:        logDir,
		MachineIDFile: machineIDFile,
		Config: prune.Config{
			Emitter:     emitter,
			FieldLogger: log.WithField("phase", params.Phase),
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &journalExecutor{
		FieldLogger: log,
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
