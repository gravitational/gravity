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

package docker

import . "gopkg.in/check.v1"

type TagSpecSuite struct{}

var _ = Suite(&TagSpecSuite{})

func (s *TagSpecSuite) TestEverything(c *C) {
	// success:
	t := TagFromString(" host:1000/repo/simple-app:1.0.2")
	c.Assert(t.IsValid(), Equals, true)
	c.Assert(t.Name, Equals, "host:1000/repo/simple-app")
	c.Assert(t.Version, Equals, "1.0.2")
	c.Assert(t.String(), Equals, "host:1000/repo/simple-app:1.0.2")

	// success with 'latest' version assumed (and no repo)
	t = TagFromString(" host:1000/simple-app")
	c.Assert(t.IsValid(), Equals, true)

	// local tag
	t = TagFromString("simple-app:1.0.2")
	c.Assert(t.String(), Equals, "simple-app:1.0.2")
}
