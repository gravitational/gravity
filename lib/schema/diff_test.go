/*
Copyright 2019 Gravitational, Inc.

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
	. "gopkg.in/check.v1"
)

type DiffSuite struct{}

var _ = Suite(&DiffSuite{})

func (_ *DiffSuite) TestDiffsPorts(c *C) {
	var testCases = []struct {
		old, new Requirements
		tcp      []int
		udp      []int
		comment  string
	}{
		{
			old: Requirements{
				Network: Network{Ports: []Port{
					{Protocol: "tcp", Ranges: []string{"3000-3099"}},
					{Protocol: "udp", Ranges: []string{"3200"}},
				}}},
			new: Requirements{
				Network: Network{Ports: []Port{
					{Protocol: "tcp", Ranges: []string{"3000-3199"}},
					{Protocol: "udp", Ranges: []string{"3202", "3200"}},
				}}},
			tcp:     newArray(3100, 3199),
			udp:     []int{3202},
			comment: "compute difference",
		},
		{
			old: Requirements{
				Network: Network{Ports: []Port{
					{Protocol: "tcp", Ranges: []string{"2099", "3000-3009", "1098"}},
					{Protocol: "udp", Ranges: []string{"1099", "3200", "2002"}},
				}}},
			new: Requirements{
				Network: Network{Ports: []Port{
					{Protocol: "tcp", Ranges: []string{"3000-3009", "3199", "1098"}},
					{Protocol: "udp", Ranges: []string{"3200", "3201", "1098"}},
				}}},
			tcp:     []int{3199},
			udp:     []int{1098, 3201},
			comment: "do not account for ports found only in old",
		},
	}

	for _, testCase := range testCases {
		old := Manifest{
			NodeProfiles: NodeProfiles{{Name: "test", Requirements: testCase.old}},
		}
		new := Manifest{
			NodeProfiles: NodeProfiles{{Name: "test", Requirements: testCase.new}},
		}

		tcp, udp, err := DiffPorts(old, new, "test")
		c.Assert(err, IsNil)
		c.Assert(tcp, DeepEquals, testCase.tcp, Commentf(testCase.comment))
		c.Assert(udp, DeepEquals, testCase.udp, Commentf(testCase.comment))
	}
}

func (_ *DiffSuite) TestDiffsVolumes(c *C) {
	var testCases = []struct {
		old, new []Volume
		diff     []Volume
		comment  string
	}{
		{
			old: []Volume{
				{Name: "foo", Path: "/foo"},
			},
			new: []Volume{
				{Name: "foo", Path: "/foo"},
			},
			diff:    []Volume{},
			comment: "no difference",
		},
		{
			old: []Volume{
				{Path: "/foo"},
				{Path: "/bar"},
			},
			new: []Volume{
				{Path: "/foo"},
				{Path: "/bar"},
			},
			diff:    []Volume{},
			comment: "no difference based on path",
		},
		{
			old: []Volume{
				{Name: "foo", Path: "/foo"},
				{Name: "qux", Path: "/qux"},
			},
			new: []Volume{
				{Name: "foo", Path: "/foo"},
				{Name: "bar", Path: "/bar"},
			},
			diff: []Volume{
				{Name: "bar", Path: "/bar"},
			},
			comment: "do not account for volumes found only in old",
		},
	}

	for _, testCase := range testCases {
		diff := DiffVolumes(testCase.old, testCase.new)
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
