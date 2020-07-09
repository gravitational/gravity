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
	"os"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

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

func (s *EnvSuite) TestGetenvsByPrefix(c *C) {
	envs := map[string]string{
		"TEST1_A": "v1",
		"TEST1_B": "v2",
	}
	for k, v := range envs {
		os.Setenv(k, v)
	}
	c.Assert(GetenvsByPrefix("TEST1_"), DeepEquals, envs)
}

func (s *EnvSuite) TestGetenvInt(c *C) {
	os.Setenv("TEST1", "42")
	val, err := GetenvInt("TEST1")
	c.Assert(err, IsNil)
	c.Assert(val, Equals, 42)

	os.Setenv("TEST1", "qwerty")
	val, err = GetenvInt("TEST1")
	c.Assert(err, NotNil)

	val, err = GetenvInt("DOESNOTEXIST")
	c.Assert(err, FitsTypeOf, trace.NotFound(""))
}
