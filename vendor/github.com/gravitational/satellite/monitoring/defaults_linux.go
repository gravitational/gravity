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
		PortRange{protoTCP, 53, 53, "internal cluster DNS"},
		PortRange{protoUDP, 53, 53, "internal cluster DNS"},
		PortRange{protoUDP, 8472, 8472, "overlay network"},
		PortRange{protoTCP, 7496, 7496, "serf (health check agents) peer to peer"},
		PortRange{protoTCP, 7373, 7373, "serf (health check agents) peer to peer"},
		PortRange{protoTCP, 2379, 2380, "etcd"},
		PortRange{protoTCP, 4001, 4001, "etcd"},
		PortRange{protoTCP, 7001, 7001, "etcd"},
		PortRange{protoTCP, 6443, 6443, "kubernetes API server"},
		PortRange{protoTCP, 30000, 32767, "kubernetes internal services range"},
		PortRange{protoTCP, 10248, 10255, "kubernetes internal services range"},
		PortRange{protoTCP, 5000, 5000, "docker registry"},
		PortRange{protoTCP, 3022, 3025, "teleport internal ssh control panel"},
		PortRange{protoTCP, 3080, 3080, "teleport Web UI"},
		PortRange{protoTCP, 3008, 3012, "internal Telekube services"},
		PortRange{protoTCP, 32009, 32009, "telekube OpsCenter control panel"},
		PortRange{protoTCP, 7575, 7575, "telekube RPC agent"},
	)
}

// PreInstallPortChecker validates ports required for installation
func PreInstallPortChecker() health.Checker {
	return NewPortChecker(
		PortRange{protoTCP, 4242, 4242, "bandwidth checker"},
		PortRange{protoTCP, 61008, 61010, "installer agent ports"},
		PortRange{protoTCP, 61022, 61024, "installer agent ports"},
		PortRange{protoTCP, 61009, 61009, "install wizard"},
	)
}

// DefaultProcessChecker returns checker which will ensure no conflicting program is running
func DefaultProcessChecker() health.Checker {
	return &ProcessChecker{[]string{
		"dockerd",
		"lxd",
		"dnsmasq",
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
			NewBridgeNetfilterChecker(),
			NewMayDetachMountsChecker(),
			DefaultProcessChecker(),
			DefaultPortChecker(),
			DefaultBootConfigParams(),
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
		BootConfigParam{Name: "CONFIG_NF_NAT_IPV4"},
		BootConfigParam{Name: "CONFIG_IP_NF_FILTER"},
		BootConfigParam{Name: "CONFIG_IP_NF_TARGET_MASQUERADE"},
		BootConfigParam{Name: "CONFIG_NETFILTER_XT_MATCH_ADDRTYPE"},
		BootConfigParam{Name: "CONFIG_NETFILTER_XT_MATCH_CONNTRACK"},
		BootConfigParam{Name: "CONFIG_NETFILTER_XT_MATCH_IPVS"},
		BootConfigParam{Name: "CONFIG_IP_NF_NAT"},
		BootConfigParam{Name: "CONFIG_NF_NAT"},
		BootConfigParam{Name: "CONFIG_NF_NAT_NEEDED"},
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
