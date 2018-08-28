// +build !linux

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
// Only implemented on Linux.
func DefaultPortChecker() health.Checker {
	return noopChecker{}
}

// PreInstallPortChecker validates ports required for installation.
// Only implemented on Linux.
func PreInstallPortChecker() health.Checker {
	return noopChecker{}
}

// DefaultProcessChecker returns checker which will ensure
// no conflicting program is running.
// Only implemented on Linux.
func DefaultProcessChecker() health.Checker {
	return noopChecker{}
}

// BasicCheckers detects common problems preventing k8s cluster from
// functioning properly.
// Only implemented on Linux.
func BasicCheckers() health.Checker {
	return noopChecker{}
}

// PreInstallCheckers are designed to run on a node before installing telekube.
// Only implemented on Linux.
func PreInstallCheckers() health.Checker {
	return noopChecker{}
}

// DefaultBootConfigParams returns standard kernel configs required for
// running kubernetes.
// Only implemented on Linux.
func DefaultBootConfigParams() health.Checker {
	return noopChecker{}
}

// GetStorageDriverBootConfigParams returns config params required for a given filesystem.
// Only implemented on Linux.
func GetStorageDriverBootConfigParams(drv string) health.Checker {
	return noopChecker{}
}

// NewStorageChecker creates a new instance of the volume checker
// using the specified checker as configuration
func NewStorageChecker(config StorageConfig) health.Checker {
	return noopChecker{}
}

// StorageConfig describes checker configuration
type StorageConfig struct {
	// Path represents volume to be checked
	Path string
	// WillBeCreated when true, then all checks will be applied to first existing dir, or fail otherwise
	WillBeCreated bool
	// MinBytesPerSecond is minimum write speed for probe to succeed
	MinBytesPerSecond uint64
	// Filesystems define list of supported filesystems, or any if empty
	Filesystems []string
	// MinFreeBytes define minimum free volume capacity
	MinFreeBytes uint64
}
