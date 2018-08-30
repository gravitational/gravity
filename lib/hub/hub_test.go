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

package hub

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/testutils"

	check "gopkg.in/check.v1"
)

func TestHub(t *testing.T) { check.TestingT(t) }

type HubSuite struct {
	hub Hub
}

var _ = check.Suite(&HubSuite{})

func (s *HubSuite) SetUpSuite(c *check.C) {
	s3 := testutils.NewS3()
	s3.Add(app1, testutils.WithStable())
	s3.Add(app2, testutils.WithLatest())
	hub, err := New(Config{
		Bucket: defaults.HubBucket,
		Prefix: defaults.HubTelekubePrefix,
		Region: defaults.AWSRegion,
		S3:     s3,
	})
	c.Assert(err, check.IsNil)
	s.hub = hub
}

func (s *HubSuite) TestList(c *check.C) {
	apps, err := s.hub.List()
	c.Assert(err, check.IsNil)
	c.Assert(apps, check.DeepEquals, []App{toHubApp(app1), toHubApp(app2)})
}

func (s *HubSuite) TestDownload(c *check.C) {
	reader, err := s.hub.Get(loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       defaults.TelekubePackage,
		Version:    app1.Version,
	})
	c.Assert(err, check.IsNil)
	defer reader.Close()
	bytes, err := ioutil.ReadAll(reader)
	c.Assert(err, check.IsNil)
	c.Assert(bytes, check.DeepEquals, app1.Data)
}

func (s *HubSuite) TestDownloadStable(c *check.C) {
	reader, err := s.hub.Get(loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       defaults.TelekubePackage,
		Version:    loc.StableVersion,
	})
	c.Assert(err, check.IsNil)
	defer reader.Close()
	bytes, err := ioutil.ReadAll(reader)
	c.Assert(err, check.IsNil)
	c.Assert(bytes, check.DeepEquals, app1.Data)
}

func (s *HubSuite) TestDownloadLatest(c *check.C) {
	reader, err := s.hub.Get(loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       defaults.TelekubePackage,
		Version:    loc.LatestVersion,
	})
	c.Assert(err, check.IsNil)
	defer reader.Close()
	bytes, err := ioutil.ReadAll(reader)
	c.Assert(err, check.IsNil)
	c.Assert(bytes, check.DeepEquals, app2.Data)
}

func toHubApp(s3App testutils.S3App) App {
	return App{
		Name:      s3App.Name,
		Version:   s3App.Version,
		Created:   s3App.Created,
		SizeBytes: int64(len(s3App.Data)),
	}
}

var (
	app1 = testutils.S3App{
		Name:     defaults.TelekubePackage,
		Version:  "1.0.0",
		Created:  time.Now(),
		Data:     []byte("version 1 (stable)"),
		Checksum: "c18f45c592cb83bae4b7e2bec437e1e874d0651b104bab3acc29baf99fb83405",
	}
	app2 = testutils.S3App{
		Name:     defaults.TelekubePackage,
		Version:  "2.0.0",
		Created:  time.Now(),
		Data:     []byte("version 2 (latest)"),
		Checksum: "5c99c4996ac2f6d7eb12420f908fc0897360de6011f458716f36e3f14898777e",
	}
)
