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

package system

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestSystem(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) TestFetchesFilesystem(c *C) {
	var testCases = []struct {
		lsblk      utils.CommandRunner
		filesystem string
		err        string
		comment    string
	}{
		{
			lsblk:      testRunner("xfs"),
			filesystem: "xfs",
			comment:    "parses the filesystem",
		},
		{
			lsblk:      testRunner(""),
			filesystem: "",
			err:        "no filesystem found for /dev/foo",
		},
		{
			lsblk: testRunner(`LVM_member

xfs
xfs
`),
			filesystem: "LVM_member",
			comment:    "only uses the top result",
		},
		{
			lsblk: failingRunner{trace.Errorf("error")},
			err:   "failed to determine filesystem type on /dev/foo\n\terror",
		},
	}
	for _, testCase := range testCases {
		comment := Commentf(testCase.comment)
		filesystem, err := GetFilesystem(context.TODO(), "/dev/foo", testCase.lsblk)
		if len(testCase.err) != 0 {
			c.Assert(err, ErrorMatches, testCase.err)
		} else {
			c.Assert(err, IsNil, comment)
		}
		c.Assert(filesystem, Equals, testCase.filesystem, comment)
	}
}

func (r testRunner) RunStream(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	fmt.Fprint(stdout, string(r))
	return nil
}

type testRunner string

func (r failingRunner) RunStream(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	return r.error
}

type failingRunner struct {
	error
}
