package utils

import (
	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

type NetSuite struct{}

var _ = check.Suite(&NetSuite{})

func (s *NetSuite) TestSelectVPCSubnet(c *check.C) {
	type testCase struct {
		inVPCBlock     string
		inSubnetBlocks []string
		outSubnet      string
		outNotFound    bool
		description    string
	}
	testCases := []testCase{
		{
			inVPCBlock:     "10.100.0.0/16",
			inSubnetBlocks: []string{"10.100.0.0/24"},
			outSubnet:      "10.100.1.0/24",
			description:    "1st /24 of VPC block is occupied, should select second /24",
		},
		{
			inVPCBlock:     "10.100.0.0/16",
			inSubnetBlocks: []string{},
			outSubnet:      "10.100.0.0/24",
			description:    "all VPC is free, should select first /24",
		},
		{
			inVPCBlock:     "10.100.0.0/16",
			inSubnetBlocks: []string{"10.100.0.0/24", "10.100.1.0/24"},
			outSubnet:      "10.100.2.0/24",
			description:    "two /24 are occupied, should select 3rd",
		},
		{
			inVPCBlock:     "10.100.0.0/16",
			inSubnetBlocks: []string{"10.100.0.0/17"},
			outSubnet:      "10.100.128.0/24",
			description:    "half of VPC range is occupied, should select first /24 from other half",
		},
		{
			inVPCBlock:     "10.100.0.0/24",
			inSubnetBlocks: []string{"10.100.0.0/24"},
			outNotFound:    true,
			description:    "subnet occupies the whole VPC, should not find free /24",
		},
		{
			inVPCBlock:     "10.100.0.0/23",
			inSubnetBlocks: []string{"10.100.0.0/24", "10.100.1.0/24"},
			outNotFound:    true,
			description:    "two subnets occupy the whole VPC, should not find free /24",
		},
	}
	for _, tc := range testCases {
		subnet, err := SelectVPCSubnet(tc.inVPCBlock, tc.inSubnetBlocks)
		if tc.outNotFound {
			c.Assert(trace.IsNotFound(err), check.Equals, true, check.Commentf(tc.description))
		} else {
			c.Assert(err, check.IsNil, check.Commentf(tc.description))
			c.Assert(subnet, check.Equals, tc.outSubnet, check.Commentf(tc.description))
		}
	}
}

func (s *NetSuite) TestSelectSubnet(c *check.C) {
	type testCase struct {
		inSubnets   []string
		outSubnet   string
		description string
	}
	testCases := []testCase{
		{
			inSubnets:   []string{},
			outSubnet:   "10.0.0.0/16",
			description: "no subnets occupied, should select 1st /16",
		},
		{
			inSubnets:   []string{"10.0.0.0/16"},
			outSubnet:   "10.1.0.0/16",
			description: "1st private subnet is occupied, should select 2nd 16",
		},
		{
			inSubnets:   []string{"10.0.0.0/15"},
			outSubnet:   "10.2.0.0/16",
			description: "/15 (= 2x /16) is occupied, should select 3rd /16",
		},
		{
			inSubnets:   []string{"10.0.0.0/16", "10.1.0.0/16"},
			outSubnet:   "10.2.0.0/16",
			description: "2x /16 are occupied, should select 3rd /16",
		},
		{
			inSubnets:   []string{"10.0.0.0/8"},
			outSubnet:   "172.16.0.0/16",
			description: "the whole first private range is occupied, should select 1st /16 of next one",
		},
	}
	for _, tc := range testCases {
		subnet, err := SelectSubnet(tc.inSubnets)
		c.Assert(err, check.IsNil, check.Commentf(tc.description))
		c.Assert(subnet, check.Equals, tc.outSubnet, check.Commentf(tc.description))
	}
}

func (s *NetSuite) TestValidateKubernetesSubnets(c *check.C) {
	type testCase struct {
		podCIDR     string
		serviceCIDR string
		ok          bool
		description string
	}
	testCases := []testCase{
		{
			podCIDR:     "10.244.0.0/16",
			serviceCIDR: "10.100.0.0/16",
			ok:          true,
			description: "default subnets should validate",
		},
		{
			podCIDR:     "10.244.0.0-10.244.255.0",
			ok:          false,
			description: "pod subnet is not a valid CIDR",
		},
		{
			serviceCIDR: "10.100.0.0-10.100.255.0",
			ok:          false,
			description: "service subnet is not a valid CIDR",
		},
		{
			podCIDR:     "10.200.0.0/20",
			ok:          false,
			description: "pod subnet is too small",
		},
		{
			podCIDR:     "10.100.0.0/16",
			serviceCIDR: "10.100.100.0/16",
			ok:          false,
			description: "pod and service subnets overlap",
		},
	}
	for _, tc := range testCases {
		err := ValidateKubernetesSubnets(tc.podCIDR, tc.serviceCIDR)
		if tc.ok {
			c.Assert(err, check.IsNil, check.Commentf(tc.description))
		} else {
			c.Assert(err, check.NotNil, check.Commentf(tc.description))
		}
	}
}
