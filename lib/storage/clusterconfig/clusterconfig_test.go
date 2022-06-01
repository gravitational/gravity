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
	"encoding/json"
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) TestParsesClusterConfiguration(c *C) {
	testCases := []struct {
		in       string
		resource *Resource
		error    error
		validate func(obtained, expected *Resource, c *C)
		comment  string
	}{
		{
			in:      `{}`,
			error:   trace.BadParameter(`clusterconfiguration resource version "" is not supported`),
			comment: "fails on empty json",
		},
		{
			in:      `{"kind": "ClusterConfiguration"}`,
			error:   trace.BadParameter(`clusterconfiguration resource version "" is not supported`),
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
					Global: Global{
						CloudProvider: "aws",
						CloudConfig: `[Global]
username=user
password=pass`,
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
    config:
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
						},
					},
				},
			},
			validate: validate(kubeletConfiguration{
				TypeMeta: metav1.TypeMeta{
					Kind:       "KubeletConfiguration",
					APIVersion: "kubelet.config.k8s.io/v1beta1",
				},
				Address: "0.0.0.0",
			}),
			comment: "can specify CLI args",
		},
		{
			in: `kind: clusterconfiguration
version: v1
spec:
  kubelet:
    config:
      kind: KubeletConfiguration
      apiVersion: kubelet.config.k8s.io/v1beta1
      address: 12`,
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
						},
					},
				},
			},
			error:   trace.BadParameter("failed to validate: spec.kubelet.config.address: Invalid type. Expected: string, given: integer"),
			comment: "validates kubelet configuration",
		},
		{
			in: `kind: clusterconfiguration
version: v1
spec:
  global:
    featureGates:
      FeatureA: true
      FeatureB: false
    podSubnetSize: "26"
    highAvailability: true
    serfEncryption: true`,
			resource: &Resource{
				Kind:    storage.KindClusterConfiguration,
				Version: "v1",
				Metadata: teleservices.Metadata{
					Name:      constants.ClusterConfigurationMap,
					Namespace: defaults.KubeSystemNamespace,
				},
				Spec: Spec{
					Global: Global{
						FeatureGates: map[string]bool{
							"FeatureA": true,
							"FeatureB": false,
						},
						PodSubnetSize:    "26",
						HighAvailability: true,
						SerfEncryption:   utils.BoolPtr(true),
					},
				},
			},
			comment: "consumes global configuration",
		},
	}
	for _, tc := range testCases {
		comment := Commentf(tc.comment)
		resource, err := Unmarshal([]byte(tc.in))
		if tc.error != nil {
			c.Assert(err, FitsTypeOf, tc.error, comment)
			c.Assert(err, ErrorMatches, tc.error.Error(), comment)
			continue
		}
		c.Assert(err, IsNil, comment)
		if tc.validate != nil {
			tc.validate(resource, tc.resource, c)
		} else {
			c.Assert(resource, compare.DeepEquals, tc.resource, comment)
		}

		bytes, err := Marshal(resource)
		c.Assert(err, IsNil, comment)

		resource2, err := Unmarshal(bytes)
		c.Assert(err, IsNil, comment)
		if tc.validate != nil {
			tc.validate(resource2, tc.resource, c)
		} else {
			c.Assert(resource2, compare.DeepEquals, tc.resource, comment)
		}
	}
}

