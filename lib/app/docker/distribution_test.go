package docker

import (
	"testing"

	. "gopkg.in/check.v1"
)

func TestDocker(t *testing.T) { TestingT(t) }

type DistributionSuite struct{}

var _ = Suite(&DistributionSuite{})

func (_ *DistributionSuite) TestCorrectlyReportsFailureToServe(c *C) {
	dir := c.MkDir()
	config := BasicConfiguration("-invalid-addr-:0", dir)
	registry, err := NewRegistry(config)
	c.Assert(err, IsNil)

	err = registry.Start()
	c.Assert(err, ErrorMatches, ".*listen tcp.*")
	registry.Close()
}
