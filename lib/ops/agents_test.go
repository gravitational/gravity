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

package ops

import (
	"testing"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/schema"

	check "gopkg.in/check.v1"
)

func TestOps(t *testing.T) { check.TestingT(t) }

type AgentReportSuite struct{}

var _ = check.Suite(&AgentReportSuite{})

func (s *AgentReportSuite) TestDiff(c *check.C) {
	server1 := serverInfo("192.168.1.1", "node")
	server2 := serverInfo("192.168.1.2", "node")
	server3 := serverInfo("192.168.1.3", "node")

	report1 := &AgentReport{
		Servers: []checks.ServerInfo{server1, server2},
	}
	report2 := &AgentReport{
		Servers: []checks.ServerInfo{server1, server3},
	}

	added, removed := report1.Diff(nil)
	c.Assert(added, compare.DeepEquals, []checks.ServerInfo{server1, server2})
	c.Assert(removed, check.IsNil)

	added, removed = report2.Diff(report1)
	c.Assert(added, compare.DeepEquals, []checks.ServerInfo{server3})
	c.Assert(removed, compare.DeepEquals, []checks.ServerInfo{server2})
}

func (s *AgentReportSuite) TestCheck(c *check.C) {
	server1 := serverInfo("192.168.1.1", "worker")
	server2 := serverInfo("192.168.1.3", "db")
	server3 := serverInfo("192.168.1.4", "api")

	report := &AgentReport{
		Servers: []checks.ServerInfo{server1, server2, server3},
	}

	needed, extra := report.MatchFlavor(schema.Flavor{
		Nodes: []schema.FlavorNode{
			{Profile: "worker", Count: 3},
			{Profile: "db", Count: 2},
		},
	})
	c.Assert(needed, compare.DeepEquals, map[string]int{"worker": 2, "db": 1})
	c.Assert(extra, compare.DeepEquals, []checks.ServerInfo{server3})
}

func serverInfo(addr, role string) checks.ServerInfo {
	return checks.ServerInfo{
		RuntimeConfig: proto.RuntimeConfig{
			AdvertiseAddr: addr,
			Role:          role,
		},
	}
}
