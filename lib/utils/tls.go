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

package utils

import (
	"archive/tar"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"

	cfsslhelpers "github.com/cloudflare/cfssl/helpers"
	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/license/authority"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

const (
	SelfSignedCertOrg   = "Gravitational"
	PemBlockCertificate = "CERTIFICATE"
)

// CertificateOutput contains information about cluster certificate
type CertificateOutput struct {
	// IssuedTo contains  certificate subject
	IssuedTo CertificateName `json:"issued_to"`
	// IssuedBy contains  certificate issuer
	IssuedBy CertificateName `json:"issued_by"`
	// Validity contains certificate validity dates
	Validity CertificateValidity `json:"validity"`
}

// CertificateName contains information about certificate subject/issuer
type CertificateName struct {
	// CommonName is the certificate common name
	CommonName string `json:"cn"`
	// Organization is the subject/issuer organization
	Organization []string `json:"org"`
	// OrganizationalUnit is the subject/issuer unit
	OrganizationalUnit []string `json:"org_unit"`
}

// GenerateSelfSignedCert generates a self signed certificate that
// is valid for given domain names and ips, returns PEM-encoded bytes with key and cert
// Generates a certificate that is compatible with the MacOS requirements described at:
// https://support.apple.com/en-us/HT210176
func GenerateSelfSignedCert(hostNames []string) (*teleutils.TLSCredentials, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(time.Hour * 24 * 825) // 825 days or fewer is the required validity period for MacOS

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	entity := pkix.Name{
		CommonName:   "localhost",
		Country:      []string{"US"},
		Organization: []string{"localhost"},
		// OrganizationalUnit is needed in order to be able to identify the cert when doing
		// automated cert rotation. If a user decides to use their own cert
		// we should not rotate.
		OrganizationalUnit: []string{SelfSignedCertOrg},
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Issuer:                entity,
		Subject:               entity,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, // MacOS specific requirement
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	// collect IP addresses localhost resolves to and add them to the cert. template:
	template.DNSNames = append(hostNames, "localhost.local")
	ips, _ := net.LookupIP("localhost")
	if ips != nil {
		template.IPAddresses = ips
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(priv.Public())
	if err != nil {
		logrus.WithError(err).Warn("Failed to marshal PKI public key.")
		return nil, trace.Wrap(err)
	}

	return &teleutils.TLSCredentials{
		PublicKey:  pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: publicKeyBytes}),
		PrivateKey: pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}),
		Cert:       pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes}),
	}, nil
}

// CertificateValidity contains information about certificate validity dates
type CertificateValidity struct {
	// NotBefore is the issue date
	NotBefore time.Time `json:"not_before"`
	// NotAfter is the expiration date
	NotAfter time.Time `json:"not_after"`
}

// ParseCertificate parses the provided data as PEM-formatted x509 certificate
// (or chain) and returns a web-UI-friendly representation of it
func ParseCertificate(data []byte) (*CertificateOutput, error) {
	certificates, err := cfsslhelpers.ParseCertificatesPEM(data)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse certificate PEM")
	}
	if len(certificates) < 1 {
		return nil, trace.BadParameter("failed to parse certificate")
	}
	certificate := certificates[0]

	return &CertificateOutput{
		IssuedTo: CertificateName{
			CommonName:         certificate.Subject.CommonName,
			Organization:       certificate.Subject.Organization,
			OrganizationalUnit: certificate.Subject.OrganizationalUnit,
		},
		IssuedBy: CertificateName{
			CommonName:         certificate.Issuer.CommonName,
			Organization:       certificate.Issuer.Organization,
			OrganizationalUnit: certificate.Issuer.OrganizationalUnit,
		},
		Validity: CertificateValidity{
			NotBefore: certificate.NotBefore,
			NotAfter:  certificate.NotAfter,
		},
	}, nil
}

// TLSArchive designed to store a set of keypairs following a special
// naming convention, where every keypair has a name and they are serialized
// using extension ".cert" and extension ".key" convention
type TLSArchive map[string]*authority.TLSKeyPair

// CreateTLSArchive creates archive with TLS keypairs, where keys are stored with extension ".key"
// and certificates are stored with extension ".cert"
func CreateTLSArchive(a TLSArchive) (io.ReadCloser, error) {
	items := make([]*archive.Item, 0, len(a)*2)
	for name, keyPair := range a {
		if len(keyPair.KeyPEM) != 0 {
			items = append(items, archive.ItemFromStringMode(
				fmt.Sprintf("%v.%v", name, KeySuffix),
				string(keyPair.KeyPEM),
				defaults.GroupReadMask,
			))
		}
		if len(keyPair.CertPEM) != 0 {
			items = append(items, archive.ItemFromStringMode(
				fmt.Sprintf("%v.%v", name, CertSuffix),
				string(keyPair.CertPEM),
				defaults.SharedReadMask,
			))
		}
	}
	archive, err := archive.CreateMemArchive(items)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ioutil.NopCloser(archive), nil
}

// ReadTLSArchive reads TLS packed archive, where keys are stored with extension ".key"
// and certificates are stored with extension ".cert"
func ReadTLSArchive(source io.Reader) (TLSArchive, error) {
	decompressed, err := dockerarchive.DecompressStream(source)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyPairs := make(TLSArchive)
	reader := tar.NewReader(decompressed)
	for {
		var hdr *tar.Header
		hdr, err = reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		_, fileName := filepath.Split(hdr.Name)
		parts := strings.SplitN(fileName, ".", 2)
		if len(parts) != 2 {
			continue
		}
		name, ext := parts[0], parts[1]
		if ext != CertSuffix && ext != KeySuffix {
			continue
		}
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		keyPair, ok := keyPairs[name]
		if !ok {
			keyPair = &authority.TLSKeyPair{}
			keyPairs[name] = keyPair
		}
		if ext == CertSuffix {
			keyPair.CertPEM = data
		} else {
			keyPair.KeyPEM = data
		}
	}
	return keyPairs, nil
}

// AddKeyPair adds TLSArchiveKeyPair to archive
func (ta TLSArchive) AddKeyPair(name string, kp authority.TLSKeyPair) error {
	if name == "" {
		return trace.BadParameter("missing key pair name")
	}
	if _, err := ta.GetKeyPair(name); err == nil {
		return trace.AlreadyExists("key pair %v already exists", name)
	}
	ta[name] = &kp
	return nil
}

// GetKeyPair returns KeyPair by name
func (ta TLSArchive) GetKeyPair(name string) (*authority.TLSKeyPair, error) {
	keyPair, ok := ta[name]
	if !ok {
		return nil, trace.NotFound("archive key pair %v is not found", name)
	}
	return keyPair, nil
}

const (
	// KeySuffix is the standard extension used for x509 key files generated by gravity
	KeySuffix = "key"
	// CertSuffix is the standard extension used for x509 cert files generated by gravity
	CertSuffix = "cert"
)
