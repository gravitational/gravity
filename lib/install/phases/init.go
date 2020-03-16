/*
Copyright 2019 Gravitational, Inc.

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

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewInit returns executor that prepares the node for the operation.
func NewInit(p fsm.ExecutorParams, operator ops.Operator, apps app.Applications, packages pack.PackageService) (*initExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         opKey(p.Plan),
		Operator:    operator,
	}
	app, err := apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &initExecutor{
		FieldLogger:    logger,
		Operator:       operator,
		Applications:   apps,
		Packages:       packages,
		Application:    *app,
		ExecutorParams: p,
	}, nil
}

type initExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// Operator is the installer or cluster operator service.
	Operator ops.Operator
	// Applications is the installer of cluster applications service.
	Applications app.Applications
	// Packages is the installer or cluster package service.
	Packages pack.PackageService
	// Application is the cluster application.
	Application app.Application
	// ExecutorParams is common executor params.
	fsm.ExecutorParams
}

// Execute prepares the node for the operation.
func (p *initExecutor) Execute(ctx context.Context) error {
	if err := p.downloadFio(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// downloadFio downloads fio tool from the configured package service and
// places it in a temporary directory on the node.
func (p *initExecutor) downloadFio() error {
	locator, err := p.Application.Manifest.Dependencies.ByName(constants.FioPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	path, err := state.InStateDir(constants.FioBin)
	if err != nil {
		return trace.Wrap(err)
	}
	err = pack.ExportExecutable(p.Packages, *locator, path, defaults.GravityFileLabel)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Exported fio v%v to %v.", locator.Version, path)
	return nil
}

// Rollback is no-op for this phase.
func (*initExecutor) Rollback(context.Context) error { return nil }

// PreCheck is no-op for this phase.
func (*initExecutor) PreCheck(context.Context) error { return nil }

// PostCheck is no-op for this phase.
func (*initExecutor) PostCheck(context.Context) error { return nil }
