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

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewFlannelRestartPhase returns executor that restarts flanneld.
func NewFlannelRestartPhase(p fsm.ExecutorParams, logger logrus.FieldLogger) (*flannelRestart, error) {
	return &flannelRestart{
		FieldLogger: logger,
	}, nil
}

// Execute cleans up after the upgrade.
//
// Clean up tasks include:
//  * trimming the container journal
func (r *flannelRestart) Execute(ctx context.Context) error {
	r.Info("Restarting flanneld.")
	out, err := fsm.RunCommand(utils.PlanetCommandArgs(defaults.SystemctlBin, "restart", "flanneld"))
	if err != nil {
		return trace.Wrap(err, "failed to restart flanneld: %s", string(out))
	}
	return nil
}

// Rollback is no-op for this phase.
func (*flannelRestart) Rollback(context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*flannelRestart) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*flannelRestart) PostCheck(context.Context) error {
	return nil
}

// flannelRestart is the phase that restarts flanneld.
type flannelRestart struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
}
