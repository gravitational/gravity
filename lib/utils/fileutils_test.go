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
