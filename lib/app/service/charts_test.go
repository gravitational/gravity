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

package service

import (
	"io/ioutil"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/ghodss/yaml"
	check "gopkg.in/check.v1"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/repo"
)

type chartsSuite struct {
	backend storage.Backend
	pack    pack.PackageService
	apps    *applications
}

var _ = check.Suite(&chartsSuite{})

func (s *chartsSuite) SetUpTest(c *check.C) {
	s.backend, s.pack, s.apps = setupServices(c)
}

func (s *chartsSuite) TestIndex(c *check.C) {
	// Initially the index file is empty.
	index := s.getIndex(c)
	c.Assert(len(index.Entries), check.Equals, 0)

	// Create a Helm-based application...
	alpine := loc.MustParseLocator("gravitational.io/alpine:0.1.0")
	test.CreateHelmChartApp(c, s.apps, alpine)

	// ... and verify the index was updated.
	index = s.getIndex(c)
	c.Assert(len(index.Entries), check.Equals, 1)
	c.Assert(index.Has(alpine.Name, alpine.Version), check.Equals, true)

	// Create another one...
	nginx := loc.MustParseLocator("gravitational.io/nginx:0.2.0")
	test.CreateHelmChartApp(c, s.apps, nginx)

	// .. and verify it's also in the index.
	index = s.getIndex(c)
	c.Assert(len(index.Entries), check.Equals, 2)
	c.Assert(index.Has(alpine.Name, alpine.Version), check.Equals, true)
	c.Assert(index.Has(nginx.Name, nginx.Version), check.Equals, true)

	// Now remove one of them...
	err := s.apps.DeleteApp(app.DeleteRequest{Package: alpine})
	c.Assert(err, check.IsNil)

	// ... and make sure it's gone from the index.
	index = s.getIndex(c)
	c.Assert(len(index.Entries), check.Equals, 1)
	c.Assert(index.Has(nginx.Name, nginx.Version), check.Equals, true)

	// Remove the second one as well...
	err = s.apps.DeleteApp(app.DeleteRequest{Package: nginx})
	c.Assert(err, check.IsNil)

	// ... and verify the index is empty again.
	index = s.getIndex(c)
	c.Assert(len(index.Entries), check.Equals, 0)
}

func (s *chartsSuite) TestRebuildIndex(c *check.C) {
	// Create a couple of Helm-based applications.
	alpine := loc.MustParseLocator("gravitational.io/alpine:0.1.0")
	test.CreateHelmChartApp(c, s.apps, alpine)
	nginx := loc.MustParseLocator("gravitational.io/nginx:0.2.0")
	test.CreateHelmChartApp(c, s.apps, nginx)

	// Corrupt the index in the backend (insert an empty one).
	emptyIndex := repo.NewIndexFile()
	err := s.backend.UpsertIndexFile(*emptyIndex)
	c.Assert(err, check.IsNil)

	// Now rebuild the index.
	err = s.apps.Charts.RebuildIndex()
	c.Assert(err, check.IsNil)

	// Verify it contains both apps.
	index := s.getIndex(c)
	c.Assert(len(index.Entries), check.Equals, 2)
	c.Assert(index.Has(alpine.Name, alpine.Version), check.Equals, true)
	c.Assert(index.Has(nginx.Name, nginx.Version), check.Equals, true)
}

func (s *chartsSuite) TestFetchChart(c *check.C) {
	// Create a test Helm-based application.
	alpine := loc.MustParseLocator("gravitational.io/alpine:0.1.0")
	test.CreateHelmChartApp(c, s.apps, alpine)

	// Fetch it as a chart archive.
	reader, err := s.apps.FetchChart(alpine)
	c.Assert(err, check.IsNil)
	defer reader.Close()

	// Load the chart archive to make sure it's valid and verify some details.
	chart, err := loader.LoadArchive(reader)
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, chart, test.Chart(alpine))
}

// getIndex returns the suite's app service's chart repo index file.
//
// The index file can also be retrieved directly from the backend in the
// proper format (as repo.IndexFile object) but this way also allows to
// test the GetIndexFile method.
func (s *chartsSuite) getIndex(c *check.C) repo.IndexFile {
	reader, err := s.apps.FetchIndexFile()
	c.Assert(err, check.IsNil)
	indexFileBytes, err := ioutil.ReadAll(reader)
	c.Assert(err, check.IsNil)
	var indexFile repo.IndexFile
	err = yaml.Unmarshal(indexFileBytes, &indexFile)
	c.Assert(err, check.IsNil)
	return indexFile
}
