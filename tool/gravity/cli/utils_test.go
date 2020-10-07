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

package cli

import (
	"testing"

	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

func TestUtils(t *testing.T) { check.TestingT(t) }

type UtilsSuite struct{}

var _ = check.Suite(&UtilsSuite{})

func (s *UtilsSuite) TestFindServer(c *check.C) {
	server := storage.Server{
		AdvertiseIP: "10.0.0.1",
		Hostname:    "hostName",
		Nodename:    "nodeNamee",
		InstanceID:  "i-sdfkwerjal",
	}
	servers := []storage.Server{server}

	tokens := []string{server.AdvertiseIP}
	out, err := findServer(servers, tokens)
	c.Assert(err, check.IsNil)
	c.Assert(out.AdvertiseIP, check.Equals, server.AdvertiseIP)

	tokens = []string{server.Hostname}
	out, err = findServer(servers, tokens)
	c.Assert(err, check.IsNil)
	c.Assert(out.Hostname, check.Equals, server.Hostname)

	tokens = []string{server.Nodename}
	out, err = findServer(servers, tokens)
	c.Assert(err, check.IsNil)
	c.Assert(out.Nodename, check.Equals, server.Nodename)

	tokens = []string{server.InstanceID}
	out, err = findServer(servers, tokens)
	c.Assert(err, check.IsNil)
	c.Assert(out.InstanceID, check.Equals, server.InstanceID)
}
