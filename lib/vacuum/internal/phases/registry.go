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

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/vacuum/prune"
	"github.com/gravitational/gravity/lib/vacuum/prune/registry"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewRegistry creates a new executor to prune unused docker images on a node
func NewRegistry(
	params libfsm.ExecutorParams,
	clusterApp loc.Locator,
	clusterApps app.Applications,
	clusterPackages pack.PackageService,
	silent localenv.Silent,
	logger log.FieldLogger,
) (*registryExecutor, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: defaults.LocalRegistryAddr,
		CertName:        defaults.DockerRegistry,
		CACertPath:      state.Secret(stateDir, defaults.RootCertFilename),
		ClientCertPath:  state.Secret(stateDir, "kubelet.cert"),
		ClientKeyPath:   state.Secret(stateDir, "kubelet.key"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pruner, err := registry.New(registry.Config{
		App:          &clusterApp,
		Apps:         clusterApps,
		Packages:     clusterPackages,
		ImageService: imageService,
		Config: prune.Config{
			Silent:      silent,
			FieldLogger: logger,
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &registryExecutor{
		FieldLogger: logger,
		Pruner:      pruner,
		silent:      silent,
	}, nil
}

// Execute prunes unused docker images on this node
func (r *registryExecutor) Execute(ctx context.Context) error {
	err := r.Prune(ctx)
	return trace.Wrap(err)
}

// PreCheck is a no-op
func (r *registryExecutor) PreCheck(context.Context) error {
	return nil
}

// PostCheck is a no-op
func (r *registryExecutor) PostCheck(context.Context) error {
	return nil
}

// Rollback is a no-op
func (r *registryExecutor) Rollback(context.Context) error {
	return nil
}

type registryExecutor struct {
	// FieldLogger is the logger the executor uses
	log.FieldLogger
	// Pruner is the actual clean up implementation
	prune.Pruner
	silent localenv.Silent
}
