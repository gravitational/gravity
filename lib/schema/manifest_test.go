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

package schema

import (
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSchema(t *testing.T) { TestingT(t) }

type ManifestSuite struct{}

var _ = Suite(&ManifestSuite{})

func (s *ManifestSuite) TestParseManifest(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: telekube
  resourceVersion: "0.0.1"
logo: file://logo.svg
releaseNotes: |
  This version is nothing but a pure awesomeness!
endpoints:
  - name: "Gravity site"
    description: "Admin control panel"
    selector:
      app: gravity-site
    protocol: https
providers:
  aws:
    network:
      type: aws-vpc
    iamPolicy:
      actions:
        - "ec2:CreateVpc"
        - "ec2:DeleteVpc"
  generic:
    network:
      type: calico
installer:
  setupEndpoints:
    - "Bandwagon"
  flavors:
    prompt: "Select a flavor"
    default: "one"
    items:
      - name: "one"
        description: "1 node"
        nodes:
          - profile: node
            count: 1
      - name: "two"
        description: "2 nodes"
        nodes:
          - profile: node
            count: 2
nodeProfiles:
  - name: node
    description: "Telekube Node"
    labels:
      role: node
    taints:
    - key: node-role.kubernetes.io/master
      effect: NoSchedule
    - key: custom-taint
      value: custom-value
      effect: NoExecute
    requirements:
      cpu:
        min: 1
      ram:
        min: "2GB"
      os:
        - name: rhel
          versions: ["7.2", "7.3"]
      network:
        minTransferRate: "50MB/s"
        ports:
          - protocol: tcp
            ranges:
              - "6443"  # kubernetes apiserver secure
              - "8080"  # kubernetes apiserver insecure
              - "10248-10255" # kubernetes kubelet
      volumes:
        - path: /var/lib/gravity
          capacity: "10GB"
          filesystems: ["xfs", "ext4"]
          uid: 1000
          gid: 1000
          mode: "0755"
    providers:
      aws:
        instanceTypes:
          - m3.xlarge
          - c3.xlarge
  - name: stateless
    description: "Telekube Stateless Node"
    labels:
      node-role.kubernetes.io/node: "true"
dependencies:
  packages:
    - gravitational.io/gravity:0.0.1
  apps:
    - gravitational.io/dns-app:0.0.3
extensions:
  logs:
    disabled: true
  monitoring:
    disabled: true
  kubernetes:
    disabled: true
  configuration:
    disabled: true
systemOptions:
  runtime:
    version: "1.4.6"
  docker:
    storageDriver: overlay
    args: ["--log-level=DEBUG"]
  etcd:
    args: ["-debug"]
  kubelet:
    args: ["--system-reserved=memory=500Mi"]
  dependencies:
    runtimePackage: gravitational.io/planet:0.0.1
`)

	m, err := ParseManifestYAML(bytes)
	c.Assert(err, IsNil)

	compare.DeepCompare(c, m.Header, Header{
		TypeMeta: metav1.TypeMeta{
			APIVersion: APIVersionV2,
			Kind:       KindBundle,
		},
		Metadata: Metadata{
			Name:            "telekube",
			ResourceVersion: "0.0.1",
			Namespace:       "default",
			Repository:      defaults.SystemAccountOrg,
		},
	})
	c.Assert(m.Logo, Equals, "file://logo.svg")
	c.Assert(m.ReleaseNotes, Equals, "This version is nothing but a pure awesomeness!\n")
	compare.DeepCompare(c, m.Endpoints, []Endpoint{
		{
			Name:        "Gravity site",
			Description: "Admin control panel",
			Selector:    map[string]string{"app": "gravity-site"},
			Protocol:    "https",
			Hidden:      false,
		},
	})
	compare.DeepCompare(c, m.Providers, &Providers{
		AWS: AWS{
			Networking: Networking{
				Type: "aws-vpc",
			},
			IAMPolicy: IAMPolicy{
				Version: "2012-10-17",
				Actions: []string{
					"ec2:CreateVpc",
					"ec2:DeleteVpc",
				},
			},
			Disabled: false,
		},
		Generic: Generic{
			Networking: Networking{
				Type: "calico",
			},
			Disabled: false,
		},
	})
	compare.DeepCompare(c, m.Installer, &Installer{
		SetupEndpoints: []string{"Bandwagon"},
		Flavors: Flavors{
			Prompt:  "Select a flavor",
			Default: "one",
			Items: []Flavor{
				{
					Name:        "one",
					Description: "1 node",
					Nodes: []FlavorNode{
						{
							Profile: "node",
							Count:   1,
						},
					},
				},
				{
					Name:        "two",
					Description: "2 nodes",
					Nodes: []FlavorNode{
						{
							Profile: "node",
							Count:   2,
						},
					},
				},
			},
		},
	})
	compare.DeepCompare(c, m.NodeProfiles, NodeProfiles{
		{
			Name:        "node",
			Description: "Telekube Node",
			Labels:      map[string]string{"role": "node"},
			Taints: []v1.Taint{
				{
					Key:    "node-role.kubernetes.io/master",
					Effect: v1.TaintEffectNoSchedule,
				},
				{
					Key:    "custom-taint",
					Value:  "custom-value",
					Effect: v1.TaintEffectNoExecute,
				},
			},
			Requirements: Requirements{
				CPU: CPU{Min: 1},
				RAM: RAM{Min: utils.MustParseCapacity("2GB")},
				OS: []OS{
					{
						Name:     "rhel",
						Versions: []string{"7.2", "7.3"},
					},
				},
				Network: Network{
					MinTransferRate: utils.MustParseTransferRate("50MB/s"),
					Ports: []Port{
						{
							Protocol: "tcp",
							Ranges:   []string{"6443", "8080", "10248-10255"},
						},
					},
				},
				Volumes: []Volume{
					{
						Path:            "/var/lib/gravity",
						Capacity:        utils.MustParseCapacity("10GB"),
						Filesystems:     []string{"xfs", "ext4"},
						CreateIfMissing: utils.BoolPtr(true),
						SkipIfMissing:   utils.BoolPtr(false),
						UID:             utils.IntPtr(1000),
						GID:             utils.IntPtr(1000),
						Mode:            "0755",
					},
				},
			},
			Providers: NodeProviders{
				AWS: NodeProviderAWS{
					InstanceTypes: []string{"m3.xlarge", "c3.xlarge"},
				},
			},
		},
		{
			Name:        "stateless",
			Description: "Telekube Stateless Node",
			Labels: map[string]string{
				constants.NodeLabel: constants.True,
			},
			ServiceRole: ServiceRoleNode,
		},
	})
	compare.DeepCompare(c, m.Dependencies, Dependencies{
		Packages: []Dependency{
			{Locator: loc.MustParseLocator("gravitational.io/gravity:0.0.1")},
		},
		Apps: []Dependency{
			{Locator: loc.MustParseLocator("gravitational.io/dns-app:0.0.3")},
		},
	})
	compare.DeepCompare(c, m.SystemOptions, &SystemOptions{
		Runtime: &Runtime{
			Locator: loc.MustCreateLocator(defaults.SystemAccountOrg, defaults.Runtime, "1.4.6"),
		},
		Docker: &Docker{
			StorageDriver: "overlay",
			ExternalService: ExternalService{
				Args: []string{"--log-level=DEBUG"},
			},
		},
		Etcd: &Etcd{
			ExternalService: ExternalService{
				Args: []string{"-debug"},
			},
		},
		Kubelet: &Kubelet{
			ExternalService: ExternalService{
				Args: []string{"--system-reserved=memory=500Mi"},
			},
		},
		Dependencies: SystemDependencies{
			Runtime: &Dependency{
				Locator: loc.MustParseLocator("gravitational.io/planet:0.0.1"),
			},
		},
	})
	compare.DeepCompare(c, m.Extensions, &Extensions{
		Logs: &LogsExtension{
			Disabled: true,
		},
		Monitoring: &MonitoringExtension{
			Disabled: true,
		},
		Kubernetes: &KubernetesExtension{
			Disabled: true,
		},
		Configuration: &ConfigurationExtension{
			Disabled: true,
		},
	})
}

func (s *ManifestSuite) TestInvalidRepository(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: application
  resourceVersion: 0.0.1
  repository: notgravitational.io`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInvalidName(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: Not Valid Name
  resourceVersion: 0.0.1`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInvalidProfileInFlavor(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
installer:
  flavors:
    items:
      - name: one
        nodes:
          - profile: worker
            count: 1
nodeProfiles:
  - name: node`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInvalidCountInFlavor(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
installer:
  flavors:
    items:
      - name: one
        nodes:
          - profile: node
            count: -1
nodeProfiles:
  - name: node`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestUnsupportedExpandPolicy(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
installer:
  flavors:
    items:
      - name: one
        nodes:
          - profile: node
            count: 1
nodeProfiles:
  - name: node
    expandPolicy: whatisthis?`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInvalidCPURequirement(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
installer:
  flavors:
    items:
      - name: one
        nodes:
          - profile: node
            count: 1
nodeProfiles:
  - name: node
    requirements:
      cpu:
        min: 2
        max: 1`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInvalidRAMRequirement(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
installer:
  flavors:
    items:
      - name: one
        nodes:
          - profile: node
            count: 1
nodeProfiles:
  - name: node
    requirements:
      ram:
        min: 2GB
        max: 1GB`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInvalidFileMode(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
installer:
  flavors:
    items:
      - name: one
        nodes:
          - profile: node
            count: 1
nodeProfiles:
  - name: node
    requirements:
      volumes:
      - path: /bad/mode
        capacity: "10GB"
        mode: "bwahahah"`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestFlavorRequiredIfProfileDefined(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
nodeProfiles:
  - name: node`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInjectDefaultFlavorAndProfile(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1`)
	m, err := ParseManifestYAML(bytes)
	c.Assert(err, IsNil)
	c.Assert(len(m.FlavorNames()), Equals, 1)
	c.Assert(len(m.NodeProfiles), Equals, 1)
}

func (s *ManifestSuite) TestInvalidSemVer(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 1.0-alpha`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInvalidWebConfig(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
webConfig: |
  {
    // invalid comment
    "a": "b"
  }`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestInvalidDockerDriver(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
systemOptions:
  docker:
    storageDriver: nosuchdriver`)
	_, err := ParseManifestYAML(bytes)
	c.Assert(err, NotNil)
}

func (s *ManifestSuite) TestCanOverrideBooleans(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
installer:
  flavors:
    items:
    - name: one
      nodes:
      - profile: node
        count: 1
nodeProfiles:
  - name: node
    requirements:
      volumes:
      - name: volume1
        path: /path/to/dir
        skipIfMissing: true
`)
	m, err := ParseManifestYAML(bytes)
	if err != nil {
		c.Errorf("Error parsing manifest: %v.", trace.DebugReport(err))
	}
	c.Assert(err, IsNil)
	c.Assert(m.NodeProfiles, compare.DeepEquals, NodeProfiles{
		NodeProfile{
			Name: "node",
			Requirements: Requirements{
				Volumes: []Volume{
					Volume{
						Name: "volume1",
						Path: "/path/to/dir",
						// Overridden in manifest
						SkipIfMissing: utils.BoolPtr(true),
						// Reset to false as a consequence of skipIfMissing
						// having been set
						CreateIfMissing: utils.BoolPtr(false),
					},
				},
			},
		},
	})
}

func (s *ManifestSuite) TestShouldSkipApp(c *C) {
	bytes := []byte(`apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: myapp
  resourceVersion: 0.0.1
ingress:
  nginx:
    enabled: false
storage:
  openebs:
    enabled: false
extensions:
  logs:
    disabled: true
  monitoring:
    disabled: true
  catalog:
    disabled: true`)
	m, err := ParseManifestYAML(bytes)
	c.Assert(err, IsNil)
	testCases := []struct {
		name string
		skip bool
	}{
		{
			name: defaults.LoggingAppName,
			skip: true,
		},
		{
			name: defaults.MonitoringAppName,
			skip: true,
		},
		{
			name: defaults.IngressAppName,
			skip: true,
		},
		{
			name: defaults.TillerAppName,
			skip: true,
		},
		{
			name: constants.DNSAppPackage,
			skip: false,
		},
		{
			name: defaults.BandwagonPackageName,
			skip: true,
		},
		{
			name: defaults.StorageAppName,
			skip: true,
		},
	}
	for _, tc := range testCases {
		c.Assert(ShouldSkipApp(*m, loc.Locator{Name: tc.name}), Equals, tc.skip,
			Commentf("Test case %v failed", tc))
	}
}
