package utils

import "gopkg.in/check.v1"

type MathSuite struct{}

var _ = check.Suite(&MathSuite{})

func (s *MathSuite) TestMin(c *check.C) {
	c.Assert(Min(1, 2), check.Equals, 1)
	c.Assert(Min(2, 1), check.Equals, 1)
	c.Assert(Min(2, 2), check.Equals, 2)
}
