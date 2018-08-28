package utils

import (
	"encoding/json"

	"gopkg.in/check.v1"
)

type UnitsSuite struct{}

var _ = check.Suite(&UnitsSuite{})

func (s *UnitsSuite) TestCapacityAndTransferRate(c *check.C) {
	var o capacityAndRate
	err := json.Unmarshal([]byte(`{"capacity": "10GB", "rate": "50MB/s"}`), &o)
	c.Assert(err, check.IsNil)

	c.Assert(o.Capacity.String(), check.Equals, "10GB")
	c.Assert(o.Capacity.Bytes(), check.Equals, uint64(10000000000))

	c.Assert(o.Rate.String(), check.Equals, "50MB/s")
	c.Assert(o.Rate.BytesPerSecond(), check.Equals, uint64(50000000))
}

type capacityAndRate struct {
	Capacity Capacity     `json:"capacity"`
	Rate     TransferRate `json:"rate"`
}
