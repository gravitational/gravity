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

package ops

import (
	"fmt"
	"strconv"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"gopkg.in/check.v1"
)

type EndpointsSuite struct{}

var _ = check.Suite(&EndpointsSuite{})

func (s *EndpointsSuite) TestClusterEndpoints(c *check.C) {
	master1 := storage.Server{
		AdvertiseIP: "192.168.1.1",
		ClusterRole: string(schema.ServiceRoleMaster),
	}
	node := storage.Server{
		AdvertiseIP: "192.168.1.2",
		ClusterRole: string(schema.ServiceRoleNode),
	}
	master2 := storage.Server{
		AdvertiseIP: "192.168.1.3",
		ClusterRole: string(schema.ServiceRoleMaster),
	}
	cluster := &Site{
		ClusterState: storage.ClusterState{
			Servers: []storage.Server{
				master1, node, master2,
			},
		},
	}
	gateway := storage.DefaultAuthGateway()

	endpoints, err := getClusterEndpoints(cluster, gateway)
	c.Assert(err, check.IsNil)
	c.Assert(endpoints, check.DeepEquals, &ClusterEndpoints{
		Internal: clusterEndpoints{
			AuthGateways: []string{
				utils.EnsurePort(master1.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
				utils.EnsurePort(master2.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
			},
			ManagementURLs: []string{
				utils.EnsurePortURL(master1.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
				utils.EnsurePortURL(master2.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
			},
		},
		Public: clusterEndpoints{},
	})
	c.Assert(endpoints.AuthGateways(), check.DeepEquals, []string{
		utils.EnsurePort(master1.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
		utils.EnsurePort(master2.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
	})
	c.Assert(endpoints.FirstAuthGateway(), check.Equals,
		utils.EnsurePort(master1.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)))
	c.Assert(endpoints.ManagementURLs(), check.DeepEquals, []string{
		utils.EnsurePortURL(master1.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
		utils.EnsurePortURL(master2.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
	})

	publicAddr := "cluster.example.com:444"
	gateway.SetPublicAddrs([]string{publicAddr})

	endpoints, err = getClusterEndpoints(cluster, gateway)
	c.Assert(err, check.IsNil)
	c.Assert(endpoints, check.DeepEquals, &ClusterEndpoints{
		Internal: clusterEndpoints{
			AuthGateways: []string{
				utils.EnsurePort(master1.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
				utils.EnsurePort(master2.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
			},
			ManagementURLs: []string{
				utils.EnsurePortURL(master1.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
				utils.EnsurePortURL(master2.AdvertiseIP, strconv.Itoa(defaults.GravitySiteNodePort)),
			},
		},
		Public: clusterEndpoints{
			AuthGateways: []string{
				publicAddr,
			},
			ManagementURLs: []string{
				fmt.Sprintf("https://%v", publicAddr),
			},
		},
	})
	c.Assert(endpoints.AuthGateways(), check.DeepEquals, []string{
		publicAddr,
	})
	c.Assert(endpoints.FirstAuthGateway(), check.Equals, publicAddr)
	c.Assert(endpoints.ManagementURLs(), check.DeepEquals, []string{
		fmt.Sprintf("https://%v", publicAddr),
	})
}
