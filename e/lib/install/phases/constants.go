package phases

import "fmt"

const (
	// InstallerPhase is a phase that downloads installer from Ops Center
	InstallerPhase = "/installer"
	// DecryptPhase is a phase that decrypts encrypted packages
	DecryptPhase = "/decrypt"
	// LicensePhase is a phase that installs license
	LicensePhase = "/license"
	// ConnectPhase is a phase that connects cluster to a remote Ops Center
	ConnectPhase = "/connect"
	// ClusterPhase is a phase that installs cluster using local Ops Center
	ClusterPhase = "/cluster"
)

var (
	// ClusterCreatePhase is a phase that creates a cluster install operation
	ClusterCreatePhase = fmt.Sprintf("%v/create", ClusterPhase)
	// ClusterWaitPhase is a phase that waits for cluster to finish install
	ClusterWaitPhase = fmt.Sprintf("%v/wait", ClusterPhase)
	// ClusterInfoPhase is a phase that collects info about installed cluster
	ClusterInfoPhase = fmt.Sprintf("%v/info", ClusterPhase)
)
