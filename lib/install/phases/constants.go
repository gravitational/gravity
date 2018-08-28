package phases

const (
	// ChecksPhase is a phase that executes preflight checks
	ChecksPhase = "/checks"
	// InstallerPhase is a phase that downloads installer from Ops Center
	InstallerPhase = "/installer"
	// DecryptPhase is a phase that decrypts encrypted packages
	DecryptPhase = "/decrypt"
	// ConfigurePhase is a phase that configures cluster packages
	ConfigurePhase = "/configure"
	// BootstrapPhase is a phase that prepares the nodes for installation
	BootstrapPhase = "/bootstrap"
	// PullPhase is a phase that pulls configured packages
	PullPhase = "/pull"
	// MastersPhase is a phase that installs system software on master nodes
	MastersPhase = "/masters"
	// NodesPhase is a phase that installs system software on regular nodes
	NodesPhase = "/nodes"
	// WaitPhase is a phase that waits for planet to start
	WaitPhase = "/wait"
	// LabelPhase is a phase that applies labels and taints to Kubernetes nodes
	LabelPhase = "/label"
	// RBACPhase is a phase that creates Kubernetes RBAC resources
	RBACPhase = "/rbac"
	// ResourcesPhase is a phase that creates user supplied Kubernetes resources
	ResourcesPhase = "/resources"
	// ExportPhase is a phase that exports application layers to registries
	ExportPhase = "/export"
	// RuntimePhase is a phase that installs system applications
	RuntimePhase = "/runtime"
	// AppPhase is a phase that installs user application
	AppPhase = "/app"
	// EnableElectionPhase turns on election participation for master nodes
	// at the end of the installation. During installation, the election is
	// off with a single master
	EnableElectionPhase = "/election"
)
