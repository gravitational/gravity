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

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/loc"

	"github.com/gravitational/trace"
	"k8s.io/helm/pkg/chartutil"
)

// NewApplicationBuilder returns a builder that produces application images.
func NewApplicationBuilder(config Config) (*applicationBuilder, error) {
	engine, err := newEngine(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &applicationBuilder{
		Engine: engine,
	}, nil
}

type applicationBuilder struct {
	*Engine
}

// ApplicationRequest combines parameters for building an application image.
type ApplicationRequest struct {
	// ChartPath is path to a Helm chart to build an image from.
	ChartPath string
	// OutputPath is the resulting cluster image output file path.
	OutputPath string
	// Overwrite is whether to overwrite existing output file.
	Overwrite bool
	// Vendor combines vendoring parameters.
	Vendor service.VendorRequest
	// From is a path to the application image used as a base for the incremental upgrade.
	From string
}

// Build builds an application image according to the provided parameters.
func (b *applicationBuilder) Build(ctx context.Context, req ApplicationRequest) error {
	chart, err := chartutil.Load(req.ChartPath)
	if err != nil {
		return trace.Wrap(err)
	}

	manifest, err := generateApplicationImageManifest(chart)
	if err != nil {
		return trace.Wrap(err)
	}

	outputPath, err := checkOutputPath(manifest, req.OutputPath, req.Overwrite)
	if err != nil {
		return trace.Wrap(err)
	}

	locator := imageLocator(manifest, req.Vendor)
	b.NextStep("Building application image %v %v from Helm chart", locator.Name,
		locator.Version)

	var upgradeFrom *loc.Locator
	if req.From != "" {
		b.NextStep("Discovering Docker images from %v", req.From)
		response, err := InspectImage(ctx, req.From)
		if err != nil {
			return trace.Wrap(err)
		}
		req.Vendor.SkipImages = response.Images
		upgradeFrom = response.Manifest.LocatorPtr()
	}

	vendorDir, err := ioutil.TempDir("", "vendor")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(vendorDir)

	b.NextStep("Discovering and embedding Docker images")
	vendorResp, err := b.Vendor(ctx, VendorRequest{
		SourceDir: req.ChartPath,
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

	b.NextStep("Packaging application image")
	installer, err := b.GenerateInstaller(manifest, app.InstallerRequest{
		Application: application.Package,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer installer.Close()

	b.NextStep("Saving application image to %v", outputPath)
	err = b.WriteInstaller(installer, outputPath)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
