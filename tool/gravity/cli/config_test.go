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

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	"gopkg.in/check.v1"
)

func TestCLI(t *testing.T) { check.TestingT(t) }

type S struct{}

var _ = check.Suite(&S{})

func (*S) TestUpdatesResourcesProperly(c *check.C) {
	config := &InstallConfig{
		CloudProvider: "generic",
		AdvertiseAddr: "10.128.0.18",
		ServiceCIDR:   "100.100.0.0/16",
		PodCIDR:       "100.96.0.0/16",
	}
	const fileConfig = `---
kind: clusterconfiguration
version: v1
spec:
  global:
    cloudProvider: gce
    serviceCIDR: "100.200.0.0/16"
    podSubnetSize: "26"
    cloudConfig: |
      [global]
      node-tags=example-cluster
      multizone="true"
      token-url="https://path/to/endpoint"
---
kind: runtimeenvironment
version: v1
metadata:
  name: runtimeenvironment
spec:
  data:
    EXAMPLE: "value"
`

	k8sResources, gravityResources, err := resources.Split(strings.NewReader(fileConfig))
	c.Assert(err, check.IsNil)
	c.Assert(len(k8sResources), check.Equals, 0)

	resources, err := config.updateClusterConfig(gravityResources)
	c.Assert(err, check.IsNil)
	var expected = []map[string]interface{}{
		runtimeEnvToMap(c, "EXAMPLE", "value"),
		// updateClusterConfig pushes the cluster configuration resource at the end
		clusterConfigToMap(clusterconfig.New(clusterconfig.Spec{
			Global: clusterconfig.Global{
				CloudProvider: "gce",
				ServiceCIDR:   "100.200.0.0/16",
				PodCIDR:       "100.96.0.0/16",
				PodSubnetSize: "26",
				CloudConfig: `[global]
node-tags=example-cluster
multizone="true"
token-url="https://path/to/endpoint"
`,
			},
		}), c),
	}
	c.Assert(resourcesToMap(resources, c), check.DeepEquals, expected)
}

func clusterConfigToMap(res *clusterconfig.Resource, c *check.C) (result map[string]interface{}) {
	bytes, err := clusterconfig.Marshal(res)
	c.Assert(err, check.IsNil)
	c.Assert(json.Unmarshal(bytes, &result), check.IsNil)
	return result
}

func runtimeEnvToMap(c *check.C, keyValues ...string) (result map[string]interface{}) {
	kvs := make(map[string]string, len(keyValues)/2)
	for i := 0; i < len(keyValues); i += 2 {
		kvs[keyValues[i]] = keyValues[i+1]
	}
	env := storage.NewEnvironment(kvs)
	bytes, err := storage.MarshalEnvironment(env)
	c.Assert(err, check.IsNil)
	c.Assert(json.Unmarshal(bytes, &result), check.IsNil)
	return result
}

func resourcesToMap(resources []storage.UnknownResource, c *check.C) (result []map[string]interface{}) {
	for _, item := range resources {
		var res map[string]interface{}
		c.Assert(json.Unmarshal(item.Raw, &res), check.IsNil)
		result = append(result, res)
	}
	return result
}