func (*S) TestMergesClusterConfiguration(c *C) {
	var testCases = []struct {
		existing Resource
		update   Resource
		expected Resource
		comment  string
	}{
		{
			update: Resource{
				Spec: Spec{
					ComponentConfigs: ComponentConfigs{
						Kubelet: &Kubelet{
							ExtraArgs: []string{"--node-labels=foo=bar"},
						},
					},
					Global: Global{
						PodCIDR:       "10.244.0.0/16",
						ServiceCIDR:   "100.10.0.0/16",
						PodSubnetSize: "26",
						FeatureGates: map[string]bool{
							"feature1": true,
							"feature2": false,
						},
						HighAvailability: true,
						SerfEncryption:   utils.BoolPtr(false),
					},
				},
			},
			expected: Resource{
				Spec: Spec{
					ComponentConfigs: ComponentConfigs{
						Kubelet: &Kubelet{
							ExtraArgs: []string{"--node-labels=foo=bar"},
						},
					},
					Global: Global{
						PodCIDR:       "10.244.0.0/16",
						ServiceCIDR:   "100.10.0.0/16",
						PodSubnetSize: "26",
						FeatureGates: map[string]bool{
							"feature1": true,
							"feature2": false,
						},
						HighAvailability: true,
						SerfEncryption:   utils.BoolPtr(false),
					},
				},
			},
			comment: "overrides source fields from non-empty fields in update",
		},
		{
			existing: Resource{
				Spec: Spec{
					ComponentConfigs: ComponentConfigs{
						Kubelet: &Kubelet{
							ExtraArgs: []string{"--node-labels=foo=bar"},
							Config: []byte(`
kind: KubeletConfiguration
apiVersion: kubelet.config.k8s.io/v1beta1
address: 10.0.0.1
`),
						},
					},
					Global: Global{
						PodCIDR:        "10.244.0.0/16",
						ServiceCIDR:    "100.10.0.0/16",
						PodSubnetSize:  "26",
						ProxyPortRange: "8080-8081",
						FeatureGates: map[string]bool{
							"feature1": true,
							"feature2": false,
						},
						SerfEncryption: utils.BoolPtr(true),
					},
				},
			},
			update: Resource{
				Spec: Spec{
					ComponentConfigs: ComponentConfigs{
						Kubelet: &Kubelet{
							ExtraArgs: []string{
								"--node-labels=baz=qux",
								"--hostname-override=example.com",
							},
						},
					},
					Global: Global{
						PodCIDR: "10.245.0.0/16",
						FeatureGates: map[string]bool{
							"feature1": true,
							"feature3": true,
						},
					},
				},
			},
			expected: Resource{
				Spec: Spec{
					ComponentConfigs: ComponentConfigs{
						Kubelet: &Kubelet{
							ExtraArgs: []string{
								"--node-labels=baz=qux",
								"--hostname-override=example.com",
							},
							Config: []byte(`
kind: KubeletConfiguration
apiVersion: kubelet.config.k8s.io/v1beta1
address: 10.0.0.1
`),
						},
					},
					Global: Global{
						PodCIDR:        "10.245.0.0/16",
						ServiceCIDR:    "100.10.0.0/16",
						PodSubnetSize:  "26",
						ProxyPortRange: "8080-8081",
						FeatureGates: map[string]bool{
							"feature1": true,
							"feature3": true,
						},
						SerfEncryption: utils.BoolPtr(true),
					},
				},
			},
			comment: "does not override source field from empty update field",
		},
		{
			existing: Resource{
				Spec: Spec{

					Global: Global{
						CloudProvider: "generic",
					},
				},
			},
			update: Resource{
				Spec: Spec{
					Global: Global{
						CloudProvider: "gce",
					},
				},
			},
			expected: Resource{
				Spec: Spec{
					Global: Global{
						CloudProvider: "generic",
					},
				},
			},
			comment: "does not override cloud provider field",
		},
	}
	for _, testCase := range testCases {
		comment := Commentf(testCase.comment)
		merged := testCase.existing.Merge(testCase.update)
		c.Assert(merged, DeepEquals, testCase.expected, comment)
	}
}

func validate(expectedConfig kubeletConfiguration) func(obtained, expected *Resource, c *C) {
	return func(obtained, expected *Resource, c *C) {
		configBytes := obtained.Spec.ComponentConfigs.Kubelet.Config
		var config kubeletConfiguration
		err := json.Unmarshal(configBytes, &config)
		c.Assert(err, IsNil)
		obtained.Spec.ComponentConfigs.Kubelet.Config = nil
		c.Assert(obtained, compare.DeepEquals, expected)
		obtained.Spec.ComponentConfigs.Kubelet.Config = configBytes
		c.Assert(config, compare.DeepEquals, expectedConfig)
	}
}

type kubeletConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	Address         string `json:"address"`
}
