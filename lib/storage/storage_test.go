/*
Copyright 2019 Gravitational, Inc.

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

package storage

import check "gopkg.in/check.v1"

type StorageSuite struct{}

var _ = check.Suite(&StorageSuite{})

// TestServersEquality verifies that server lists can be properly compared.
func (s *StorageSuite) TestServersEquality(c *check.C) {
	servers := Servers{{
		AdvertiseIP: "192.168.1.1",
		Hostname:    "node-1",
		Role:        "worker",
	}}
	testCases := []struct {
		servers Servers
		result  bool
		comment string
	}{
		{
			servers: Servers{{
				AdvertiseIP: "192.168.1.1",
				Hostname:    "node-1",
				Role:        "worker",
			}},
			result:  true,
			comment: "Servers should be equal",
		},
		{
			servers: Servers{
				{
					AdvertiseIP: "192.168.1.1",
					Hostname:    "node-1",
					Role:        "worker",
				},
				{
					AdvertiseIP: "192.168.1.2",
					Hostname:    "node-2",
					Role:        "worker",
				},
			},
			result:  false,
			comment: "Servers should not be equal: different number of servers",
		},
		{
			servers: Servers{{
				AdvertiseIP: "192.168.1.2",
				Hostname:    "node-1",
				Role:        "worker",
			}},
			result:  false,
			comment: "Servers should not be equal: different IPs",
		},
		{
			servers: Servers{{
				AdvertiseIP: "192.168.1.1",
				Hostname:    "node-2",
				Role:        "worker",
			}},
			result:  false,
			comment: "Servers should not be equal: different hostnames",
		},
		{
			servers: Servers{{
				AdvertiseIP: "192.168.1.1",
				Hostname:    "node-1",
				Role:        "db",
			}},
			result:  false,
			comment: "Servers should not be equal: different roles",
		},
	}
	for _, tc := range testCases {
		c.Assert(servers.IsEqualTo(tc.servers), check.Equals, tc.result,
			check.Commentf(tc.comment))
	}
}
