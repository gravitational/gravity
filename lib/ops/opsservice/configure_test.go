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
	"fmt"

	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

type ConfigureSuite struct {
	site *site
}

var _ = check.Suite(&ConfigureSuite{})

func (s *ConfigureSuite) SetUpTest(c *check.C) {
	s.site = &site{domainName: "example.com"}
}

func (s *ConfigureSuite) TestEtcdConfigAllFullMembers(c *check.C) {
	servers := makeServers(3, 3)
	config := s.site.prepareEtcdConfig(&operationContext{
		provisionedServers: servers,
	})
	for _, server := range servers {
		c.Assert(config[server.AdvertiseIP].proxyMode, check.Equals, etcdProxyOff)
	}
}

func (s *ConfigureSuite) TestEtcdConfigHasProxies(c *check.C) {
	servers := makeServers(3, 5)
	config := s.site.prepareEtcdConfig(&operationContext{
		provisionedServers: servers,
	})
	for i, server := range servers {
		if i < 3 {
			c.Assert(config[server.AdvertiseIP].proxyMode, check.Equals, etcdProxyOff)
		} else {
			c.Assert(config[server.AdvertiseIP].proxyMode, check.Equals, etcdProxyOn)
		}
	}
}

func makeServers(masters int, total int) provisionedServers {
	var servers provisionedServers
	for i := 0; i < total; i++ {
		var role schema.ServiceRole
		if i < masters {
			role = schema.ServiceRoleMaster
		} else {
			role = schema.ServiceRoleNode
		}
		servers = append(servers, &ProvisionedServer{
			Profile: schema.NodeProfile{
				ServiceRole: role,
			},
			Server: storage.Server{
				AdvertiseIP: fmt.Sprintf("10.10.0.%v", i),
			},
		})
	}
	return servers
}
