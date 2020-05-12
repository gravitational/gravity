/*
Copyright 2020 Gravitational, Inc.

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

import (
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"

	"github.com/gravitational/teleport/lib/services"
	"gopkg.in/check.v1"
)

type TrustedClusterSuite struct{}

var _ = check.Suite(&TrustedClusterSuite{})

// TestTrustedClusterDefaults verifies basic trusted cluster resource parsing and
// default field values.
func (s *TrustedClusterSuite) TestTrustedClusterDefaults(c *check.C) {
	spec := `kind: trusted_cluster
version: v2
metadata:
  name: hub.example.com
spec:
  enabled: true
  token: trusted_cluster_token
  tunnel_addr: "hub.example.com:3024"
  web_proxy_addr: "hub.example.com:32009"
`
	tc, err := UnmarshalTrustedCluster([]byte(spec))
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, tc, NewTrustedCluster("hub.example.com", TrustedClusterSpecV2{
		Enabled:              true,
		Token:                "trusted_cluster_token",
		ProxyAddress:         "hub.example.com:32009",
		ReverseTunnelAddress: "hub.example.com:3024",
		RoleMap: services.RoleMap{
			{Remote: constants.RoleAdmin, Local: []string{constants.RoleAdmin}},
		},
	}))
}

// TestTrustedClusterRoles makes sure roles field can be set.
func (s *TrustedClusterSuite) TestTrustedClusterRoles(c *check.C) {
	spec := `kind: trusted_cluster
version: v2
metadata:
  name: hub.example.com
spec:
  enabled: true
  token: trusted_cluster_token
  tunnel_addr: "hub.example.com:3024"
  web_proxy_addr: "hub.example.com:32009"
  roles: ["admin", "developer"]
`
	tc, err := UnmarshalTrustedCluster([]byte(spec))
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, tc, NewTrustedCluster("hub.example.com", TrustedClusterSpecV2{
		Enabled:              true,
		Token:                "trusted_cluster_token",
		ProxyAddress:         "hub.example.com:32009",
		ReverseTunnelAddress: "hub.example.com:3024",
		Roles:                []string{"admin", "developer"},
	}))
}

// TestTrustedClusterRoleMap makes sure roles are not populated when role_map
// is set.
func (s *TrustedClusterSuite) TestTrustedClusterRoleMap(c *check.C) {
	spec := `kind: trusted_cluster
version: v2
metadata:
  name: hub.example.com
spec:
  enabled: true
  token: trusted_cluster_token
  tunnel_addr: "hub.example.com:3024"
  web_proxy_addr: "hub.example.com:32009"
  role_map:
  - remote: "admin"
    local: ["admin"]
  - remote: "developer"
    local: ["developer", "viewer"]
`
	tc, err := UnmarshalTrustedCluster([]byte(spec))
	c.Assert(err, check.IsNil)
	compare.DeepCompare(c, tc, NewTrustedCluster("hub.example.com", TrustedClusterSpecV2{
		Enabled:              true,
		Token:                "trusted_cluster_token",
		ProxyAddress:         "hub.example.com:32009",
		ReverseTunnelAddress: "hub.example.com:3024",
		RoleMap: services.RoleMap{
			{
				Remote: "admin",
				Local:  []string{"admin"},
			},
			{
				Remote: "developer",
				Local:  []string{"developer", "viewer"},
			},
		},
	}))
}

// TestTrustedClusterRolesAndRoleMaps makes sure roles and role_map can't be both set.
func (s *TrustedClusterSuite) TestTrustedClusterRolesAndRoleMaps(c *check.C) {
	spec := `kind: trusted_cluster
version: v2
metadata:
  name: hub.example.com
spec:
  enabled: true
  token: trusted_cluster_token
  tunnel_addr: "hub.example.com:3024"
  web_proxy_addr: "hub.example.com:32009"
  roles: ["admin"]
  role_map:
  - remote: "admin"
    local: ["admin"]
  - remote: "developer"
    local: ["developer", "viewer"]
`
	_, err := UnmarshalTrustedCluster([]byte(spec))
	c.Assert(err, check.NotNil)
}
