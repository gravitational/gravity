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

package service

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"

	. "gopkg.in/check.v1"
)

type VendorSuite struct{}

var _ = Suite(&VendorSuite{})

func (s *VendorSuite) TestRewriteManifestMetadata(c *C) {
	rFiles := createResourceFile("testmetadata", manifestWithMetadata, c)
	// 1st pass: rewrite package details
	target := loc.Locator{Repository: "gravitational.io", Name: "n1", Version: "7.7.7"}
	err := rFiles.RewriteManifest(func(m *schema.Manifest) error {
		m.Metadata.Repository = target.Repository
		m.Metadata.Name = target.Name
		m.Metadata.ResourceVersion = target.Version
		return nil
	})
	c.Assert(err, IsNil)

	var out loc.Locator
	// 2st pass: check the result
	err = rFiles.RewriteManifest(func(m *schema.Manifest) error {
		out.Repository = m.Metadata.Repository
		out.Name = m.Metadata.Name
		out.Version = m.Metadata.ResourceVersion
		return nil
	})
	c.Assert(err, IsNil)

	c.Assert(out, DeepEquals, target)
}

func (s *VendorSuite) TestRewriteDeps(c *C) {
	rFiles := createResourceFile("testdeps", manifestWithDeps, c)
	deps := []loc.Locator{
		loc.Locator{Repository: "gravitational.io", Name: "gravity", Version: "0.0.2"},
		loc.Locator{Repository: "gravitational.io", Name: "site", Version: "0.0.3"},
		loc.Locator{Repository: "gravitational.io", Name: "k8s-aws", Version: "0.0.30-cdef12.130"},
	}

	// 1st pass: rewrite all deps
	c.Assert(rFiles.RewriteManifest(makeRewriteDepsFunc(deps)), IsNil)

	var locators []loc.Locator
	// 2st pass: collect all deps and check the result
	err := rFiles.RewriteManifest(func(m *schema.Manifest) error {
		for _, dep := range m.Dependencies.Packages {
			locators = append(locators, dep.Locator)
		}
		for _, dep := range m.Dependencies.Apps {
			locators = append(locators, dep.Locator)
		}
		base := m.Base()
		if base != nil {
			locators = append(locators, *base)
		}
		return nil
	})
	c.Assert(err, IsNil)

	c.Assert(locators, DeepEquals, deps)
}

func (s *VendorSuite) TestRewitePackagesMetadata(c *C) {
	rFiles := createResourceFile("testmeta", manifestWithPackagesMetadata, c)

	packages, _ := setupServices(c)

	err := packages.UpsertRepository("gravitational.io", time.Time{})
	c.Assert(err, IsNil)

	locators := []string{
		"gravitational.io/k8s-aws:1.0.0",
		"gravitational.io/k8s-aws:1.0.1",
		"gravitational.io/gravity:2.0.0",
		"gravitational.io/site:3.0.0",
	}
	for _, l := range locators {
		_, err := packages.CreatePackage(loc.MustParseLocator(l), bytes.NewBuffer([]byte("data")))
		c.Assert(err, IsNil)
	}

	err = rFiles.RewriteManifest(makeRewritePackagesMetadataFunc(packages))
	c.Assert(err, IsNil)

	// collect rewritten locators and check them
	var result []string
	err = rFiles.RewriteManifest(func(m *schema.Manifest) error {
		base := m.Base()
		if base != nil {
			result = append(result, base.String())
		}
		for _, dep := range m.Dependencies.Packages {
			result = append(result, dep.Locator.String())
		}
		for _, dep := range m.Dependencies.Apps {
			result = append(result, dep.Locator.String())
		}
		return nil
	})
	c.Assert(err, IsNil)
	c.Assert(result, DeepEquals, []string{
		"gravitational.io/k8s-aws:1.0.1",
		"gravitational.io/gravity:2.0.0",
		"gravitational.io/site:3.0.0",
	})
}

func (*VendorSuite) TestGeneratesProperPackageNames(c *C) {
	var testCases = []struct {
		image     string
		validator func(loc loc.Locator, generated map[string]struct{}) bool
		comment   string
	}{
		{
			image: "planet-gpu:1.0.0",
			validator: func(loc loc.Locator, generated map[string]struct{}) bool {
				return "planet-gpu" == loc.Name
			},
			comment: "package name equals image name",
		},
		{
			image: "planet-master:1.0.0",
			validator: func(loc loc.Locator, generated map[string]struct{}) bool {
				return loc.Name != "planet-master" &&
					strings.HasPrefix(loc.Name, "planet-master")
			},
			comment: "avoids collision with a legacy package name",
		},
		{
			image: "planet-master:2.0.0",
			validator: func(loc loc.Locator, generated map[string]struct{}) bool {
				_, exists := generated[loc.Name]
				return loc.Name != "planet-master" &&
					strings.HasPrefix(loc.Name, "planet-master") && !exists
			},
			comment: "gets a unique package name",
		},
		{
			image: "repo-a/image:2.0.0",
			validator: func(loc loc.Locator, generated map[string]struct{}) bool {
				return "repo-a-image" == loc.Name
			},
			comment: "can handle image names with repository",
		},
		{
			image: "repo-b/image:2.0.0",
			validator: func(loc loc.Locator, generated map[string]struct{}) bool {
				return "repo-b-image" == loc.Name
			},
			comment: "gets a unique package name with a respository",
		},
		{
			image: "repo.io/subrepo/image:2.0.0",
			validator: func(loc loc.Locator, generated map[string]struct{}) bool {
				return "repo.io-subrepo-image" == loc.Name
			},
			comment: "can handle images with nested paths",
		},
	}

	imageToPackage := make(map[string]loc.Locator)
	newName := newRuntimePackage(imageToPackage, nil)
	generated := make(map[string]struct{})
	for _, testCase := range testCases {
		loc, err := newName(testCase.image)
		comment := Commentf(testCase.comment)
		c.Assert(err, IsNil, comment)
		if !testCase.validator(*loc, generated) {
			c.Errorf("Failed to validate result %v (%v).", loc, testCase.comment)
		}
	}
}

func createResourceFile(path, manifest string, c *C) resources.ResourceFiles {
	dir := c.MkDir()
	fileName := filepath.Join(dir, path)

	err := ioutil.WriteFile(fileName, []byte(manifest), defaults.PrivateFileMask)
	c.Assert(err, IsNil)

	var rFiles resources.ResourceFiles
	rFile, err := resources.NewResourceFile(fileName)
	c.Assert(err, IsNil)
	rFiles = append(rFiles, *rFile)
	return rFiles
}

const manifestWithMetadata = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  repository: gravitational.io
  namespace: kube-system
  name: k8s-aws
  resourceVersion: "1.2.3-1"`

const manifestWithDeps = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  repository: gravitational.io
  namespace: kube-system
  name: dns-app
  resourceVersion: "0.0.1"
systemOptions:
  runtime:
    name: k8s-aws
    version: 0.0.30-afbd71.130
dependencies:
  packages:
  - gravitational.io/gravity:0.0.1
  apps:
  - gravitational.io/site:0.0.1`

const manifestWithPackagesMetadata = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  repository: gravitational.io
  namespace: kube-system
  name: dns-app
  resourceVersion: "0.0.1"
systemOptions:
  runtime:
    name: k8s-aws
    version: 0.0.0+latest
dependencies:
  packages:
  - gravitational.io/gravity:0.0.0+latest
  apps:
  - gravitational.io/site:0.0.0+latest`
