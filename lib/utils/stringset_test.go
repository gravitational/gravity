package utils

import "gopkg.in/check.v1"

type StringSetSuite struct{}

var _ = check.Suite(&StringSetSuite{})

func (s *StringSetSuite) TestEverything(c *check.C) {
	set := NewStringSet()

	set.Add("one")
	set.Add("two")
	set.Add("two")
	c.Assert(set, check.HasLen, 2)

	set.Remove("two")
	c.Assert(set, check.HasLen, 1)

	another := NewStringSet()
	another.Add("1")
	another.Add("2")
	another.Add("3")

	set.AddSet(another)
	c.Assert(set.Slice(), check.DeepEquals, []string{"1", "2", "3", "one"})

	set.AddSlice([]string{"bad", "santa"})
	c.Assert(set.Slice(), check.DeepEquals, []string{"1", "2", "3", "bad", "one", "santa"})
}
