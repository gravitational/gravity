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

package service

import (
	"context"
	"io/ioutil"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/run"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// PackagePullRequest describes a request to pull a package from one package service to another
type PackagePullRequest struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// SrcPack is the package service to pull package from
	SrcPack pack.PackageService
	// DstPack is the package service to push package into
	DstPack pack.PackageService
	// Package is the package to pull
	Package loc.Locator
	// Labels is the labels to assign to the pulled package
	Labels map[string]string
	// Progress is optional progress reporter
	Progress pack.ProgressReporter
	// Upsert is whether to create or upsert the pulled package
	Upsert bool
	// MetadataOnly allows to pull only package metadata without body
	MetadataOnly bool
}

// CheckAndSetDefaults checks the package pull request and sets some defaults
func (r *PackagePullRequest) CheckAndSetDefaults() error {
	if r.FieldLogger == nil {
		r.FieldLogger = logrus.StandardLogger()
	}
	return nil
}

// AppPullRequest describes a request to pull an app with all its dependencies from one app service
// into another
type AppPullRequest struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// SrcPack is the package service to pull app from
	SrcPack pack.PackageService
	// DstPack is the package service to push app into
	DstPack pack.PackageService
	// SrcApp is the app service to pull app from
	SrcApp app.Applications
	// DstApp is the app service to push app into
	DstApp app.Applications
	// Package is the application package to pull
	Package loc.Locator
	// Labels is the labels to assign to the pulled app
	Labels map[string]string
	// Progress is optional progress reporter
	Progress pack.ProgressReporter
	// Upsert is whether to create or upsert the application
	Upsert bool
	// MetadataOnly allows to pull only app metadata without body
	MetadataOnly bool
	// Parallel defines the number of tasks to run in parallel.
	// If < 0, the number of tasks is unrestricted.
	// If in [0,1], the tasks are executed sequentially.
	Parallel int
}

// CheckAndSetDefaults checks the app pull request and sets some defaults
func (r *AppPullRequest) CheckAndSetDefaults() error {
	if r.FieldLogger == nil {
		r.FieldLogger = logrus.StandardLogger()
	}
	return nil
}

// Clone returns a copy of this request replacing package with the provided one
func (r *AppPullRequest) Clone(locator loc.Locator) AppPullRequest {
	return AppPullRequest{
		FieldLogger:  r.FieldLogger,
		SrcPack:      r.SrcPack,
		DstPack:      r.DstPack,
		SrcApp:       r.SrcApp,
		DstApp:       r.DstApp,
		Package:      locator,
		Upsert:       r.Upsert,
		Progress:     r.Progress,
		Parallel:     r.Parallel,
		MetadataOnly: r.MetadataOnly,
	}
}

// PullPackage pulls a package from the "source" package service and creates it in the "destination" service
func PullPackage(req PackagePullRequest) (*pack.PackageEnvelope, error) {
	state := newPullState()
	return pullPackageWithRetries(req, state)
}

// pullPackageHandler returns a function to pull the specified package loc for running
// as part of parallel pull operation.
// If the package already exists, the error is ignored
func pullPackageHandler(loc loc.Locator, req AppPullRequest, state *pullState) func() error {
	return func() error {
		_, err := pullPackageWithRetries(PackagePullRequest{
			FieldLogger:  req.FieldLogger,
			SrcPack:      req.SrcPack,
			DstPack:      req.DstPack,
			Package:      loc,
			Upsert:       req.Upsert,
			Progress:     req.Progress,
			MetadataOnly: req.MetadataOnly,
		}, state)
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		req.Infof("Package %v already exists.", loc)
		return nil
	}
}

