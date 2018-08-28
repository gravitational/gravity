package opsservice

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

type ReportSuite struct{}

var _ = check.Suite(&ReportSuite{})

func (s *ReportSuite) TestClusterInfo(c *check.C) {
	cluster := storage.Site{
		Domain:    "example.com",
		AccountID: defaults.SystemAccountID,
		License:   "license",
	}
	var b bytes.Buffer
	err := collectSiteInfo(cluster)(func(name string) (io.WriteCloser, error) {
		return &nopCloser{&b}, nil
	}, site{})
	c.Assert(err, check.IsNil)
	var fromReport storage.Site
	c.Assert(json.Unmarshal(b.Bytes(), &fromReport), check.IsNil)
	c.Assert(fromReport.Domain, check.Equals, cluster.Domain)
	c.Assert(fromReport.AccountID, check.Equals, cluster.AccountID)
	c.Assert(fromReport.License, check.Equals, "redacted")
}

type nopCloser struct {
	io.Writer
}

func (b *nopCloser) Close() error {
	return nil
}
