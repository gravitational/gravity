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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
)

type provisionedServers []*ProvisionedServer

// InitialCluster combines the addresses of all servers in the specified cluster plan
// into a string of domain:addr key/value pairs.
func (p provisionedServers) InitialCluster(domain string) string {
	members := make([]string, len(p))
	for i, s := range p {
		members[i] = fmt.Sprintf(
			"%v:%v", s.EtcdMemberName(domain), s.AdvertiseIP)
	}
	return strings.Join(members, ",")
}

// ExistingCluster returns combined addresses of all the existing servers
// as reported by etcd member add
func ExistingCluster(domain string, cluster map[string]string) string {
	members := make([]string, 0, len(cluster))
	for name, ip := range cluster {
		members = append(members, fmt.Sprintf("%v:%v", name, ip))
	}
	return strings.Join(members, ",")
}

// FirstMaster returns the first master server from this server list
func (p provisionedServers) FirstMaster() *ProvisionedServer {
	return p.Masters()[0]
}

// Masters returns a sub-list of this server list that contains only masters
func (p provisionedServers) Masters() []*ProvisionedServer {
	servers := make([]*ProvisionedServer, 0)
	for _, s := range p {
		if s.Profile.ServiceRole == schema.ServiceRoleMaster {
			servers = append(servers, s)
		}
	}
	return servers
}

// MasterIPs returns a list of advertise IPs of master nodes.
func (p provisionedServers) MasterIPs() (ips []string) {
	for _, master := range p.Masters() {
		ips = append(ips, master.AdvertiseIP)
	}
	return ips
}

// Nodes returns a sub-list of this server list that contains only nodes
func (p provisionedServers) Nodes() []*ProvisionedServer {
	nodes := make([]*ProvisionedServer, 0)
	for _, s := range p {
		if s.Profile.ServiceRole != schema.ServiceRoleMaster {
			nodes = append(nodes, s)
		}
	}
	return nodes
}

// Contains returns true if this server list contains server with the provided advertise IP
func (p provisionedServers) Contains(ip string) bool {
	for _, server := range p {
		if server.AdvertiseIP == ip {
			return true
		}
	}
	return false
}

// ProvisionedServer has all information we need to set up the server
type ProvisionedServer struct {
	storage.Server
	Profile schema.NodeProfile
}

// Address returns the address this server is accessible on
// Address implements remoteServer.Address
func (s *ProvisionedServer) Address() string { return s.Server.AdvertiseIP }

// HostName returns the hostname of this server.
// HostName implements remoteServer.HostName
func (s *ProvisionedServer) HostName() string { return s.Server.Hostname }

// Debug provides a reference to the specified server useful for logging
// Debug implements remoteServer.Debug
func (s *ProvisionedServer) Debug() string { return s.Server.Hostname }

// EtcdMemberName returns generated unique etcd member name for this server
func (s *ProvisionedServer) EtcdMemberName(domain string) string {
	return s.FQDN(domain)
}

// AgentName returns a unique agent name for this server.
// The name of the agent is used to refer to this server in the cluster.
func (s *ProvisionedServer) AgentName(domain string) string {
	return s.FQDN(domain)
}

func (s *ProvisionedServer) IsMaster() bool {
	return s.Profile.ServiceRole == schema.ServiceRoleMaster
}

func (s *ProvisionedServer) PackageSuffix(domain string) string {
	return PackageSuffix(s, domain)
}

// Name returns unqualified name - without cluster name
func (s *ProvisionedServer) UnqualifiedName(clusterName string) string {
	return strings.TrimSuffix(s.FQDN(clusterName), "."+clusterName)
}

func (s *ProvisionedServer) FQDN(domain string) string {
	return FQDN(domain, s.Server.Hostname, s.AdvertiseIP)
}

func (s *ProvisionedServer) String() string {
	return fmt.Sprintf(
		"ProvisionedServer(hostname=%v,ip=%v)", s.Hostname, s.AdvertiseIP)
}

// InGravity returns a directory within gravity state dir on this server
func (s *ProvisionedServer) InGravity(dir ...string) string {
	return filepath.Join(append([]string{s.StateDir()}, dir...)...)
}

func FQDN(domain, hostname, ip string) string {
	// in case hostname comes already with this domain suffix,
	// we assume that it's set by provisioner or user in on-prem
	// scenario, so we reuse it
	if strings.HasSuffix(hostname, "."+domain) {
		return hostname
	}
	prefix := strings.Replace(ip, ".", "_", -1)
	return fmt.Sprintf("%v.%v", prefix, domain)
}
