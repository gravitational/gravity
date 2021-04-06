/*
Copyright 2017 Gravitational, Inc.

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

package monitoring

import "github.com/gravitational/satellite/agent/health"

// DefaultPortChecker returns a port range checker with a default set of port ranges
func DefaultPortChecker() health.Checker {
	return NewPortChecker(
		PortRange{Protocol: protoTCP, From: 53, To: 53, Description: "internal cluster DNS"},
		PortRange{Protocol: protoUDP, From: 53, To: 53, Description: "internal cluster DNS"},
		PortRange{Protocol: protoUDP, From: 8472, To: 8472, Description: "overlay network"},
		PortRange{Protocol: protoTCP, From: 7496, To: 7496, Description: "serf (health check agents) peer to peer"},
		PortRange{Protocol: protoTCP, From: 7373, To: 7373, Description: "serf (health check agents) peer to peer"},
		PortRange{Protocol: protoTCP, From: 2379, To: 2380, Description: "etcd"},
		PortRange{Protocol: protoTCP, From: 4001, To: 4001, Description: "etcd"},
		PortRange{Protocol: protoTCP, From: 7001, To: 7001, Description: "etcd"},
		PortRange{Protocol: protoTCP, From: 6443, To: 6443, Description: "kubernetes API server"},
		PortRange{Protocol: protoTCP, From: 10248, To: 10255, Description: "kubernetes internal services range"},
		PortRange{Protocol: protoTCP, From: 5000, To: 5000, Description: "docker registry"},
		PortRange{Protocol: protoTCP, From: 3022, To: 3025, Description: "teleport internal ssh control panel"},
		PortRange{Protocol: protoTCP, From: 3080, To: 3080, Description: "teleport Web UI"},
		PortRange{Protocol: protoTCP, From: 3008, To: 3012, Description: "internal Telekube services"},
		PortRange{Protocol: protoTCP, From: 32009, To: 32009, Description: "telekube OpsCenter control panel"},
		PortRange{Protocol: protoTCP, From: 7575, To: 7575, Description: "telekube RPC agent"},
	)
}

// PreInstallPortChecker validates ports required for installation
func PreInstallPortChecker() health.Checker {
	return NewPortChecker(
		PortRange{Protocol: protoTCP, From: 4242, To: 4242, Description: "bandwidth checker"},
		PortRange{Protocol: protoTCP, From: 61008, To: 61010, Description: "installer agent ports"},
		PortRange{Protocol: protoTCP, From: 61022, To: 61024, Description: "installer agent ports"},
		PortRange{Protocol: protoTCP, From: 61009, To: 61009, Description: "install wizard"},
	)
}

// DefaultProcessChecker returns checker which will ensure no conflicting program is running
func DefaultProcessChecker() health.Checker {
	return &ProcessChecker{[]string{
		"dockerd",
		"docker-current", // Docker daemon name when installed from RHEL repos.
		"lxd",
		"coredns",
		"kube-apiserver",
		"kube-scheduler",
		"kube-controller-manager",
		"kube-proxy",
		"kubelet",
		"planet",
		"teleport",
	}}
}

// BasicCheckers detects common problems preventing k8s cluster from
// functioning properly
func BasicCheckers(checkers ...health.Checker) health.Checker {
	c := &compositeChecker{
		name: "local",
		checkers: []health.Checker{
			NewIPForwardChecker(),
			NewCNIForwardingChecker(),
			NewFlannelForwardingChecker(),
			NewWormholeBridgeForwardingChecker(),
			NewWormholeWgForwardingChecker(),
			NewBridgeNetfilterChecker(),
			NewMayDetachMountsChecker(),
			DefaultProcessChecker(),
			DefaultPortChecker(),
			DefaultBootConfigParams(),
			NewInotifyChecker(),
		},
	}
	c.checkers = append(c.checkers, checkers...)
	return c
}

// PreInstallCheckers are designed to run on a node before installing telekube
func PreInstallCheckers() health.Checker {
	return BasicCheckers(PreInstallPortChecker())
}

// DefaultBootConfigParams returns standard kernel configs required for running kubernetes
func DefaultBootConfigParams() health.Checker {
	return NewBootConfigParamChecker(
		BootConfigParam{Name: "CONFIG_NET_NS"},
		BootConfigParam{Name: "CONFIG_PID_NS"},
		BootConfigParam{Name: "CONFIG_IPC_NS"},
		BootConfigParam{Name: "CONFIG_UTS_NS"},
		BootConfigParam{Name: "CONFIG_CGROUPS"},
		BootConfigParam{Name: "CONFIG_CGROUP_CPUACCT"},
		BootConfigParam{Name: "CONFIG_CGROUP_DEVICE"},
		BootConfigParam{Name: "CONFIG_CGROUP_FREEZER"},
		BootConfigParam{Name: "CONFIG_CGROUP_SCHED"},
		BootConfigParam{Name: "CONFIG_CPUSETS"},
		BootConfigParam{Name: "CONFIG_MEMCG"},
		BootConfigParam{Name: "CONFIG_KEYS"},
		BootConfigParam{Name: "CONFIG_VETH"},
		BootConfigParam{Name: "CONFIG_BRIDGE"},
		BootConfigParam{Name: "CONFIG_BRIDGE_NETFILTER"},
		BootConfigParam{Name: "CONFIG_IP_NF_FILTER"},
		BootConfigParam{Name: "CONFIG_IP_NF_TARGET_MASQUERADE"},
		BootConfigParam{Name: "CONFIG_NETFILTER_XT_MATCH_ADDRTYPE"},
		BootConfigParam{Name: "CONFIG_NETFILTER_XT_MATCH_CONNTRACK"},
		BootConfigParam{Name: "CONFIG_NETFILTER_XT_MATCH_IPVS"},
		BootConfigParam{Name: "CONFIG_IP_NF_NAT"},
		BootConfigParam{Name: "CONFIG_NF_NAT"},
		BootConfigParam{
			// https://cateee.net/lkddb/web-lkddb/NF_NAT_NEEDED.html
			// CONFIG_NF_NAT_NEEDED has been removed as of kernel 5.2
			Name:             "CONFIG_NF_NAT_NEEDED",
			KernelConstraint: KernelVersionLessThan(KernelVersion{Release: 5, Major: 2}),
		},
		BootConfigParam{Name: "CONFIG_POSIX_MQUEUE"},
		BootConfigParam{
			// See: https://lists.gt.net/linux/kernel/2465684#2465684
			//  and https://github.com/lxc/lxc/pull/1217
			// CONFIG_DEVPTS_MULTIPLE_INSTANCES has been removed as of kernel 4.7
			Name:             "CONFIG_DEVPTS_MULTIPLE_INSTANCES",
			KernelConstraint: KernelVersionLessThan(KernelVersion{Release: 4, Major: 7}),
		},
	)
}

// NewDNSChecker sends some default queries to monitor DNS / service discovery health
func NewDNSChecker(questionA []string, nameservers ...string) health.Checker {
	return &DNSChecker{
		QuestionA:   questionA,
		Nameservers: nameservers,
	}
}
