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

package pack

import (
	"context"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/vacuum/prune"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New creates a new package vacuum cleaner
func New(config Config) (*cleanup, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	base := config.App.Manifest.Base()
	if base == nil {
		return nil, trace.BadParameter("cluster application does not have a runtime")
	}
	baseVersion, err := base.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &cleanup{
		Config:         config,
		runtimeVersion: *baseVersion,
		state:          make(map[loc.Locator]statePackage),
	}, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.App == nil {
		return trace.BadParameter("application package is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("package service is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "gc:package")
	}
	if r.Emitter == nil {
		r.Emitter = utils.NopEmitter()
	}
	return nil
}

// Config describes configuration for the cleaner of unused packages
type Config struct {
	// Config specifies the common pruner configuration
	prune.Config
	// App specifies the cluster application
	App *Application
	// Apps lists other cluster applications.
	// There might be several applications meaningful for the cluster
	// if it's an Ops Center and has been connected with multiple remote
	// clusters.
	Apps []Application
	// Packages specifies the package service to prune
	Packages packageService
}

// Application describes an application for the package cleaner
type Application struct {
	// Locator references the application package
	loc.Locator
	// Manifest is the application's manifest
	schema.Manifest
}

// packageService defines the subset of package APIs as required for pruning
type packageService interface {
	GetRepositories() ([]string, error)
	GetPackages(respository string) ([]pack.PackageEnvelope, error)
	ReadPackageEnvelope(loc.Locator) (*pack.PackageEnvelope, error)
	DeletePackage(loc.Locator) error
}

// Prune removes unused packages from the conigured package service.
// It uses the direct application dependencies to determine the set of packages
// that are still required, and sweeps the rest.
// It will not remove packages from repositories other than the defaults.SystemAccountOrg
// unless it can tell if a package is safe to remove.
func (r *cleanup) Prune(context.Context) error {
	required, err := r.mark()
	r.Infof("Required package list: %v.", required)
	if err != nil {
		return trace.Wrap(err)
	}

	err = r.build(required)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, item := range r.state {
		for _, dep := range item.dependencies {
			err = r.deletePackage(dep)
			if err != nil && !trace.IsNotFound(err) {
				return trace.Wrap(err)
			}
		}
		err = r.deletePackage(item)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	return nil
}

// mark marks the direct application and package dependencies of the cluster
// application as required.
// Returns the map of package locator -> descriptor for packages that are not
// eligible for removal
func (r *cleanup) mark() (required packageMap, err error) {
	dependencies := append(r.App.Manifest.AllPackageDependencies(),
		r.App.Manifest.Dependencies.GetApps()...)
	dependencies = append(dependencies, r.App.Locator)
	required = make(packageMap)
	for _, app := range r.Apps {
		dependencies = append(dependencies, app.Manifest.AllPackageDependencies()...)
		dependencies = append(dependencies, app.Manifest.Dependencies.GetApps()...)
		dependencies = append(dependencies, app.Locator)
	}
	for _, dependency := range dependencies {
		semver, err := dependency.SemVer()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		r.PrintStep("Mark package %v as required.", dependency)
		required[dependency.ZeroVersion()] = existingPackage{
			Version:         *semver,
			PackageEnvelope: pack.PackageEnvelope{Locator: dependency},
		}
	}
	return required, nil
}

// build builds a tree of package dependencies to be able to remove
// packages properly
func (r *cleanup) build(required packageMap) error {
	repositories, err := r.Packages.GetRepositories()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, repository := range repositories {
		envelopes, err := r.Packages.GetPackages(repository)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, envelope := range envelopes {
			version, err := envelope.Locator.SemVer()
			if err != nil {
				return trace.Wrap(err)
			}
			current := existingPackage{PackageEnvelope: envelope, Version: *version}

			var items []statePackage
			switch {
			case isPlanetConfigPackage(envelope):
				r.Infof("Check whether should prune planet configuration package %v.", envelope.Locator)
				items, err = r.withDependencies(current, packageForConfig)
			case isAppResourcesPackage(envelope):
				r.Infof("Check whether should prune app resource package %v.", envelope.Locator)
				items, err = r.withDependencies(current, packageForResources)
			default:
				items = append(items, statePackage{existingPackage: current})
			}
			if err != nil {
				return trace.Wrap(err)
			}

			for _, item := range items {
				deletePackage, err := r.shouldDeletePackage(item.existingPackage, required)
				if err != nil {
					return trace.Wrap(err)
				}

				if deletePackage {
					r.Infof("Will delete package %v.", envelope.Locator)
					r.state[item.Locator] = item
				}
			}
		}
	}
	return nil
}

func (r *cleanup) deletePackage(item statePackage) error {
	r.PrintStep("Deleting package %v.", item.Locator)
	if r.DryRun {
		return nil
	}
	err := r.Packages.DeletePackage(item.Locator)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// shouldDeletePackage determines if the specified package pkg is eligible for removal.
// It will match the package against the specified map of required packages.
// It will also apply a couple of additional ad-hoc heuristics to decide if a package
// should be deleted
func (r *cleanup) shouldDeletePackage(pkg existingPackage, required packageMap) (delete bool, err error) {
	log := r.WithField("package", pkg.Locator)

	if isLegacyRuntimePackage(pkg.PackageEnvelope) {
		log.Debug("Will delete a legacy runtime package.")
		return true, nil
	}

	if isRPCUpdateCredentialsPackage(pkg.PackageEnvelope) && pkg.Version.Compare(r.runtimeVersion) < 0 {
		// Remove obsolete update RPC credentials used in prior update operations.
		// All RPC packages with versions prior or equal to the currently
		// installed runtime version are eligible for removal.
		log.Debug("Will delete an obsolete RPC credentials package.")
		return true, nil
	}

	if existingVersion, exists := required[pkg.Locator.ZeroVersion()]; exists {
		if existingVersion.Compare(pkg.Version) > 0 {
			log.Debug("Will delete an obsolete package.")
			return true, nil
		}
		log.Debug("Will not delete a package still in use.")
	}

	if pkg.Locator.Repository != defaults.SystemAccountOrg {
		log.Debug("Will not delete from a custom repository.")
		return false, nil
	}

	return false, nil
}

func (r *cleanup) withDependencies(pkg existingPackage, depender dependerFunc) (items []statePackage, err error) {
	dependerPackages, err := depender(pkg.PackageEnvelope, r.Packages)
	r.Infof("Depender packages for %v: %v (%v).", pkg.Locator, dependerPackages, err)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		r.Warnf("Orphaned package %v.", pkg.Locator)
		return []statePackage{
			statePackage{existingPackage: pkg},
		}, nil
	}

	for _, dependerPackage := range dependerPackages {
		dependerVersion, err := dependerPackage.Locator.SemVer()
		r.Infof("Depender package version: %v (%v).", dependerVersion, err)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var parent statePackage
		var exists bool
		if parent, exists = r.state[dependerPackage.Locator]; !exists {
			parent = statePackage{
				existingPackage: existingPackage{
					PackageEnvelope: dependerPackage,
					Version:         *dependerVersion,
				},
			}
		}
		parent.dependencies = append(parent.dependencies, statePackage{existingPackage: pkg})
		items = append(items, parent)
	}
	return items, nil
}

type dependerFunc func(pack.PackageEnvelope, packageService) ([]pack.PackageEnvelope, error)

func packageForConfig(envelope pack.PackageEnvelope, service packageService) ([]pack.PackageEnvelope, error) {
	parentPackageRef := envelope.RuntimeLabels[pack.ConfigLabel]
	parentPackageFilter, err := loc.ParseLocator(parentPackageRef)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	parentPackage := loc.Locator{
		Repository: parentPackageFilter.Repository,
		Name:       parentPackageFilter.Name,
		Version:    envelope.Locator.Version,
	}

	return dependerForPackage(envelope, service, parentPackage)
}

func packageForResources(envelope pack.PackageEnvelope, service packageService) ([]pack.PackageEnvelope, error) {
	appPackageName := strings.TrimSuffix(envelope.Locator.Name, "-resources")
	appPackage := loc.Locator{
		Repository: envelope.Locator.Repository,
		Name:       appPackageName,
		Version:    envelope.Locator.Version,
	}
	return dependerForPackage(envelope, service, appPackage)
}

func dependerForPackage(envelope pack.PackageEnvelope, service packageService, depender loc.Locator) ([]pack.PackageEnvelope, error) {
	dependerEnv, err := service.ReadPackageEnvelope(depender)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no depender package %v found for package %v",
				depender, envelope.Locator)
		}
		return nil, trace.Wrap(err)
	}
	return []pack.PackageEnvelope{*dependerEnv}, nil
}

