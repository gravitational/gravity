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

package systeminfo

import (
	"testing"

	. "gopkg.in/check.v1"
)

func TestId(t *testing.T) { TestingT(t) }

type IdSuite struct{}

var _ = Suite(&IdSuite{})

func (r *IdSuite) TestGetReleaseVersion(c *C) {
	var testCases = []struct {
		source string
		result string
	}{
		{
			source: "Red Hat Enterprise Linux Server release 7.3 (Maipo)",
			result: "7.3",
		},
		{
			source: "CentOS Linux release 7.3.1611 (Core) ",
			result: "7.3.1611",
		},
		{
			source: "CentOS Linux release 7.2.1511 (Core)",
			result: "7.2.1511",
		},
		{
			source: "Red Hat Enterprise Linux Server release 7.2 (Maipo)",
			result: "7.2",
		},
		{
			source: "Release version without the embedded version",
			result: "",
		},
	}
	for _, testCase := range testCases {
		localResult := getReleaseVersion(testCase.source)
		c.Assert(localResult, Equals, testCase.result)
	}
}
