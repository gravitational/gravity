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
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

var (
	// OptionalDependencies specifies what dependencies can be disabled
	OptionalDependencies = []loc.Locator{
		{Name: defaults.LoggingAppName},
		{Name: defaults.MonitoringAppName},
		{Name: defaults.IngressAppName},
		{Name: defaults.TillerAppName},
		{Name: defaults.StorageAppName},
		{Name: defaults.BandwagonPackageName},
	}
)

// GetDependencies transitively collects dependencies for the specified application package
func GetDependencies(app *Application, apps Applications) (result *Dependencies, err error) {
	state := &state{
		visitedPackages: map[string]struct{}{},
		visitedApps:     map[string]struct{}{},
	}
	if err = getDependencies(app, apps, state); err != nil {
		return nil, trace.Wrap(err)
	}
	result = &Dependencies{}
	result.Packages = append(result.Packages, state.packages...)
	if state.runtimePackage != nil {
		result.Packages = append(result.Packages, *state.runtimePackage)
	}
	for _, locator := range state.apps {
		if !locator.IsEqualTo(app.Package) {
			result.Apps = append(result.Apps, locator)
		}
	}
	result.Packages = loc.Deduplicate(result.Packages)
	return result, nil
}

// VerifyDependencies verifies that the specified application app has all the dependent
// packages available in the provided package service
func VerifyDependencies(app *Application, apps Applications, packages pack.PackageService) error {
	dependencies, err := GetDependencies(app, apps)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, dependency := range dependencies.Packages {
		_, err := packages.ReadPackageEnvelope(dependency)
		if err != nil {
			if trace.IsNotFound(err) {
				log.Debugf("Package dependency %v is not present.", dependency)
			}
			return trace.Wrap(err)
		}
		log.Debugf("Package dependency %v is present.", dependency)
	}
	for _, dependency := range dependencies.Apps {
		_, err := apps.GetApp(dependency)
		if err != nil {
			if trace.IsNotFound(err) {
				log.Debugf("App dependency %v is not present.", dependency)
			}
			return trace.Wrap(err)
		}
		log.Debugf("App dependency %v is present.", dependency)
	}
	return nil
}

// Dependencies defines a set of package and application dependencies
// for an application
type Dependencies struct {
	// Packages defines a set of package dependencies
	Packages []loc.Locator
	// Apps defines a set of application package dependencies
	Apps []loc.Locator
}

func getDependencies(app *Application, apps Applications, state *state) error {
	log.Infof("Getting dependencies for %v.", app.Package)
	packageDeps := loc.Deduplicate(append(
		app.Manifest.Dependencies.GetPackages(),
		app.Manifest.NodeProfiles.RuntimePackages()...))
	for _, dependency := range packageDeps {
		packageName := dependency.String()
		if _, ok := state.visitedPackages[packageName]; !ok {
			state.visitedPackages[packageName] = struct{}{}
			state.packages = append(state.packages, dependency)
		}
	}
	// collect application dependencies, including those of the base application
	var appDeps []loc.Locator
	baseLocator := app.Manifest.Base()
	if baseLocator != nil {
		appDeps = append(appDeps, *baseLocator)
	}
	for _, dependency := range append(appDeps, app.Manifest.Dependencies.FilterApps(app.ExcludeApps)...) {
		packageName := dependency.String()
		if _, ok := state.visitedApps[packageName]; !ok {
			app, err := apps.GetApp(dependency)
			if err != nil {
				return trace.Wrap(err)
			}
			if err := getDependencies(app, apps, state); err != nil {
				return trace.Wrap(err)
			}
			state.visitedApps[packageName] = struct{}{}
			state.apps = append(state.apps, dependency)
		}
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

// AppsToExclude returns a list of apps that should be excluded
func AppsToExclude(manifest schema.Manifest) []loc.Locator {
	var excludeApps []loc.Locator
	for _, app := range OptionalDependencies {
		if schema.ShouldSkipApp(manifest, app) {
			excludeApps = append(excludeApps, app)
		}
	}

	return excludeApps
}

// AppsToExcludeFromManifest returns a list of dependencies that should be excluded
func AppsToExcludeFromManifest(manifestPath string) ([]loc.Locator, error) {
	if manifestPath != "" {
		manifest, err := schema.ParseManifest(manifestPath)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return AppsToExclude(*manifest), nil
	}

	return nil, nil
}

type state struct {
	// packages lists collected package dependencies w/o duplicates
	packages []loc.Locator
	// runtimePackage is the runtime package dependency.
	//
	// The runtime package is computed bottom-up - from dependencies to the top-level application.
	// Without customization, the top-level application gets the runtime package
	// from the runtime (base) application.
	// If the global system options block specifies a custom docker image for the runtime
	// package, the generated package will replace the one from the base application.
	runtimePackage *loc.Locator
	// apps lists collected application dependencies w/o duplicates
	apps []loc.Locator

	visitedApps     map[string]struct{}
	visitedPackages map[string]struct{}
}
