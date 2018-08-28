/*
Copyright 2017 Gravitational, Inc.

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

package license

import (
	"strings"
	"time"

	"github.com/gravitational/license/authority"

	"github.com/gravitational/trace"
)

// License defines a common interface to support both gravity-generated and
// customer licenses.
type License interface {
	// Verify verifies the license is valid.
	Verify(caPEM []byte) error
	// GetPayload returns various details encoded into licenses.
	GetPayload() Payload
}

// NewLicenseInfo encapsulates fields needed to generate a license
type NewLicenseInfo struct {
	// MaxNodes is maximum number of nodes the license allows
	MaxNodes int
	// ValidFor is validity period for the license
	ValidFor time.Duration
	// StopApp indicates whether the app should be stopped when the license expires
	StopApp bool
	// CustomerName is the name of the customer the license is generated for
	CustomerName string
	// CustomerName is the email of the customer the license is generated for
	CustomerEmail string
	// CustomerMetadata is arbitrary metadata to add to the license
	CustomerMetadata string
	// ProductName is the name of the product the license is for
	ProductName string
	// ProductVersion is product version the license is for
	ProductVersion string
	// EncryptionKey is the passphrase for decoding encrypted packages
	EncryptionKey []byte
	// TLSKeyPair is the certificate authority to sign the license with
	TLSKeyPair authority.TLSKeyPair
}

// Check checks the new license request
func (i NewLicenseInfo) Check() error {
	if i.MaxNodes < 1 {
		return trace.BadParameter("maximum number of servers should be 1 or more")
	}
	if time.Now().Add(i.ValidFor).Before(time.Now()) {
		return trace.BadParameter("expiration date can't be in the past")
	}
	if len(i.TLSKeyPair.CertPEM) == 0 {
		return trace.BadParameter("certificate authority should be provided")
	}
	return nil
}

// NewLicense generates a new license according to the provided request.
func NewLicense(info NewLicenseInfo) (string, error) {
	err := info.Check()
	if err != nil {
		return "", trace.Wrap(err)
	}
	certificateBytes, err := newCertificate(info)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(certificateBytes), nil
}

// ParseLicense tries to detect the type of the provided license and parse it.
func ParseLicense(license string) (License, error) {
	if license == "" {
		return Payload{}, nil
	}
	if strings.HasPrefix(license, "{") {
		return parsePayload(license)
	}
	return parseCertificate(license)
}

// ParseLicenseByType tries to parse the provided license string as a license of
// the specified type.
func ParseLicenseByType(license, type_ string) (License, error) {
	switch type_ {
	case LicenseTypePayload:
		return parsePayload(license)
	case LicenseTypeCertificate:
		return parseCertificate(license)
	default:
		return nil, trace.BadParameter("unknown license type: %v", type_)
	}
}

const (
	// LicenseTypeCertificate means that the license is a x509 certificate with encoded payload,
	// this is the license normally used by deployments that rely on our own license
	LicenseTypeCertificate = "certificate"
	// LicenseTypePayload means that the license is the the payload itself with a signature,
	// this is used by some vendors
	LicenseTypePayload = "payload"
)
