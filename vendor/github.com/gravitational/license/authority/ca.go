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

// package authority implements X509 certificate authority features
package authority

import (
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"io/ioutil"
	"time"

	"github.com/gravitational/license/constants"

	"github.com/cloudflare/cfssl/config"
	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	"github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/gravitational/trace"
)

// TLSKeyPair is a pair with TLS private key and certificate
type TLSKeyPair struct {
	// KeyPEM is private key PEM encoded contents
	KeyPEM []byte
	// CertPEM is certificate PEM encoded contents
	CertPEM []byte
}

// NewTLSKeyPair returns a new TLSKeyPair with private key and certificate found
// at the provided paths
func NewTLSKeyPair(keyPath, certPath string) (*TLSKeyPair, error) {
	keyBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &TLSKeyPair{
		KeyPEM:  keyBytes,
		CertPEM: certBytes,
	}, nil
}

// GenerateSelfSignedCA generates self signed certificate authority
func GenerateSelfSignedCA(req csr.CertificateRequest) (*TLSKeyPair, error) {
	if req.KeyRequest == nil {
		req.KeyRequest = &csr.BasicKeyRequest{
			A: constants.TLSKeyAlgo,
			S: constants.TLSKeySize,
		}
	}
	cert, _, key, err := initca.New(&req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &TLSKeyPair{
		KeyPEM:  key,
		CertPEM: cert,
	}, nil
}

// ProcessCSR processes CSR (certificate sign request) with given cert authority
func ProcessCSR(req signer.SignRequest, ttl time.Duration, certAuthority *TLSKeyPair) ([]byte, error) {
	s, err := getSigner(certAuthority, ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := s.Sign(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cert, nil
}

// GenerateCertificate generates a certificate/key pair signed by the provided CA,
// if privateKeyPEM is provided, uses the key instead of generating it
func GenerateCertificate(req csr.CertificateRequest, certAuthority *TLSKeyPair, privateKeyPEM []byte, validFor time.Duration) (*TLSKeyPair, error) {
	return GenerateCertificateWithExtensions(req, certAuthority, privateKeyPEM, validFor, nil)
}

// GenerateCertificateWithExtensions is like GenerateCertificate but allows to specify
// extensions to include into generated certificate
func GenerateCertificateWithExtensions(req csr.CertificateRequest, certAuthority *TLSKeyPair, privateKeyPEM []byte, validFor time.Duration, extensions []signer.Extension) (*TLSKeyPair, error) {
	s, err := getSigner(certAuthority, validFor)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	csrBytes, key, err := GenerateCSR(req, privateKeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var cert []byte
	signRequest := signer.SignRequest{
		Subject: &signer.Subject{
			CN:    req.CN,
			Names: req.Names,
		},
		Request:    string(csrBytes),
		Hosts:      req.Hosts,
		Extensions: extensions,
	}

	cert, err = s.Sign(signRequest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &TLSKeyPair{
		CertPEM: cert,
		KeyPEM:  key,
	}, nil
}

// GenerateCSR generates new certificate signing request for existing key if supplied
// or generates new private key otherwise
func GenerateCSR(req csr.CertificateRequest, privateKeyPEM []byte) (csrBytes []byte, key []byte, err error) {
	generator := &csr.Generator{
		Validator: func(req *csr.CertificateRequest) error {
			return nil
		},
	}
	if len(privateKeyPEM) != 0 {
		existingKey, err := NewExistingKey(privateKeyPEM)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		req.KeyRequest = existingKey
	} else {
		req.KeyRequest = &csr.BasicKeyRequest{
			A: constants.TLSKeyAlgo,
			S: constants.TLSKeySize,
		}
	}

	csrBytes, key, err = generator.ProcessRequest(&req)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return csrBytes, key, nil
}

// getSigner returns signer from TLSKeyPair assuming that keypair is a valid X509 certificate authority
func getSigner(certAuthority *TLSKeyPair, validFor time.Duration) (signer.Signer, error) {
	cert, err := helpers.ParseCertificatePEM(certAuthority.CertPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := helpers.ParsePrivateKeyPEM(certAuthority.KeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	profile := config.DefaultConfig()

	// whitelist our custom extension where encoded license payload will go
	profile.ExtensionWhitelist = map[string]bool{
		constants.LicenseASN1ExtensionID.String(): true,
	}

	// the default profile has 1 year expiration time, override it if it was provided
	if validFor != 0 {
		profile.NotAfter = time.Now().Add(validFor).UTC()
	}

	// set "not before" in the past to alleviate skewed clock issues
	profile.NotBefore = time.Now().Add(-time.Hour).UTC()

	policy := &config.Signing{
		Default: profile,
	}

	return local.NewSigner(key, cert, signer.DefaultSigAlgo(key), policy)
}

// ExistingKey tells signer to use existing key instead
type ExistingKey struct {
	key *rsa.PrivateKey
}

func NewExistingKey(keyPEM []byte) (*ExistingKey, error) {
	key, err := helpers.ParsePrivateKeyPEMWithPassword(keyPEM, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rkey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, trace.BadParameter("only RSA keys supported, got %T", key)
	}
	return &ExistingKey{key: rkey}, nil
}

// Algo returns the requested key algorithm represented as a string.
func (kr *ExistingKey) Algo() string {
	return "rsa"
}

// Size returns the requested key size.
func (kr *ExistingKey) Size() int {
	return kr.key.N.BitLen()
}

// Generate generates a key as specified in the request. Currently,
// only ECDSA and RSA are supported.
func (kr *ExistingKey) Generate() (crypto.PrivateKey, error) {
	return kr.key, nil
}

// SigAlgo returns an appropriate X.509 signature algorithm given the
// key request's type and size.
func (kr *ExistingKey) SigAlgo() x509.SignatureAlgorithm {
	switch {
	case kr.Size() >= 4096:
		return x509.SHA512WithRSA
	case kr.Size() >= 3072:
		return x509.SHA384WithRSA
	case kr.Size() >= 2048:
		return x509.SHA256WithRSA
	default:
		return x509.SHA1WithRSA
	}
}
