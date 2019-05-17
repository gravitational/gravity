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

import (
	"time"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/utils"

	teleconfig "github.com/gravitational/teleport/lib/config"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"

	check "gopkg.in/check.v1"
)

type AuthGatewaySuite struct{}

var _ = check.Suite(&AuthGatewaySuite{})

func (s *AuthGatewaySuite) TestResourceParsing(c *check.C) {
	spec := `kind: authgateway
version: v1
spec:
  connection_limits:
    max_connections: 2000
    max_users: 20
  authentication:
    type: oidc
    second_factor: "off"
    connector_name: google
  client_idle_timeout: 60s
  disconnect_expired_cert: true
  public_addr: ["example.com"]
  ssh_public_addr: ["ssh.example.com"]
  kubernetes_public_addr: ["k8s.example.com"]
  web_public_addr: ["web.example.com"]
`
	gw, err := UnmarshalAuthGateway([]byte(spec))
	c.Assert(err, check.IsNil)
	c.Assert(gw, compare.DeepEquals, NewAuthGateway(AuthGatewaySpecV1{
		ConnectionLimits: &ConnectionLimits{
			MaxConnections: utils.Int64Ptr(2000),
			MaxUsers:       utils.IntPtr(20),
		},
		Authentication: &teleservices.AuthPreferenceSpecV2{
			Type:          "oidc",
			SecondFactor:  "off",
			ConnectorName: "google",
		},
		ClientIdleTimeout:     utils.DurationPtr(60 * time.Second),
		DisconnectExpiredCert: teleservices.NewBoolOption(true),
		PublicAddr:            &[]string{"example.com"},
		SSHPublicAddr:         &[]string{"ssh.example.com"},
		KubernetesPublicAddr:  &[]string{"k8s.example.com"},
		WebPublicAddr:         &[]string{"web.example.com"},
	}))
}

func (s *AuthGatewaySuite) TestPrincipalsChanged(c *check.C) {
	testCases := []struct {
		gw1, gw2 AuthGateway
		result   bool
	}{
		{
			gw1: NewAuthGateway(AuthGatewaySpecV1{}),
			gw2: NewAuthGateway(AuthGatewaySpecV1{
				PublicAddr: &[]string{"example.com"},
			}),
			result: true,
		},
		{
			gw1: NewAuthGateway(AuthGatewaySpecV1{
				PublicAddr: &[]string{"example.com"},
			}),
			gw2: NewAuthGateway(AuthGatewaySpecV1{
				SSHPublicAddr: &[]string{"ssh.example.com"},
			}),
			result: true,
		},
		{
			gw1: NewAuthGateway(AuthGatewaySpecV1{
				PublicAddr: &[]string{"example.com"},
			}),
			gw2: NewAuthGateway(AuthGatewaySpecV1{
				SSHPublicAddr:        &[]string{"example.com"},
				KubernetesPublicAddr: &[]string{"example.com"},
				WebPublicAddr:        &[]string{"example.com"},
			}),
			result: false,
		},
		{
			gw1: NewAuthGateway(AuthGatewaySpecV1{
				PublicAddr: &[]string{"example.com"},
			}),
			gw2: NewAuthGateway(AuthGatewaySpecV1{
				SSHPublicAddr:        &[]string{"example.com"},
				KubernetesPublicAddr: &[]string{"example.com"},
			}),
			result: true,
		},
		{
			gw1: NewAuthGateway(AuthGatewaySpecV1{
				KubernetesPublicAddr: &[]string{"k8s.example.com:3036"},
			}),
			gw2: NewAuthGateway(AuthGatewaySpecV1{
				KubernetesPublicAddr: &[]string{"k8s.example.com:3027"},
			}),
			result: false,
		},
	}
	for _, tc := range testCases {
		c.Assert(tc.gw1.PrincipalsChanged(tc.gw2), check.Equals, tc.result,
			check.Commentf("Test case %s/%s/%v failed.", tc.gw1, tc.gw2, tc.result))
	}
}

func (s *AuthGatewaySuite) TestResourceValidation(c *check.C) {
	tests := []struct {
		desc string
		spec string
	}{
		{
			desc: "Invalid connections limit",
			spec: `kind: authgateway
version: v1
spec:
  connection_limits:
    max_connections: -10`,
		},
		{
			desc: "Invalid users limit",
			spec: `kind: authgateway
version: v1
spec:
  connection_limits:
    max_users: abc`,
		},
		{
			desc: "Invalid auth preference",
			spec: `kind: authgateway
version: v1
spec:
  authentication:
    type: g00gle
    second_factor: "off"`,
		},
		{
			desc: "Invalid principal (empty address)",
			spec: `kind: authgateway
version: v1
spec:
  ssh_public_addr: [""]`,
		},
	}
	for _, t := range tests {
		_, err := UnmarshalAuthGateway([]byte(t.spec))
		c.Assert(err, check.NotNil, check.Commentf("Test case %q failed.", t.desc))
	}
}

