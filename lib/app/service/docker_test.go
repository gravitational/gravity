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
	"github.com/gravitational/gravity/lib/compare"

	. "gopkg.in/check.v1"
)

func (s *VendorSuite) TestParsesImageToNameTag(c *C) {
	type result struct {
		name, tag string
	}
	var testCases = []struct {
		image   string
		result  result
		comment string
	}{
		{
			image:   "foo:5.1.0",
			result:  result{name: "foo", tag: "5.1.0"},
			comment: "image reference w/o repository",
		},
		{
			image:   "repo/foo:1.0.0",
			result:  result{name: "repo-foo", tag: "1.0.0"},
			comment: "image reference with repository",
		},
		{
			image:   "repo/foo:latest",
			result:  result{name: "repo-foo", tag: "latest"},
			comment: "literal tag",
		},
		{
			image:   "repo.io/subrepo/foo:latest",
			result:  result{name: "repo.io-subrepo-foo", tag: "latest"},
			comment: "nested repositories",
		},
	}

	for _, testCase := range testCases {
		name, tag, err := parseImageNameTag(testCase.image)
		comment := Commentf(testCase.comment)
		c.Assert(err, IsNil, comment)
		c.Assert(result{name: name, tag: tag}, compare.DeepEquals, testCase.result, comment)
	}
}
