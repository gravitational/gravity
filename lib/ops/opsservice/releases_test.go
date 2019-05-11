/*
Copyright 2019 Gravitational, Inc.

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
	"time"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

type ReleasesSuite struct {
	services     TestServices
	localCluster *storage.Site
}

var _ = check.Suite(&ReleasesSuite{})

func (s *ReleasesSuite) SetUpSuite(c *check.C) {
	s.services = SetupTestServices(c)
	// Setup local cluster.
	var err error
	s.localCluster, err = s.services.Backend.CreateSite(storage.Site{
		AccountID: defaults.SystemAccountID,
		Domain:    "example.com",
		Local:     true,
		App: storage.Package{
			Repository: defaults.SystemAccountOrg,
			Name:       "example",
			Version:    "0.0.1",
		},
		DNSConfig: storage.DNSConfig{
			Addrs: []string{"127.0.0.42"},
			Port:  54,
		},
		Created: time.Now(),
	})
	c.Assert(err, check.IsNil)
}

func (s *ReleasesSuite) TestListReleases(c *check.C) {
	// Prepare a couple of releases.
	release1, err := s.services.HelmClient.Install(helm.InstallParameters{Name: "release1"})
	c.Assert(err, check.IsNil)
	release2, err := s.services.HelmClient.Install(helm.InstallParameters{Name: "release2"})
	c.Assert(err, check.IsNil)

	// Make sure they're returned.
	releases, err := s.services.Operator.ListReleases(ops.ListReleasesRequest{
		SiteKey: ops.SiteKey{
			AccountID:  s.localCluster.AccountID,
			SiteDomain: s.localCluster.Domain,
		},
	})
	c.Assert(err, check.IsNil)
	c.Assert(releases, compare.DeepEquals, []storage.Release{release1, release2})
}
