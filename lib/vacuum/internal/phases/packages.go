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

	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	libpack "github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/vacuum/prune"
	"github.com/gravitational/gravity/lib/vacuum/prune/pack"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewPackages creates a new executor that removes unused telekube packages
func NewPackages(
	params libfsm.ExecutorParams,
	app storage.Application,
	packages libpack.PackageService,
	silent localenv.Silent,
	logger log.FieldLogger,
) (*packageExecutor, error) {
	var remoteApps []storage.Application
	if params.Phase.Data != nil && params.Phase.Data.GarbageCollect != nil {
		remoteApps = params.Phase.Data.GarbageCollect.RemoteApps
	}
	pruner, err := pack.New(pack.Config{
		Packages: packages,
		App:      &app,
		Apps:     remoteApps,
		Config: prune.Config{
			Silent:      silent,
			FieldLogger: logger,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &packageExecutor{
		FieldLogger: logger,
		Pruner:      pruner,
	}, nil
}

// Execute executes phase
func (r *packageExecutor) Execute(ctx context.Context) error {
	err := r.Prune(ctx)
	return trace.Wrap(err)
}

// PreCheck is a no-op
func (r *packageExecutor) PreCheck(context.Context) error {
	return nil
}

// Postheck is a no-op
func (r *packageExecutor) PostCheck(context.Context) error {
	return nil
}

// Rollback is a no-op
func (r *packageExecutor) Rollback(context.Context) error {
	return nil
}

type packageExecutor struct {
	// FieldLogger is the logger the executor uses
	log.FieldLogger
	// Pruner is the actual clean up implementation
	prune.Pruner
}
