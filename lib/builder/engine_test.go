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

package builder

import (
	"testing"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	check "gopkg.in/check.v1"
)

func TestBuilder(t *testing.T) { check.TestingT(t) }

type BuilderSuite struct{}

var _ = check.Suite(&BuilderSuite{})

func (s *BuilderSuite) TestVersionsCompatibility(c *check.C) {
	testCases := []struct {
		teleVer    semver.Version
		runtimeVer semver.Version
		compatible bool
		comment    string
	}{
		{
			teleVer:    *semver.New("5.5.0"),
			runtimeVer: *semver.New("5.5.0"),
			compatible: true,
			comment:    "tele and runtime versions are the same",
		},
		{
			teleVer:    *semver.New("5.5.1"),
			runtimeVer: *semver.New("5.5.0"),
			compatible: true,
			comment:    "tele version is newer than runtime version",
		},
		{
			teleVer:    *semver.New("5.5.0"),
			runtimeVer: *semver.New("5.5.1"),
			compatible: false,
			comment:    "runtime version is newer than tele version",
		},
		{
			teleVer:    *semver.New("5.5.0"),
			runtimeVer: *semver.New("5.4.0"),
			compatible: false,
			comment:    "tele and runtime versions are different releases",
		},
	}
	for _, t := range testCases {
		c.Assert(versionsCompatible(t.teleVer, t.runtimeVer), check.Equals,
			t.compatible, check.Commentf(t.comment))
	}
}

func (s *BuilderSuite) TestSelectRuntimeVersion(c *check.C) {
	b := &Engine{
		Config: Config{
			Progress: utils.DiscardProgress,
		},
	}

	manifest := schema.MustParseManifestYAML([]byte(manifestWithBase))
	ver, err := b.SelectRuntime(&manifest)
	c.Assert(err, check.IsNil)
	c.Assert(ver, check.DeepEquals, semver.New("5.5.0"))

	manifest = schema.MustParseManifestYAML([]byte(manifestWithoutBase))
	version.Init("5.4.2")
	ver, err = b.SelectRuntime(&manifest)
	c.Assert(err, check.IsNil)
	c.Assert(ver, check.DeepEquals, semver.New("5.4.2"))

	manifest = schema.MustParseManifestYAML([]byte(manifestInvalidBase))
	_, err = b.SelectRuntime(&manifest)
	c.Assert(err, check.FitsTypeOf, trace.BadParameter(""))
	c.Assert(err, check.ErrorMatches, "unsupported base image .*")
}

const (
	manifestWithBase = `apiVersion: cluster.gravitational.io/v2
kind: Cluster
baseImage: gravity:5.5.0
metadata:
  name: test
  resourceVersion: 1.0.0`

	manifestWithoutBase = `apiVersion: cluster.gravitational.io/v2
kind: Cluster
metadata:
  name: test
  resourceVersion: 1.0.0`

	manifestInvalidBase = `apiVersion: cluster.gravitational.io/v2
kind: Cluster
baseImage: example:1.2.3
metadata:
  name: test
  resourceVersion: 1.0.0`
)
