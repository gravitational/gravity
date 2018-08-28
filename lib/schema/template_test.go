package schema

import (
	"os"

	"gopkg.in/check.v1"
)

type TemplateSuite struct{}

var _ = check.Suite(&TemplateSuite{})

func (s *TemplateSuite) TestExpandEnvVars(c *check.C) {
	text := []byte("Hello ${ENV_VAR}!")

	err := os.Setenv("ENV_VAR", "world")
	c.Assert(err, check.IsNil)

	expanded := ExpandEnvVars(text)
	c.Assert(string(expanded), check.Equals, "Hello world!")
}
