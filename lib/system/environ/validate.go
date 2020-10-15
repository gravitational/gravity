/*
Copyright 2019 Gravitational, Inc.

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

// Package environ implements utilities for managing host environment
// during an operation
package environ

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/system"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ValidateInstall performs a local environment sanity check to make sure
// that install on this node can proceed without issues
func ValidateInstall(baseDir, serviceName string, env *localenv.LocalEnvironment) func() error {
	return func() error {
		stateDir := state.GravityInstallDirAt(baseDir)
		if err := validateNonVolatileDirectory(stateDir, env); err != nil {
			// Operational state directory requirements are advisory
			log.WithError(err).Warn("Failed to validate state directory requirements.")
		}
		if err := validateNoActiveService(serviceName); err != nil {
			if !trace.IsAlreadyExists(err) {
				log.WithError(err).Warn("Failed to determine if service is not active.")
			}
			return trace.BadParameter("detected an active installer service in %v, "+
				"please resume the agent with `gravity resume` or "+
				"clean it up using `gravity leave --force` before proceeding "+
				"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
				stateDir)
		}
		if err := validateNoActiveOperation(stateDir); err != nil {
			if !trace.IsAlreadyExists(err) {
				log.WithError(err).Warn("Failed to detect an active operation.")
			}
			return trace.BadParameter("detected previous installation state in %v, "+
				"please resume the agent with `gravity resume` or "+
				"clean it up using `gravity leave --force` before proceeding "+
				"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
				stateDir)
		}
		if err := ValidateNoPackageState(env.Packages, env.StateDir); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
}

func validateNonVolatileDirectory(stateDir string, printer utils.Printer) error {
	fstype, err := system.GetFilesystemForPath(stateDir)
	if err != nil {
		return trace.Wrap(err)
	}
	if fstype == system.FilesystemTemporary {
		printer.Print(color.YellowString("installer is running from a temporary file system.\n" +
			"It is recommended to run the installer from a non-volatile location to support " +
			"operation resumption after a node is rebooted."))
	}
	var volatileDirectories = []string{"/tmp", "/var/tmp"}
	if os.Getenv("TMPDIR") != "" {
		// Non-empty $TMPDIR overrides default temporary directories
		// See https://www.freedesktop.org/software/systemd/man/tmpfiles.d.html
		volatileDirectories = []string{os.Getenv("TMPDIR")}
	}
	if isRootedAt(stateDir, volatileDirectories...) {
		printer.Print(color.YellowString("installer is running from a temporary directory.\n" +
			"It is recommended to run the installer from a non-volatile location to support " +
			"operation resumption after a node is rebooted."))
	}
	return nil
}

func validateNoActiveService(serviceName string) error {
	manager, err := systemservice.New()
	if err != nil {
		return trace.Wrap(err)
	}
	status, err := manager.StatusService(serviceName)
	if err != nil && !systemservice.IsUnknownServiceError(err) {
		return trace.Wrap(err)
	}
	switch status {
	case systemservice.ServiceStatusFailed, systemservice.ServiceStatusUnknown:
		return nil
	default:
		// Consider service running if in another status
		return trace.AlreadyExists("service already running")
	}
}

// ValidateNoPackageState checks whether the specified package service
// has state (i.e. has packages) and returns an error in this case.
func ValidateNoPackageState(packages pack.PackageService, stateDir string) error {
	// make sure that there are no packages in the local state left from
	// some improperly cleaned up installation
	installedPackages, err := packages.GetPackages(defaults.SystemAccountOrg)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(installedPackages) != 0 {
		return trace.BadParameter("detected previous installation state in %v, "+
			"please clean it up using `gravity leave --force` before proceeding "+
			"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
			stateDir)
	}
	return nil
}

func validateNoActiveOperation(stateDir string) error {
	_, err := os.Stat(stateDir)
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	backend, err := newBackendFromDir(stateDir)
	if err != nil {
		return trace.Wrap(err)
	}
	defer backend.Close()
	_, err = storage.GetLastOperation(backend)
	if err == nil {
		return trace.AlreadyExists("operation already exists")
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

func newBackendFromDir(dir string) (storage.Backend, error) {
	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path:     filepath.Join(dir, defaults.GravityDBFile),
		Multi:    true,
		Readonly: true,
		Timeout:  keyval.NoTimeout,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return backend, nil
}

// isRootedAt returns true iff path is rooted at any of the directories
// specified with dirs
func isRootedAt(path string, dirs ...string) bool {
	for _, dir := range dirs {
		if strings.HasPrefix(path, dir) {
			return true
		}
	}
	return false
}
