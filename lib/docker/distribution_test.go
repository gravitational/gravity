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

import (
	"testing"

	. "gopkg.in/check.v1"
)

func TestDocker(t *testing.T) { TestingT(t) }

type DistributionSuite struct{}

var _ = Suite(&DistributionSuite{})

func (*DistributionSuite) TestCorrectlyReportsFailureToServe(c *C) {
	dir := c.MkDir()
	config := BasicConfiguration("-invalid-addr-:0", dir)
	registry, err := NewRegistry(config)
	c.Assert(err, IsNil)

	err = registry.Start()
	c.Assert(err, ErrorMatches, ".*listen tcp.*")
	registry.Close()
}
