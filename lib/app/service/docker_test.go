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
	"fmt"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/loc"
	. "gopkg.in/check.v1"
)

func (s *VendorSuite) TestGeneratesProperPackageName(c *C) {
	var testCases = []struct {
		image        string
		result       loc.Locator
		visited      map[string]loc.Locator
		randomSuffix func(string) string
		comment      string
	}{
		{
			image:   "foo:5.1.0",
			result:  loc.MustParseLocator("gravitational.io/foo:5.1.0"),
			comment: "image reference w/o repository",
		},
		{
			image:   "repo/foo:1.0.0",
			result:  loc.MustParseLocator("gravitational.io/repo-foo:1.0.0"),
			comment: "image reference with repository",
		},
		{
			image:   "repo.io/subrepo/foo:0.0.1",
			result:  loc.MustParseLocator("gravitational.io/repo.io-subrepo-foo:0.0.1"),
			comment: "nested repositories",
		},
		{
			image:   "repo.io:123/subrepo/foo:0.0.1",
			result:  loc.MustParseLocator("gravitational.io/repo.io-123-subrepo-foo:0.0.1"),
			comment: "repository with a port",
		},
		{
			image:  "repo.io:123/subrepo/foo:0.0.1",
			result: loc.MustParseLocator("foo/bar:0.0.1"),
			visited: map[string]loc.Locator{
				"repo.io:123/subrepo/foo:0.0.1": loc.MustParseLocator("foo/bar:0.0.1"),
			},
			comment: "uses cached value",
		},
		{
			image:  "planet-master:0.0.1",
			result: loc.MustParseLocator("gravitational.io/planet-master-qux:0.0.1"),
			randomSuffix: func(name string) string {
				return fmt.Sprintf("%v-qux", name)
			},
			comment: "avoids collision with legacy name",
		},
	}

	for _, testCase := range testCases {
		comment := Commentf(testCase.comment)
		visited := testCase.visited
		if visited == nil {
			visited = make(map[string]loc.Locator)
		}
		generate := newRuntimePackage(visited, testCase.randomSuffix)
		runtimePackage, err := generate(testCase.image)
		c.Assert(err, IsNil, comment)
		c.Assert(*runtimePackage, compare.DeepEquals, testCase.result, comment)
	}
}
