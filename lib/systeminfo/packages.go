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
