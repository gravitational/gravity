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

package update

import (
	"github.com/gravitational/gravity/lib/schema"

	. "gopkg.in/check.v1"
)

type UpdateSuite struct {
}

var _ = Suite(&UpdateSuite{})

func (_ *UpdateSuite) TestDiffsPorts(c *C) {
	var testCases = []struct {
		old, new schema.Requirements
		tcp      []int
		udp      []int
		comment  string
	}{
		{
			old: schema.Requirements{
				Network: schema.Network{Ports: []schema.Port{
					{Protocol: "tcp", Ranges: []string{"3000-3099"}},
					{Protocol: "udp", Ranges: []string{"3200"}},
				}}},
			new: schema.Requirements{
				Network: schema.Network{Ports: []schema.Port{
					{Protocol: "tcp", Ranges: []string{"3000-3199"}},
					{Protocol: "udp", Ranges: []string{"3200", "3202"}},
				}}},
			tcp:     newArray(3100, 3199),
			udp:     []int{3202},
			comment: "compute difference",
		},
		{
			old: schema.Requirements{
				Network: schema.Network{Ports: []schema.Port{
					{Protocol: "tcp", Ranges: []string{"3000-3009", "2099"}},
					{Protocol: "udp", Ranges: []string{"3200", "1099"}},
				}}},
			new: schema.Requirements{
				Network: schema.Network{Ports: []schema.Port{
					{Protocol: "tcp", Ranges: []string{"3000-3009", "3199"}},
					{Protocol: "udp", Ranges: []string{"3200", "3201"}},
				}}},
			tcp:     []int{3199},
			udp:     []int{3201},
			comment: "do not account for ports found only in old",
		},
	}

	for _, testCase := range testCases {
		old := schema.Manifest{
			NodeProfiles: schema.NodeProfiles{{Name: "test", Requirements: testCase.old}},
		}
		new := schema.Manifest{
			NodeProfiles: schema.NodeProfiles{{Name: "test", Requirements: testCase.new}},
		}

		tcp, udp, err := diffPorts(old, new, "test")
		c.Assert(err, IsNil)
		c.Assert(tcp, DeepEquals, testCase.tcp, Commentf(testCase.comment))
		c.Assert(udp, DeepEquals, testCase.udp, Commentf(testCase.comment))
	}
}

func (_ *UpdateSuite) TestDiffsVolumes(c *C) {
	var testCases = []struct {
		old, new []schema.Volume
		diff     []schema.Volume
		comment  string
	}{
		{
			old: []schema.Volume{
				schema.Volume{Name: "foo", Path: "/foo"},
			},
			new: []schema.Volume{
				schema.Volume{Name: "foo", Path: "/foo"},
			},
			diff:    []schema.Volume{},
			comment: "no difference",
		},
		{
			old: []schema.Volume{
				schema.Volume{Path: "/foo"},
				schema.Volume{Path: "/bar"},
			},
			new: []schema.Volume{
				schema.Volume{Path: "/foo"},
				schema.Volume{Path: "/bar"},
			},
			diff:    []schema.Volume{},
			comment: "no difference based on path",
		},
		{
			old: []schema.Volume{
				schema.Volume{Name: "foo", Path: "/foo"},
				schema.Volume{Name: "qux", Path: "/qux"},
			},
			new: []schema.Volume{
				schema.Volume{Name: "foo", Path: "/foo"},
				schema.Volume{Name: "bar", Path: "/bar"},
			},
			diff: []schema.Volume{
				schema.Volume{Name: "bar", Path: "/bar"},
			},
			comment: "do not account for volumes found only in old",
		},
	}

	for _, testCase := range testCases {
		diff := diffVolumes(testCase.old, testCase.new)
		c.Assert(diff, DeepEquals, testCase.diff, Commentf(testCase.comment))
	}
}

// newArray generates a new array in range [from:to+1]
func newArray(from, to int) (result []int) {
	result = make([]int, to-from+1)
	for i := 0; i < cap(result); i++ {
		result[i] = from + i
	}
	return result
}
