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

package layerpack

import (
	"io"
	"sort"
	"time"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// New returns a layered package service,
// where inner layer is read-only and all new packages and repositories
// are created in the outer layer
func New(inner, outer pack.PackageService) *Layer {
	return &Layer{
		inner: inner,
		outer: outer,
	}
}

// Layer performs reads from inner, outer layers (in that order)
// but writes to outer layer only
type Layer struct {
	inner pack.PackageService
	outer pack.PackageService
}

func (l *Layer) PortalURL() string {
	return l.outer.PortalURL()
}

func (l *Layer) PackageDownloadURL(loc loc.Locator) string {
	return l.outer.PackageDownloadURL(loc)
}

func (l *Layer) UpsertRepository(repository string, expires time.Time) error {
	return l.outer.UpsertRepository(repository, expires)
}

// DeleteRepository deletes repository - packages will remain in the
// packages repository
func (l *Layer) DeleteRepository(repository string) error {
	return l.outer.DeleteRepository(repository)
}

// GetRepository returns a repository by name
func (l *Layer) GetRepository(repository string) (storage.Repository, error) {
	repo, err := l.inner.GetRepository(repository)
	if err == nil {
		return repo, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return l.outer.GetRepository(repository)
}

// GetRepositories returns a list of repositories
func (l *Layer) GetRepositories() ([]string, error) {
	inner, err := l.inner.GetRepositories()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	outer, err := l.outer.GetRepositories()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// inner and outer layers may contain same repositories, deduplicate them
	reposSet := make(map[string]bool)
	for _, r := range append(inner, outer...) {
		reposSet[r] = true
	}
	repos := make([]string, 0, len(reposSet))
	for k := range reposSet {
		repos = append(repos, k)
	}
	return repos, nil
}

// GetPackages returns a list of packages in repository
func (l *Layer) GetPackages(repository string) ([]pack.PackageEnvelope, error) {
	innerPackages, err := l.inner.GetPackages(repository)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	outerPackages, err := l.outer.GetPackages(repository)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	// inner and outer layers may contain same packages, deduplicate them
	packagesSet := make(map[string]pack.PackageEnvelope)
	for _, p := range append(innerPackages, outerPackages...) {
		packagesSet[p.Locator.String()] = p
	}
	packages := make([]pack.PackageEnvelope, 0, len(packagesSet))
	for _, p := range packagesSet {
		packages = append(packages, p)
	}
	sort.Sort(pack.PackageSorter(packages))
	return packages, nil
}

// UpdatePackageLabels updates package's labels
func (l *Layer) UpdatePackageLabels(loc loc.Locator, addLabels map[string]string, removeLabels []string) error {
	err := l.inner.UpdatePackageLabels(loc, addLabels, removeLabels)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return l.outer.UpdatePackageLabels(loc, addLabels, removeLabels)
}

// CreatePackage creates package and adds it to to the existing repository
func (l *Layer) CreatePackage(loc loc.Locator, data io.Reader, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	return l.outer.CreatePackage(loc, data, options...)
}

// UpsertPackage upserts package and adds it to to the existing repository
func (l *Layer) UpsertPackage(loc loc.Locator, data io.Reader, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	return l.outer.UpsertPackage(loc, data, options...)
}

// DeletePackage deletes package from all repositories
func (l *Layer) DeletePackage(loc loc.Locator) error {
	return l.outer.DeletePackage(loc)
}

// ReadPackage package opens and returns package contents
func (l *Layer) ReadPackage(loc loc.Locator) (*pack.PackageEnvelope, io.ReadCloser, error) {
	e, rc, err := l.outer.ReadPackage(loc)
	if err == nil {
		return e, rc, nil
	}
	if !trace.IsNotFound(err) {
		return nil, nil, trace.Wrap(err)
	}
	return l.inner.ReadPackage(loc)
}

// ReadPackageEnvelope returns package envelope
func (l *Layer) ReadPackageEnvelope(loc loc.Locator) (*pack.PackageEnvelope, error) {
	e, err := l.outer.ReadPackageEnvelope(loc)
	if err == nil {
		return e, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return l.inner.ReadPackageEnvelope(loc)
}
