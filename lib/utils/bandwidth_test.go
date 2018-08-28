package utils

import (
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/codahale/hdrhistogram"
	"gopkg.in/check.v1"
)

type BandwidthSuite struct{}

var _ = check.Suite(&BandwidthSuite{})

func (s *BandwidthSuite) TestBandwidthWriter(c *check.C) {
	// create writer manually (not via New method) so the goroutine does not start
	w := &BandwidthWriter{
		histogram: hdrhistogram.New(0, defaults.BandwidthMaxSpeedBytes, 5),
		closeCh:   make(chan struct{}),
	}

	n, err := w.Write([]byte("hello"))
	c.Assert(err, check.IsNil)
	c.Assert(n, check.Equals, 5)
	w.tick()
	c.Assert(w.Max(), check.Equals, uint64(5))

	for i := 0; i < 2; i++ {
		n, err := w.Write([]byte("hello"))
		c.Assert(err, check.IsNil)
		c.Assert(n, check.Equals, 5)
	}
	w.tick()
	c.Assert(w.Max(), check.Equals, uint64(10))

	for i := 0; i < 5; i++ {
		n, err := w.Write([]byte("h"))
		c.Assert(err, check.IsNil)
		c.Assert(n, check.Equals, 1)
	}
	w.tick()
	c.Assert(w.Max(), check.Equals, uint64(10))
}
