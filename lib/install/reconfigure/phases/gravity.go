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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewGravity returns executor that waits for gravity-site to become available.
func NewGravity(p fsm.ExecutorParams, operator ops.Operator) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	return &gravityExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
	}, nil
}

type gravityExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
}

// Execute waits for gravity-site to become available.
func (p *gravityExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Waiting for Gravity API to become available")
	operator, err := localenv.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	return utils.RetryFor(ctx, 5*time.Minute, func() error {
		if _, err := operator.GetLocalSite(ctx); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
}

// Rollback is no-op for this phase.
func (*gravityExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*gravityExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*gravityExecutor) PostCheck(ctx context.Context) error {
	return nil
}
