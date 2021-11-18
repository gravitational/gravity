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

	libapp "github.com/gravitational/gravity/lib/app"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/system"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewRestart returns a new executor to restart the runtime container to apply
// the environment variables update
func NewRestart(
	params libfsm.ExecutorParams,
	operator LocalClusterGetter,
	operationID string,
	apps appGetter,
	backend storage.Backend,
	packages pack.PackageService,
	localPackages update.LocalPackageService,
	logger log.FieldLogger,
) (*Restart, error) {
	if params.Phase.Data == nil || params.Phase.Data.Package == nil {
		return nil, trace.NotFound("no installed application package specified for phase %q",
			params.Phase.ID)
	}
	if params.Phase.Data.Update == nil || len(params.Phase.Data.Update.Servers) != 1 {
		return nil, trace.NotFound("no server specified for phase %q",
			params.Phase.ID)
	}
	return &Restart{
		FieldLogger:          logger,
		WaitStatusOnRollback: true,
		operationID:          operationID,
		backend:              backend,
		packages:             packages,
		localPackages:        localPackages,
		update:               params.Phase.Data.Update.Servers[0],
	}, nil
}

// Execute restarts the runtime container with the new configuration package
func (r *Restart) Execute(ctx context.Context) error {
	err := r.pullUpdates(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	config := system.Config{
		ChangesetID: r.operationID,
		Backend:     r.backend,
		Packages:    r.localPackages,
		PackageUpdates: system.PackageUpdates{
			Runtime: storage.PackageUpdate{
				From: r.update.Runtime.Installed,
				To:   r.update.Runtime.Update.Package,
				ConfigPackage: &storage.PackageUpdate{
					To: r.update.Runtime.Update.ConfigPackage,
				},
			},
		},
	}
	if r.update.Runtime.SecretsPackage != nil {
		config.RuntimeSecrets = &storage.PackageUpdate{
			To: *r.update.Runtime.SecretsPackage,
		}
	}
	updater, err := system.New(config)
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.Update(ctx, true)
	return trace.Wrap(err)
}

// Rollback reverses the update and restarts the container with the old
// configuration package
func (r *Restart) Rollback(ctx context.Context) error {
	updater, err := system.New(system.Config{
		ChangesetID: r.operationID,
		Backend:     r.backend,
		Packages:    r.localPackages,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(updater.Rollback(ctx, r.WaitStatusOnRollback))
}

// PreCheck is a no-op
func (*Restart) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (*Restart) PostCheck(context.Context) error {
	return nil
}

func (r *Restart) pullUpdates(ctx context.Context) error {
	updates := []loc.Locator{r.update.Runtime.Update.Package, r.update.Runtime.Update.ConfigPackage}
	if r.update.Runtime.SecretsPackage != nil {
		updates = append(updates, *r.update.Runtime.SecretsPackage)
	}
	for _, update := range updates {
		r.Infof("Pulling package update: %v.", update)
		puller := libapp.Puller{
			SrcPack: r.packages,
			DstPack: r.localPackages,
		}
		err := puller.PullPackage(ctx, update)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

type Restart struct {
	// FieldLogger specifies the logger for the phase
	log.FieldLogger
	// WaitStatusOnRollback specifies whether the step blocks waiting for healthy status
	// when rolling back
	WaitStatusOnRollback bool
	backend              storage.Backend
	packages             pack.PackageService
	localPackages        update.LocalPackageService
	update               storage.UpdateServer
	operationID          string
}
