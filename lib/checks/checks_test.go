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

package checks

import (
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func TestChecks(t *testing.T) { check.TestingT(t) }

type ChecksSuite struct {
	info ServerInfo
}

var _ = check.Suite(&ChecksSuite{})

func (s *ChecksSuite) SetUpSuite(c *check.C) {
	sysinfo := storage.NewSystemInfo(storage.SystemSpecV2{
		Hostname: "foo",
		Filesystems: []storage.Filesystem{
			{
				DirName: "/foo/bar",
				Type:    "tmpfs",
			},
		},
		FilesystemStats: map[string]storage.FilesystemUsage{
			"/foo/bar": {
				TotalKB: 2,
				FreeKB:  1,
			},
		},
		Memory: storage.Memory{Total: 1000, Free: 500, ActualFree: 640},
		NumCPU: 4,
	})
	s.info = ServerInfo{
		System: sysinfo,
	}
}

func (s *ChecksSuite) TestCheckCPU(c *check.C) {
	enoughCPU := schema.CPU{Min: 4}
	c.Assert(checkCPU(s.info, enoughCPU), check.IsNil)

	notEnoughCPU := schema.CPU{Min: 5}
	c.Assert(checkCPU(s.info, notEnoughCPU), check.NotNil)
}

func (s *ChecksSuite) TestCheckRAM(c *check.C) {
	enoughRAM := schema.RAM{Min: 800}
	c.Assert(checkRAM(s.info, enoughRAM), check.IsNil)

	notEnoughRAM := schema.RAM{Min: 1100}
	c.Assert(checkRAM(s.info, notEnoughRAM), check.NotNil)
}

func (s *ChecksSuite) TestTime(c *check.C) {
	server := storage.NewSystemInfo(storage.SystemSpecV2{
		Hostname: "node-1",
	})
	server2 := storage.NewSystemInfo(storage.SystemSpecV2{
		Hostname: "node-2",
	})
	server3 := storage.NewSystemInfo(storage.SystemSpecV2{
		Hostname: "node-3",
	})

	now := time.Date(2016, 12, 1, 2, 3, 40, 5000, time.UTC)
	c.Assert(checkTime(now, nil), check.IsNil)
	// anchor time is the time on the first server
	anchorTime := time.Date(2016, 12, 1, 2, 3, 4, 5000, time.UTC)
	// we have received the first info 10 seconds ago
	localTime := now.Add(-10 * time.Second)
	err := checkTime(now, []Server{
		{ServerInfo: ServerInfo{ServerTime: anchorTime, LocalTime: localTime}}},
	)
	c.Assert(err, check.IsNil)

	// we have received the second info 9 seconds ago
	server2LocalTime := now.Add(-9 * time.Second)
	server2Time := anchorTime.Add(defaults.MaxOutOfSyncTimeDelta / 2).Add(time.Second)
	err = checkTime(now, []Server{
		{ServerInfo: ServerInfo{ServerTime: anchorTime, LocalTime: localTime}},
		{ServerInfo: ServerInfo{ServerTime: server2Time, LocalTime: server2LocalTime}},
	})
	c.Assert(err, check.IsNil)

	// we have received the third info 8 seconds ago
	server3LocalTime := now.Add(-8 * time.Second)
	server3Time := anchorTime.Add(defaults.MaxOutOfSyncTimeDelta + time.Nanosecond).Add(2 * time.Second)
	err = checkTime(now, []Server{
		{ServerInfo: ServerInfo{System: server, ServerTime: anchorTime, LocalTime: localTime}},
		{ServerInfo: ServerInfo{System: server2, ServerTime: server2Time, LocalTime: server2LocalTime}},
		{ServerInfo: ServerInfo{System: server3, ServerTime: server3Time, LocalTime: server3LocalTime}},
	})
	c.Assert(trace.IsBadParameter(err), check.Equals, true, check.Commentf("expected BadParameter, got %v", err))
}