func getPackagesByFilter(service packageService, filter loc.Locator) (result []pack.PackageEnvelope, err error) {
	envelopes, err := service.GetPackages(filter.Repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, env := range envelopes {
		if env.Locator.Name == filter.Name {
			result = append(result, env)
		}
	}
	return result, nil
}

func isAppResourcesPackage(envelope pack.PackageEnvelope) bool {
	return strings.HasSuffix(envelope.Locator.Name, "-resources")
}

func isPlanetConfigPackage(envelope pack.PackageEnvelope) bool {
	label, exists := envelope.RuntimeLabels[pack.PurposeLabel]
	return exists && label == pack.PurposePlanetConfig
}

// isRPCUpdateCredentialsPackage returns true if the specified package is an RPC credentials
// package used in prior update operations
func isRPCUpdateCredentialsPackage(envelope pack.PackageEnvelope) bool {
	if envelope.Locator.Repository == defaults.SystemAccountOrg {
		// Skip the long-term RPC credentials package
		return false
	}
	if label, exists := envelope.RuntimeLabels[pack.PurposeLabel]; exists && label == pack.PurposeRPCCredentials {
		return true
	}
	// Match the RPC package w/o label by name
	pattern := loc.Locator{
		Repository: envelope.Locator.Repository,
		Name:       defaults.RPCAgentSecretsPackage,
		Version:    loc.ZeroVersion,
	}
	return envelope.Locator.ZeroVersion().IsEqualTo(pattern)
}

type packageMap map[loc.Locator]existingPackage

type existingPackage struct {
	semver.Version
	pack.PackageEnvelope
}

type cleanup struct {
	Config
	state map[loc.Locator]statePackage
	// runtimeVersion specifies the version of telekube
	runtimeVersion semver.Version
}

type statePackage struct {
	existingPackage
	dependencies []statePackage
}
