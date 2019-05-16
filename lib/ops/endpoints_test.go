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

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

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
				fmt.Sprintf("%v:%v", master1.AdvertiseIP,
					defaults.GravitySiteNodePort),
				fmt.Sprintf("%v:%v", master2.AdvertiseIP,
					defaults.GravitySiteNodePort),
			},
			ManagementURLs: []string{
				fmt.Sprintf("https://%v:%v", master1.AdvertiseIP,
					defaults.GravitySiteNodePort),
				fmt.Sprintf("https://%v:%v", master2.AdvertiseIP,
					defaults.GravitySiteNodePort),
			},
		},
		Public: clusterEndpoints{},
	})
	c.Assert(endpoints.AuthGateways(), check.DeepEquals, []string{
		fmt.Sprintf("%v:%v", master1.AdvertiseIP,
			defaults.GravitySiteNodePort),
		fmt.Sprintf("%v:%v", master2.AdvertiseIP,
			defaults.GravitySiteNodePort),
	})
	c.Assert(endpoints.AuthGateway(), check.Equals,
		fmt.Sprintf("%v:%v", master1.AdvertiseIP,
			defaults.GravitySiteNodePort))
	c.Assert(endpoints.ManagementURLs(), check.DeepEquals, []string{
		fmt.Sprintf("https://%v:%v", master1.AdvertiseIP,
			defaults.GravitySiteNodePort),
		fmt.Sprintf("https://%v:%v", master2.AdvertiseIP,
			defaults.GravitySiteNodePort),
	})

	publicAddr := "cluster.example.com:444"
	gateway.SetPublicAddrs([]string{publicAddr})

	endpoints, err = getClusterEndpoints(cluster, gateway)
	c.Assert(err, check.IsNil)
	c.Assert(endpoints, check.DeepEquals, &ClusterEndpoints{
		Internal: clusterEndpoints{
			AuthGateways: []string{
				fmt.Sprintf("%v:%v", master1.AdvertiseIP,
					defaults.GravitySiteNodePort),
				fmt.Sprintf("%v:%v", master2.AdvertiseIP,
					defaults.GravitySiteNodePort),
			},
			ManagementURLs: []string{
				fmt.Sprintf("https://%v:%v", master1.AdvertiseIP,
					defaults.GravitySiteNodePort),
				fmt.Sprintf("https://%v:%v", master2.AdvertiseIP,
					defaults.GravitySiteNodePort),
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
	c.Assert(endpoints.AuthGateway(), check.Equals, publicAddr)
	c.Assert(endpoints.ManagementURLs(), check.DeepEquals, []string{
		fmt.Sprintf("https://%v", publicAddr),
	})
}
