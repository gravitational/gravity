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

package utils

import . "gopkg.in/check.v1"

func (*UtilsSuite) TestFlattensVersion(c *C) {
	var testCases = []struct {
		input   string
		output  string
		comment string
	}{
		{
			input:   "3.1.2",
			output:  "312",
			comment: "removes punctuation",
		},
		{
			input:   "3.1.2+abc",
			output:  "312-abc",
			comment: "normalizes splitters",
		},
	}

	for _, testCase := range testCases {
		obtained := FlattenVersion(testCase.input)
		c.Assert(obtained, Equals, testCase.output, Commentf(testCase.comment))
	}
}
