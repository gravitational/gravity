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

import (
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

type FileutilsSuite struct{}

var _ = Suite(&FileutilsSuite{})

func (s *FileutilsSuite) TestRecursiveGlob(c *C) {
	var matches []string
	//nolint:errcheck
	RecursiveGlob("../../assets/site/fixtures/glob", []string{"*yaml"}, func(match string) error {
		matches = append(matches, match)
		return nil
	})
	c.Assert(matches, HasLen, 2)
	for _, n := range matches {
		c.Assert(strings.HasSuffix(n, ".yaml"), Equals, true)
	}
}

func (s *FileutilsSuite) TestEnsureLine(c *C) {
	tempDir := c.MkDir()
	path := filepath.Join(tempDir, "test")
	c.Assert(ioutil.WriteFile(path, []byte(`    789
qwe`), defaults.SharedReadMask), IsNil)
	c.Assert(EnsureLineInFile(path, "1234"), IsNil)
	c.Assert(EnsureLineInFile(path, "\n\t5678\n\n"), IsNil)
	c.Assert(EnsureLineInFile(path, "1234"), FitsTypeOf, trace.AlreadyExists(""))
	c.Assert(EnsureLineInFile(path, " 1234  "), FitsTypeOf, trace.AlreadyExists(""))
	c.Assert(EnsureLineInFile(path, "789"), FitsTypeOf, trace.AlreadyExists(""))
	c.Assert(EnsureLineInFile(path, "5678\t"), FitsTypeOf, trace.AlreadyExists(""))
	data, err := ioutil.ReadFile(path)
	c.Assert(err, IsNil)
	c.Assert(data, DeepEquals, []byte(`    789
qwe
1234
5678`))
}
