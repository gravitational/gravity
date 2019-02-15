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
	"github.com/gravitational/gravity/lib/storage/clusterconfig/component"

	teleservices "github.com/gravitational/teleport/lib/services"
	"gopkg.in/check.v1"
)

type ConfigureSuite struct {
	cluster *site
}

var _ = check.Suite(&ConfigureSuite{})

func (s *ConfigureSuite) SetUpTest(c *check.C) {
	s.cluster = &site{
		domainName: "example.com",
		backendSite: &storage.Site{
			// FIXME: make sure that specifying cloud provider in cluster configuration
			// also generates the node tag for the cluster on GCE
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
	konfig := component.KubeletConfiguration{
		TypeMeta: component.KubeletTypeMeta,
		Address:  "0.0.0.0",
		Port:     10250,
	}
	konfigBytes, err := json.Marshal(konfig)
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
			"VAR": "value",
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
					Kubelet: &clusterconfig.Kubelet{Config: konfigBytes},
				},
				Global: &clusterconfig.Global{
					CloudProvider: "gce",
					CloudConfig: clusterconfig.CloudConfig{
						Config: `
[global]
username=user
password=pass`,
					},
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
	obtained := argsToMap(args, c)
	configs := obtained["kubelet-config"]
	c.Assert(len(configs), check.Equals, 1)
	obtainedKonfigBytes := configs[0]
	delete(obtained, "kubelet-config")
	decoded, err := base64.StdEncoding.DecodeString(obtainedKonfigBytes)
	c.Assert(err, check.IsNil)
	var obtainedKonfig component.KubeletConfiguration
	err = json.Unmarshal(decoded, &obtainedKonfig)
	c.Assert(err, check.IsNil)
	c.Assert(obtainedKonfig, compare.DeepEquals, konfig)
	c.Assert(obtained, compare.DeepEquals, map[string][]string{
		"node-name":                  []string{"172.12.13.0"},
		"hostname":                   []string{"node-1"},
		"master-ip":                  []string{"172.12.13.0"},
		"public-ip":                  []string{"172.12.13.0"},
		"cluster-id":                 []string{"example.com"},
		"etcd-proxy":                 []string{"off"},
		"etcd-member-name":           []string{"172_12_13_0.example.com"},
		"initial-cluster":            []string{"172.12.13.0.example.com"},
		"etcd-initial-cluster-state": []string{"new"},
		"secrets-dir":                []string{"/var/lib/gravity/secrets"},
		"election-enabled":           []string{"true"},
		"service-uid":                []string{"1000"},
		"env":                        []string{"VAR=value"},
		"volume": sorted(
			"/var/lib/gravity/planet/etcd:/ext/etcd",
			"/var/lib/gravity/planet/docker:/ext/docker",
			"/var/lib/gravity/planet/registry:/ext/registry",
			"/var/lib/gravity/planet/share:/ext/share",
			"/var/lib/gravity/planet/state:/ext/state",
			"/var/lib/gravity/planet/log:/var/log",
			"/var/lib/gravity:/var/lib/gravity",
		),
		"cloud-provider":          []string{"gce"},
		"cloud-config":            []string{"\n[global]\nusername=user\npassword=pass"},
		"gce-node-tags":           []string{"example-com"},
		"role":                    []string{"node"},
		"docker-promiscuous-mode": []string{"true"},
		"dns-listen-addr":         []string{"127.0.0.2"},
		"dns-port":                []string{"53"},
		"docker-backend":          []string{"overlay2"},
		"docker-options":          []string{"--storage-opt=overlay2.override_kernel_check=1"},
		"kubelet-options":         []string{"--hairpin-mode=none"},
		"node-label":              []string{"gravitational.io/advertise-ip=172.12.13.0"},
		"service-subnet":          []string{"10.0.0.1/8"},
		"pod-subnet":              []string{"10.0.1.1/8"},
		"feature-gates":           []string{"FeatureA=true,FeatureB=false"},
	})
}

func argsToMap(args []string, c *check.C) (result map[string][]string) {
	name := func(s string) string {
		return s[2:]
	}
	result = make(map[string][]string)
	for _, arg := range args {
		sep := strings.Index(arg, "=")
		if sep == -1 {
			result[name(arg)] = []string(nil)
			continue
		}
		key, value := name(arg[:sep]), arg[sep+1:]
		result[key] = append(result[key], value)
	}
	for key, values := range result {
		if len(values) != 0 {
			sort.Strings(values)
			result[key] = values
		}
	}
	return result
}

func sorted(ss ...string) []string {
	sort.Strings(ss)
	return ss
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
