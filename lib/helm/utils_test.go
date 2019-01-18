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

package helm

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/gravitational/gravity/lib/defaults"
	check "gopkg.in/check.v1"
)

func TestHelm(t *testing.T) { check.TestingT(t) }

type helmUtilsSuite struct {
	dir string
}

var _ = check.Suite(&helmUtilsSuite{})

func (s *helmUtilsSuite) SetUpSuite(c *check.C) {
	// Prepare some temp. value files.
	s.dir = c.MkDir()
	c.Assert(ioutil.WriteFile(filepath.Join(s.dir, "values1.yaml"),
		valuesFile1, defaults.SharedReadMask), check.IsNil)
	c.Assert(ioutil.WriteFile(filepath.Join(s.dir, "values2.yaml"),
		valuesFile2, defaults.SharedReadMask), check.IsNil)
}

func (s *helmUtilsSuite) TestHasVar(c *check.C) {
	testCases := []struct {
		valueFiles []string
		values     []string
		name       string
		result     bool
		desc       string
	}{
		{
			valueFiles: []string{
				filepath.Join(s.dir, "values1.yaml"),
				filepath.Join(s.dir, "values2.yaml"),
			},
			name:   "image.registry",
			result: true,
			desc:   "Var is present in a values file",
		},
		{
			valueFiles: []string{
				filepath.Join(s.dir, "values2.yaml"),
			},
			name:   "image.registry",
			result: false,
			desc:   "Var is not present",
		},
		{
			valueFiles: []string{
				filepath.Join(s.dir, "values2.yaml"),
			},
			values: []string{
				"image.registry=localhost:5000",
			},
			name:   "image.registry",
			result: true,
			desc:   "Var is present in string values",
		},
	}
	for _, tc := range testCases {
		result, err := HasVar(tc.name, tc.valueFiles, tc.values)
		c.Assert(err, check.IsNil)
		c.Assert(result, check.Equals, tc.result, check.Commentf(
			"Test case %q failed.", tc.desc))
	}
}

func (s *helmUtilsSuite) TestChartFilename(c *check.C) {
	filename := ToChartFilename("alpine", "0.1.0")
	name, version, err := ParseChartFilename(filename)
	c.Assert(err, check.IsNil)
	c.Assert(name, check.Equals, "alpine")
	c.Assert(version, check.Equals, "0.1.0")
}

var (
	valuesFile1 = []byte(`image:
  registry:
    registry.private:5000`)
	valuesFile2 = []byte(`image:
  name:
    alpine:1.0.0`)
)
