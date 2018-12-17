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
	"bytes"
	"context"
	"os/exec"

	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewSync returns a new executor to update cluster environment variables on the specified node
func NewSync(params libfsm.ExecutorParams, emitter utils.Emitter, logger log.FieldLogger) (*nodeSyncer, error) {
	return &nodeSyncer{
		FieldLogger: logger,
	}, nil
}

// Execute updates environment variables on the underlying node
func (r *nodeSyncer) Execute(ctx context.Context) error {
	args := utils.PlanetCommandArgs("update-env")
	cmd := exec.Command(args[0], args[1:]...)
	var buf bytes.Buffer
	err := utils.ExecL(cmd, &buf, r.FieldLogger)
	if err != nil {
		r.WithField("output", buf.String()).Warn("Failed to update cluster environment variables.")
		return trace.Wrap(err)
	}
	return nil
}

// Rollback restores the previous cluster environment variables
func (r *nodeSyncer) Rollback(context.Context) error {
	// FIXME: use a simple rollback from a backup file
	args := utils.PlanetCommandArgs("restore-env")
	cmd := exec.Command(args[0], args[1:]...)
	var buf bytes.Buffer
	err := utils.ExecL(cmd, &buf, r.FieldLogger)
	if err != nil {
		r.WithField("output", buf.String()).Warn("Failed to restore cluster environment variables.")
		return trace.Wrap(err)
	}
	return nil
}

// PreCheck is a no-op
func (r *nodeSyncer) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (r *nodeSyncer) PostCheck(context.Context) error {
	return nil
}

type nodeSyncer struct {
	// FieldLogger is the logger the executor uses
	log.FieldLogger
}
