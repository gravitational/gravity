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
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewExport returns a new "export" phase executor.
//
// This phase exports Docker images of the application and its dependencies
// to the locally running Docker registry.
func NewExport(p fsm.ExecutorParams, operator ops.Operator, packages pack.PackageService, apps app.Applications,
	remote fsm.Remote) (*ExportExecutor, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: defaults.LocalRegistryAddr,
		CACertPath:      state.Secret(stateDir, defaults.RootCertFilename),
		ClientCertPath:  state.Secret(stateDir, "kubelet.cert"),
		ClientKeyPath:   state.Secret(stateDir, "kubelet.key"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase:       p.Phase.ID,
			constants.FieldAdvertiseIP: p.Phase.Data.Server.AdvertiseIP,
			constants.FieldHostname:    p.Phase.Data.Server.Hostname,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &ExportExecutor{
		FieldLogger:    logger,
		Packages:       packages,
		Apps:           apps,
		ImageService:   imageService,
		StateDir:       stateDir,
		ExecutorParams: p,
		remote:         remote,
	}, nil
}

type ExportExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Packages is the installer process pack service
	Packages pack.PackageService
	// Apps is the installer process app service
	Apps app.Applications
	// ImageService is the Docker image service
	ImageService docker.ImageService
	// StateDir is the local gravity state dir
	StateDir string
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	// remote specifies the server remote control interface
	remote fsm.Remote
}

// Execute executes the export phase
func (p *ExportExecutor) Execute(ctx context.Context) error {
	app, err := p.Apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, dep := range app.Manifest.Dependencies.Apps {
		err = p.unpackApp(dep.Locator)
		if err != nil {
			return trace.Wrap(err)
		}
		err = p.exportApp(ctx, dep.Locator)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err = p.unpackApp(app.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.exportApp(ctx, app.Package)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Application %v exported.", app.Package)
	return nil
}

// Rollback is no-op for this phase
func (*ExportExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure the phase is executed on a proper node
func (p *ExportExecutor) PreCheck(ctx context.Context) error {
	err := p.remote.CheckServer(ctx, *p.Phase.Data.Server)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*ExportExecutor) PostCheck(ctx context.Context) error {
	return nil
}

func (p *ExportExecutor) unpackApp(locator loc.Locator) error {
	p.Progress.NextStep("Unpacking application %v:%v",
		locator.Name, locator.Version)
	return trace.Wrap(pack.UnpackIfNotUnpacked(p.Packages, locator,
		p.packagePath(locator), nil))
}

func (p *ExportExecutor) exportApp(ctx context.Context, locator loc.Locator) error {
	p.Progress.NextStep("Exporting application %v:%v to local registry",
		locator.Name, locator.Version)
	p.Infof("Exporting application %v:%v to local registry.",
		locator.Name, locator.Version)
	_, err := p.ImageService.Sync(ctx, p.registryPath(locator), p.Progress)
	return trace.Wrap(err)
}

func (p *ExportExecutor) packagePath(locator loc.Locator) string {
	return pack.PackagePath(filepath.Join(p.StateDir, defaults.LocalDir,
		defaults.PackagesDir, defaults.UnpackedDir), locator)
}

func (p *ExportExecutor) registryPath(locator loc.Locator) string {
	return filepath.Join(p.packagePath(locator), defaults.RegistryDir)
}
