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

package clusterconfig

import (
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) TestParsesClusterConfiguration(c *C) {
	testCases := []struct {
		in       string
		resource *Resource
		error    error
		comment  string
	}{
		{
			in:      `{}`,
			error:   trace.BadParameter("failed to validate: name: name is required"),
			comment: "chokes on empty json",
		},
		{
			in:      `{"kind": "ClusterConfiguration"}`,
			error:   trace.BadParameter("failed to validate: name: name is required"),
			comment: "invalid with missing required fields",
		},
		{
			in: `kind: clusterconfiguration
version: v1
metadata:
  name: foo
spec: {}`,
			resource: &Resource{
				Kind:    storage.KindClusterConfiguration,
				Version: "v1",
				Metadata: teleservices.Metadata{
					Name:      constants.ClusterConfigurationMap,
					Namespace: defaults.KubeSystemNamespace,
				},
				Spec: Spec{},
			},
			comment: "overrides metadata.name and metadata.namespace",
		},
		{
			in: `kind: clusterconfiguration
version: v1
spec:
  global:
    cloudProvider: aws
    cloudConfig: |
      [Global]
      username=user
      password=pass`,
			resource: &Resource{
				Kind:    storage.KindClusterConfiguration,
				Version: "v1",
				Metadata: teleservices.Metadata{
					Name:      constants.ClusterConfigurationMap,
					Namespace: defaults.KubeSystemNamespace,
				},
				Spec: Spec{
					Global: &Global{
						CloudProvider: "aws",
						CloudConfig: CloudConfig{
							Config: `[Global]
username=user
password=pass`,
						},
					},
				},
			},
			comment: "correctly parses the spec",
		},
		{
			in: `kind: clusterconfiguration
version: v1
spec:
  kubelet:
    extraArgs: ['--foo', '--bar=baz']
    config: |
      kind: KubeletConfiguration
      apiVersion: kubelet.config.k8s.io/v1beta1
      address: "0.0.0.0"`,
			resource: &Resource{
				Kind:    storage.KindClusterConfiguration,
				Version: "v1",
				Metadata: teleservices.Metadata{
					Name:      constants.ClusterConfigurationMap,
					Namespace: defaults.KubeSystemNamespace,
				},
				Spec: Spec{
					ComponentConfigs: ComponentConfigs{
						Kubelet: &Kubelet{
							ExtraArgs: []string{"--foo", "--bar=baz"},
							Config: `kind: KubeletConfiguration
apiVersion: kubelet.config.k8s.io/v1beta1
address: "0.0.0.0"`,
						},
					},
				},
			},
			comment: "can specify CLI args",
		},
	}
	for _, tc := range testCases {
		comment := Commentf(tc.comment)
		resource, err := Unmarshal([]byte(tc.in))
		if tc.error != nil {
			c.Assert(err, FitsTypeOf, tc.error, comment)
			continue
		}
		c.Assert(err, IsNil, comment)
		c.Assert(resource, compare.DeepEquals, tc.resource, comment)

		bytes, err := Marshal(resource)
		c.Assert(err, IsNil, comment)

		resource2, err := Unmarshal(bytes)
		c.Assert(err, IsNil, comment)
		c.Assert(resource2, compare.DeepEquals, resource, comment)
	}
}
