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

package catalog

import (
	"testing"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage"

	check "gopkg.in/check.v1"
)

func TestCatalog(t *testing.T) { check.TestingT(t) }

type catalogSuite struct {
	services opsservice.TestServices
	catalog  Catalog
	alpine   *app.Application
	nginx    *app.Application
}

var _ = check.Suite(&catalogSuite{})

func (s *catalogSuite) SetUpSuite(c *check.C) {
	s.services = opsservice.SetupTestServices(c)
	var err error
	s.catalog, err = New(Config{
		Name:     "test-catalog",
		Operator: s.services.Operator,
		Apps:     s.services.Apps,
	})
	c.Assert(err, check.IsNil)

	// Prepare the backend.
	_, err = s.services.Backend.CreateAccount(storage.Account{
		ID:  defaults.SystemAccountID,
		Org: defaults.SystemAccountOrg,
	})
	c.Assert(err, check.IsNil)

	// Setup a couple of Helm-based applications.
	s.alpine = test.CreateHelmChartApp(c, s.services.Apps, loc.MustParseLocator(
		"gravitational.io/alpine:0.1.0"))
	s.nginx = test.CreateHelmChartApp(c, s.services.Apps, loc.MustParseLocator(
		"gravitational.io/nginx:0.2.0"))
}

func (s *catalogSuite) TestSearch(c *check.C) {
	testCases := []struct {
		pattern string
		result  []app.Application
		desc    string
	}{
		{
			pattern: "",
			result:  []app.Application{*s.alpine, *s.nginx},
			desc:    "Should find both apps with empty pattern",
		},
		{
			pattern: "alp",
			result:  []app.Application{*s.alpine},
			desc:    "Should find alpine only",
		},
		{
			pattern: "nginx",
			result:  []app.Application{*s.nginx},
			desc:    "Should find nginx only",
		},
		{
			pattern: "in",
			result:  []app.Application{*s.alpine, *s.nginx},
			desc:    "Should find both apps by pattern",
		},
		{
			pattern: "kafka",
			result:  nil,
			desc:    "Should find nothing",
		},
	}
	for _, tc := range testCases {
		result, err := s.catalog.Search(tc.pattern)
		c.Assert(err, check.IsNil,
			check.Commentf("Test case %q failed", tc.desc))
		c.Assert(tc.result, compare.DeepEquals, result,
			check.Commentf("Test case %q failed", tc.desc))
	}
}

func (s *catalogSuite) TestDownload(c *check.C) {
	// Download the alpine application.
	reader, err := s.catalog.Download(s.alpine.Package.Name, s.alpine.Package.Version)
	c.Assert(err, check.IsNil)
	defer reader.Close()

	// Unpack the tarball.
	unpackedDir := c.MkDir()
	err = archive.Extract(reader, unpackedDir)
	c.Assert(err, check.IsNil)

	// Create the local env using unpacked tarball as a state directory.
	env, err := localenv.New(unpackedDir)
	c.Assert(err, check.IsNil)
	defer env.Close()

	// Find the alpine application and make sure it's valid.
	alpine, err := env.Apps.GetApp(s.alpine.Package)
	c.Assert(err, check.IsNil)
	// Exclude "created" timestamp from comparison b/c apps are in
	// different package services so it will be different for them.
	alpine.PackageEnvelope.Created = s.alpine.PackageEnvelope.Created
	c.Assert(alpine, compare.DeepEquals, s.alpine)
}
