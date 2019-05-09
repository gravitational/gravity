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

package process

import (
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/processconfig"

	telecfg "github.com/gravitational/teleport/lib/config"
	teleutils "github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

func TestProcess(t *testing.T) { check.TestingT(t) }

type ConfigSuite struct {
}

var _ = check.Suite(&ConfigSuite{})

func (s *ConfigSuite) TestMergeConfig(c *check.C) {
	config := &processconfig.Config{
		Hostname: "test.example.com",
		Pack: processconfig.PackageServiceConfig{
			AdvertiseAddr: *teleutils.MustParseAddr("0.0.0.0:61009"),
		},
		Users: []processconfig.User{
			{
				Password: "example",
				Type:     "agent",
				Org:      "acme.io",
				Email:    "wizard@acme.io",
			},
		},
	}
	from := &processconfig.Config{
		Hostname: "from.hostname",
		Pack: processconfig.PackageServiceConfig{
			AdvertiseAddr: *teleutils.MustParseAddr("ops.example.com:443"),
		},
		Users: []processconfig.User{
			{
				Owner:    true,
				Password: "test",
				Type:     "admin",
				Org:      "example.com",
				Email:    "alice@example.com",
			},
		},
	}
	err := processconfig.MergeConfig(config, from)
	c.Assert(err, check.IsNil)
	c.Assert(config, compare.DeepEquals, &processconfig.Config{
		Hostname: "from.hostname",
		Pack: processconfig.PackageServiceConfig{
			AdvertiseAddr: *teleutils.MustParseAddr("ops.example.com:443"),
		},
		Users: []processconfig.User{
			{
				Password: "example",
				Type:     "agent",
				Org:      "acme.io",
				Email:    "wizard@acme.io",
			},
			{
				Owner:    true,
				Password: "test",
				Type:     "admin",
				Org:      "example.com",
				Email:    "alice@example.com",
			},
		},
	})
}

func (s *ConfigSuite) TestMergeTeleConfig(c *check.C) {
	config := WizardTeleportConfig("example.com", "statedir")
	c.Assert(config, check.NotNil)

	from := &telecfg.FileConfig{
		Global: telecfg.Global{
			AdvertiseIP: "127.0.0.2",
		},
		Auth: telecfg.Auth{
			ClusterName: "test.example.com",
			OIDCConnectors: []telecfg.OIDCConnector{
				{
					ID:           "test",
					RedirectURL:  "https://test.example.com",
					ClientID:     "testclientid",
					ClientSecret: "secret",
					IssuerURL:    "https://auth.example.com",
				},
			},
		},
		Proxy: telecfg.Proxy{
			KeyFile:  "/tmp/key.pem",
			CertFile: "/tmp/cert.pem",
		},
	}
	err := processconfig.MergeTeleConfig(config, from)
	c.Assert(err, check.IsNil)
	c.Assert(config.AdvertiseIP, check.Equals, from.AdvertiseIP)
	c.Assert(config.Auth.ClusterName, check.Equals, from.Auth.ClusterName)
	c.Assert(config.Auth.OIDCConnectors, check.DeepEquals, from.Auth.OIDCConnectors)
}
