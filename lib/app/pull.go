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

package app

import (
	"context"
	"io/ioutil"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/run"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// PullApp pulls the specified application and its dependencies
func PullApp(ctx context.Context, loc loc.Locator, puller Puller) error {
	if err := puller.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	app, err := puller.SrcApp.GetApp(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	deps, err := GetDependencies(GetDependenciesRequest{
		App:  *app,
		Apps: puller.SrcApp,
		Pack: puller.SrcPack,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	deps.Apps = append(deps.Apps, *app)
	puller.Dependencies = *deps
	return puller.Pull(ctx)
}

// PullPackage pulls the package specified with loc
func PullPackage(ctx context.Context, loc loc.Locator, puller Puller) error {
	if err := puller.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	return puller.pullPackageWithRetries(ctx, loc)
}

// PullAppDeps pulls only dependencies of the specified application
// without the application itself
func PullAppDeps(ctx context.Context, loc loc.Locator, puller Puller) error {
	if err := puller.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	app, err := puller.SrcApp.GetApp(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	deps, err := GetDependencies(GetDependenciesRequest{
		App:  *app,
		Apps: puller.SrcApp,
		Pack: puller.SrcPack,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	puller.Dependencies = *deps
	return puller.Pull(ctx)
}

// Pull pulls the packages specified by r
func (r Puller) Pull(ctx context.Context) error {
	group, ctx := run.WithContext(ctx, run.WithParallel(r.Parallel))
	for _, env := range r.Dependencies.Packages {
		group.Go(ctx, r.pullPackageHandler(ctx, env.Locator))
	}
	for _, app := range r.Dependencies.Apps {
		group.Go(ctx, r.pullAppHandler(ctx, app.Package))
	}
	return trace.Wrap(group.Wait())
}

// Puller pulls packages from one service to another
type Puller struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Dependencies specifies the dependent packages/applications to pull
	Dependencies Dependencies
	// SrcPack is the package service to pull application from
	SrcPack pack.PackageService
	// DstPack is the package service to push application into
	DstPack pack.PackageService
	// SrcApp is the application service to pull application from
	SrcApp Applications
	// DstApp is the application service to push application into
	DstApp Applications
	// Labels is the labels to assign to pulled packages
	Labels map[string]string
	// Upsert is whether to create or upsert the application
	Upsert bool
	// SkipIfExists indicates whether existing application should not
	// be pulled
	SkipIfExists bool
	// MetadataOnly allows to pull only app metadata without body
	MetadataOnly bool
	// Parallel defines the number of tasks to run in parallel.
	// If < 0, the number of tasks is unrestricted.
	// If in [0,1], the tasks are executed sequentially.
	Parallel int
}

func (r *Puller) checkAndSetDefaults() error {
	if r.FieldLogger == nil {
		r.FieldLogger = logrus.WithField(trace.Component, "pull")
	}
	return nil
}

func (r Puller) pullPackageHandler(ctx context.Context, loc loc.Locator) func() error {
	return func() error {
		return r.pullPackageWithRetries(ctx, loc)
	}
}

func (r Puller) pullAppHandler(ctx context.Context, loc loc.Locator) func() error {
	return func() error {
		return r.pullAppWithRetries(ctx, loc)
	}
}

func (r Puller) pullPackageWithRetries(ctx context.Context, loc loc.Locator) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	return utils.RetryTransient(ctx,
		backoff.NewConstantBackOff(defaults.RetryInterval),
		func() (err error) {
			return r.pullPackage(loc)
		})
}

func (r Puller) pullPackage(loc loc.Locator) error {
	logger := r.WithField("package", loc)
	env, err := r.DstPack.ReadPackageEnvelope(loc)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		if r.SkipIfExists {
			return nil
		}
		if !r.Upsert {
			logger.Info("Package already exists.")
			return trace.AlreadyExists("package %v already exists", loc)
		}
	}
	logger.Info("Pull package.")
	reader := ioutil.NopCloser(utils.NopReader())
	if r.MetadataOnly {
		env, err = r.SrcPack.ReadPackageEnvelope(loc)
	} else {
		env, reader, err = r.SrcPack.ReadPackage(loc)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	err = r.DstPack.UpsertRepository(loc.Repository, time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}

	labels := utils.CombineLabels(env.RuntimeLabels, r.Labels)
	if r.Upsert {
		_, err = r.DstPack.UpsertPackage(
			loc, reader, pack.WithLabels(labels))
	} else {
		_, err = r.DstPack.CreatePackage(
			loc, reader, pack.WithLabels(labels))
	}
	return trace.Wrap(err)
}

func (r Puller) pullAppWithRetries(ctx context.Context, loc loc.Locator) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	return utils.RetryTransient(ctx,
		backoff.NewConstantBackOff(defaults.RetryInterval),
		func() error {
			return r.pullApp(loc)
		})
}

func (r Puller) pullApp(loc loc.Locator) error {
	app, err := r.SrcApp.GetApp(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	app, err = r.DstApp.GetApp(loc)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	upsert := r.Upsert
	if app != nil && pack.IsMetadataPackage(app.PackageEnvelope) {
		// Allow to overwrite the application if pushing over the existing metadata package
		// i.e. package that describes an application on a remote cluster
		upsert = true
	}
	logger := r.WithField("app", loc)
	if app != nil {
		if r.SkipIfExists {
			return nil
		}
		if !upsert {
			logger.Info("Application already exists.")
			return trace.AlreadyExists("app %v already exists", loc)
		}
	}
	logger.Info("Pull application.")
	var env *pack.PackageEnvelope
	reader := ioutil.NopCloser(utils.NopReader())
	if r.MetadataOnly {
		env, err = r.SrcPack.ReadPackageEnvelope(loc)
	} else {
		env, reader, err = r.SrcPack.ReadPackage(loc)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	labels := utils.CombineLabels(env.RuntimeLabels, r.Labels)
	if upsert {
		_, err = r.DstApp.UpsertApp(env.Locator, reader, labels)
	} else {
		_, err = r.DstApp.CreateAppWithManifest(
			env.Locator, env.Manifest, reader, labels)
	}
	return trace.Wrap(err)
}
