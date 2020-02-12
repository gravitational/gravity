package schema

// DefaultPortRanges defines the list of default ports for the cluster
var DefaultPortRanges = PortRanges{
	Kubernetes: []PortRange{
		{Protocol: "tcp", From: 2379, To: 2380, Description: "etcd"},
		{Protocol: "tcp", From: 6443, To: 6443, Description: "kubernetes API server"},
		{Protocol: "tcp", From: 10248, To: 10255, Description: "kubernetes internal services range"},
		{Protocol: "tcp", From: 7496, To: 7496, Description: "serf (health check agents) peer to peer"},
		{Protocol: "tcp", From: 7373, To: 7373, Description: "serf (health check agents) peer to peer"},
	},
	Installer: []PortRange{
		{Protocol: "tcp", From: 61009, To: 61010, Description: "installer ports"},
		{Protocol: "tcp", From: 61022, To: 61025, Description: "installer ports"},
		{Protocol: "tcp", From: 61080, To: 61080, Description: "installer ports"},
		{Protocol: "tcp", From: 3012, To: 3012, Description: "gravity agent RPC"},
	},
	Vxlan: PortRange{
		Protocol: "udp", From: 8472, To: 8472, Description: "overlay network",
	},
	Generic: []PortRange{
		{Protocol: "tcp", From: 3022, To: 3025, Description: "teleport internal SSH control panel"},
		{Protocol: "tcp", From: 3080, To: 3080, Description: "teleport Web UI"},
		{Protocol: "tcp", From: 3008, To: 3011, Description: "internal gravity services"},
		{Protocol: "tcp", From: 32009, To: 32009, Description: "gravity Hub control panel"},
		{Protocol: "tcp", From: 7575, To: 7575, Description: "planet agent RPC"},
	},
	Reserved: []PortRange{
		// Defines as kubernetes_port_t by default
		{Protocol: "tcp", From: 4001, To: 4001, Description: "etcd"},
		// Defines as afs3_callback_port_t by default
		{Protocol: "tcp", From: 7001, To: 7001, Description: "etcd"},
		// Defines as commplex_main_port_t by default
		{Protocol: "tcp", From: 5000, To: 5000, Description: "docker registry"},
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
