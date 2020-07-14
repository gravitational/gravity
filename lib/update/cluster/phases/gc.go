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

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewGarbageCollectPhase returns a new executor for the garbage collection phase
func NewGarbageCollectPhase(p fsm.ExecutorParams, remote fsm.Remote, logger log.FieldLogger) (*phaseGC, error) {
	return &phaseGC{
		FieldLogger: logger,
		Server:      *p.Phase.Data.Server,
		remote:      remote,
	}, nil
}

// Execute cleans up after the upgrade.
//
// Clean up tasks include:
//  * trimming the container journal
func (r *phaseGC) Execute(ctx context.Context) error {
	if err := trimJournalFiles(r.remote, r.FieldLogger); err != nil {
		r.Warnf("Failed to clean up obsolete journal files: %v.", err)
	}
	return nil
}

// Rollback is a no-op for this phase
func (*phaseGC) Rollback(context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*phaseGC) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*phaseGC) PostCheck(context.Context) error {
	return nil
}

func trimJournalFiles(remote fsm.Remote, logger log.FieldLogger) error {
	logger.Info("Garbage collect obsolete journal files.")
	commands := [][]string{
		// Force flush journal buffers and rotate files
		utils.PlanetCommandArgs(defaults.JournalctlBin, "--flush", "--rotate"),
		// Discard stale journal directories left from previous container starts
		utils.PlanetCommandArgs(defaults.GravityBin,
			"system", "gc", "journal", "--debug"),
	}
	for _, command := range commands {
		out, err := fsm.RunCommand(command)
		if err != nil {
			return trace.Wrap(err, "failed to execute %q: %s", command, out)
		}
		log.WithField("command", command).Debug(string(out))
	}
	return nil
}

// phaseGC is the phase that executes clean up tasks after the upgrade
type phaseGC struct {
	log.FieldLogger
	// Server is the server this phase operates on
	Server storage.Server
	remote fsm.Remote
}
