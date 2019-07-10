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

package service

import (
	"testing"

	"github.com/gravitational/gravity/lib/archive"

	. "gopkg.in/check.v1"
)

func TestService(t *testing.T) { TestingT(t) }

type ManifestSuite struct{}

var _ = Suite(&ManifestSuite{})

func (r *ManifestSuite) TestReadsManifestFromUnpacked(c *C) {
	files := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
	}
	input := archive.MustCreateMemArchive(files)
	manifest, _, cleanup, err := manifestFromUnpackedSource(input)
	defer cleanup()
	c.Assert(err, IsNil)
	c.Assert(manifest, NotNil)
}

const manifestBytes = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: sample
  resourceVersion: "0.0.1"
installer:
  flavors:
    prompt: "Test flavors"
    items:
      - name: flavor1
        nodes:
          - profile: master
            count: 1
nodeProfiles:
  - name: master
	description: "control plane server"
	labels:
	  role: master`
