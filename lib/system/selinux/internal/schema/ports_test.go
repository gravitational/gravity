/*
Copyright 2020 Gravitational, Inc.

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

package schema

import (
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

func TestCommandParser(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) TestParsesPortCommand(c *C) {
	var testCases = []struct {
		cmd      string
		expected PortCommand
		comment  string
	}{
		{
			cmd: "port -a -t gravity_kubernetes_port_t -r 's0' -p tcp 2379-2380",
			expected: PortCommand{
				Type:          "gravity_kubernetes_port_t",
				SecurityRange: "s0",
				Protocol:      "tcp",
				Range:         "2379-2380",
			},
			comment: "Can parse port add command",
		},
		{
			cmd: "port -d -p tcp 2379-2380",
			expected: PortCommand{
				Protocol:      "tcp",
				Range:         "2379-2380",
				SecurityRange: "s0",
			},
			comment: "Can parse port delete command",
		},
	}
	for _, testCase := range testCases {
		comment := Commentf(testCase.comment)
		var portCmd PortCommand
		c.Assert(portCmd.ParseFromString(testCase.cmd), IsNil, comment)
		c.Assert(portCmd, DeepEquals, testCase.expected, comment)
	}
}

func (*S) TestParsesLocalPortChanges(c *C) {
	const ports = `
port -a -t gravity_kubernetes_port_t -r 's0' -p tcp 7496
port -a -t gravity_vxlan_port_t -r 's0' -p udp 8472
`
	localPorts, err := GetLocalPortChangesFromReader(strings.NewReader(ports))
	c.Assert(err, IsNil)
	c.Assert(localPorts, DeepEquals, []PortCommand{
		{
			Type:          "gravity_kubernetes_port_t",
			SecurityRange: "s0",
			Protocol:      "tcp",
			Range:         "7496",
		},
		{
			Type:          "gravity_vxlan_port_t",
			SecurityRange: "s0",
			Protocol:      "udp",
			Range:         "8472",
		},
	})
}
