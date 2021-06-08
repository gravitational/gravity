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

package localpack

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
)

// Config represents package server configuration
type Config struct {
	// Backend is a storage backend for metadata, e.g. package
	// name, version, SHA hash
	Backend storage.Backend

	// Objects is BLOB storage
	Objects blob.Objects

	// DownloadURL sets up download URL used by the package service
	DownloadURL string

	// Clock is used to mock time in tests, if omitted, system time
	// will be used
	Clock timetools.TimeProvider

	// UnpackedDir is the path for unpacked packages
	UnpackedDir string
}

// PackageServer manages BLOBs of data and their metadata as packages
type PackageServer struct {
	cfg     Config
	backend storage.Backend
}

func New(cfg Config) (*PackageServer, error) {
	if cfg.Backend == nil {
		return nil, trace.BadParameter("missing Backend parameter")
	}
	if cfg.Objects == nil {
		return nil, trace.BadParameter("missing Objects parameter")
	}
	if cfg.UnpackedDir == "" {
		return nil, trace.BadParameter("missing UnpackedDir parameter")
	}
	if err := os.MkdirAll(cfg.UnpackedDir, defaults.SharedDirMask); err != nil {
		return nil, trace.Wrap(err)
	}
	if cfg.Clock == nil {
		cfg.Clock = &timetools.RealTime{}
	}
	s := &PackageServer{
		cfg:     cfg,
		backend: cfg.Backend,
	}
	return s, nil
}

func (p *PackageServer) PortalURL() string {
	return p.cfg.DownloadURL
}

// PackageDownloadURL returns download url for this package
func (p *PackageServer) PackageDownloadURL(loc loc.Locator) string {
	return strings.Join([]string{
		p.cfg.DownloadURL, "pack", "v1", "repositories", loc.Repository,
		"packages", loc.Name, loc.Version, "file"}, "/")
}

// GetRepositories repositories returns a list of repositories
func (p *PackageServer) GetRepositories() ([]string, error) {
	out := []string{}

	repos, err := p.backend.GetRepositories()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, r := range repos {
		out = append(out, r.GetName())
	}
	return out, nil
}

// GetRepository returns nil if repository exists, error otherwise
func (p *PackageServer) GetRepository(repository string) (storage.Repository, error) {
	return p.backend.GetRepository(repository)
}

