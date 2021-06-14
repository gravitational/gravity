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
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/systemservice"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewRestart returns executor that restarts systemd unit for the specified package.
func NewRestart(p fsm.ExecutorParams, operator ops.Operator, packages pack.PackageService) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	return &restartExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		LocalPackages:  packages,
		Package:        *p.Phase.Data.Package,
	}, nil
}

type restartExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
	// LocalPackages is the machine-local package service.
	LocalPackages pack.PackageService
	// Package is the locator of the package service to restart.
	Package loc.Locator
}

// Execute restarts specified package service.
func (p *restartExecutor) Execute(ctx context.Context) error {
	installed, err := pack.FindInstalledPackage(p.LocalPackages, p.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	svm, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	p.Progress.NextStep("Restarting system service %s", installed)
	if err := svm.RestartPackageService(*installed); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is no-op for this phase.
func (*restartExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*restartExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*restartExecutor) PostCheck(ctx context.Context) error {
	return nil
}
