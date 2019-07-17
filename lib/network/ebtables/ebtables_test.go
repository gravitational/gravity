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

package ebtables

import (
	"testing"

	"gopkg.in/check.v1"
)

func TestEbtables(t *testing.T) { check.TestingT(t) }

type S struct{}

var _ = check.Suite(&S{})

func (*S) TestParsesToolVersion(c *check.C) {
	var testCases = []struct {
		comment  string
		input    string
		expected string
		err      string
	}{
		{
			comment:  "version with v prefix",
			input:    "ebtables v2.0.10-4 (December 2011)",
			expected: "2.0.10-4",
		},
		{
			comment:  "version without v prefix",
			input:    "ebtables 1.8.3 (nf_tables)",
			expected: "1.8.3",
		},
		{
			comment: "invalid version",
			input:   "not a version",
			err:     `no ebtables version found in string "not a version"`,
		},
	}
	for _, testCase := range testCases {
		comment := check.Commentf(testCase.comment)
		v, err := getVersionFromString(testCase.input)
		if testCase.err != "" {
			c.Assert(err, check.ErrorMatches, testCase.err)
		} else {
			c.Assert(err, check.IsNil, comment)
			c.Assert(v, check.Equals, testCase.expected, comment)
		}
	}
}
