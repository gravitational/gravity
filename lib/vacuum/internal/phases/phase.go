package phases

const (
	// Journal is the phase to remove obsolete systemd journal directories
	Journal = "/journal"
	// Packages is the phase to remove unused telekube packages
	Packages = "/packages"
	// ClusterPackages is the sub-phase to remove unused telekube packages
	// from cluster package storage
	ClusterPackages = "/packages/cluster"
	// Registry is the phase to remove unused docker images
	Registry = "/registry"
)
