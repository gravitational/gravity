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

package gce

import (
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

func TestGCE(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) TestValidatesTag(c *C) {
	var testCases = []struct {
		tag     string
		err     string
		comment string
	}{
		{
			tag:     "foobaR",
			err:     "(?s).*must start and end with either a number or a lowercase character.*",
			comment: "tag value must start and end either with a number or a lowercase letter",
		},
		{
			tag:     "my.cluster",
			err:     "(?s).*must start and end with either a number or a lowercase character.*",
			comment: "tag value must start and end either with a number or a lowercase letter",
		},
		{
			tag:     strings.Repeat("my-bigger-and-better-cluster-name", 2),
			err:     "tag value cannot be longer than 63 characters",
			comment: "tag value cannot be longer than 63 characters",
		},
		{
			err:     "tag value cannot be empty",
			comment: "tag value cannot be empty",
		},
		{
			tag:     "a",
			comment: "single character tag",
		},
		{
			tag:     "valid-cluster-2-name",
			comment: "tag value conforms",
		},
	}

	for _, testCase := range testCases {
		err := ValidateTag(testCase.tag)
		comment := Commentf(testCase.comment)
		if len(testCase.err) == 0 {
			c.Assert(err, IsNil, comment)
		}
		if err != nil || len(testCase.err) != 0 {
			c.Assert(err, ErrorMatches, testCase.err, comment)
		}
	}
}
