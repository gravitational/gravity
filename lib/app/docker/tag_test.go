package docker

import . "gopkg.in/check.v1"

type TagSpecSuite struct{}

var _ = Suite(&TagSpecSuite{})

func (s *TagSpecSuite) TestEverything(c *C) {
	// success:
	t := TagFromString(" host:1000/repo/simple-app:1.0.2")
	c.Assert(t.IsValid(), Equals, true)
	c.Assert(t.Name, Equals, "host:1000/repo/simple-app")
	c.Assert(t.Version, Equals, "1.0.2")
	c.Assert(t.String(), Equals, "host:1000/repo/simple-app:1.0.2")

	// success with 'latest' version assumed (and no repo)
	t = TagFromString(" host:1000/simple-app")
	c.Assert(t.IsValid(), Equals, true)

	// local tag
	t = TagFromString("simple-app:1.0.2")
	c.Assert(t.String(), Equals, "simple-app:1.0.2")
}
