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
	"io"

	"github.com/gravitational/gravity/lib/app"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewUpdateConfig returns a new executor to update runtime configuration on the specified node
func NewUpdateConfig(
	params libfsm.ExecutorParams,
	operator runtimePackageRotator,
	operation ops.SiteOperation,
	apps appGetter,
	packages packageService,
	logger log.FieldLogger,
) (*updateConfig, error) {
	if params.Phase.Data == nil || params.Phase.Data.Package == nil {
		return nil, trace.NotFound("no installed application package specified for phase %q",
			params.Phase.ID)
	}
	app, err := apps.GetApp(*params.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query installed application")
	}
	runtimePackage, err := app.Manifest.RuntimePackageForProfile(params.Phase.Data.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &updateConfig{
		FieldLogger:    logger,
		server:         *params.Phase.Data.Server,
		operator:       operator,
		operation:      operation,
		packages:       packages,
		manifest:       app.Manifest,
		runtimePackage: *runtimePackage,
	}, nil
}

// Execute generates new runtime configuration with the specified environment
func (r *updateConfig) Execute(ctx context.Context) error {
	r.Infof("Generate new runtime configuration package for %v.", r.server)
	resp, err := r.operator.RotatePlanetConfig(ops.RotatePlanetConfigRequest{
		AccountID:   r.operation.AccountID,
		ClusterName: r.operation.SiteDomain,
		OperationID: r.operation.ID,
		Server:      r.server,
		Manifest:    r.manifest,
		Env:         r.operation.UpdateEnvars.Env,
		Package:     r.runtimePackage,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = r.packages.UpsertPackage(resp.Locator, resp.Reader,
		pack.WithLabels(resp.Labels))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is a no-op for this phase
func (r *updateConfig) Rollback(context.Context) error {
	return nil
}

// PreCheck is a no-op
func (r *updateConfig) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (r *updateConfig) PostCheck(context.Context) error {
	return nil
}

type updateConfig struct {
	// FieldLogger specifies the logger for the phase
	log.FieldLogger
	operator       runtimePackageRotator
	operation      ops.SiteOperation
	packages       packageService
	server         storage.Server
	manifest       schema.Manifest
	runtimePackage loc.Locator
}

type runtimePackageRotator interface {
	RotatePlanetConfig(ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error)
}

type appGetter interface {
	GetApp(loc.Locator) (*app.Application, error)
}

type packageService interface {
	UpsertPackage(loc.Locator, io.Reader, ...pack.PackageOption) (*pack.PackageEnvelope, error)
	DeletePackage(loc.Locator) error
}
