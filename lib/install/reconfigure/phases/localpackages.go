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
	"github.com/gravitational/gravity/lib/pack"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewPackages returns executor that removes old packages on the node.
//
// Specifically, it removes configuration and secret packages left from
// the original installation from the node local state.
func NewPackages(p fsm.ExecutorParams, operator ops.Operator, packages pack.PackageService) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	return &packagesExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		LocalPackages:  packages,
	}, nil
}

type packagesExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
	// LocalPackages is the node-local package service.
	LocalPackages pack.PackageService
}

// Execute removes old configuration & secret packages from the node.
func (p *packagesExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Cleaning up local packages")
	err := pack.ForeachPackage(p.LocalPackages, func(e pack.PackageEnvelope) error {
		if val, ok := e.RuntimeLabels[pack.AdvertiseIPLabel]; ok {
			if val != p.Phase.Data.Server.AdvertiseIP {
				err := p.LocalPackages.DeletePackage(e.Locator)
				if err != nil {
					return trace.Wrap(err)
				}
				p.Infof("Removed local package %v", e.Locator)
			}
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Rollback is no-op for this phase.
func (*packagesExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*packagesExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*packagesExecutor) PostCheck(ctx context.Context) error {
	return nil
}
