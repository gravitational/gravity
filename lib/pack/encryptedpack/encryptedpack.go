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

package encryptedpack

import (
	"io"
	"io/ioutil"
	"time"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func New(packages pack.PackageService, encryptionKey string) *EncryptedPack {
	return &EncryptedPack{
		packages:      packages,
		encryptionKey: encryptionKey,
	}
}

type EncryptedPack struct {
	packages      pack.PackageService
	encryptionKey string
}

func (p *EncryptedPack) PortalURL() string {
	return p.packages.PortalURL()
}

func (p *EncryptedPack) PackageDownloadURL(locator loc.Locator) string {
	return p.packages.PackageDownloadURL(locator)
}

func (p *EncryptedPack) UpsertRepository(repository string, expires time.Time) error {
	return p.packages.UpsertRepository(repository, expires)
}

func (p *EncryptedPack) DeleteRepository(repository string) error {
	return p.packages.DeleteRepository(repository)
}

func (p *EncryptedPack) GetRepositories() ([]string, error) {
	return p.packages.GetRepositories()
}

func (p *EncryptedPack) GetRepository(repository string) (storage.Repository, error) {
	return p.packages.GetRepository(repository)
}

func (p *EncryptedPack) GetPackages(repository string) ([]pack.PackageEnvelope, error) {
	return p.packages.GetPackages(repository)
}

func (p *EncryptedPack) CreatePackage(locator loc.Locator, data io.Reader, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	if isSystemPackage(locator) {
		return p.packages.CreatePackage(locator, data, options...)
	}
	log.Infof("encrypting %v", locator)
	encrypted, err := utils.EncryptPGP(data, p.encryptionKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer encrypted.Close()
	return p.packages.CreatePackage(locator, encrypted, append(options, pack.WithEncrypted(true))...)
}

func (p *EncryptedPack) UpsertPackage(locator loc.Locator, data io.Reader, options ...pack.PackageOption) (*pack.PackageEnvelope, error) {
	if isSystemPackage(locator) {
		return p.packages.UpsertPackage(locator, data, options...)
	}
	log.Infof("encrypting %v", locator)
	encrypted, err := utils.EncryptPGP(data, p.encryptionKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer encrypted.Close()
	return p.packages.UpsertPackage(locator, encrypted, append(options, pack.WithEncrypted(true))...)
}

func (p *EncryptedPack) UpdatePackageLabels(locator loc.Locator, addLabels map[string]string, removeLabels []string) error {
	return p.packages.UpdatePackageLabels(locator, addLabels, removeLabels)
}

func (p *EncryptedPack) DeletePackage(locator loc.Locator) error {
	return p.packages.DeletePackage(locator)
}

func (p *EncryptedPack) ReadPackage(locator loc.Locator) (*pack.PackageEnvelope, io.ReadCloser, error) {
	envelope, data, err := p.packages.ReadPackage(locator)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if !envelope.Encrypted {
		return envelope, data, err
	}
	log.Infof("decrypting %v", locator)
	decrypted, err := utils.DecryptPGP(data, p.encryptionKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return envelope, ioutil.NopCloser(decrypted), nil
}

func (p *EncryptedPack) ReadPackageEnvelope(locator loc.Locator) (*pack.PackageEnvelope, error) {
	return p.packages.ReadPackageEnvelope(locator)
}

func isSystemPackage(locator loc.Locator) bool {
	for _, p := range systemPackages {
		if p.Repository == locator.Repository && p.Name == locator.Name {
			return true
		}
	}
	return false
}

// systemPackages is a list of packages that are not encrypted because they
// are required for bootstrapping the installer when license has not yet
// been provided by the user.
var systemPackages = []loc.Locator{
	loc.Gravity,
	loc.Fio,
	loc.WebAssetsPackageLocator,
}
