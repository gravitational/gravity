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

package schema

import "github.com/gravitational/gravity/lib/defaults"

// DefaultPortRanges defines the list of default ports for the cluster
var DefaultPortRanges = PortRanges{
	Kubernetes: []PortRange{
		{Protocol: "tcp", From: 10248, To: 10255, Description: "kubernetes internal services range"},
		{
			Protocol:    "tcp",
			From:        defaults.EtcdAPIPort,
			To:          defaults.EtcdAPIPort,
			Description: "etcd API port",
		},
		{
			Protocol:    "tcp",
			From:        defaults.EtcdPeerPort,
			To:          defaults.EtcdPeerPort,
			Description: "etcd peer port",
		},
		{
			Protocol:    "tcp",
			From:        defaults.APIServerSecurePort,
			To:          defaults.APIServerSecurePort,
			Description: "kubernetes API server",
		},
		{
			Protocol:    "tcp",
			From:        defaults.AlertmanagerServicePort,
			To:          defaults.AlertmanagerServicePort,
			Description: "alert manager service port",
		},
	},
	Installer: []PortRange{
		{
			Protocol:    "tcp",
			From:        defaults.WizardPackServerPort,
			To:          defaults.WizardPackServerPort,
			Description: "wizard package service",
		},
		{
			Protocol:    "tcp",
			From:        defaults.WizardHealthPort,
			To:          defaults.WizardHealthPort,
			Description: "wizard health endpoint",
		},
		{
			Protocol:    "tcp",
			From:        defaults.WizardSSHServerPort,
			To:          defaults.WizardSSHServerPort,
			Description: "wizard SSH port",
		},
		{
			Protocol:    "tcp",
			From:        defaults.WizardProxyServerPort,
			To:          defaults.WizardProxyServerPort,
			Description: "wizard proxy port",
		},
		{
			Protocol:    "tcp",
			From:        defaults.WizardReverseTunnelPort,
			To:          defaults.WizardReverseTunnelPort,
			Description: "installer reverse tunnel port",
		},
		{
			Protocol:    "tcp",
			From:        defaults.WizardAuthServerPort,
			To:          defaults.WizardAuthServerPort,
			Description: "installer auth port",
		},
		{
			Protocol:    "tcp",
			From:        defaults.GravityRPCAgentPort,
			To:          defaults.GravityRPCAgentPort,
			Description: "gravity agent RPC",
		},
		{
			Protocol:    "tcp",
			From:        defaults.BandwidthTestPort,
			To:          defaults.BandwidthTestPort,
			Description: "bandwidth test port",
		},
	},
	Vxlan: PortRange{
		Protocol:    "udp",
		From:        defaults.VxlanPort,
		To:          defaults.VxlanPort,
		Description: "overlay network",
	},
	Generic: []PortRange{
		{Protocol: "tcp", From: 3022, To: 3026, Description: "teleport internal SSH control panel"},
		{Protocol: "tcp", From: 3007, To: 3011, Description: "internal gravity services"},
		{
			Protocol:    "tcp",
			From:        defaults.GravitySiteNodePort,
			To:          defaults.GravitySiteNodePort,
			Description: "gravity Hub control panel",
		},
		{
			Protocol:    "tcp",
			From:        defaults.SatelliteRPCAgentPort,
			To:          defaults.SatelliteRPCAgentPort,
			Description: "planet agent RPC",
		},
		{
			Protocol:    "tcp",
			From:        defaults.SatelliteMetricsPort,
			To:          defaults.SatelliteMetricsPort,
			Description: "planet agent monitoring API port",
		},
	},
	Reserved: []PortRange{
		// Defined as kubernetes_port_t by default
		{
			Protocol:    "tcp",
			From:        defaults.EtcdAPILegacyPort,
			To:          defaults.EtcdAPILegacyPort,
			Description: "etcd",
		},
		// Defined as afs3_callback_port_t by default
		{
			Protocol:    "tcp",
			From:        defaults.EtcdPeerLegacyPort,
			To:          defaults.EtcdPeerLegacyPort,
			Description: "etcd",
		},
		// Defined as commplex_main_port_t by default
		{
			Protocol:    "tcp",
			From:        defaults.DockerRegistryPort,
			To:          defaults.DockerRegistryPort,
			Description: "docker registry",
		},
	},
}

// PortRanges arranges ports into groups
type PortRanges struct {
	// Kubernetes lists kubernetes-specific ports
	Kubernetes []PortRange
	// Installer lists installer-specific ports
	Installer []PortRange
	// Generic lists other ports
	Generic []PortRange
	// Reserved lists ports that are reserved by default
	Reserved []PortRange
	// Vxlan defines the xvlan port
	Vxlan PortRange
}

// PortRange describes a range of cluster ports
type PortRange struct {
	// Protocol specifies the port's protocol
	Protocol string
	// From and To specify the port range
	From, To uint64
	// Description specifies the optional port description
	Description string
}
