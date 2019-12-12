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

package selinux

import (
	"bytes"
	"testing"

	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) TestBootstrapScript(c *C) {
	var buf bytes.Buffer
	config := BootstrapConfig{
		Path:      "/home/redhat/installer",
		VxlanPort: 8472,
	}
	err := WriteBootstrapScript(&buf, config)
	c.Assert(err, IsNil)
	c.Logf("%s", buf.Bytes())
}
