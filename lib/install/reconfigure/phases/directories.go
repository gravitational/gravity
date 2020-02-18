/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewDirectories returns executor that cleans up directories on the node.
//
// Specifically, it removes the directories where Teleport node and auth server
// keep their data so they regenerate their secrets upon startup.
func NewDirectories(p fsm.ExecutorParams, operator ops.Operator) (*directoriesExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	return &directoriesExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
	}, nil
}

type directoriesExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
}

// Execute wipes directories where Teleport node and auth server keep their data
// so they can regenerate their secrets simulating the first start.
func (p *directoriesExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Cleaning up directories")
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, dir := range []string{
		state.TeleportNodeDataDir(stateDir),
		state.TeleportAuthDataDir(stateDir),
	} {
		if err := utils.RemoveContents(dir); err != nil {
			return trace.Wrap(err)
		}
		p.Infof("Cleaned up directory %v", dir)
	}
	return nil
}

// Rollback is no-op for this phase.
func (*directoriesExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*directoriesExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*directoriesExecutor) PostCheck(ctx context.Context) error {
	return nil
}
