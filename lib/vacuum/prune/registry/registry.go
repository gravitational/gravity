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

package registry

import (
	"context"
	"strings"

	apps "github.com/gravitational/gravity/lib/app"
	appservice "github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/vacuum/prune"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New creates a new registry cleaner
func New(config Config) (*cleanup, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &cleanup{
		Config: config,
	}, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.App == nil {
		return trace.BadParameter("application package is required")
	}
	if r.ImageService == nil {
		return trace.BadParameter("docker image service is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("cluster package service is required")
	}
	if r.Apps == nil {
		return trace.BadParameter("cluster application service is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "gc:registry")
	}
	return nil
}

// Config describes configuration for the registry cleaner
type Config struct {
	// Config specifies the common pruner configuration
	prune.Config
	// App specifies the cluster application
	App *loc.Locator
	// Packages specifies the cluster package service
	Packages pack.PackageService
	// Apps specifies the cluster application service
	Apps apps.Applications
	// ImageService specifies the docker image service
	ImageService docker.ImageService
}

// Prune removes unused docker images.
// The registry state is reset by deleting the state from the filesystem
// and re-running the docker image export for the cluster application.
func (r *cleanup) Prune(ctx context.Context) (err error) {
	r.PrintStep("Stop registry service")
	if !r.DryRun {
		err = r.registryStop(ctx)
		defer func() {
			if err == nil {
				return
			}
			if errStart := r.registryStart(ctx); errStart != nil {
				r.Warn(errStart)
			}
		}()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}

	dir := state.RegistryDir(stateDir)
	r.PrintStep("Delete registry state directory %v", dir)
	if !r.DryRun {
		err = utils.RemoveContents(dir)
		if err != nil {
			return trace.Wrap(trace.ConvertSystemError(err),
				"failed to remove old registry state from %v.", dir)
		}
	}

	r.PrintStep("Start registry service")
	if !r.DryRun {
		err = r.registryStart(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	r.PrintStep("Sync application state with registry")
	if r.DryRun {
		return nil
	}
	err = appservice.SyncApp(ctx, appservice.SyncRequest{
		PackService:  r.Packages,
		AppService:   r.Apps,
		ImageService: r.ImageService,
		Package:      *r.App,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (r *cleanup) registryStart(ctx context.Context) error {
	out, err := r.serviceStart(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to start the registry service: %s.", out)
	}

	err = r.waitForService(ctx, systemservice.ServiceStatusActive)
	if err != nil {
		return trace.Wrap(err, "failed to wait for the registry service to start")
	}
	return nil
}

func (r *cleanup) registryStop(ctx context.Context) error {
	out, err := r.serviceStop(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to stop the registry service: %s.", out)
	}

	err = r.waitForService(ctx, systemservice.ServiceStatusInactive)
	if err != nil {
		return trace.Wrap(err, "failed to wait for the registry service to stop")
	}

	return nil
}

func (r *cleanup) waitForService(ctx context.Context, status string) error {
	localCtx, cancel := defaults.WithTimeout(ctx)
	defer cancel()
	b := utils.NewUnlimitedExponentialBackOff()
	err := utils.RetryWithInterval(localCtx, b, func() error {
		out, err := r.serviceStatus(localCtx)
		actualStatus := strings.TrimSpace(string(out))
		if strings.HasPrefix(actualStatus, status) {
			return nil
		}
		return trace.Retry(err, "unexpected service status: %s", actualStatus)
	})
	return trace.Wrap(err)
}

func (r *cleanup) serviceStop(ctx context.Context) (output []byte, err error) {
	return serviceCtl(ctx, r.FieldLogger, "stop")
}

func (r *cleanup) serviceStart(ctx context.Context) (output []byte, err error) {
	return serviceCtl(ctx, r.FieldLogger, "start")
}

func (r *cleanup) serviceStatus(ctx context.Context) (output []byte, err error) {
	return serviceCtl(ctx, r.FieldLogger, "is-active")
}

type cleanup struct {
	// Config specifies the configuration for the cleanup
	Config
}

func serviceCtl(ctx context.Context, log log.FieldLogger, args ...string) (output []byte, err error) {
	args = append([]string{"/bin/systemctl"}, append(args, "registry.service")...)
	output, err = utils.RunCommand(ctx, log, utils.PlanetCommandArgs(args...)...)
	return output, trace.Wrap(err)
}
