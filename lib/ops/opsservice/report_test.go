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

package opsservice

import (
	"bytes"
	"context"
	"encoding/json"
	"io"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

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
	w := &bufferWriter{}
	err := collectClusterInfo(cluster)(context.TODO(), w, site{})
	c.Assert(err, check.IsNil)
	var fromReport storage.Site
	c.Assert(json.Unmarshal(w.Bytes(), &fromReport), check.IsNil)
	c.Assert(fromReport.Domain, check.Equals, cluster.Domain)
	c.Assert(fromReport.AccountID, check.Equals, cluster.AccountID)
	c.Assert(fromReport.License, check.Equals, "redacted")
}

type bufferWriter struct {
	bytes.Buffer
}

func (b *bufferWriter) NewWriter(name string) (io.WriteCloser, error) {
	return utils.NopWriteCloser(&b.Buffer), nil
}
