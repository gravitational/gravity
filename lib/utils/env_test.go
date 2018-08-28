package utils

import . "gopkg.in/check.v1"

type EnvSuite struct{}

var _ = Suite(&EnvSuite{})

func (s *EnvSuite) TestEnv(c *C) {
	path := "/tmp/env"
	env := map[string]string{"PATH": "/bin", "NAME": "value"}
	err := WriteEnv(path, env)
	c.Assert(err, IsNil)
	readEnv, err := ReadEnv(path)
	c.Assert(err, IsNil)
	c.Assert(readEnv, DeepEquals, env)
}
