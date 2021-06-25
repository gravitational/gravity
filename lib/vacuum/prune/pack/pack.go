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
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/vacuum/prune"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New creates a new package vacuum cleaner
func New(config Config) (*Cleanup, error) {
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

	return &Cleanup{
		config:         config,
		runtimeVersion: *baseVersion,
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
	return nil
}

// Config describes configuration for the cleaner of unused packages
type Config struct {
	// Config specifies the common pruner configuration
	prune.Config
	// App specifies the cluster application
	App *storage.Application
	// Apps lists other cluster applications.
	// There might be several applications meaningful for the cluster
	// if it's an Ops Center and has been connected with multiple remote
	// clusters.
	Apps []storage.Application
	// Packages specifies the package service to prune
	Packages packageService
}

// packageService defines the subset of package APIs as required for pruning
type packageService interface {
	GetRepositories() ([]string, error)
	GetPackages(repository string) ([]pack.PackageEnvelope, error)
	ReadPackageEnvelope(loc.Locator) (*pack.PackageEnvelope, error)
	DeletePackage(loc.Locator) error
}

// Prune removes unused packages from the configured package service.
// It uses the direct application dependencies to determine the set of packages
// that are still required, and sweeps the rest.
// It will not remove packages from repositories other than the defaults.SystemAccountOrg
// unless it can tell if a package is safe to remove.
func (r *Cleanup) Prune(context.Context) error {
	required, err := r.mark()
	if err != nil {
		return trace.Wrap(err)
	}

	state, err := r.build(required)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, item := range state {
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
func (r *Cleanup) mark() (required packageMap, err error) {
	dependencies := append(r.config.App.Manifest.AllPackageDependencies(),
		r.config.App.Manifest.Dependencies.GetApps()...)
	dependencies = append(dependencies, r.config.App.Locator)
	if base := r.config.App.Manifest.Base(); base != nil {
		dependencies = append(dependencies, *base)
	}

	required = make(packageMap)
	for _, app := range r.config.Apps {
		dependencies = append(dependencies, app.Manifest.AllPackageDependencies()...)
		dependencies = append(dependencies, app.Manifest.Dependencies.GetApps()...)
		dependencies = append(dependencies, app.Locator)
		if base := app.Manifest.Base(); base != nil {
			dependencies = append(dependencies, *base)
		}
	}

	for _, dependency := range dependencies {
		semver, err := dependency.SemVer()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		r.config.PrintStepf("Mark package %v as required.", dependency)
		required[dependency.ZeroVersion()] = existingPackage{
			Version:         *semver,
			PackageEnvelope: pack.PackageEnvelope{Locator: dependency},
		}
	}

	return required, nil
}

// build builds a package tree to be able to track package dependencies
// and prune packages in proper order
func (r *Cleanup) build(required packageMap) (state map[loc.Locator]statePackage, err error) {
	state = make(map[loc.Locator]statePackage)
	repositories, err := r.config.Packages.GetRepositories()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, repository := range repositories {
		envelopes, err := r.config.Packages.GetPackages(repository)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, envelope := range envelopes {
			version, err := envelope.Locator.SemVer()
			if err != nil {
				return nil, trace.Wrap(err)
			}
			current := existingPackage{PackageEnvelope: envelope, Version: *version}

			var items []statePackage
			switch {
			case isPlanetConfigPackage(envelope):
				items, err = r.withDependencies(current, packageForConfig, state)
			case isAppResourcesPackage(envelope):
				items, err = r.withDependencies(current, packageForResources, state)
			default:
				items = append(items, statePackage{existingPackage: current})
			}
			if err != nil {
				return nil, trace.Wrap(err)
			}

			for _, item := range items {
				if !r.shouldDeletePackage(item.existingPackage, required) {
					continue
				}

				var existingItem statePackage
				var exists bool
				if existingItem, exists = state[item.Locator]; exists {
					existingItem.dependencies = append(existingItem.dependencies,
						item.dependencies...)
				} else {
					existingItem = item
				}
				state[item.Locator] = existingItem
			}
		}
	}
	r.config.Debug("Package state:", state)
	return state, nil
}

func (r *Cleanup) deletePackage(item statePackage) error {
	r.config.PrintStepf("Deleting package %v.", item.Locator)
	if r.config.DryRun {
		return nil
	}
	err := r.config.Packages.DeletePackage(item.Locator)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// shouldDeletePackage determines if the specified package pkg is eligible for removal.
// It will match the package against the specified map of required packages.
// It will also apply a couple of additional ad-hoc heuristics to decide if a package
// should be deleted
func (r *Cleanup) shouldDeletePackage(pkg existingPackage, required packageMap) (delete bool) {
	log := r.config.WithField("package", pkg.Locator)

	if existingVersion, exists := required[pkg.Locator.ZeroVersion()]; exists {
		if existingVersion.Compare(pkg.Version) > 0 {
			log.Debug("Will delete an obsolete package.")
			return true
		}
		log.Debug("Will not delete a package still in use.")
		return false
	}

	if loc.IsLegacyRuntimePackage(pkg.PackageEnvelope.Locator) {
		log.Debug("Will delete a legacy runtime package.")
		return true
	}

	if isRPCUpdateCredentialsPackage(pkg.PackageEnvelope) && pkg.Version.Compare(r.runtimeVersion) < 0 {
		// Remove obsolete update RPC credentials used in prior update operations.
		// All RPC packages with versions prior or equal to the currently
		// installed runtime version are eligible for removal.
		log.Debug("Will delete an obsolete RPC credentials package.")
		return true
	}

	if pkg.Locator.Repository != defaults.SystemAccountOrg {
		log.Debug("Will not delete from a custom repository.")
		return false
	}

	log.Debug("Will not delete an unknown package.")
	return false
}

// withDependencies computes owner packages for the specified package pkg using
// the specified owner search algorithm and the existing package state
func (r *Cleanup) withDependencies(pkg existingPackage, owner ownerFunc, state map[loc.Locator]statePackage) (items []statePackage, err error) {
	ownerPackages, err := owner(pkg.PackageEnvelope, r.config.Packages)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		r.config.Warnf("Orphaned package %v.", pkg.Locator)
		return []statePackage{
			statePackage{existingPackage: pkg},
		}, nil
	}

	for _, ownerPackage := range ownerPackages {
		ownerVersion, err := ownerPackage.Locator.SemVer()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		var parent statePackage
		var exists bool
		if parent, exists = state[ownerPackage.Locator]; !exists {
			parent = statePackage{
				existingPackage: existingPackage{
					PackageEnvelope: ownerPackage,
					Version:         *ownerVersion,
				},
			}
		}
		parent.dependencies = append(parent.dependencies, statePackage{existingPackage: pkg})
		items = append(items, parent)
	}
	return items, nil
}

// ownerFunc computes the owner of the specified package.
//
// Pruner will remove dependee packages before owner packages.
//
// For example, the application resource package is considered a dependee
// for the related application package, so the resource package is removed
// before the application package is.
type ownerFunc func(pack.PackageEnvelope, packageService) ([]pack.PackageEnvelope, error)

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

	return ownerForPackage(envelope, service, parentPackage)
}

func packageForResources(envelope pack.PackageEnvelope, service packageService) ([]pack.PackageEnvelope, error) {
	appPackageName := strings.TrimSuffix(envelope.Locator.Name, "-resources")
	appPackage := loc.Locator{
		Repository: envelope.Locator.Repository,
		Name:       appPackageName,
		Version:    envelope.Locator.Version,
	}
	return ownerForPackage(envelope, service, appPackage)
}

func ownerForPackage(envelope pack.PackageEnvelope, service packageService, owner loc.Locator) ([]pack.PackageEnvelope, error) {
	ownerEnv, err := service.ReadPackageEnvelope(owner)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no owner package %v found for package %v",
				owner, envelope.Locator)
		}
		return nil, trace.Wrap(err)
	}
	return []pack.PackageEnvelope{*ownerEnv}, nil
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

// Cleanup implements garbage collection for packages
type Cleanup struct {
	config Config
	// runtimeVersion specifies the version of gravity
	runtimeVersion semver.Version
}

func (r statePackage) String() string {
	var deps []string
	for _, dep := range r.dependencies {
		deps = append(deps, dep.String())
	}
	formatDeps := func(deps []string) string {
		if len(deps) == 0 {
			return "none"
		}
		return strings.Join(deps, ",")
	}
	return fmt.Sprintf("%v(deps=%v)", r.Locator.String(), formatDeps(deps))
}

type statePackage struct {
	existingPackage
	dependencies []statePackage
}
