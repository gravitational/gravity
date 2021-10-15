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

package loc

import (
	"testing"

	"github.com/gravitational/gravity/lib/compare"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestLocators(t *testing.T) { TestingT(t) }

type LocatorSuite struct {
}

var _ = Suite(&LocatorSuite{})

func (s *LocatorSuite) TestLocatorOK(c *C) {
	tcs := []struct {
		loc  string
		repo string
		name string
		ver  string
	}{
		{
			loc:  "example.com/package:0.0.1",
			repo: "example.com",
			name: "package",
			ver:  "0.0.1",
		},
		{
			loc:  "example.com/package:0.0.1+something",
			name: "package",
			repo: "example.com",
			ver:  "0.0.1+something",
		},
	}
	for i, tc := range tcs {
		comment := Commentf("test #%d (%v) loc=%v", i+1, tc, tc.loc)
		loc, err := ParseLocator(tc.loc)
		c.Assert(err, IsNil, comment)
		c.Assert(loc.Name, Equals, tc.name, comment)
		c.Assert(loc.Version, Equals, tc.ver, comment)
	}
}

func (s *LocatorSuite) TestNewerThan(c *C) {
	l1, err := ParseLocator("gravitational.io/k8s-aws:1.1.218-138")
	c.Assert(err, IsNil)
	l2, err := ParseLocator("gravitational.io/k8s-aws:1.2.0")
	c.Assert(err, IsNil)
	newer, err := l2.IsNewerThan(*l1)
	c.Assert(err, IsNil)
	c.Assert(newer, Equals, true)
}

func (s *LocatorSuite) TestLocatorFail(c *C) {
	tcs := []string{
		"example:0.0.1",                 // missing repository
		"example.com/example:blabla",    // not a sem ver
		"example.com/example com:0.0.2", // unallowed chars
		"",                              //emtpy
		"arffewfaef aefeafaesf e",       //garbage
		"-:.",
	}
	for i, tc := range tcs {
		comment := Commentf("test #%d (%v) loc=%v", i+1, tc)
		_, err := ParseLocator(tc)
		c.Assert(err, NotNil, comment)
	}
}

func (s *LocatorSuite) TestParseDockerImage(c *C) {
	tcs := []struct {
		input    string
		expected DockerImage
		error    bool
	}{
		{input: "", expected: DockerImage{Repository: "test"}, error: true},
		{input: "test", expected: DockerImage{Repository: "test"}},
		{input: "test/test:v0.0.1", expected: DockerImage{Repository: "test/test", Tag: "v0.0.1"}},
		{input: "test/test@sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb", expected: DockerImage{Repository: "test/test", Digest: "sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb"}},
		{input: "apiserver:5000/test", expected: DockerImage{Registry: "apiserver:5000", Repository: "test"}},
		{input: "apiserver:5000/test:v0.0.1", expected: DockerImage{Registry: "apiserver:5000", Repository: "test", Tag: "v0.0.1"}},
		{input: "apiserver:5000/test@sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb", expected: DockerImage{Registry: "apiserver:5000", Repository: "test", Digest: "sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb"}},
		{input: "apiserver:5000/test/test:v0.0.1@sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb", expected: DockerImage{Registry: "apiserver:5000", Repository: "test/test", Tag: "v0.0.1", Digest: "sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb"}},
	}
	for i, tc := range tcs {
		comment := Commentf("test case %v", i+1)
		image, err := ParseDockerImage(tc.input)
		if tc.error {
			c.Assert(err, NotNil, comment)
		} else {
			c.Assert(err, IsNil, comment)
			c.Assert(*image, DeepEquals, tc.expected, comment)
			c.Assert(image.String(), Equals, tc.input)
		}
	}
}

func (s *LocatorSuite) TestIsUpdate(c *C) {
	tests := []struct {
		locators []Locator
		locator  Locator
		result   bool
	}{
		{
			locators: []Locator{MustParseLocator("repo/package1:1.0.0"), MustParseLocator("repo/package2:1.0.0")},
			locator:  MustParseLocator("repo/package2:1.0.0"),
			result:   false,
		},
		{
			locators: []Locator{MustParseLocator("repo/package1:1.0.0"), MustParseLocator("repo/package2:1.0.0")},
			locator:  MustParseLocator("repo/package2:2.0.0"),
			result:   true,
		},
		{
			locators: []Locator{MustParseLocator("repo/package1:1.0.0")},
			locator:  MustParseLocator("repo/package2:1.0.0"),
			result:   true,
		},
	}
	for _, test := range tests {
		result, err := IsUpdate(test.locator, test.locators)
		c.Assert(err, IsNil)
		c.Assert(result, Equals, test.result, Commentf("%v", test))
	}
}

func (s *LocatorSuite) TestDeduplicate(c *C) {
	locs := []Locator{
		MustParseLocator("test1/foo:1.0.0"),
		MustParseLocator("test2/bar:1.0.0"),
		MustParseLocator("test3/qux:1.0.0"),
		MustParseLocator("test2/bar:1.0.0"),
	}
	uniq := Deduplicate(locs)
	expected := []Locator{
		MustParseLocator("test1/foo:1.0.0"),
		MustParseLocator("test2/bar:1.0.0"),
		MustParseLocator("test3/qux:1.0.0"),
	}
	c.Assert(uniq, compare.DeepEquals, expected)
}

func (s *LocatorSuite) TestMakeLocator(c *C) {
	tests := []struct {
		input   string
		version string
		error   error
		output  string
	}{
		{
			input:  "gravity",
			output: "gravitational.io/gravity:0.0.0+latest",
		},
		{
			input:  "gravity:0.0.1",
			output: "gravitational.io/gravity:0.0.1",
		},
		{
			input:  "gravitational.io/gravity:0.0.2",
			output: "gravitational.io/gravity:0.0.2",
		},
		{
			input:   "gravity",
			version: "1.0.0",
			output:  "gravitational.io/gravity:1.0.0",
		},
		{
			input:  "gravity:latest",
			output: "gravitational.io/gravity:0.0.0+latest",
		},
		{
			input:  "gravity:stable",
			output: "gravitational.io/gravity:0.0.0+stable",
		},
		{
			input: "gravity:0.0.1:1.0.0",
			error: trace.BadParameter(""),
		},
	}
	for _, test := range tests {
		var loc *Locator
		var err error
		if test.version != "" {
			loc, err = MakeLocatorWithDefault(test.input, func(name string) string {
				return test.version
			})
		} else {
			loc, err = MakeLocator(test.input)
		}
		if test.error != nil {
			c.Assert(err, FitsTypeOf, test.error)
		} else {
			c.Assert(err, IsNil)
			c.Assert(loc.String(), Equals, test.output)
		}
	}
}
