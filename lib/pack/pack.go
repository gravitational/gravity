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

// package pack implements simple package management capabilities
package pack

import (
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/dustin/go-humanize"
)

// LatestVersion is the meta version representing the latest version
var LatestVersion = fmt.Sprintf("0.0.0+%v", LatestLabel)

type Status struct {
	Status string      `json:"status"` // Status of the running container, one of 'running', 'stopped', 'degraded'
	Info   interface{} `json:"info"`   // App-specific information about the container
}

// PackageEnvelope returns extended information about
// a package, such as name, version ,size in bytes, hash, manifest
// and date and time it was created and list of tags
type PackageEnvelope struct {
	// Locator references the package
	Locator loc.Locator `json:"locator"`
	// SizeBytes is the size of the package in bytes
	SizeBytes int64 `json:"size_bytes"`
	// SHA512 specifies the sha-512 checksum of the package contents
	SHA512 string `json:"sha512"`
	// RuntimeLabels specifies a set of labels attached to the package
	RuntimeLabels map[string]string `json:"runtime_labels"`
	// Hidden is whether the package should not be displayed
	Hidden bool `json:"hidden"`
	// Encrypted is whether the package is encrypted
	Encrypted bool `json:"encrypted"`
	// Type specifies the application package type.
	// Empty for regular packages
	Type string `json:"type"`
	// Manifest contains the base64-encoded application manifest
	Manifest []byte `json:"manifest"`
	// Created specifies the package creation timestamp
	Created time.Time `json:"created"`
	// CreatedBy specifies the package creator
	CreatedBy string `json:"created_by"`
}

// HasLabel returns true if envelope has the requested label
func (p *PackageEnvelope) HasLabel(key, val string) bool {
	outval, ok := p.RuntimeLabels[key]
	return ok && outval == val
}

// HasAnyLabel returns true if envelope has any of the provided labels
func (p *PackageEnvelope) HasAnyLabel(labels map[string][]string) bool {
	for name, values := range labels {
		for _, value := range values {
			if p.HasLabel(name, value) {
				return true
			}
		}
	}
	return false
}

// HasLabels returns true if envelope has all of the provided labels
func (p *PackageEnvelope) HasLabels(labels map[string]string) bool {
	for label, value := range labels {
		if !p.HasLabel(label, value) {
			return false
		}
	}
	return true
}

func (p PackageEnvelope) String() string {
	desc := fmt.Sprintf("%v %v", p.Locator.String(), humanize.Bytes(uint64(p.SizeBytes)))
	if p.Encrypted {
		desc += " (encrypted)"
	}
	return desc
}

// Options returns a list of options for this package envelope
func (p PackageEnvelope) Options() []PackageOption {
	var options []PackageOption
	if len(p.RuntimeLabels) != 0 {
		options = append(options, WithLabels(p.RuntimeLabels))
	}
	if p.Hidden {
		options = append(options, WithHidden(p.Hidden))
	}
	if p.Encrypted {
		options = append(options, WithEncrypted(p.Encrypted))
	}
	if len(p.Manifest) != 0 {
		options = append(options, WithManifest(p.Type, p.Manifest))
	}
	if p.CreatedBy != "" {
		options = append(options, WithCreatedBy(p.CreatedBy))
	}
	return options
}

// ToPackage converts package envelope to the storage package format
func (p PackageEnvelope) ToPackage() storage.Package {
	return *p.ToPackagePtr()
}

// ToPackagePtr converts package envelope to the storage package format
func (p PackageEnvelope) ToPackagePtr() *storage.Package {
	return &storage.Package{
		Repository:    p.Locator.Repository,
		Name:          p.Locator.Name,
		Version:       p.Locator.Version,
		SHA512:        p.SHA512,
		SizeBytes:     int(p.SizeBytes),
		Created:       p.Created,
		CreatedBy:     p.CreatedBy,
		RuntimeLabels: p.RuntimeLabels,
		Type:          p.Type,
		Hidden:        p.Hidden,
		Encrypted:     p.Encrypted,
		Manifest:      p.Manifest,
	}
}

type PackageService interface {
	// PackageDownloadURL returns download url for this package
	PackageDownloadURL(loc loc.Locator) string

	// PortalURL returns url for this portal
	PortalURL() string

	// UpsertRepository creates or updates repository, if expires is not
	// zero time the repository and all packages will be set to be expired
	UpsertRepository(repository string, expires time.Time) error

	// DeleteRepository deletes repository - packages will remain in the
	// packages repository
	DeleteRepository(repository string) error

	// GetRepository returns repository by name
	GetRepository(repository string) (storage.Repository, error)

	// Get repositories returns a list of repositories
	GetRepositories() ([]string, error)

	// GetPackages returns a list of packages in repository
	GetPackages(repository string) ([]PackageEnvelope, error)

	// CreatePackage creates package and adds it to to the existing repository
	CreatePackage(loc loc.Locator, data io.Reader, options ...PackageOption) (*PackageEnvelope, error)

	// UpsertPackage creates or updates package and adds it to to the existing repository
	UpsertPackage(loc loc.Locator, data io.Reader, options ...PackageOption) (*PackageEnvelope, error)

	// UpdatePackageLabels updates package's labels
	UpdatePackageLabels(loc loc.Locator, addLabels map[string]string, removeLabels []string) error

	// DeletePackage deletes package from repository
	DeletePackage(locator loc.Locator) error

	// Read package opens and returns package contents
	ReadPackage(loc loc.Locator) (*PackageEnvelope, io.ReadCloser, error)

	// ReadPackageEnvelope returns package envelope
	ReadPackageEnvelope(loc loc.Locator) (*PackageEnvelope, error)
}

// PackageSorter is a package sort helper,
// is used to return deterministic results by lexicographically sorting
// packages
type PackageSorter []PackageEnvelope

func (s PackageSorter) Len() int {
	return len(s)
}

func (s PackageSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s PackageSorter) Less(i, j int) bool {
	return s[i].String() < s[j].String()
}

// PackageOption is a function that can make attribute modifications to the specified package
type PackageOption func(pkg *storage.Package)

type ProgressWriter struct {
	Size    int64
	current int64
	R       ProgressReporter
}

func (w *ProgressWriter) Write(b []byte) (int, error) {
	w.current += int64(len(b))
	w.R.Report(w.current, w.Size)
	return len(b), nil
}

type ProgressReporter interface {
	Report(current, target int64)
}

type ProgressReporterFn func(current, target int64)

func (f ProgressReporterFn) Report(current, target int64) {
	f(current, target)
}

// DiscardReporter is a ProgressReporter that discards all input
var DiscardReporter = nopReporter{}

// DiscardReporter discards report progress
type nopReporter struct{}

// Report of DiscardReporter does nothing
func (nopReporter) Report(current, target int64) {
}
