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
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"time"

	"github.com/gravitational/license/authority"
	"github.com/gravitational/license/constants"

	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/signer"
	"github.com/gravitational/trace"
)

func newCertificate(data NewLicenseInfo) ([]byte, error) {
	private, err := rsa.GenerateKey(rand.Reader, constants.LicenseKeyBits)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privatePEM := pem.EncodeToMemory(&pem.Block{
		Type:  constants.RSAPrivateKeyPEMBlock,
		Bytes: x509.MarshalPKCS1PrivateKey(private),
	})

	// encrypt encryption key
	var encryptedKey []byte
	if len(data.EncryptionKey) != 0 {
		encryptedKey, err = rsa.EncryptOAEP(sha256.New(), rand.Reader,
			private.Public().(*rsa.PublicKey), data.EncryptionKey, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	payload := Payload{
		MaxNodes:       data.MaxNodes,
		Expiration:     time.Now().UTC().Add(data.ValidFor),
		Shutdown:       data.StopApp,
		Person:         data.CustomerName,
		Email:          data.CustomerEmail,
		Metadata:       data.CustomerMetadata,
		ProductName:    data.ProductName,
		ProductVersion: data.ProductVersion,
		EncryptionKey:  encryptedKey,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// make an extension to encode into certificate
	extensions := []signer.Extension{{
		ID:    config.OID(constants.LicenseASN1ExtensionID),
		Value: hex.EncodeToString(bytes),
	}}

	req := csr.CertificateRequest{
		CN:    constants.LicenseKeyPair,
		Hosts: []string{constants.LoopbackIP},
		Names: []csr.Name{{
			O: constants.LicenseOrg,
		}},
	}

	// generate certificate signed by the provided certificate authority
	tlsKeyPair, err := authority.GenerateCertificateWithExtensions(
		req, &data.TLSKeyPair, privatePEM, data.ValidFor, extensions)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return append(tlsKeyPair.CertPEM, tlsKeyPair.KeyPEM...), nil
}

// parseCertificate parses the provided license string in PEM format
func parseCertificate(certPEM string) (License, error) {
	// parse the certificate and private key
	certificateBytes, privateBytes, err := parseCertificatePEM(certPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certificate, err := x509.ParseCertificate(certificateBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// parse the extension
	var p *Payload
	for _, ext := range certificate.Extensions {
		if ext.Id.Equal(constants.LicenseASN1ExtensionID) {
			p = new(Payload)
			if err := json.Unmarshal(ext.Value, p); err != nil {
				return nil, trace.Wrap(err)
			}
			break
		}
	}
	if p == nil {
		return nil, trace.BadParameter("could not find payload extension")
	}

	// decrypt encryption key
	if len(p.EncryptionKey) != 0 {
		private, err := x509.ParsePKCS1PrivateKey(privateBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		p.EncryptionKey, err = rsa.DecryptOAEP(sha256.New(), rand.Reader,
			private, p.EncryptionKey, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return &license{
		certificate: certificate,
		payload:     *p,
	}, nil
}

// parseCertificatePEM parses the concatenated certificate/private key in PEM format
// and returns certificate and private key in decoded DER ASN.1 structure
func parseCertificatePEM(certPEM string) ([]byte, []byte, error) {
	var certificateBytes, privateBytes []byte
	block, rest := pem.Decode([]byte(certPEM))
	for block != nil {
		switch block.Type {
		case constants.CertificatePEMBlock:
			certificateBytes = block.Bytes
		case constants.RSAPrivateKeyPEMBlock:
			privateBytes = block.Bytes
		}
		// parse the next block
		block, rest = pem.Decode(rest)
	}
	if len(certificateBytes) == 0 || len(privateBytes) == 0 {
		return nil, nil, trace.BadParameter("could not parse the license")
	}
	return certificateBytes, privateBytes, nil
}

// license represents gravity license.
type license struct {
	certificate *x509.Certificate
	payload     Payload
}

// Verify makes sure the certificate is valid.
func (l *license) Verify(caPEM []byte) error {
	roots := x509.NewCertPool()

	// add the provided CA certificate to the roots
	ok := roots.AppendCertsFromPEM(caPEM)
	if !ok {
		return trace.BadParameter("could not find any CA certificates")
	}

	_, err := l.certificate.Verify(x509.VerifyOptions{Roots: roots})
	if err != nil {
		certErr, ok := err.(x509.CertificateInvalidError)
		if ok && certErr.Reason == x509.Expired {
			return trace.BadParameter("the license has expired")
		}
		return trace.Wrap(err, "failed to verify the license")
	}

	return nil
}

// GetPayload returns payload.
func (l *license) GetPayload() Payload {
	return l.payload
}
