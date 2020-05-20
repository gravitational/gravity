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

package cli

import (
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
)

// Application represents the command-line "tele" application and contains
// definitions of all its flags, arguments and subcommands
type Application struct {
	*kingpin.Application
	// Debug allows to run the command in debug mode
	Debug *bool
	// Insecure turns off TLS hostname validation
	Insecure *bool
	// StateDir is the local state directory
	StateDir *string
	// VersionCmd outputs the binary version
	VersionCmd VersionCmd
	// BuildCmd builds a cluster image.
	BuildCmd BuildCmd
	// HelmCmd combines commands operating on helm charts.
	HelmCmd HelmCmd
	// HelmBuildCmd builds an application image out of a helm chart.
	HelmBuildCmd HelmBuildCmd
	// ListCmd lists available apps and runtimes
	ListCmd ListCmd
	// PullCmd downloads app installer from Ops Center
	PullCmd PullCmd
}

// VersionCmd outputs the binary version
type VersionCmd struct {
	*kingpin.CmdClause
	// Output is output format
	Output *constants.Format
}

// HelmCmd combines commands operating on helm charts.
type HelmCmd struct {
	*kingpin.CmdClause
}

// HelmBuildCmd builds an application image out of a helm chart.
type HelmBuildCmd struct {
	*kingpin.CmdClause
	// Path is the path to a helm chart.
	Path *string
	// OutFile is the output tarball file
	OutFile *string
	// Overwrite overwrites existing tarball
	Overwrite *bool
	// VendorPatters is file pattern to search for images
	VendorPatterns *[]string
	// VendorIgnorePatterns if file pattern to ignore when searching for images
	VendorIgnorePatterns *[]string
	// SetImages rewrites images to specified versions
	SetImages *loc.DockerImages
	// Parallel defines the number of tasks to execute concurrently
	Parallel *int
	// Quiet allows to suppress console output
	Quiet *bool
	// Verbose enables more detailed build output.
	Verbose *bool
	// Set is a list of Helm chart values set on the CLI.
	Set *[]string
	// Values is a list of YAML files with Helm chart values.
	Values *[]string
	// Pull allows to force-pull Docker images even if they're already present.
	Pull *bool
	// UpgradeFrom is a path to the image to build an incremental image off of.
	UpgradeFrom *string
	// Diff shows differences between two images without building the image.
	Diff *bool
}

// BuildCmd builds app installer tarball
type BuildCmd struct {
	*kingpin.CmdClause
	// Path is the path to manifest file or Helm chart
	Path *string
	// OutFile is the output tarball file
	OutFile *string
	// Overwrite overwrites existing tarball
	Overwrite *bool
	// Name allows to override app name
	Name *string
	// Version allows to override app version
	Version *string
	// VendorPatters is file pattern to search for images
	VendorPatterns *[]string
	// VendorIgnorePatterns if file pattern to ignore when searching for images
	VendorIgnorePatterns *[]string
	// SetImages rewrites images to specified versions
	SetImages *loc.DockerImages
	// SetDeps rewrites app dependencies to specified versions
	SetDeps *loc.Locators
	// SkipVersionCheck suppresses version mismatch check
	SkipVersionCheck *bool
	// Parallel defines the number of tasks to execute concurrently
	Parallel *int
	// Quiet allows to suppress console output
	Quiet *bool
	// Verbose enables more detailed build output.
	Verbose *bool
	// Set is a list of Helm chart values set on the CLI.
	Set *[]string
	// Values is a list of YAML files with Helm chart values.
	Values *[]string
	// Pull allows to force-pull Docker images even if they're already present.
	Pull *bool
	// BaseImage allows to specify base image on the CLI.
	BaseImage *string
	// UpgradeFrom is a path to the image to build an incremental image off of.
	UpgradeFrom *string
	// Diff shows differences between two images without building the image.
	Diff *bool
	// SkipBaseCheck allows to skip base version check when building incremental image.
	SkipBaseCheck *bool
}

type ListCmd struct {
	*kingpin.CmdClause
	// Runtimes shows available runtimes
	Runtimes *bool
	// Format is the output format
	Format *constants.Format
	// All displays all available versions
	All *bool
}

// PullCmd downloads app installer from Ops Center
type PullCmd struct {
	*kingpin.CmdClause
	// App is app name
	App *string
	// OutFile is installer tarball file name
	OutFile *string
	// Force overwrites existing tarball
	Force *bool
	// Quiet allows to suppress console output
	Quiet *bool
}
