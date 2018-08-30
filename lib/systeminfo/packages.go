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

package systeminfo

import (
	"os/exec"
	"strings"

	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// GetSystemPackages queries the system for a set of installed packages
func GetSystemPackages(info OS) (packages []storage.SystemPackage) {
	var queries []packageQuery
	switch {
	case info.IsRedHat():
		queries = redHatPackageQueries
	}
	for _, query := range queries {
		out, err := exec.Command(query.Command[0], query.Command[1:]...).CombinedOutput()
		systemPackage := storage.SystemPackage{
			Name: query.Package,
		}
		if err != nil {
			systemPackage.Error = trace.ConvertSystemError(err).Error()
		} else {
			systemPackage.Version = strings.TrimSpace(string(out))
		}
		packages = append(packages, systemPackage)
	}

	return packages
}

// packageQuery describes a package query command
type packageQuery struct {
	// Package identifies the package by name
	Package string
	// Command to run to query package install details.
	// There's no strict format this has to conform to,
	// but it is expected to include at least the name of the package
	// and any other relevant details like the version.
	Command []string
}

// redHatPackageQueries lists queries for prerequisite packages on systems
// of RedHat descent
var redHatPackageQueries = []packageQuery{
	{
		Package: PackageLVM,
		Command: []string{"lvm", "version"},
	},
}

const (
	// PackageLVM refers to logical volume management facilities on Linux
	PackageLVM = "lvm2"
)
