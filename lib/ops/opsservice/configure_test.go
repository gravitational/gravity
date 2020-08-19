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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	teleservices "github.com/gravitational/teleport/lib/services"
	"gopkg.in/check.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConfigureSuite struct {
	cluster *site
}

var _ = check.Suite(&ConfigureSuite{})

func (s *ConfigureSuite) SetUpTest(c *check.C) {
	s.cluster = &site{
		domainName: "example.com",
		provider:   "gce",
		backendSite: &storage.Site{
			CloudConfig: storage.CloudConfig{
				GCENodeTags: []string{"example-com"},
			},
		},
	}
}

func (s *ConfigureSuite) TestEtcdConfigAllFullMembers(c *check.C) {
	servers := makeServers(3, 3)
	config := s.cluster.prepareEtcdConfig(&operationContext{
		provisionedServers: servers,
	})
	for _, server := range servers {
		c.Assert(config[server.AdvertiseIP].proxyMode, check.Equals, etcdProxyOff)
	}
}

func (s *ConfigureSuite) TestEtcdConfigHasProxies(c *check.C) {
	servers := makeServers(3, 5)
	config := s.cluster.prepareEtcdConfig(&operationContext{
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

func (s *ConfigureSuite) TestGeneratesPlanetConfigPackage(c *check.C) {
	server := storage.Server{
		Hostname:    "node-1",
		ClusterRole: "master",
		Role:        "node",
		AdvertiseIP: "172.12.13.0",
	}
	// Dummy kubelet configuration to capture a subset of values
	var kubeletConfig = struct {
		TypeMeta metav1.TypeMeta
		Address  string
		Port     int64
	}{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KubeletConfiguration",
			APIVersion: "kubelet.config.k8s.io/v1beta1",
		},
		Address: "0.0.0.0",
		Port:    10250,
	}
	configBytes, err := json.Marshal(kubeletConfig)
	c.Assert(err, check.IsNil)
	config := planetConfig{
		master: masterConfig{
			addr:            server.AdvertiseIP,
			electionEnabled: true,
		},
		manifest: schema.Manifest{
			NodeProfiles: schema.NodeProfiles{
				{
					Name: "node",
				},
			},
		},
		installExpand: ops.SiteOperation{
			ID:         "operation-id",
			AccountID:  "local",
			SiteDomain: s.cluster.domainName,
			InstallExpand: &storage.InstallExpandOperationState{
				Servers: []storage.Server{server},
				Subnets: storage.Subnets{
					Service: "10.0.0.1/8",
					Overlay: "10.0.1.1/8",
				},
			},
		},
		server: ProvisionedServer{
			Server: server,
		},
		etcd: etcdConfig{
			initialCluster:      fmt.Sprintf("172.12.13.0.%v", s.cluster.domainName),
			initialClusterState: "new",
			proxyMode:           etcdProxyOff,
		},
		docker:        storage.DockerConfig{StorageDriver: "overlay2"},
		planetPackage: loc.MustParseLocator("gravitational.io/planet:0.0.1"),
		configPackage: loc.MustParseLocator("gravitational.io/planet-config:0.0.1"),
		env: map[string]string{
			"VAR":  "value",
			"VAR2": "value2",
			"VAR3": "value1,value2",
		},
		config: &clusterconfig.Resource{
			Kind:    storage.KindClusterConfiguration,
			Version: "v1",
			Metadata: teleservices.Metadata{
				Name:      constants.ClusterConfigurationMap,
				Namespace: defaults.KubeSystemNamespace,
			},
			Spec: clusterconfig.Spec{
				ComponentConfigs: clusterconfig.ComponentConfigs{
					Kubelet: &clusterconfig.Kubelet{Config: configBytes},
				},
				Global: clusterconfig.Global{
					CloudProvider: "gce",
					CloudConfig: `
[global]
node-tags=example-com
multizone=true`,
					FeatureGates: map[string]bool{
						"FeatureA": true,
						"FeatureB": false,
					},
				},
			},
		},
	}
	args, err := s.cluster.getPlanetConfig(config)
	c.Assert(err, check.IsNil)
	features, args := stripItem(args, "--feature-gates")
	c.Assert(sort.StringSlice(args), compare.SortedSliceEquals, mapToArgs(map[string][]string{
		"node-name":                  {"172.12.13.0"},
		"hostname":                   {"node-1"},
		"master-ip":                  {"172.12.13.0"},
		"public-ip":                  {"172.12.13.0"},
		"cluster-id":                 {"example.com"},
		"etcd-proxy":                 {"off"},
		"etcd-member-name":           {"172_12_13_0.example.com"},
		"initial-cluster":            {"172.12.13.0.example.com"},
		"etcd-initial-cluster-state": {"new"},
		"secrets-dir":                {"/var/lib/gravity/secrets"},
		"election-enabled":           {"true"},
		"service-uid":                {"1000"},
		"service-gid":                {"1000"},
		"env":                        {`VAR="value"`, `VAR2="value2"`, `VAR3="value1,value2"`},
		"volume": {
			"/var/lib/gravity/planet/etcd:/ext/etcd",
			"/var/lib/gravity/planet/docker:/ext/docker",
			"/var/lib/gravity/planet/registry:/ext/registry",
			"/var/lib/gravity/planet/share:/ext/share",
			"/var/lib/gravity/planet/state:/ext/state",
			"/var/lib/gravity/planet/log:/var/log",
			"/var/lib/gravity:/var/lib/gravity",
		},
		"cloud-provider":  {"gce"},
		"cloud-config":    {base64.StdEncoding.EncodeToString([]byte("\n[global]\nnode-tags=example-com\nmultizone=true"))},
		"role":            {"node"},
		"dns-listen-addr": {"127.0.0.2"},
		"dns-port":        {"53"},
		"docker-backend":  {"overlay2"},
		"docker-options":  {"--storage-opt=overlay2.override_kernel_check=1"},
		"kubelet-config":  {base64.StdEncoding.EncodeToString(configBytes)},
		"node-label":      {"gravitational.io/advertise-ip=172.12.13.0"},
		"service-subnet":  {"10.0.0.1/8"},
		"pod-subnet":      {"10.0.1.1/8"},
	}))
	assertFeatures(features, []string{"FeatureA=true", "FeatureB=false"}, c)
}

func (s *ConfigureSuite) TestCanSetCloudProviderWithoutCloudConfig(c *check.C) {
	s.cluster.provider = schema.ProviderGCE
	server := storage.Server{
		Hostname:    "node-1",
		ClusterRole: "master",
		Role:        "node",
		AdvertiseIP: "172.12.13.0",
	}
	config := planetConfig{
		master: masterConfig{
			addr:            server.AdvertiseIP,
			electionEnabled: true,
		},
		manifest: schema.Manifest{
			NodeProfiles: schema.NodeProfiles{
				{
					Name: "node",
				},
			},
		},
		installExpand: ops.SiteOperation{
			ID:         "operation-id",
			AccountID:  "local",
			SiteDomain: s.cluster.domainName,
			InstallExpand: &storage.InstallExpandOperationState{
				Servers: []storage.Server{server},
				Subnets: storage.Subnets{
					Service: "10.0.0.1/8",
					Overlay: "10.0.1.1/8",
				},
			},
		},
		server: ProvisionedServer{
			Server: server,
		},
		etcd: etcdConfig{
			initialCluster:      fmt.Sprintf("172.12.13.0.%v", s.cluster.domainName),
			initialClusterState: "new",
			proxyMode:           etcdProxyOff,
		},
		docker:        storage.DockerConfig{StorageDriver: "overlay2"},
		planetPackage: loc.MustParseLocator("gravitational.io/planet:0.0.1"),
		configPackage: loc.MustParseLocator("gravitational.io/planet-config:0.0.1"),
		config: &clusterconfig.Resource{
			Kind:    storage.KindClusterConfiguration,
			Version: "v1",
			Metadata: teleservices.Metadata{
				Name:      constants.ClusterConfigurationMap,
				Namespace: defaults.KubeSystemNamespace,
			},
			Spec: clusterconfig.Spec{},
		},
	}
	args, err := s.cluster.getPlanetConfig(config)
	c.Assert(err, check.IsNil)
	c.Assert(sort.StringSlice(args), compare.SortedSliceEquals, mapToArgs(map[string][]string{
		"node-name":                  {"172.12.13.0"},
		"hostname":                   {"node-1"},
		"master-ip":                  {"172.12.13.0"},
		"public-ip":                  {"172.12.13.0"},
		"cluster-id":                 {"example.com"},
		"etcd-proxy":                 {"off"},
		"etcd-member-name":           {"172_12_13_0.example.com"},
		"initial-cluster":            {"172.12.13.0.example.com"},
		"etcd-initial-cluster-state": {"new"},
		"secrets-dir":                {"/var/lib/gravity/secrets"},
		"election-enabled":           {"true"},
		"service-uid":                {"1000"},
		"service-gid":                {"1000"},
		"volume": {
			"/var/lib/gravity/planet/etcd:/ext/etcd",
			"/var/lib/gravity/planet/docker:/ext/docker",
			"/var/lib/gravity/planet/registry:/ext/registry",
			"/var/lib/gravity/planet/share:/ext/share",
			"/var/lib/gravity/planet/state:/ext/state",
			"/var/lib/gravity/planet/log:/var/log",
			"/var/lib/gravity:/var/lib/gravity",
		},
		"cloud-provider":  {"gce"},
		"gce-node-tags":   {"example-com"},
		"role":            {"node"},
		"dns-listen-addr": {"127.0.0.2"},
		"dns-port":        {"53"},
		"docker-backend":  {"overlay2"},
		"docker-options":  {"--storage-opt=overlay2.override_kernel_check=1"},
		"node-label":      {"gravitational.io/advertise-ip=172.12.13.0"},
		"service-subnet":  {"10.0.0.1/8"},
		"pod-subnet":      {"10.0.1.1/8"},
	}))
}

func mapToArgs(args map[string][]string) sort.Interface {
	var result []string
	for k, v := range args {
		for _, arg := range v {
			result = append(result, fmt.Sprintf("--%v=%v", k, arg))
		}
	}
	return sort.StringSlice(result)
}

func stripItem(args []string, name string) (item string, rest []string) {
	for i, arg := range args {
		if strings.HasPrefix(arg, name) {
			item = arg
			args = append(args[:i], args[i+1:]...)
			return item, args
		}
	}
	return "", args
}

// assertFeatures validates that features in the form of "--feature-gates=FeatureA=true,FeatureB=false"
// is equal to expected []string{"FeatureA=true", "FeatrueB=false"}
func assertFeatures(features string, expected []string, c *check.C) {
	parts := strings.SplitN(features, "=", 2)
	c.Assert(parts, check.HasLen, 2)
	items := strings.Split(parts[1], ",")
	c.Assert(sort.StringSlice(items), compare.SortedSliceEquals, sort.StringSlice(expected))
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
