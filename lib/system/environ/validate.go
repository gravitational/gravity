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

// package environ implements utilities for managing host environment
package environ

import (
	"os"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/system"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// ValidateInstall performs a local environment sanity check to make sure
// that install on this node can proceed without issues
func ValidateInstall(env *localenv.LocalEnvironment) func() error {
	return func() error {
		if err := validateNonVolatileDirectory(state.GravityInstallDir()); err != nil {
			return trace.Wrap(err)
		}
		if err := validateDirectoryEmpty(state.GravityInstallDir()); err != nil {
			return trace.Wrap(err)
		}
		if err := validateNoPackageState(env.Packages, env.StateDir); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
}

func validateNonVolatileDirectory(stateDir string) error {
	fstype, err := system.GetFilesystemForPath(stateDir)
	if err != nil {
		return trace.Wrap(err)
	}
	if fstype == system.FilesystemTemporary {
		return trace.BadParameter("Installer is running from a temporary file system." +
			"It is required to run the installer from a non-volatile location.")
	}
	var volatileDirectories = []string{"/tmp", "/var/tmp"}
	if os.Getenv("TMPDIR") != "" {
		// Non-empty $TMPDIR overrides default temporary directories
		// See https://www.freedesktop.org/software/systemd/man/tmpfiles.d.html
		volatileDirectories = []string{os.Getenv("TMPDIR")}
	}
	if !isRootedAt(stateDir, volatileDirectories...) {
		return trace.BadParameter("Installer is running from a temporary directory." +
			"Consider running the installer from a non-volatile location.")
	}
	return nil
}

func validateDirectoryEmpty(stateDir string) error {
	empty, err := utils.IsDirectoryEmpty(stateDir)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil && !empty {
		return trace.BadParameter("detected previous installation state in %v, "+
			"please resume the agent with `gravity resume` or "+
			"clean it up using `gravity leave --force` before proceeding "+
			"(see https://gravitational.com/gravity/docs/cluster/#deleting-a-cluster for more details)",
			stateDir)
	}
	return nil
}

func validateNoPackageState(packages pack.PackageService, stateDir string) error {
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
