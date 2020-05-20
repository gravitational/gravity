/*
Copyright 2020 Gravitational, Inc.

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
	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/builder"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/utils"
)

// BuildParameters represents the arguments provided for building an application
type BuildParameters struct {
	// StateDir is build state directory, if was specified
	StateDir string
	// SourcePath is the path to a manifest file or a Helm chart to build image from
	SourcePath string
	// OutPath holds the path to the installer tarball to be output
	OutPath string
	// Overwrite indicates whether or not to overwrite an existing installer file
	Overwrite bool
	// SkipVersionCheck indicates whether or not to perform the version check of the tele binary with the application's runtime at build time
	SkipVersionCheck bool
	// Silent is whether builder should report progress to the console
	Silent bool
	// Verbose turns on more detailed progress output
	Verbose bool
	// Insecure turns on insecure verify mode
	Insecure bool
	// Vendor combines vendoring parameters
	Vendor service.VendorRequest
	// BaseImage sets base image for the cluster image
	BaseImage string
	// UpgradeFrom is the path to the base image when building incremental upgrade image.
	UpgradeFrom string
	// SkipBaseCheck bypasses base image version check when building incremental image.
	SkipBaseCheck bool
}

// Level returns level at which the progress should be reported based on the CLI parameters.
func (p BuildParameters) Level() utils.ProgressLevel {
	if p.Silent { // No output.
		return utils.ProgressLevelNone
	} else if p.Verbose { // Detailed output.
		return utils.ProgressLevelDebug
	}
	return utils.ProgressLevelInfo // Normal output.
}

// BuilderConfig makes builder config from CLI parameters.
func (p BuildParameters) BuilderConfig() builder.Config {
	return builder.Config{
		StateDir:         p.StateDir,
		Insecure:         p.Insecure,
		SkipVersionCheck: p.SkipVersionCheck,
		Parallel:         p.Vendor.Parallel,
		Level:            p.Level(),
	}
}

// BuildCommandParameters returns build parameters for the tele build command.
func BuildCommandParameters(tele Application) BuildParameters {
	return BuildParameters{
		StateDir:         *tele.StateDir,
		SourcePath:       *tele.BuildCmd.Path,
		OutPath:          *tele.BuildCmd.OutFile,
		Overwrite:        *tele.BuildCmd.Overwrite,
		SkipVersionCheck: *tele.BuildCmd.SkipVersionCheck,
		Silent:           *tele.BuildCmd.Quiet,
		Verbose:          *tele.BuildCmd.Verbose,
		BaseImage:        *tele.BuildCmd.BaseImage,
		Insecure:         *tele.Insecure,
		UpgradeFrom:      *tele.BuildCmd.UpgradeFrom,
		SkipBaseCheck:    *tele.BuildCmd.SkipBaseCheck,
		Vendor: service.VendorRequest{
			PackageName:            *tele.BuildCmd.Name,
			PackageVersion:         *tele.BuildCmd.Version,
			ResourcePatterns:       *tele.BuildCmd.VendorPatterns,
			IgnoreResourcePatterns: *tele.BuildCmd.VendorIgnorePatterns,
			SetImages:              *tele.BuildCmd.SetImages,
			SetDeps:                *tele.BuildCmd.SetDeps,
			Parallel:               *tele.BuildCmd.Parallel,
			VendorRuntime:          true,
			Helm: helm.RenderParameters{
				Values: *tele.BuildCmd.Values,
				Set:    *tele.BuildCmd.Set,
			},
			Pull: *tele.BuildCmd.Pull,
		},
	}
}

// HelmBuildCommandParameters returns build parameters for the tele helm build command.
func HelmBuildCommandParameters(tele Application) BuildParameters {
	return BuildParameters{
		StateDir:    *tele.StateDir,
		SourcePath:  *tele.HelmBuildCmd.Path,
		OutPath:     *tele.HelmBuildCmd.OutFile,
		Overwrite:   *tele.HelmBuildCmd.Overwrite,
		Silent:      *tele.HelmBuildCmd.Quiet,
		Verbose:     *tele.HelmBuildCmd.Verbose,
		Insecure:    *tele.Insecure,
		UpgradeFrom: *tele.HelmBuildCmd.UpgradeFrom,
		Vendor: service.VendorRequest{
			ResourcePatterns:       *tele.HelmBuildCmd.VendorPatterns,
			IgnoreResourcePatterns: *tele.HelmBuildCmd.VendorIgnorePatterns,
			SetImages:              *tele.HelmBuildCmd.SetImages,
			Parallel:               *tele.HelmBuildCmd.Parallel,
			Helm: helm.RenderParameters{
				Values: *tele.HelmBuildCmd.Values,
				Set:    *tele.HelmBuildCmd.Set,
			},
			Pull: *tele.HelmBuildCmd.Pull,
		},
	}
}
