/*
Copyright 2021 Gravitational, Inc.

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

// PullApp pulls the specified application and its dependencies.
//
// When an application is pulled (or pushed) from a service, the behavior regarding
// the conflicts is as following:
//  * if a dependent (application) package already exists in the destination service,
//    the operation does nothing or upserts the package (subject to upsert attribute)
//  * if the top-level application package already exists in the destination service,
//    the operation will either fail with the corresponding error or upsert the package
//    (subject to upsert attribute)
func (r Puller) PullApp(ctx context.Context, loc loc.Locator) error {
	r.setDefaults()
	app, err := r.SrcApp.GetApp(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	deps, err := GetDependencies(GetDependenciesRequest{
		App:  *app,
		Apps: r.SrcApp,
		Pack: r.SrcPack,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	r.OnConflict = getDependencyConflictHandler(r.Upsert)
	err = r.pull(ctx, *deps)
	if err != nil {
		return trace.Wrap(err)
	}
	// Pull the application
	r.OnConflict = getConflictHandler(r.Upsert)
	return r.pullApp(ctx, app.Package)
}

// PullPackage pulls the package specified with loc
func (r Puller) PullPackage(ctx context.Context, loc loc.Locator) error {
	r.setDefaults()
	return r.pullPackage(ctx, loc)
}

// PullAppOnly pulls just the application package specified with loc.
// No dependencies are pulled
func (r Puller) PullAppOnly(ctx context.Context, loc loc.Locator) error {
	r.setDefaults()
	return r.pullApp(ctx, loc)
}

// PullAppDeps pulls only dependencies of the specified application
func (r Puller) PullAppDeps(ctx context.Context, app Application) error {
	r.setDefaults()
	deps, err := GetDependencies(GetDependenciesRequest{
		App:  app,
		Apps: r.SrcApp,
		Pack: r.SrcPack,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return r.Pull(ctx, *deps)
}

// Pull pulls the packages specified by deps
func (r Puller) Pull(ctx context.Context, deps Dependencies) error {
	r.setDefaults()
	return r.pull(ctx, deps)
}

func (r Puller) pull(ctx context.Context, deps Dependencies) error {
	if err := r.pullPackages(ctx, deps.Packages); err != nil {
		return trace.Wrap(err)
	}
	return r.pullApps(ctx, deps.Apps)
}

func (r Puller) pullPackages(ctx context.Context, packages []pack.PackageEnvelope) error {
	group, ctx := run.WithContext(ctx, run.WithParallel(r.Parallel))
	for _, env := range packages {
		env := env
		group.Go(ctx, func() error {
			return r.pullPackage(ctx, env.Locator)
		})
	}
	return group.Wait()
}

func (r Puller) pullApps(ctx context.Context, apps []Application) error {
	group, ctx := run.WithContext(ctx, run.WithParallel(r.Parallel))
	for _, app := range apps {
		app := app
		group.Go(ctx, func() error {
			return r.pullApp(ctx, app.Package)
		})
	}
	return group.Wait()
}

// Puller pulls packages from one service to another
type Puller struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
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
	// Progress is optional progress reporter
	Progress pack.ProgressReporter
	// Upsert is whether to create or upsert the application or package.
	// The flag is applied to all dependencies
	Upsert bool
	// MetadataOnly specifies whether to only pull package metadata (w/o contents)
	MetadataOnly bool
	// Parallel defines the number of tasks to run in parallel.
	// If < 0, the number of tasks is unrestricted.
	// If in [0,1], the tasks are executed sequentially.
	Parallel int
	// OnConflict specifies the package conflict handler for when the package already
	// exists in DstPack.
	OnConflict ConflictHandler
}

func (r *Puller) setDefaults() {
	if r.FieldLogger == nil {
		r.FieldLogger = logrus.WithField(trace.Component, "pull")
	}
	if r.OnConflict == nil {
		r.OnConflict = getConflictHandler(r.Upsert)
	}
}

func (r Puller) pullPackage(ctx context.Context, loc loc.Locator) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()

	return utils.RetryWithInterval(ctx, backoff.NewConstantBackOff(defaults.RetryInterval), func() error {
		err := r.pullPackageOnce(loc)
		if err == nil {
			return nil
		}
		switch {
		case utils.IsTransientClusterError(err):
			// Retry on transient errors
			return trace.Wrap(err)
		case trace.IsNotFound(err):
			// Retry for not found packages as it might take some time
			// for a package to get replicated.
			// TODO(dima): make this only a case when source package store
			// is a replicated one
			return trace.Wrap(err)
		default:
			return &backoff.PermanentError{Err: err}
		}
	})
}

func (r Puller) pullPackageOnce(loc loc.Locator) error {
	_, err := r.DstPack.ReadPackageEnvelope(loc)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil {
		err = r.OnConflict(loc)
		if utils.IsAbortError(err) {
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}
	reader := ioutil.NopCloser(utils.NewNopReader())
	var env *pack.PackageEnvelope
	if r.MetadataOnly {
		env, err = r.SrcPack.ReadPackageEnvelope(loc)
	} else {
		env, reader, err = r.SrcPack.ReadPackage(loc)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	if r.Progress != nil {
		reader = utils.TeeReadCloser(reader, &pack.ProgressWriter{
			Size: env.SizeBytes,
			R:    r.Progress,
		})
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
	r.WithField("package", loc).Debug("Pulled package.")
	return trace.Wrap(err)
}

func (r Puller) pullApp(ctx context.Context, loc loc.Locator) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	return utils.RetryTransient(ctx,
		backoff.NewConstantBackOff(defaults.RetryInterval),
		func() error {
			return r.pullAppOnce(loc)
		})
}

func (r Puller) pullAppOnce(loc loc.Locator) error {
	app, err := r.DstApp.GetApp(loc)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	upsert := r.Upsert
	if app != nil && pack.IsMetadataPackage(app.PackageEnvelope) {
		// Allow to overwrite the application if pushing over the existing metadata package
		// i.e. package that describes an application on a remote cluster
		upsert = true
	}
	if app != nil && !upsert {
		err = r.OnConflict(loc)
		if utils.IsAbortError(err) {
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}
	}
	var env *pack.PackageEnvelope
	reader := ioutil.NopCloser(utils.NewNopReader())
	if r.MetadataOnly {
		env, err = r.SrcPack.ReadPackageEnvelope(loc)
	} else {
		env, reader, err = r.SrcPack.ReadPackage(loc)
	}
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("application package %v not found", loc)
		}
		return trace.Wrap(err)
	}
	if r.Progress != nil {
		reader = utils.TeeReadCloser(reader, &pack.ProgressWriter{
			Size: env.SizeBytes,
			R:    r.Progress,
		})
	}
	defer reader.Close()

	labels := utils.CombineLabels(env.RuntimeLabels, r.Labels)
	if upsert {
		_, err = r.DstApp.UpsertApp(env.Locator, reader, labels)
	} else {
		_, err = r.DstApp.CreateAppWithManifest(
			env.Locator, env.Manifest, reader, labels)
	}
	r.WithField("app", loc).Info("Pulled application.")
	return trace.Wrap(err)
}

// getDependencyConflictHandler returns the conflict handler that ignores package
// conflicts (subject to specified upsert flag)
func getDependencyConflictHandler(upsert bool) ConflictHandler {
	if upsert {
		return OnConflictContinue
	}
	return OnConflictSkip
}

// getConflictHandler returns the conflict handler that fails for package
// conflicts (subject to specified upsert flag)
func getConflictHandler(upsert bool) ConflictHandler {
	if upsert {
		return OnConflictContinue
	}
	return OnConflictAbort
}

// OnConflictAbort is a conflict handler that aborts the pull operation
// with an error
func OnConflictAbort(loc loc.Locator) error {
	return trace.AlreadyExists("package %v already exists", loc)
}

// OnConflictContinue is a conflict handler that continues the pull operation
// if a package already exists in the destination package service
func OnConflictContinue(loc loc.Locator) error {
	return nil
}

// OnConflictSkip is a conflict handler that aborts the pull operation
// w/o error
func OnConflictSkip(loc loc.Locator) error {
	return utils.Abort(nil)
}

// ConflictHandler defines a functional handler to decide whether the active
// pull operation is aborted if the specified package already exists in the
// destination package service.
// If the return is nil, the pull operation continues.
// If the return is a special utils.Abort error, the pull operation is aborted without error.
// If the return is any other error, the pull operation is aborted with said error.
type ConflictHandler func(loc.Locator) error
