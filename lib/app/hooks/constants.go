package hooks

import (
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
)

const (
	// InitContainerName is the name for the init container
	InitContainerName = "init"

	// ContainerHostBinDir is where /usr/bin directory from host gets mounted inside hook containers
	ContainerHostBinDir = "/opt/bin"

	// ContainerPath is the default PATH inside hook containers
	ContainerPath = "/opt/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

	// ResourcesDir is where the app's resources directory is mounted inside a hook container
	ResourcesDir = "/var/lib/gravity/resources"

	// GravityDir is the root directory with gravity state
	GravityDir = "/var/lib/gravity"

	// StateDir specifies the directory for temporary state during hook's lifetime
	StateDir = "/tmp/state"

	// ContainerBackupDir defines a directory mounted inside a hook container that the backup/restore hooks
	// store/read backup results to/from
	ContainerBackupDir = "/var/lib/gravity/backup"

	// KubectlPath is where kubectl binary gets mounted inside hook containers
	KubectlPath = "/usr/local/bin/kubectl"

	// Helm is where helm binary gets mounted inside hook containers
	HelmPath = "/usr/local/bin/helm"

	// VolumeBin is the name of the volume with host's /usr/bin dir
	VolumeBin = "bin"

	// VolumeKubectl is the name of the volume with kubectl
	VolumeKubectl = "kubectl"

	// VolumeHelm is the name of the volume with helm binary
	VolumeHelm = "helm"

	// VolumeBackup is the name of the volume that stores results of the backup hook
	// as mounted inside the hook container
	VolumeBackup = "backup"

	// VolumeCerts is the name of the volume with host's certificates
	VolumeCerts = "certs"

	// VolumeGravity is the name of the volume with local gravity state
	VolumeGravity = "gravity"

	// VolumeResources is the name of the volume with unpacked app resources
	VolumeResources = "resources"

	// VolumeStateDir is the name of the volume for temporary state
	VolumeStateDir = "state-dir"

	// ApplicationPackage specifies the name of the environment variable
	// that defines the name of the application package the hook originated from.
	// This environment variable is made available to the hook job's init container
	ApplicationPackageEnv = "APP_PACKAGE"
)

// InitContainerImage is the image for the init container
var InitContainerImage = fmt.Sprintf("%v/gravitational/debian-tall:0.0.1", constants.DockerRegistry)
