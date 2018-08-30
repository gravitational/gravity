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

package devicemapper

import (
	"strings"
	"testing"

	"github.com/gravitational/gravity/lib/storage"

	. "gopkg.in/check.v1"
)

func TestDevices(t *testing.T) { TestingT(t) }

type DeviceSuite struct{}

var _ = Suite(&DeviceSuite{})

func (r *DeviceSuite) TestParsesDevices(c *C) {
	const input = `
NAME="xvda" TYPE="disk" SIZE="10000" PKNAME="" FSTYPE=""
NAME="xvda1" TYPE="part" SIZE="5000" PKNAME="xvda" FSTYPE="xfs"
NAME="xvda2" TYPE="part" SIZE="5000" PKNAME="xvda" FSTYPE="LVM2_member"
NAME="xvdf" TYPE="disk" SIZE="15728640" PKNAME="" FSTYPE=""
NAME="docker-thinpool_tmeta" TYPE="lvm" SIZE="1000" PKNAME="" FSTYPE="LVM2_member"
NAME="docker-thinpool" TYPE="lvm" SIZE="1000" PKNAME="" FSTYPE="LVM2_member"
NAME="docker-thinpool_tdata" TYPE="lvm" SIZE="1000" PKNAME="" FSTYPE="LVM2_member"
NAME="docker-thinpool" TYPE="lvm" SIZE="1000" PKNAME="" FSTYPE="LVM2_member"
	`

	disks, err := parseDevices(strings.NewReader(input))
	c.Assert(err, IsNil)
	c.Assert(disks, DeepEquals, []storage.Device{
		{Name: storage.DeviceName("/dev/xvdf"), Type: storage.DeviceDisk, SizeMB: 15},
	})
}

func (r *DeviceSuite) TestRejectsInvalidInput(c *C) {
	const input = `
NAME="xvda", ="disk"
NAME="docker-thinpool" TYPE="lvm"`

	disks, err := parseDevices(strings.NewReader(input))
	c.Assert(err, ErrorMatches, ".*expected Ident but got \",\".*")
	c.Assert(disks, IsNil)
}
