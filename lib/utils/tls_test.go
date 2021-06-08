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
	"crypto/x509"
	"encoding/pem"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license/authority"
	. "gopkg.in/check.v1"
)

type TLSSuite struct{}

var _ = Suite(&TLSSuite{})

func (s *TLSSuite) TestCertArchive(c *C) {
	ca, err := authority.GenerateSelfSignedCA(csr.CertificateRequest{
		CN: "cluster.local",
	})
	c.Assert(err, IsNil)
	c.Assert(ca, NotNil)

	keyPair, err := authority.GenerateCertificate(csr.CertificateRequest{
		CN:    "apiserver",
		Hosts: []string{"127.0.0.1"},
		Names: []csr.Name{
			{
				O:  "Gravitational",
				OU: "Local Cluster",
			},
		},
	}, ca, nil, 0)

	c.Assert(err, IsNil)
	c.Assert(keyPair, NotNil)

	certOnly := *ca
	certOnly.KeyPEM = nil

	reader, err := CreateTLSArchive(TLSArchive{
		"apiserver": keyPair,
		"root":      &certOnly,
	})
	c.Assert(err, IsNil)
	defer reader.Close()

	archive, err := ReadTLSArchive(reader)
	c.Assert(err, IsNil)
	c.Assert(archive, NotNil)

	ocertOnly, err := archive.GetKeyPair("root")
	c.Assert(err, IsNil)
	c.Assert(string(ocertOnly.CertPEM), Equals, string(certOnly.CertPEM))
	c.Assert(ocertOnly.KeyPEM, IsNil)

	okeyPair, err := archive.GetKeyPair("apiserver")
	c.Assert(err, IsNil)
	c.Assert(string(okeyPair.CertPEM), Equals, string(keyPair.CertPEM))
	c.Assert(string(okeyPair.KeyPEM), Equals, string(keyPair.KeyPEM))
}

// TestGenerateSelfSignedCert verifies that MacOS specific requirements are met according to:
// https://support.apple.com/en-us/HT210176
func (s *TLSSuite) TestGenerateSelfSignedCert(c *C) {
	hostNames := []string{"localhost"}
	credentials, err := GenerateSelfSignedCert(hostNames)
	c.Assert(err, IsNil)

	block, _ := pem.Decode(credentials.Cert)
	c.Assert(block, NotNil)

	cert, err := x509.ParseCertificate(block.Bytes)
	c.Assert(err, IsNil)

	validityPeriod := int(cert.NotAfter.Sub(cert.NotBefore).Hours() / 24)
	c.Assert(validityPeriod, Equals, 825)

	testExtKeyUsage := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	c.Assert(cert.ExtKeyUsage, DeepEquals, testExtKeyUsage)
}
