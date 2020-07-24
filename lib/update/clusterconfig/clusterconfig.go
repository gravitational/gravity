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

package clusterconfig

import (
	"context"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/clusterconfig/phases"
	"github.com/gravitational/gravity/lib/update/internal/rollingupdate"
	libphase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// New returns new updater for the specified configuration
func New(ctx context.Context, config Config) (*update.Updater, error) {
	dispatcher := &dispatcher{
		Dispatcher: rollingupdate.NewDefaultDispatcher(),
	}
	machine, err := rollingupdate.NewMachine(ctx, rollingupdate.Config{
		Config:            config.Config,
		Apps:              config.Apps,
		ClusterPackages:   config.ClusterPackages,
		HostLocalPackages: config.HostLocalPackages,
		Client:            config.Client,
		Dispatcher:        dispatcher,
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

// Config describes configuration for updating cluster configuration
type Config struct {
	update.Config
	// HostLocalPackages specifies the package service on local host
	HostLocalPackages update.LocalPackageService
	// Apps is the cluster application service
	Apps app.Applications
	// ClusterPackages specifies the cluster package service
	ClusterPackages pack.PackageService
	// Client specifies the optional kubernetes client
	Client *kubernetes.Clientset
}

// Dispatch returns the appropriate phase executor based on the provided parameters
func (r *dispatcher) Dispatch(config rollingupdate.Config, params fsm.ExecutorParams, remote fsm.Remote, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	switch params.Phase.Executor {
	case libphase.UpdateConfig:
		return phases.NewUpdateConfig(params,
			config.Operator, *config.Operation, config.Apps,
			config.ClusterPackages, config.HostLocalPackages,
			logger)
	case libphase.RestartContainer:
		return phases.NewRestart(params,
			config.Operator, *config.Operation,
			config.Apps, config.LocalBackend,
			config.ClusterPackages, config.HostLocalPackages,
			logger)
	default:
		return r.Dispatcher.Dispatch(config, params, remote, logger)
	}
}

type dispatcher struct {
	rollingupdate.Dispatcher
}
