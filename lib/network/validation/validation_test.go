package validation

import (
	"sort"
	"testing"

	pb "github.com/gravitational/gravity/lib/network/validation/proto"

	"gopkg.in/check.v1"
)

func TestValidation(t *testing.T) { check.TestingT(t) }

type ValidationSuite struct{}

var _ = check.Suite(&ValidationSuite{})

func (r *ValidationSuite) TestComputesDiff(c *check.C) {
	var testCases = []struct {
		total  []*pb.Addr
		actual []*pb.ServerResult
		diff   []*pb.Addr
	}{
		{
			total: []*pb.Addr{
				&pb.Addr{Network: "udp", Addr: "1.2.3.6:1012"},
				&pb.Addr{Network: "tcp", Addr: "1.2.3.4:1010"},
				&pb.Addr{Network: "tcp", Addr: "1.2.3.5:1011"},
			},
			actual: []*pb.ServerResult{
				&pb.ServerResult{Code: 1, Error: "connection refused",
					Server: &pb.Addr{Network: "tcp", Addr: "1.2.3.4:1010"},
				},
			},
			diff: []*pb.Addr{
				&pb.Addr{Network: "tcp", Addr: "1.2.3.5:1011"},
				&pb.Addr{Network: "udp", Addr: "1.2.3.6:1012"},
			},
		},
	}

	for _, testCase := range testCases {
		diff := computeDiff(testCase.total, testCase.actual)
		c.Assert(sorted(diff), check.DeepEquals, testCase.diff)
	}
}

func sorted(servers []*pb.Addr) []*pb.Addr {
	sort.Sort(byIPPort(servers))
	return servers
}

type byIPPort []*pb.Addr

func (r byIPPort) Len() int      { return len(r) }
func (r byIPPort) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byIPPort) Less(i, j int) bool {
	return r[i].Address() < r[j].Address()
}
