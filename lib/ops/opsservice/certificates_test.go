package opsservice

import (
	"os"
	"strconv"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

type CertificatesSuite struct{}

var _ = check.Suite(&CertificatesSuite{})

func (s *CertificatesSuite) SetUpTest(c *check.C) {
	testEnabled := os.Getenv(defaults.TestK8s)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		c.Skip("skipping Kubernetes test")
	}
}

func (s *CertificatesSuite) TestCertificates(c *check.C) {
	client, err := utils.GetLocalKubeClient()
	c.Assert(err, check.IsNil)

	cert, err := teleutils.GenerateSelfSignedCert([]string{"test.localhost"})
	c.Assert(err, check.IsNil)

	// key valid, cert invalid
	err = UpdateClusterCertificate(client, ops.UpdateCertificateRequest{
		AccountID:   defaults.SystemAccountID,
		SiteDomain:  defaults.SystemAccountOrg,
		Certificate: []byte("invalid"),
		PrivateKey:  cert.PrivateKey,
	})
	c.Assert(trace.IsBadParameter(err), check.Equals, true)

	// cert valid, key invalid
	err = UpdateClusterCertificate(client, ops.UpdateCertificateRequest{
		AccountID:   defaults.SystemAccountID,
		SiteDomain:  defaults.SystemAccountOrg,
		Certificate: cert.Cert,
		PrivateKey:  []byte("invalid"),
	})
	c.Assert(trace.IsBadParameter(err), check.Equals, true)

	// ok
	err = UpdateClusterCertificate(client, ops.UpdateCertificateRequest{
		AccountID:   defaults.SystemAccountID,
		SiteDomain:  defaults.SystemAccountOrg,
		Certificate: cert.Cert,
		PrivateKey:  cert.PrivateKey,
	})
	c.Assert(err, check.IsNil)

	certBytes, keyBytes, err := GetClusterCertificate(client)
	c.Assert(err, check.IsNil)
	c.Assert(certBytes, check.DeepEquals, cert.Cert)
	c.Assert(keyBytes, check.DeepEquals, cert.PrivateKey)
}
