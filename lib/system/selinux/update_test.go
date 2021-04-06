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

package selinux

import (
	"bytes"
	"context"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func (*S) TestWritesUpdateScript(c *C) {
	var testCases = []struct {
		config            UpdateConfig
		existingVxlanPort string
		expected          string
		fcontext          string
		comment           string
	}{
		{
			config: UpdateConfig{
				Generic: []schema.PortRange{
					{Protocol: "tcp", From: 10000, To: 10001},
					{Protocol: "udp", From: 20000, To: 20010},
				},
				Paths: Paths{
					{Path: "/var/data"},
					{Path: "/var/data2", Label: "system_u:object_r:data_file_t:s0"},
				},
				VxlanPort:       utils.IntPtr(8472),
				vxlanPortGetter: testGetPortNotFound,
			},
			expected: `

port -a -t gravity_vxlan_port_t -r 's0' -p udp 8472

port -a -t gravity_port_t -r 's0' -p tcp 10000-10001
port -a -t gravity_port_t -r 's0' -p udp 20000-20010

fcontext -a -f d -t container_file_t -r 's0' '/var/data'
fcontext -a -f d -t data_file_t -r 's0' '/var/data2'`,
			comment: "uses all attributes",
		},
		{
			config: UpdateConfig{
				VxlanPort:       utils.IntPtr(8472),
				vxlanPortGetter: testGetPort("8472"),
			},
			expected: `



`,
			comment: "does not do redundant update of vxlan port",
		},
		{
			config: UpdateConfig{
				VxlanPort:       utils.IntPtr(8473),
				vxlanPortGetter: testGetPort("8472"),
			},
			expected: `
port -d -p udp 8472
port -a -t gravity_vxlan_port_t -r 's0' -p udp 8473

`,
			comment: "updates existing vxlan port",
		},
	}
	for _, testCase := range testCases {
		comment := Commentf(testCase.comment)
		var buf bytes.Buffer
		err := testCase.config.write(context.TODO(), &buf)
		c.Assert(err, IsNil, comment)
		c.Assert(buf.String(), compare.DeepEquals, testCase.expected, comment)
	}
}

func testGetPortNotFound(context.Context) (string, error) {
	return "", trace.NotFound("no vxlan port configuration")
}

func testGetPort(port string) vxlanPortGetter {
	return func(context.Context) (string, error) {
		return port, nil
	}
}
