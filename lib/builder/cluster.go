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

package builder

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
)

// NewClusterBuilder returns a builder that produces cluster images.
func NewClusterBuilder(config Config) (*clusterBuilder, error) {
	engine, err := newEngine(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &clusterBuilder{
		Engine: engine,
	}, nil
}

type clusterBuilder struct {
	*Engine
}

// ClusterRequest combines parameters for building a cluster image.
type ClusterRequest struct {
	// SourcePath specifies the path to build the cluster image out of.
	SourcePath string
	// OutputPath is the resulting cluster image output file path.
	OutputPath string
	// Overwrite is whether to overwrite existing output file.
	Overwrite bool
	// Vendor combines vendoring parameters.
	Vendor service.VendorRequest
	// BaseImage is optional base image provided on the command line.
	BaseImage string
	// From is a path to the cluster image used as a base for the incremental upgrade.
	From string
	// SkipBaseCheck allows to bypass base image version check when building incremental image.
	SkipBaseCheck bool
}

// Build builds a cluster image according to the provided parameters.
func (b *clusterBuilder) Build(ctx context.Context, req ClusterRequest) error {
	imageSource, err := GetClusterImageSource(req.SourcePath)
	if err != nil {
		return trace.Wrap(err)
	}

	manifest, err := imageSource.Manifest()
	if err != nil {
		return trace.Wrap(err)
	}

	if req.BaseImage != "" {
		locator, err := loc.MakeLocator(req.BaseImage)
		if err != nil {
			return trace.Wrap(err)
		}
		manifest.SetBase(*locator)
	}

	outputPath, err := checkOutputPath(manifest, req.OutputPath, req.Overwrite)
	if err != nil {
		return trace.Wrap(err)
	}

	locator := imageLocator(manifest, req.Vendor)
	b.NextStep("Building cluster image %v %v from %v", locator.Name,
		locator.Version, imageSource.Type())

	b.NextStep("Selecting base image version")
	runtimeVersion, err := b.SelectRuntime(manifest)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.checkVersion(runtimeVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.SyncPackageCache(manifest, runtimeVersion)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("base image version %v not found", runtimeVersion)
		}
		return trace.Wrap(err)
	}

	var upgradeFrom *loc.Locator
	if req.From != "" {
		b.NextStep("Discovering Docker images from %v", req.From)
		response, err := InspectImage(ctx, req.From)
		if err != nil {
			return trace.Wrap(err)
		}
		version, err := response.Manifest.Base().SemVer()
		if err != nil {
			return trace.Wrap(err)
		}
		if !version.Equal(*runtimeVersion) {
			if !req.SkipBaseCheck {
				return trace.BadParameter(`The specified upgrade-from cluster image uses base image %v, while
this image is based on %v.

Building an incremental upgrade image from the image with a different base is
not recommended because any differences in system dependencies will not be
packaged which may result into a failed upgrade.

You can provide --skip-base-check flag to bypass this check.`,
					version,
					runtimeVersion)
			}
			b.NextStep(color.YellowString("Upgrade-from image %v uses different base image %v",
				response.Manifest.Locator().Human(),
				version))
		}
		req.Vendor.SkipImages = response.Images
		upgradeFrom = response.Manifest.LocatorP()
	}

	vendorDir, err := ioutil.TempDir("", "vendor")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(vendorDir)

	b.NextStep("Discovering and embedding Docker images")
	vendorResp, err := b.Vendor(ctx, VendorRequest{
		SourceDir: imageSource.Dir(),
		VendorDir: vendorDir,
		Manifest:  manifest,
		Vendor:    req.Vendor,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer vendorResp.Stream.Close()

	b.NextStep("Creating application")
	application, err := b.CreateApplication(createAppRequest{
		Stream:      vendorResp.Stream,
		Images:      vendorResp.Images,
		UpgradeFrom: upgradeFrom,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	b.NextStep("Packaging cluster image")
	installer, err := b.GenerateInstaller(manifest, app.InstallerRequest{
		Application: application.Package,
		Incremental: upgradeFrom != nil,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer installer.Close()

	b.NextStep("Saving cluster image to %v", outputPath)
	err = b.WriteInstaller(installer, outputPath)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// imageLocator returns locator of the image that's being built.
func imageLocator(manifest *schema.Manifest, vendor service.VendorRequest) loc.Locator {
	name := manifest.Metadata.Name
	if vendor.PackageName != "" {
		name = vendor.PackageName
	}
	version := manifest.Metadata.ResourceVersion
	if vendor.PackageVersion != "" {
		version = vendor.PackageVersion
	}
	return loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       name,
		Version:    version,
	}
}