func pullPackageWithRetries(req PackagePullRequest, state *pullState) (env *pack.PackageEnvelope, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.TransientErrorTimeout)
	defer cancel()
	err = utils.RetryTransient(ctx,
		backoff.NewConstantBackOff(defaults.RetryInterval),
		func() (err error) {
			env, err = pullPackage(req)
			return trace.Wrap(err)
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Mark package as pulled
	state.markPulled(req.Package)
	return env, nil
}

func pullPackage(req PackagePullRequest) (*pack.PackageEnvelope, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	env, err := req.DstPack.ReadPackageEnvelope(req.Package)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if env != nil && !req.Upsert {
		req.Infof("Package %v already exists.", req.Package)
		return nil, trace.AlreadyExists("package %v already exists", req.Package)
	}

	req.Infof("Pulling package %v.", req.Package)

	reader := ioutil.NopCloser(utils.NopReader())
	if req.MetadataOnly {
		env, err = req.SrcPack.ReadPackageEnvelope(req.Package)
	} else {
		env, reader, err = req.SrcPack.ReadPackage(req.Package)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Progress != nil {
		reader = utils.TeeReadCloser(reader, &pack.ProgressWriter{
			Size: env.SizeBytes,
			R:    req.Progress,
		})
	}
	defer reader.Close()

	err = req.DstPack.UpsertRepository(env.Locator.Repository, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Copy runtime labels
	if req.Labels == nil {
		req.Labels = make(map[string]string)
	}
	for label, value := range env.RuntimeLabels {
		if _, exists := req.Labels[label]; !exists {
			req.Labels[label] = value
		}
	}

	if req.Upsert {
		env, err = req.DstPack.UpsertPackage(
			env.Locator, reader, pack.WithLabels(req.Labels))
	} else {
		env, err = req.DstPack.CreatePackage(
			env.Locator, reader, pack.WithLabels(req.Labels))
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return env, nil
}

// PullApp pulls the application specified with app, along with all its dependencies
// and base application, from the "source" application service and replicates it in
// the "destination" application service
func PullApp(req AppPullRequest) (*app.Application, error) {
	state := newPullState()
	return pullAppWithRetries(req, state)
}

func pullAppWithRetries(req AppPullRequest, state *pullState) (app *app.Application, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.TransientErrorTimeout)
	defer cancel()
	err = utils.RetryTransient(ctx,
		backoff.NewConstantBackOff(defaults.RetryInterval),
		func() error {
			var err error
			app, err = pullApp(req, state)
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}

func pullApp(req AppPullRequest, state *pullState) (*app.Application, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// see if the app itself already exists
	application, err = req.DstApp.GetApp(req.Package)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if application != nil && IsMetadataPackage(application.PackageEnvelope) {
		// Allow to overwrite the application if pushing over the existing metadata package
		// i.e. package that describes an application on a remote cluster
		req.Upsert = true
	}

	if application != nil && !req.Upsert {
		req.Infof("Application %v already exists.", req.Package)
		return nil, trace.AlreadyExists("app %v already exists", req.Package)
	}

	application, err = req.SrcApp.GetApp(req.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Use the raw manifest to avoid issues with remote side
	// being unable to interpret recent changes to the manifest format
	manifest, err := schema.ParseManifestYAMLNoValidate(
		application.PackageEnvelope.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// first pull all app dependencies
	switch manifest.Kind {
	case schema.KindBundle, schema.KindCluster, schema.KindRuntime:
		err = pullAppDeps(req, *manifest, state)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	req.Infof("Pulling application %v.", req.Package)

	// pull the application itself
	var env *pack.PackageEnvelope
	reader := ioutil.NopCloser(utils.NopReader())
	if req.MetadataOnly {
		env, err = req.SrcPack.ReadPackageEnvelope(req.Package)
	} else {
		env, reader, err = req.SrcPack.ReadPackage(req.Package)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if req.Progress != nil {
		reader = utils.TeeReadCloser(reader, &pack.ProgressWriter{
			Size: env.SizeBytes,
			R:    req.Progress,
		})
	}
	defer reader.Close()

	if req.Upsert {
		application, err = req.DstApp.UpsertApp(env.Locator, reader, req.Labels)
	} else {
		application, err = req.DstApp.CreateAppWithManifest(
			env.Locator, env.Manifest, reader, req.Labels)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return application, nil
}

// PullAppDeps downloads only dependencies for an application described by the provided manifest
func PullAppDeps(req AppPullRequest, manifest schema.Manifest) error {
	state := newPullState()
	return trace.Wrap(pullAppDeps(req, manifest, state))
}

func pullAppDeps(req AppPullRequest, manifest schema.Manifest, state *pullState) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	// pull base application if any
	base := manifest.Base()
	if base != nil {
		_, err := pullAppWithRetries(req.Clone(*base), state)
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
			req.Infof("Application %v already exists.", base)
		}
	}

	// pull dependent packages
	group, ctx := run.WithContext(context.TODO(), run.WithParallel(req.Parallel))
	for _, dep := range manifest.AllPackageDependencies() {
		if state.pulled(dep) {
			req.Infof("Package %v already pulled.", dep)
			continue
		}
		group.Go(ctx, pullPackageHandler(dep, req, state))
	}
	if err := group.Wait(); err != nil {
		return trace.Wrap(err)
	}

	// pull dependent applications
	for _, dep := range manifest.Dependencies.GetApps() {
		_, err := pullApp(req.Clone(dep), state)
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
			req.Infof("Application %v already exists.", dep)
		}
	}

	return nil
}

func newPullState() *pullState {
	return &pullState{
		packages: make(map[loc.Locator]struct{}),
	}
}

func (r *pullState) pulled(loc loc.Locator) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, pulled := r.packages[loc]
	return pulled
}

func (r *pullState) markPulled(loc loc.Locator) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.packages[loc] = struct{}{}
}

type pullState struct {
	mu       sync.RWMutex
	packages map[loc.Locator]struct{}
}

// IsMetadataPackage determines if the specified package is a metadata package.
// A metadata package describes a remote package and deserves
// special handling in certain cases.
func IsMetadataPackage(envelope pack.PackageEnvelope) bool {
	return envelope.RuntimeLabels[pack.PurposeLabel] == pack.PurposeMetadata
}
