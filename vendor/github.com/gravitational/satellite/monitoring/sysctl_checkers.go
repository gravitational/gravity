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

// NewIPForwardChecker returns new IP forward checker
func NewIPForwardChecker() *SysctlChecker {
	return &SysctlChecker{
		CheckerName:     IPForwardCheckerID,
		Param:           "net.ipv4.ip_forward",
		Expected:        "1",
		OnMissing:       "ipv4 forwarding status unknown",
		OnValueMismatch: "ipv4 forwarding is off, see https://www.gravitational.com/gravity/docs/faq/#ipv4-forwarding",
	}
}

// NewBridgeNetfilterChecker checks if kernel bridge netfilter module is enabled
func NewBridgeNetfilterChecker() *SysctlChecker {
	return &SysctlChecker{
		CheckerName:     NetfilterCheckerID,
		Param:           "net.bridge.bridge-nf-call-iptables",
		Expected:        "1",
		OnMissing:       "br_netfilter module is either not loaded, or sysctl net.bridge.bridge-nf-call-iptables is not set, see https://www.gravitational.com/gravity/docs/faq/#bridge-driver",
		OnValueMismatch: "kubernetes requires net.bridge.bridge-nf-call-iptables sysctl set to 1, https://www.gravitational.com/gravity/docs/faq/#bridge-driver",
	}
}

// NewMayDetachMountsChecker checks if fs.may_detach_mounts is set
// On RHEL 7.4 based kernels, device removals may fail with "device or resource busy" if fs.may_detach_mounts isn't set
// Under kubernetes this can cause pods to get stuck in the terminating state
// Docker issue: https://github.com/moby/moby/issues/22260
// Docker will be setting the option on startup: https://github.com/moby/moby/pull/34886/files
func NewMayDetachMountsChecker() *SysctlChecker {
	return &SysctlChecker{
		CheckerName:     MountsCheckerID,
		Param:           "fs.may_detach_mounts",
		Expected:        "1",
		OnValueMismatch: "fs.may_detach_mounts should be set to 1 or pods may get stuck in the Terminating state, see https://www.gravitational.com/gravity/docs/faq/#kubernetes-pods-stuck-in-terminating-state",
		SkipNotFound:    true, // It appears that this setting may not appear in non RHEL or older kernels, so don't fire the alert if we don't find the setting
	}
}

// NewCNIForwardingChecker checks if CNI interface has forwarding enabled
func NewCNIForwardingChecker() *SysctlChecker {
	return &SysctlChecker{
		CheckerName:     CNIForwardChecker,
		Param:           "net.ipv4.conf.cni0.forwarding",
		Expected:        "1",
		OnValueMismatch: "ipv4 forwarding is off on interface cni0, see https://www.gravitational.com/gravity/docs/faq/#ipv4-forwarding",
		SkipNotFound:    true, // interface may not exist, so skip if not found
	}
}

// NewFlannelForwardingChecker checks if flannel interface has forwarding enabled
func NewFlannelForwardingChecker() *SysctlChecker {
	return &SysctlChecker{
		CheckerName:     FlannelForwardChecker,
		Param:           "net.ipv4.conf.flannel/1.forwarding",
		Expected:        "1",
		OnValueMismatch: "ipv4 forwarding is off on interface flannel.1, see https://www.gravitational.com/gravity/docs/faq/#ipv4-forwarding",
		SkipNotFound:    true, // interface may not exist, so skip if not found
	}
}

// NewWormholeBridgeForwardingChecker checks if wormhole-br0 interface has forwarding enabled
func NewWormholeBridgeForwardingChecker() *SysctlChecker {
	return &SysctlChecker{
		CheckerName:     WormholeBridgeForwardChecker,
		Param:           "net.ipv4.conf.wormhole-br0.forwarding",
		Expected:        "1",
		OnValueMismatch: "ipv4 forwarding is off on interface wormhole-br0, see https://www.gravitational.com/gravity/docs/faq/#ipv4-forwarding",
		SkipNotFound:    true, // interface may not exist, so skip if not found
	}
}

// NewWormholeWgForwardingChecker checks if wormhole-wg0 interface has forwarding enabled
func NewWormholeWgForwardingChecker() *SysctlChecker {
	return &SysctlChecker{
		CheckerName:     WormholeWgForwardChecker,
		Param:           "net.ipv4.conf.wormhole-wg0.forwarding",
		Expected:        "1",
		OnValueMismatch: "ipv4 forwarding is off on interface wormhole-wg0, see https://www.gravitational.com/gravity/docs/faq/#ipv4-forwarding",
		SkipNotFound:    true, // interface may not exist, so skip if not found
	}
}

const (
	// IPForwardCheckerID is the ID of the checker of ipv4 forwarding
	IPForwardCheckerID = "ip-forward"
	// NetfilterCheckerID is the ID of the checker of bridge netfilter
	NetfilterCheckerID = "br-netfilter"
	// MountsCheckerID is the ID of the checker of mounts detaching
	MountsCheckerID = "may-detach-mounts"
	// CNIForwardChecker is the ID of the checker of cni interface set to forwarding
	CNIForwardChecker = "cni0-forwarding"
	// FlannelForwardChecker is the ID of the checker of flannel interface set to forwarding
	FlannelForwardChecker = "flannel.1-forwarding"
	// WormholeBridgeForwardChecker is the ID of the checker of wormhole bridge interface set to forwarding
	WormholeBridgeForwardChecker = "wormhole-br0-forwarding"
	// WormholeWgForwardChecker is the ID of the checker of wormhole bridge interface set to forwarding
	WormholeWgForwardChecker = "wormhole-wg0-forwarding"
)
