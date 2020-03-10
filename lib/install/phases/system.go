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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update/system"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewSystem returns a new "system" phase executor
func NewSystem(p fsm.ExecutorParams, operator ops.Operator, localPackages *localpack.PackageServer, remote fsm.Remote) (*systemExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase:       p.Phase.ID,
			constants.FieldAdvertiseIP: p.Phase.Data.Server.AdvertiseIP,
			constants.FieldHostname:    p.Phase.Data.Server.Hostname,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	updater := system.PackageUpdater{
		Packages:    localPackages,
		ClusterRole: p.Phase.Data.Server.ClusterRole,
	}
	return &systemExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		remote:         remote,
		updater:        updater,
	}, nil
}

type systemExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	// remote specifies the server remote control interface
	remote  fsm.Remote
	updater system.PackageUpdater
}

// Execute executes the system phase
func (p *systemExecutor) Execute(ctx context.Context) error {
	locator := p.Phase.Data.Package
	p.Progress.NextStep("Installing system service %v:%v",
		locator.Name, locator.Version)
	p.Infof("Installing system service %v:%v", locator.Name, locator.Version)
	return p.updater.Reinstall(storage.PackageUpdate{
		From:   *p.Phase.Data.Package,
		To:     *p.Phase.Data.Package,
		Labels: p.Phase.Data.Labels,
	})
}

// Rollback is no-op for this phase
func (*systemExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure this phase is executed on a proper node
func (p *systemExecutor) PreCheck(ctx context.Context) error {
	err := p.remote.CheckServer(ctx, *p.Phase.Data.Server)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*systemExecutor) PostCheck(ctx context.Context) error {
	return nil
}