func (s *AuthGatewaySuite) TestSettingsChanged(c *check.C) {
	testCases := []struct {
		gw1, gw2 AuthGateway
		result   bool
	}{
		{
			gw1: NewAuthGateway(AuthGatewaySpecV1{
				ConnectionLimits: &ConnectionLimits{
					MaxConnections: utils.Int64Ptr(1000),
					MaxUsers:       utils.IntPtr(10),
				},
			}),
			gw2: NewAuthGateway(AuthGatewaySpecV1{
				ConnectionLimits: &ConnectionLimits{
					MaxConnections: utils.Int64Ptr(1500),
					MaxUsers:       utils.IntPtr(10),
				},
			}),
			result: true,
		},
		{
			gw1: NewAuthGateway(AuthGatewaySpecV1{
				SSHPublicAddr: &[]string{"example.com"},
			}),
			gw2: NewAuthGateway(AuthGatewaySpecV1{
				SSHPublicAddr: &[]string{"ssh.example.com"},
			}),
			result: false,
		},
	}
	for _, tc := range testCases {
		c.Assert(tc.gw1.SettingsChanged(tc.gw2), check.Equals, tc.result,
			check.Commentf("Test case %s/%s/%v failed.", tc.gw1, tc.gw2, tc.result))
	}
}

func (s *AuthGatewaySuite) TestApplyTo(c *check.C) {
	gw1 := NewAuthGateway(AuthGatewaySpecV1{
		ConnectionLimits: &ConnectionLimits{
			MaxConnections: utils.Int64Ptr(1000),
			MaxUsers:       utils.IntPtr(10),
		},
		SSHPublicAddr: &[]string{"ssh.example.com"},
	})
	gw2 := NewAuthGateway(AuthGatewaySpecV1{
		ConnectionLimits: &ConnectionLimits{
			MaxUsers: utils.IntPtr(5),
		},
		PublicAddr: &[]string{"example.com"},
	})
	gw2.ApplyTo(gw1)
	c.Assert(gw1, compare.DeepEquals, NewAuthGateway(AuthGatewaySpecV1{
		ConnectionLimits: &ConnectionLimits{
			MaxConnections: utils.Int64Ptr(1000),
			MaxUsers:       utils.IntPtr(5),
		},
		SSHPublicAddr:        &[]string{"example.com"},
		KubernetesPublicAddr: &[]string{"example.com"},
		WebPublicAddr:        &[]string{"example.com"},
	}))
}

func (s *AuthGatewaySuite) TestApplyToTeleportConfig(c *check.C) {
	gw := NewAuthGateway(AuthGatewaySpecV1{
		ConnectionLimits: &ConnectionLimits{
			MaxConnections: utils.Int64Ptr(1000),
			MaxUsers:       utils.IntPtr(10),
		},
		ClientIdleTimeout: utils.DurationPtr(60 * time.Second),
		Authentication: &teleservices.AuthPreferenceSpecV2{
			Type:         "oidc",
			SecondFactor: "off",
		},
		PublicAddr:           &[]string{"example.com"},
		KubernetesPublicAddr: &[]string{"k8s.example.com"},
	})
	var config teleconfig.FileConfig
	gw.ApplyToTeleportConfig(&config)
	c.Assert(config, compare.DeepEquals, teleconfig.FileConfig{
		Global: teleconfig.Global{
			Limits: teleconfig.ConnectionLimits{
				MaxConnections: 1000,
				MaxUsers:       10,
			},
		},
		Auth: teleconfig.Auth{
			ClientIdleTimeout: teleservices.NewDuration(60 * time.Second),
			Authentication: &teleconfig.AuthenticationConfig{
				Type:         "oidc",
				SecondFactor: "off",
			},
			PublicAddr: teleutils.Strings([]string{"example.com"}),
		},
		Proxy: teleconfig.Proxy{
			PublicAddr:    teleutils.Strings([]string{"example.com"}),
			SSHPublicAddr: teleutils.Strings([]string{"example.com"}),
			Kube: teleconfig.Kube{
				PublicAddr: teleutils.Strings([]string{"k8s.example.com"}),
			},
		},
	})
}
