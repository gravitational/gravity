package gce

import (
	"strings"
	"testing"

	. "gopkg.in/check.v1"
)

func TestGCE(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (_ *S) TestValidatesTag(c *C) {
	var testCases = []struct {
		tag     string
		err     string
		comment string
	}{
		{
			tag:     "foobaR",
			err:     "(?s).*must start and end with either a number or a lowercase character.*",
			comment: "tag value must start and end either with a number or a lowercase letter",
		},
		{
			tag:     "my.cluster",
			err:     "(?s).*must start and end with either a number or a lowercase character.*",
			comment: "tag value must start and end either with a number or a lowercase letter",
		},
		{
			tag:     strings.Repeat("my-bigger-and-better-cluster-name", 2),
			err:     "tag value cannot be longer than 63 characters",
			comment: "tag value cannot be longer than 63 characters",
		},
		{
			err:     "tag value cannot be empty",
			comment: "tag value cannot be empty",
		},
		{
			tag:     "a",
			comment: "single character tag",
		},
		{
			tag:     "valid-cluster-2-name",
			comment: "tag value conforms",
		},
	}

	for _, testCase := range testCases {
		err := ValidateTag(testCase.tag)
		comment := Commentf(testCase.comment)
		if len(testCase.err) == 0 {
			c.Assert(err, IsNil, comment)
		}
		if err != nil || len(testCase.err) != 0 {
			c.Assert(err, ErrorMatches, testCase.err, comment)
		}
	}
}
