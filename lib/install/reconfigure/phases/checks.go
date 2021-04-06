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

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewChecks returns executor that executes preflight checks on the node.
func NewChecks(p fsm.ExecutorParams, operator ops.Operator) (*checksExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	return &checksExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Operator:       operator,
	}, nil
}

type checksExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams is common executor params.
	fsm.ExecutorParams
	// Operator is the cluster operator service.
	Operator ops.Operator
}

// Execute executes preflight checks on the joining node.
func (p *checksExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Executing preflight checks")
	cluster, err := ops.GetWizardCluster(p.Operator)
	if err != nil {
		return trace.Wrap(err)
	}
	err = checks.RunLocalChecks(ctx, checks.LocalChecksRequest{
		Manifest: cluster.App.Manifest,
		Role:     p.Phase.Data.Server.Role,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is no-op for this phase.
func (*checksExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*checksExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*checksExecutor) PostCheck(ctx context.Context) error {
	return nil
}