// GetPackages returns a list of package in a given repository
func (p *PackageServer) GetPackages(repository string) ([]pack.PackageEnvelope, error) {
	envelopes := []pack.PackageEnvelope{}
	packages, err := p.backend.GetPackages(repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, p := range packages {
		loc, err := loc.NewLocator(repository, p.Name, p.Version)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		envelopes = append(envelopes, pack.PackageEnvelope{
			Locator:       *loc,
			SizeBytes:     int64(p.SizeBytes),
			SHA512:        p.SHA512,
			RuntimeLabels: p.RuntimeLabels,
			Hidden:        p.Hidden,
			Encrypted:     p.Encrypted,
			Type:          p.Type,
			Manifest:      p.Manifest,
			Created:       p.Created,
			CreatedBy:     p.CreatedBy,
		})
	}

	return envelopes, nil
}

// CreatePackage creates a new package in existing repository
func (p *PackageServer) CreatePackage(loc loc.Locator, data io.Reader, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	// check that the repository exists
	_, err := p.backend.GetRepository(loc.Repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	blobEnvelope, err := p.cfg.Objects.WriteBLOB(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pkg := storage.Package{
		Repository: loc.Repository,
		Name:       loc.Name,
		Version:    loc.Version,
		SHA512:     blobEnvelope.SHA512,
		SizeBytes:  int(blobEnvelope.SizeBytes),
		Created:    p.cfg.Clock.UtcNow(),
	}
	for _, option := range options {
		option(&pkg)
	}

	envelope := &pack.PackageEnvelope{
		Locator:       loc,
		SHA512:        blobEnvelope.SHA512,
		SizeBytes:     blobEnvelope.SizeBytes,
		RuntimeLabels: pkg.RuntimeLabels,
		Hidden:        pkg.Hidden,
		Encrypted:     pkg.Encrypted,
		Type:          pkg.Type,
		Manifest:      pkg.Manifest,
		Created:       pkg.Created,
		CreatedBy:     pkg.CreatedBy,
	}

	// check that the repository exists
	_, err = p.backend.GetRepository(loc.Repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// create package in repository
	_, err = p.backend.CreatePackage(pkg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return envelope, nil
}

// UpsertPackage upserts package and repository
func (p *PackageServer) UpsertPackage(loc loc.Locator, data io.Reader, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	blobEnvelope, err := p.cfg.Objects.WriteBLOB(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pkg := storage.Package{
		Repository: loc.Repository,
		Name:       loc.Name,
		Version:    loc.Version,
		SHA512:     blobEnvelope.SHA512,
		SizeBytes:  int(blobEnvelope.SizeBytes),
		Created:    p.cfg.Clock.UtcNow(),
	}
	for _, option := range options {
		option(&pkg)
	}

	envelope := &pack.PackageEnvelope{
		Locator:       loc,
		SHA512:        blobEnvelope.SHA512,
		SizeBytes:     blobEnvelope.SizeBytes,
		RuntimeLabels: pkg.RuntimeLabels,
		Hidden:        pkg.Hidden,
		Encrypted:     pkg.Encrypted,
		Type:          pkg.Type,
		Manifest:      pkg.Manifest,
		Created:       pkg.Created,
		CreatedBy:     pkg.CreatedBy,
	}

	_, err = p.backend.CreateRepository(storage.NewRepository(loc.Repository))
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err)
		}
	}
	_, err = p.backend.UpsertPackage(pkg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return envelope, nil
}

func (p *PackageServer) processMetadata(locator loc.Locator) (loc.Locator, error) {
	locatorPtr, err := pack.ProcessMetadata(p, &locator)
	if err != nil {
		return locator, trace.Wrap(err)
	}
	return *locatorPtr, nil
}

// ReadPackage package opens and returns package contents
func (p *PackageServer) ReadPackage(loc loc.Locator) (*pack.PackageEnvelope, io.ReadCloser, error) {
	var err error
	loc, err = p.processMetadata(loc)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	pk, err := p.backend.GetPackage(loc.Repository, loc.Name, loc.Version)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	_, err = p.backend.GetRepository(loc.Repository)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	f, err := p.cfg.Objects.OpenBLOB(pk.SHA512)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return newEnvelope(loc, pk), f, nil
}

// DeletePackage removes package from all repository and deletes the package
func (p *PackageServer) DeletePackage(loc loc.Locator) error {
	repo, err := p.backend.GetRepository(loc.Repository)
	if err != nil {
		return trace.Wrap(err)
	}
	loc, err = p.processMetadata(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	pk, err := p.backend.GetPackage(repo.GetName(), loc.Name, loc.Version)
	if err != nil {
		return trace.Wrap(err)
	}
	// remove package from all repositories
	err = p.backend.DeletePackage(loc.Repository, loc.Name, loc.Version)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.tryDeleteBlob(*pk)
	if err != nil {
		return trace.Wrap(err)
	}
	unpackedPath, err := p.UnpackedPath(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	if _, err := os.Stat(unpackedPath); err == nil {
		log.WithFields(log.Fields{
			"unpacked-dir": unpackedPath,
			"package":      loc,
		}).Infof("Delete unpacked directory.")
		if err := os.RemoveAll(unpackedPath); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UpdatePackageLabels updates package's labels
func (p *PackageServer) UpdatePackageLabels(loc loc.Locator, addLabels map[string]string, removeLabels []string) error {
	var err error
	loc, err = p.processMetadata(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.backend.UpdatePackageRuntimeLabels(loc.Repository, loc.Name, loc.Version, addLabels, removeLabels)
	return trace.Wrap(err)
}

// UpsertRepository creates or updates repository, note that expiration
// parameter will not be updated if repository already exists
func (p *PackageServer) UpsertRepository(repository string, expires time.Time) error {
	if !cstrings.IsValidDomainName(repository) {
		return trace.BadParameter(
			"expected a valid domain name, got '%v'", repository)
	}
	_, err := p.backend.GetRepository(repository)
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	repo := storage.NewRepository(repository)
	if !expires.IsZero() {
		repo.SetExpiry(expires)
	}
	_, err = p.backend.CreateRepository(repo)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteRepository deletes repository and all its packages
func (p *PackageServer) DeleteRepository(repository string) error {
	err := pack.ForeachPackageInRepo(p, repository, func(e pack.PackageEnvelope) error {
		return trace.Wrap(p.DeletePackage(e.Locator))
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(p.backend.DeleteRepository(repository))
}

// ReadPackageEnvelope returns package envelope without reading the BLOB
func (p *PackageServer) ReadPackageEnvelope(loc loc.Locator) (*pack.PackageEnvelope, error) {
	var err error
	loc, err = p.processMetadata(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pk, err := p.backend.GetPackage(loc.Repository, loc.Name, loc.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newEnvelope(loc, pk), nil
}

func (p *PackageServer) UnpackedPath(loc loc.Locator) (string, error) {
	// the path can not be too long because it leads to problems like this:
	// https: //github.com/golang/go/issues/6895
	var err error
	loc, err = p.processMetadata(loc)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return pack.PackagePath(p.cfg.UnpackedDir, loc), nil
}

func (p *PackageServer) Unpack(loc loc.Locator, targetDir string) error {
	var err error

	loc, err = p.processMetadata(loc)
	if err != nil {
		return trace.Wrap(err)
	}

	if targetDir == "" {
		targetDir, err = p.UnpackedPath(loc)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	unpacked, err := pack.IsUnpacked(targetDir)
	if err != nil {
		return trace.Wrap(err)
	}
	if unpacked {
		log.WithField("package", loc).Info("Package is already unpacked.")
		return nil
	}
	if err := pack.Unpack(p, loc, targetDir, nil); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *PackageServer) GetPackageManifest(loc loc.Locator) (*pack.Manifest, error) {
	var err error
	loc, err = p.processMetadata(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := p.Unpack(loc, ""); err != nil {
		return nil, trace.Wrap(err)
	}
	path, err := p.UnpackedPath(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return pack.OpenManifest(path)
}

func (p *PackageServer) ConfigurePackage(loc loc.Locator, confLoc loc.Locator, args []string) error {
	var err error
	loc, err = p.processMetadata(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	return pack.ConfigurePackage(p, loc, confLoc, args, nil)
}

func (p *PackageServer) tryDeleteBlob(toDelete storage.Package) error {
	log := log.WithFields(log.Fields{
		"package":  toDelete.Locator(),
		"checksum": toDelete.SHA512,
	})
	repositories, err := p.backend.GetRepositories()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, repository := range repositories {
		packages, err := p.backend.GetPackages(repository.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		for _, pkg := range packages {
			if pkg.SHA512 == toDelete.SHA512 && !pkg.Locator().IsEqualTo(toDelete.Locator()) {
				log.WithField("existing", pkg.Locator()).
					Info("Collision with existing package, will not delete blob.")
				return nil
			}
		}
	}
	log.Info("Delete BLOB.")
	return trace.Wrap(p.cfg.Objects.DeleteBLOB(toDelete.SHA512))
}

func newEnvelope(loc loc.Locator, p *storage.Package) *pack.PackageEnvelope {
	return &pack.PackageEnvelope{
		Locator:       loc,
		SizeBytes:     int64(p.SizeBytes),
		SHA512:        p.SHA512,
		RuntimeLabels: p.RuntimeLabels,
		Hidden:        p.Hidden,
		Encrypted:     p.Encrypted,
		Type:          p.Type,
		Manifest:      p.Manifest,
		Created:       p.Created,
		CreatedBy:     p.CreatedBy,
	}
}
