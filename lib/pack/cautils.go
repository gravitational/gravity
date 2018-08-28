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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
)

// ReadCertificateAuthority reads the OpsCenter certificate authority package from the
// provided package service and returns its key pair
func ReadCertificateAuthority(packages PackageService) (*authority.TLSKeyPair, error) {
	_, reader, err := packages.ReadPackage(loc.OpsCenterCertificateAuthority)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()

	tlsArchive, err := utils.ReadTLSArchive(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return tlsArchive.GetKeyPair(constants.OpsCenterKeyPair)
}

// HasCertificateAuthority returns boolean indicating whether the provided package service
// has the OpsCenter certificate authority in it
func HasCertificateAuthority(packages PackageService) (bool, error) {
	envelope, err := packages.ReadPackageEnvelope(loc.OpsCenterCertificateAuthority)
	if err != nil && !trace.IsNotFound(err) {
		return false, trace.Wrap(err)
	}
	return envelope != nil, nil
}

// CreateCAParams combines parameters for creating a CA package
type CreateCAParams struct {
	// Packages is the package service to create the package in
	Packages PackageService
	// KeyPair is the CA certificate / key pair
	KeyPair authority.TLSKeyPair
	// Upsert if true upserts the package
	Upsert bool
}

// CreateCertificateAuthority creates the OpsCenter certificate authority package in the
// provided package service using the provided key pair
func CreateCertificateAuthority(p CreateCAParams) error {
	reader, err := utils.CreateTLSArchive(utils.TLSArchive{
		constants.OpsCenterKeyPair: &p.KeyPair,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	err = p.Packages.UpsertRepository(defaults.SystemAccountOrg, time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}

	if p.Upsert {
		_, err = p.Packages.UpsertPackage(loc.OpsCenterCertificateAuthority, reader)
	} else {
		_, err = p.Packages.CreatePackage(loc.OpsCenterCertificateAuthority, reader)
	}
	return trace.Wrap(err)
}
