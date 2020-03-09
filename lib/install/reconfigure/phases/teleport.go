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
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewTeleport returns executor that restarts teleport node so it can regenerate
// the secrets and reconnect to the auth server.
func NewTeleport(p fsm.ExecutorParams, operator ops.Operator) (*teleportExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	return &teleportExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Package:        *p.Phase.Data.Package,
	}, nil
}

type teleportExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
	// Package is the locator of the installed teleport package.
	Package loc.Locator
}

// Execute restarts teleport node service.
func (p *teleportExecutor) Execute(ctx context.Context) error {
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	p.Progress.NextStep("Restarting system service %s", p.Package)
	if err := svm.RestartPackageService(p.Package); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is no-op for this phase.
func (*teleportExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*teleportExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*teleportExecutor) PostCheck(ctx context.Context) error {
	return nil
}
