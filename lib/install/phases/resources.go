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
	"io/ioutil"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewResources returns an executor that creates user-supplied Kubernetes resources
func NewResources(p fsm.ExecutorParams, operator ops.Operator) (*resourcesExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.Install == nil {
		return nil, trace.BadParameter("phase data is mandatory")
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &resourcesExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		resources:      p.Phase.Data.Install.Resources,
	}, nil
}

type resourcesExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	resources []byte
}

// Execute executes the resources phase
func (p *resourcesExecutor) Execute(ctx context.Context) error {
	const filename = "resources.yaml"
	p.Progress.NextStep("Creating user-supplied Kubernetes resources")
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = ioutil.WriteFile(filepath.Join(state.ShareDir(stateDir), filename),
		p.resources, defaults.SharedReadMask)
	if err != nil {
		return trace.Wrap(err, "failed to write user resources on disk")
	}
	out, err := utils.RunInPlanetCommand(
		ctx,
		p.FieldLogger,
		defaults.KubectlBin,
		"--kubeconfig",
		constants.PrivilegedKubeconfig,
		"apply",
		"-f",
		filepath.Join(defaults.PlanetShareDir, filename),
	)
	if err != nil {
		return trace.Wrap(err, "failed to create user resources: %s", out)
	}
	p.Info("Created user-supplied Kubernetes resources.")
	return nil
}

// Rollback is no-op for this phase
func (*resourcesExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure this phase is executed on a master node
func (p *resourcesExecutor) PreCheck(ctx context.Context) error {
	err := fsm.CheckMasterServer(p.Plan.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*resourcesExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// NewGravityResourcesPhase returns executor that creates Gravity resources
func NewGravityResourcesPhase(p fsm.ExecutorParams, operator ops.Operator, factory resources.Resources) (*gravityExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.Install == nil {
		return nil, trace.BadParameter("phase data is mandatory")
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &gravityExecutor{
		FieldLogger: logger,
		progress:    p.Progress,
		factory:     factory,
		resources:   p.Phase.Data.Install.GravityResources,
	}, nil
}

// Execute creates the Gravity resources from the configured list
func (r *gravityExecutor) Execute(context.Context) (err error) {
	r.progress.NextStep("Creating user-supplied cluster resources")
	for _, resource := range r.resources {
		r.Infof("Creating resource %q", resource.Kind)
		err := r.factory.Create(resources.CreateRequest{
			Resource: teleservices.UnknownResource{
				ResourceHeader: resource.ResourceHeader,
				Raw:            resource.Raw,
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Rollback is no-op for this phase
func (*gravityExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*gravityExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*gravityExecutor) PostCheck(ctx context.Context) error {
	return nil
}

type gravityExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	progress  utils.Progress
	factory   resources.Resources
	resources []storage.UnknownResource
}
