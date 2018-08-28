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
