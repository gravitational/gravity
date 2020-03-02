/*
Copyright 2018 Gravitational, Inc.

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

package hooks

import (
	"fmt"

	"github.com/gravitational/gravity/lib/defaults"
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

	// HelmDir is the directory inside hooks where helm values files is mounted
	HelmDir = "/var/lib/gravity/helm"

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

	// HelmValuesFile is the name of the file with helm values
	HelmValuesFile = "values.yaml"

	// VolumeBin is the name of the volume with host's /usr/bin dir
	VolumeBin = "bin"

	// VolumeKubectl is the name of the volume with kubectl
	VolumeKubectlBin = "kubectl-bin"

	// VolumeHelmBin is the name of the volume with helm binary
	VolumeHelmBin = "helm-bin"

	// VolumeHelmValues is the name of the volume with helm values file
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

	// ApplicationPackageEnv specifies the name of the environment variable
	// that defines the name of the application package the hook originated from.
	// This environment variable is made available to the hook job's init container
	ApplicationPackageEnv = "APP_PACKAGE"

	// PodIPEnv specifies the name of variable associated with Pod IP address
	PodIPEnv = "POD_IP"
)

// InitContainerImage is the image for the init container
var InitContainerImage = fmt.Sprintf("%v/gravitational/debian-tall:buster", defaults.DockerRegistry)
