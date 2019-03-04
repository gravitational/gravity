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

package environ

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/internal/rollingupdate"

	"github.com/gravitational/trace"
	"k8s.io/client-go/kubernetes"
)

// New returns new cluster runtime environment updater for the specified configuration
func New(ctx context.Context, config Config) (*update.Updater, error) {
	machine, err := rollingupdate.NewMachine(ctx, rollingupdate.Config{
		Config:          config.Config,
		Apps:            config.Apps,
		ClusterPackages: config.ClusterPackages,
		Client:          config.Client,
		RequestAdaptor:  rollingupdate.RequestAdaptorFunc(updateRequest),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updater, err := update.NewUpdater(ctx, config.Config, machine)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return updater, nil
}

// Config describes configuration for updating cluster environment variables
type Config struct {
	update.Config
	// Apps is the cluster application service
	Apps app.Applications
	// ClusterPackages specifies the cluster package service
	ClusterPackages pack.PackageService
	// Client specifies the optional kubernetes client
	Client *kubernetes.Clientset
}

func updateRequest(req ops.RotatePlanetConfigRequest, operation ops.SiteOperation) ops.RotatePlanetConfigRequest {
	result := req
	result.Env = operation.UpdateEnviron.Env
	return result
}
