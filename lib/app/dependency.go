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

package app

import (
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// VerifyDependencies verifies that all dependencies for the specified application are available
// in the provided package service.
func VerifyDependencies(app Application, apps Applications, packages pack.PackageService) error {
	_, err := GetDependencies(GetDependenciesRequest{
		App:  app,
		Apps: apps,
		Pack: packages,
	})
	return trace.Wrap(err)
}

// GetDependencies transitively collects dependencies for the specified application package
func GetDependencies(req GetDependenciesRequest) (result *Dependencies, err error) {
	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	state := &state{
		packages: make(map[loc.Locator]struct{}),
		apps:     make(map[loc.Locator]struct{}),
	}
	if err = req.getDependencies(req.App, state); err != nil {
		return nil, trace.Wrap(err)
	}
	return &state.deps, nil
}

// GetDependenciesRequest describes a request to transitively enumerate packages dependencies
// for a specific application package
type GetDependenciesRequest struct {
	// App specifies the application to fetch dependencies for
	App Application
	// Apps specifies the application service
	Apps Applications
	// Pack specifies the package service
	Pack pack.PackageService
	// FieldLogger specifies the logger
	log.FieldLogger
}

// AsPackages returns dependencies as a list of package identifiers
func (r Dependencies) AsPackages() (result []loc.Locator) {
	result = make([]loc.Locator, 0, len(r.Packages)+len(r.Apps))
	for _, pkg := range r.Packages {
		result = append(result, pkg.Locator)
	}
	for _, app := range r.Apps {
		result = append(result, app.Package)
	}
	return result
}

// Dependencies defines a set of package and application dependencies
// for an application
type Dependencies struct {
	// Packages defines a set of package dependencies
	Packages []pack.PackageEnvelope `json:"packages,omitempty"`
	// Apps defines a set of application package dependencies
	Apps []Application `json:"apps,omitempty"`
}

func (r *GetDependenciesRequest) checkAndSetDefaults() error {
	if r.Apps == nil {
		return trace.BadParameter("application service is required")
	}
	if r.Pack == nil {
		return trace.BadParameter("package service is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "deps")
	}
	return nil
}

func (r GetDependenciesRequest) getDependencies(app Application, state *state) error {
	r.WithField("app", app.Package).Info("Get dependencies.")
	packageDeps := loc.Deduplicate(app.Manifest.Dependencies.GetPackages())
	packageDeps = append(packageDeps, app.Manifest.NodeProfiles.RuntimePackages()...)
	for _, dependency := range packageDeps {
		if state.hasPackage(dependency) {
			continue
		}
		envelope, err := r.Pack.ReadPackageEnvelope(dependency)
		if err != nil {
			return trace.Wrap(err)
		}
		state.addPackage(*envelope)
	}
	// collect application dependencies, including those of the base application
	var appDeps []loc.Locator
	baseLocator := app.Manifest.Base()
	if baseLocator != nil {
		appDeps = append(appDeps, *baseLocator)
	}
	appDeps = append(appDeps, app.Manifest.Dependencies.GetApps()...)
	for _, dependency := range appDeps {
		if state.hasApp(dependency) {
			continue
		}
		app, err := r.Apps.GetApp(dependency)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := r.getDependencies(*app, state); err != nil {
			return trace.Wrap(err)
		}
		state.addApp(*app)
	}
	// Fetch and persist the default runtime package.
	// If the top-level application overwrites the runtime package,
	// only the top-level runtime package is pulled
	// Ignore the error, since here we're only interested if a custom package
	// has been defined
	if runtimePackage, _ := app.Manifest.DefaultRuntimePackage(); runtimePackage != nil {
		state.runtimePackage = runtimePackage
	}
	return nil
}

func (r *state) hasPackage(pkg loc.Locator) bool {
	_, ok := r.packages[pkg]
	return ok
}

func (r *state) hasApp(pkg loc.Locator) bool {
	_, ok := r.apps[pkg]
	return ok
}

func (r *state) addPackage(pkg pack.PackageEnvelope) {
	r.packages[pkg.Locator] = struct{}{}
	r.deps.Packages = append(r.deps.Packages, pkg)
}

func (r *state) addApp(app Application) {
	r.apps[app.Package] = struct{}{}
	r.deps.Apps = append(r.deps.Apps, app)
}

type state struct {
	deps Dependencies
	// packages lists collected package dependencies
	packages map[loc.Locator]struct{}
	// apps lists collected application dependencies
	apps map[loc.Locator]struct{}
	// runtimePackage is the runtime package dependency.
	//
	// The runtime package is computed bottom-up - from dependencies to the top-level application.
	// Without customization, the top-level application gets the runtime package
	// from the runtime (base) application.
	// If the global system options block specifies a custom docker image for the runtime
	// package, the generated package will replace the one from the base application.
	runtimePackage *loc.Locator
}
