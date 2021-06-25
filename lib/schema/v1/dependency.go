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

package v1

import (
	"encoding/json"
	"strconv"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/trace"
)

const (
	// SelectorRole defines a package role selector
	SelectorRole = "role"
	// SelectorPlacement defines a node placement selector
	SelectorPlacement = "placement"

	// PlacementMaster defines the value of the placement selector to be on master
	PlacementMaster = "master"
)

// Dependencies defines a set of dependencies for an application
type Dependencies struct {
	// Packages defines the package dependencies
	Packages PackageDependencies `json:"packages,omitempty"`
	// Apps defines the application dependencies
	Apps AppDependencies `json:"apps,omitempty"`
}

// PackageDependencies defines a list of package dependencies
type PackageDependencies []PackageDependency

// PackageDependency defines a dependency on a package
type PackageDependency struct {
	// Package defines the package name of the dependency
	Package loc.Locator
	// Selector defines the list of selectors assigned to this package
	Selector map[string]string `json:"selector,omitempty"`
}

// UnmarshalJSON reads package dependency from the specified data
func (r *PackageDependency) UnmarshalJSON(data []byte) error {
	var item packageRawDependency
	if err := json.Unmarshal(data, &item); err != nil {
		return trace.Wrap(err)
	}
	locator, err := loc.ParseLocator(item.Name)
	if err == nil {
		r.Package = *locator
	}
	r.Selector = item.Selector
	return trace.Wrap(err)
}

// MarshalJSON formats package dependency as JSON
func (r *PackageDependency) MarshalJSON() ([]byte, error) {
	var item = packageRawDependency{
		Name:     r.Package.String(),
		Selector: r.Selector,
	}
	return json.Marshal(&item)
}

type packageRawDependency struct {
	Name     string            `json:"name"`
	Selector map[string]string `json:"selector,omitempty"`
}

// WithRole returns a list of packages matching the specified role
func (r PackageDependencies) WithRole(packageName string) (*loc.Locator, error) {
	packages := packagesWithMatcher(r, matchWithSelector(SelectorRole, packageName))
	if len(packages) != 1 {
		return nil, trace.BadParameter("expected a package with role `%s` in manifest", packageName)
	}
	return &packages[0], nil
}

func (r PackageDependencies) WithName(name string) (*loc.Locator, error) {
	packages := packagesWithMatcher(r, matchWithName(name))
	if len(packages) != 1 {
		return nil, trace.BadParameter("expected a package with name `%v` in manifest", name)
	}
	return &packages[0], nil
}

// WithSelector returns a list of packages matching the specified selector value
func (r PackageDependencies) WithSelector(selector, value string) []loc.Locator {
	return packagesWithMatcher(r, matchWithSelector(selector, value))
}

// All returns a list of package locators in this dependency list
func (r PackageDependencies) All() (packages []loc.Locator) {
	packages = make([]loc.Locator, len(r))
	for i, dependency := range r {
		packages[i] = dependency.Package
	}
	return packages
}

// Merge inserts packages into this dependency list avoiding duplicates
func (r *PackageDependencies) Merge(packages PackageDependencies) {
L:
	for _, inputDependency := range *r {
		for _, dependency := range packages {
			if dependency.Package.IsEqualTo(inputDependency.Package) {
				continue L
			}
		}
		packages = append(packages, inputDependency)
	}
	*r = packages
}

// AppDependencies defines a list of application package dependencies
type AppDependencies []AppDependency

// AppDependency defines a dependency on another application
type AppDependency struct {
	// Package defines the package name of the dependency
	Package loc.Locator
}

// UnmarshalJSON reads application package dependency from the specified data
func (r *AppDependency) UnmarshalJSON(data []byte) error {
	return PackageUnmarshalJSON(data, &r.Package)
}

// MarshalJSON formats application dependency as JSON
func (r *AppDependency) MarshalJSON() ([]byte, error) {
	return PackageMarshalJSON(&r.Package)
}

// All returns a list of package locators in this dependency list
func (r AppDependencies) All() (packages []loc.Locator) {
	packages = make([]loc.Locator, len(r))
	for i, dependency := range r {
		packages[i] = dependency.Package
	}
	return packages
}

// Merge inserts apps into this dependency list avoiding duplicates
func (r *AppDependencies) Merge(apps AppDependencies) {
L:
	for _, inputDependency := range *r {
		for _, dependency := range apps {
			if dependency.Package.IsEqualTo(inputDependency.Package) {
				continue L
			}
		}
		apps = append(apps, inputDependency)
	}
	*r = apps
}

func PackageUnmarshalJSON(data []byte, locator *loc.Locator) error {
	value, err := strconv.Unquote(string(data))
	if err != nil {
		return trace.Wrap(err)
	}
	parsedLocator, err := loc.ParseLocator(value)
	if err == nil {
		*locator = *parsedLocator
	}
	return trace.Wrap(err)
}

func PackageMarshalJSON(locator *loc.Locator) ([]byte, error) {
	return []byte(strconv.Quote(locator.String())), nil
}

type packageMatcher func(PackageDependency) bool

func matchWithSelector(selector, value string) packageMatcher {
	return func(dependency PackageDependency) bool {
		return dependency.Selector[selector] == value
	}
}

func matchWithName(name string) packageMatcher {
	return func(dependency PackageDependency) bool {
		return dependency.Package.Name == name
	}
}

func packagesWithMatcher(dependencies PackageDependencies, matcher packageMatcher) (packages []loc.Locator) {
	for _, dependency := range dependencies {
		if matcher(dependency) {
			packages = append(packages, dependency.Package)
		}
	}
	return packages
}
