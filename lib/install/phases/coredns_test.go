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

package phases

import (
	"testing"

	"gopkg.in/check.v1"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { check.TestingT(t) }

type StartSuite struct{}

var _ = check.Suite(&StartSuite{})

func (*StartSuite) TestCoreDNSConf(c *check.C) {
	var configTable = []struct {
		config   coreDNSConfig
		expected string
	}{
		{
			coreDNSConfig{
				Zones: map[string][]string{
					"example.com":  []string{"1.1.1.1", "2.2.2.2"},
					"example2.com": []string{"1.1.1.1", "2.2.2.2"},
				},
				Hosts: map[string]string{
					"override.com":  "5.5.5.5",
					"override2.com": "1.2.3.4",
				},
				UpstreamNameservers: []string{"1.1.1.1", "8.8.8.8"},
			},
			`
.:53 {
  reload
  errors
  health
  prometheus :9153
  cache 30
  loop
  reload
  loadbalance
  hosts { 
    5.5.5.5 override.com
    1.2.3.4 override2.com
    fallthrough
  }
  kubernetes cluster.local in-addr.arpa ip6.arpa {
    pods verified
    fallthrough in-addr.arpa ip6.arpa
  }
  proxy example.com 1.1.1.1 2.2.2.2 {
    policy sequential
  }
  proxy example2.com 1.1.1.1 2.2.2.2 {
    policy sequential
  }
  forward . 1.1.1.1 8.8.8.8 {
    policy sequential
    health_check 0
  }
}
`,
		},
		{
			coreDNSConfig{
				UpstreamNameservers: []string{"1.1.1.1"},
				Rotate:              true,
			},
			`
.:53 {
  reload
  errors
  health
  prometheus :9153
  cache 30
  loop
  reload
  loadbalance
  hosts { 
    fallthrough
  }
  kubernetes cluster.local in-addr.arpa ip6.arpa {
    pods verified
    fallthrough in-addr.arpa ip6.arpa
  }
  forward . 1.1.1.1 {
    policy random
    health_check 0
  }
}
`,
		},
	}

	for _, tt := range configTable {
		config, err := generateCoreDNSConfig(tt.config, coreDNSTemplate)

		c.Assert(err, check.IsNil)
		c.Assert(config, check.Equals, tt.expected)
	}

}
