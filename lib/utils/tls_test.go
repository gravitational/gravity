package utils

import (
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
