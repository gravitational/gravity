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

package systemservice

import . "gopkg.in/check.v1"

func (*SystemdSuite) TestEscapesSystemdUnitNames(c *C) {
	var testCases = []struct {
		input    string
		expected string
		comment  string
	}{
		{"foo+-!.service", `foo\x2b-\x21.service`, "transforms characters outside allowed set"},
		{".foo_BAR-baz.service", ".foo_BAR-baz.service", "no escaping necessary"},
	}

	for _, testCase := range testCases {
		result := SystemdNameEscape(testCase.input)
		c.Assert(result, DeepEquals, testCase.expected, Commentf(testCase.comment))
	}
}
