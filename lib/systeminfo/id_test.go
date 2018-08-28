package systeminfo

import (
	"testing"

	. "gopkg.in/check.v1"
)

func TestId(t *testing.T) { TestingT(t) }

type IdSuite struct{}

var _ = Suite(&IdSuite{})

func (r *IdSuite) TestGetReleaseVersion(c *C) {
	var testCases = []struct {
		source string
		result string
	}{
		{
			source: "Red Hat Enterprise Linux Server release 7.3 (Maipo)",
			result: "7.3",
		},
		{
			source: "CentOS Linux release 7.3.1611 (Core) ",
			result: "7.3.1611",
		},
		{
			source: "CentOS Linux release 7.2.1511 (Core)",
			result: "7.2.1511",
		},
		{
			source: "Red Hat Enterprise Linux Server release 7.2 (Maipo)",
			result: "7.2",
		},
		{
			source: "Release version without the embedded version",
			result: "",
		},
	}
	for _, testCase := range testCases {
		localResult := getReleaseVersion(testCase.source)
		c.Assert(localResult, Equals, testCase.result)
	}
}
