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

func (_ *S) TestFetchesFilesystem(c *C) {
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
			err:   `error, failed to determine filesystem type on /dev/foo`,
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

func (r testRunner) RunStream(w io.Writer, args ...string) error {
	fmt.Fprintf(w, string(r))
	return nil
}

type testRunner string

func (r failingRunner) RunStream(w io.Writer, args ...string) error {
	return r.error
}

type failingRunner struct {
	error
}
