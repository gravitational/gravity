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
	"bufio"
	"context"
	"io"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// updatePhaseApp is the executor for the app update phase
type updatePhaseApp struct {
	phaseApp
}

// NewUpdatePhaseApp returns a new app phase executor
func NewUpdatePhaseApp(
	p fsm.ExecutorParams,
	operator ops.Operator,
	apps app.Applications,
	client *kubernetes.Clientset,
	logger log.FieldLogger,
) (*updatePhaseApp, error) {
	cluster, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if p.Phase.Data.Package == nil {
		return nil, trace.NotFound("no package specified for phase %q", p.Phase.ID)
	}
	return &updatePhaseApp{
		phaseApp: phaseApp{
			FieldLogger:    logger,
			Apps:           apps,
			Client:         client,
			GravityPackage: p.Plan.GravityPackage,
			Package:        *p.Phase.Data.Package,
			Servers:        p.Plan.Servers,
			ServiceUser:    cluster.ServiceUser,
		}}, nil
}

// Execute runs update/post-update hooks for the app
func (p *updatePhaseApp) Execute(ctx context.Context) error {
	var err error
	if p.Package.Name == constants.BootstrapConfigPackage {
		// the "bootstrap" app (rbac-app) is a special case as its resources contain
		// cluster roles and pod security policies which are needed for proper cluster
		// functioning so we create these resources directly because we may not have
		// permissions to launch jobs just yet
		err = p.createBootstrapResources()
	} else {
		err = p.runHooks(ctx,
			schema.HookNetworkUpdate,
			schema.HookUpdate,
			schema.HookUpdated,
			schema.HookStatus)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback runs rollback/post-rollback hooks for the app
func (p *updatePhaseApp) Rollback(ctx context.Context) error {
	err := p.runHooks(ctx,
		schema.HookRollback,
		schema.HookRolledBack,
		schema.HookNetworkRollback)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *updatePhaseApp) createBootstrapResources() error {
	reader, err := p.Apps.GetAppResources(p.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	err = utils.WithTempDir(func(dir string) error {
		err := dockerarchive.Untar(reader, dir, archive.DefaultOptions())
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(resources.ForEachObjectInFile(
			filepath.Join(dir, defaults.ResourcesDir, defaults.ResourcesFile),
			fsm.GetUpsertBootstrapResourceFunc(p.Client)))
	}, "resources")
	return trace.Wrap(err)
}

// updatePhaseBeforeApp is an executor for application's pre-update hook
type updatePhaseBeforeApp struct {
	phaseApp
}

// NewUpdatePhaseBeforeApp returns a new executor for running application pre-update hook
func NewUpdatePhaseBeforeApp(
	p fsm.ExecutorParams,
	apps app.Applications,
	client *kubernetes.Clientset,
	logger log.FieldLogger,
) (*updatePhaseBeforeApp, error) {
	if p.Phase.Data.Package == nil {
		return nil, trace.NotFound("no package specified for phase %q", p.Phase.ID)
	}
	return &updatePhaseBeforeApp{
		phaseApp: phaseApp{
			FieldLogger:    logger,
			Apps:           apps,
			Client:         client,
			GravityPackage: p.Plan.GravityPackage,
			Package:        *p.Phase.Data.Package,
			Servers:        p.Plan.Servers,
		}}, nil
}

// Execute runs pre-update hook for the app
func (p *updatePhaseBeforeApp) Execute(ctx context.Context) error {
	err := p.runHooks(ctx, schema.HookBeforeUpdate)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is a no-op for this phase
func (p *updatePhaseBeforeApp) Rollback(context.Context) error {
	return nil
}

type phaseApp struct {
	// Apps is the cluster apps service
	Apps app.Applications
	// Client is the cluster Kubernetes client
	Client *kubernetes.Clientset
	// GravityPackage is the gravity binary package to run hooks with
	GravityPackage loc.Locator
	// Package is the package to run hooks for
	Package loc.Locator
	// Servers is the list of local cluster servers
	Servers []storage.Server
	// ServiceUser is the user used for services and system storage
	ServiceUser storage.OSUser
	log.FieldLogger
}

// PreCheck makes sure this phase is being executed on a master node
func (p *phaseApp) PreCheck(context.Context) error {
	return trace.Wrap(fsm.CheckMasterServer(p.Servers))
}

// PostCheck is no-op for this phase
func (p *phaseApp) PostCheck(context.Context) error {
	return nil
}

func (p *phaseApp) runHooks(ctx context.Context, hooks ...schema.HookType) error {
	for _, hook := range hooks {
		req := app.HookRunRequest{
			Application:    p.Package,
			GravityPackage: p.GravityPackage,
			Hook:           hook,
			Env: map[string]string{
				// TODO(r0mant) see if we can get rid of this flag
				constants.ManualUpdateEnvVar: "true",
			},
			ServiceUser: p.ServiceUser,
		}
		_, err := app.CheckHasAppHook(p.Apps, req)
		if err != nil {
			if trace.IsNotFound(err) {
				p.Debugf("%v does not have %v hook.", p.Package, hook)
				continue
			}
			return trace.Wrap(err)
		}
		p.Infof("Execute %v(%v) hook.", p.Package, hook)
		reader, writer := io.Pipe()
		defer writer.Close()
		go streamHook(hook, reader, p.FieldLogger)
		_, err = app.StreamAppHook(ctx, p.Apps, req, writer)
		if err != nil {
			return trace.Wrap(err, "%v(%v) hook failed", p.Package, hook)
		}
	}
	return nil
}

func streamHook(hook schema.HookType, reader io.ReadCloser, logger log.FieldLogger) {
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		logger.Info(scanner.Text())
	}
	err := scanner.Err()
	if err != nil && err != io.EOF {
		logger.Warnf("Failed to stream logs for hook %v: %v.", hook, err)
	}
}
