package app

import (
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
			return trace.Wrap(err)
		}
	}
	for _, dependency := range dependencies.Apps {
		_, err := apps.GetApp(dependency)
		if err != nil {
			return trace.Wrap(err)
		}
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
	for _, dependency := range append(
		app.Manifest.Dependencies.GetPackages(),
		app.Manifest.NodeProfiles.RuntimePackages()...) {
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
	for _, dependency := range append(appDeps, app.Manifest.Dependencies.GetApps()...) {
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
