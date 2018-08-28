package api

// ExportConfig defines a set of configuration attributes
// for application export endpoint
type ExportConfig struct {
	// RegistryHostPort is a host:port of a docker registry
	// running on a particular master node.
	// It is used to replicate container images of all application
	// packages located on the given master node usually as part
	// of a cluster expand operation.
	RegistryHostPort string `json:"registryHostPort"`
}
