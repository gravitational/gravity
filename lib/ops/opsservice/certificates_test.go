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
