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

	libapp "github.com/gravitational/gravity/lib/app/service"
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
) (*restart, error) {
	if params.Phase.Data == nil || params.Phase.Data.Package == nil {
		return nil, trace.NotFound("no installed application package specified for phase %q",
			params.Phase.ID)
	}
	if params.Phase.Data.Update == nil || len(params.Phase.Data.Update.Servers) != 1 {
		return nil, trace.NotFound("no server specified for phase %q",
			params.Phase.ID)
	}
	return &restart{
		FieldLogger:   logger,
		operationID:   operationID,
		backend:       backend,
		packages:      packages,
		localPackages: localPackages,
		update:        params.Phase.Data.Update.Servers[0],
	}, nil
}

// Execute restarts the runtime container with the new configuration package
func (r *restart) Execute(ctx context.Context) error {
	err := r.pullUpdates()
	if err != nil {
		return trace.Wrap(err)
	}
	updater, err := system.New(system.Config{
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
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.Update(ctx, true)
	return trace.Wrap(err)
}

// Rollback reverses the update and restarts the container with the old
// configuration package
func (r *restart) Rollback(ctx context.Context) error {
	updater, err := system.New(system.Config{
		ChangesetID: r.operationID,
		Backend:     r.backend,
		Packages:    r.localPackages,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = updater.Rollback(ctx, true)
	return trace.Wrap(err)
}

// PreCheck is a no-op
func (*restart) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (*restart) PostCheck(context.Context) error {
	return nil
}

func (r *restart) pullUpdates() error {
	updates := []loc.Locator{r.update.Runtime.Update.Package, r.update.Runtime.Update.ConfigPackage}
	for _, update := range updates {
		r.Infof("Pulling package update: %v.", update)
		_, err := libapp.PullPackage(libapp.PackagePullRequest{
			SrcPack: r.packages,
			DstPack: r.localPackages,
			Package: update,
		})
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

type restart struct {
	// FieldLogger specifies the logger for the phase
	log.FieldLogger
	backend       storage.Backend
	packages      pack.PackageService
	localPackages update.LocalPackageService
	update        storage.UpdateServer
	operationID   string
}
