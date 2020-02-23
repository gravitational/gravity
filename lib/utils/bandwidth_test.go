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
	c.Assert(w.tick(), check.IsNil)
	c.Assert(w.Max(), check.Equals, uint64(5))

	for i := 0; i < 2; i++ {
		n, err := w.Write([]byte("hello"))
		c.Assert(err, check.IsNil)
		c.Assert(n, check.Equals, 5)
	}
	c.Assert(w.tick(), check.IsNil)
	c.Assert(w.Max(), check.Equals, uint64(10))

	for i := 0; i < 5; i++ {
		n, err := w.Write([]byte("h"))
		c.Assert(err, check.IsNil)
		c.Assert(n, check.Equals, 1)
	}
	c.Assert(w.tick(), check.IsNil)
	c.Assert(w.Max(), check.Equals, uint64(10))
}
