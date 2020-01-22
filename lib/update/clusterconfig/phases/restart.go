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
	"bytes"
	"context"
	"os/exec"

	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	libbasephase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewRestart returns a new executor to restart the runtime container to apply
// the environment variables update
func NewRestart(
	params libfsm.ExecutorParams,
	operator libbasephase.LocalClusterGetter,
	operationID string,
	apps appGetter,
	backend storage.Backend,
	packages pack.PackageService,
	localPackages update.LocalPackageService,
	logger log.FieldLogger,
) (*restart, error) {
	base, err := libbasephase.NewRestart(params, operator, operationID, apps, backend, packages, localPackages, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &restart{
		FieldLogger: logger,
		base:        base,
	}, nil
}

// Execute restarts the runtime container with the new configuration package
func (r *restart) Execute(ctx context.Context) error {
	r.Info("Removing network interface cni0")
	var out bytes.Buffer
	if err := utils.Exec(exec.Command("ip", "link", "del", "cni0"), &out); err != nil {
		return trace.Wrap(err, out.String())
	}
	return trace.Wrap(r.base.Execute(ctx))
}

// Rollback reverses the update and restarts the container with the old
// configuration package
func (r *restart) Rollback(ctx context.Context) error {
	return trace.Wrap(r.base.Rollback(ctx))
}

// PreCheck is a no-op
func (*restart) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (*restart) PostCheck(context.Context) error {
	return nil
}

type restart struct {
	// FieldLogger specifies the logger for the phase
	log.FieldLogger
	base libfsm.PhaseExecutor
}
