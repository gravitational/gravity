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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
	"k8s.io/helm/pkg/chartutil"
)

// InspectResponse contains information about inspected cluster/application images.
type InspectResponse struct {
	// Manifest is the image manifest.
	Manifest *schema.Manifest
	// Images is a list of Docker images vendored in the image.
	Images loc.DockerImages
}

// ImagesAsStrings returns a list of images as strings without registry.
func (r InspectResponse) ImagesAsStrings() (result []string) {
	for _, image := range r.Images {
		result = append(result, fmt.Sprintf("%v:%v", image.Repository, image.Tag))
	}
	return result
}

// InspectChart returns information about the specified Helm chart.
func InspectChart(ctx context.Context, path string, vendor service.VendorRequest) (*InspectResponse, error) {
	chart, err := chartutil.Load(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := generateApplicationImageManifest(chart)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vendorer, err := service.NewVendorer(service.VendorerConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	images, err := vendorer.Images(path, vendor)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InspectResponse{
		Manifest: manifest,
		Images:   images,
	}, nil
}

// InspectCluster returns information about the specified cluster image source.
func InspectCluster(ctx context.Context, path string, vendor service.VendorRequest) (*InspectResponse, error) {
	source, err := GetClusterImageSource(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := source.Manifest()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vendorer, err := service.NewVendorer(service.VendorerConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	vendor.ManifestPath = path
	images, err := vendorer.Images(source.Dir(), vendor)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InspectResponse{
		Manifest: manifest,
		Images:   images,
	}, nil
}

// InspectImage returns information about the tarball image specified with path.
func InspectImage(ctx context.Context, path string) (*InspectResponse, error) {
	// Cluster and application images built with recent versions of tele include
	// Docker images information in the application metadata.
	response, err := getImagesFromAppMetadata(path)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if response != nil {
		return response, nil
	}
	// For older images fallback to extracting image references directly from
	// the registry layers (which is much slower).
	response, err = getImagesFromRegistry(ctx, path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// getImagesFromAppMetadata returns Docker images information from the metadata
// of the application packaged in the specified image.
func getImagesFromAppMetadata(path string) (*InspectResponse, error) {
	env, err := localenv.NewImageEnvironment(localenv.ImageEnvironmentConfig{
		Path:   path,
		DBOnly: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer env.Close()
	if len(env.Manifest.Status.DockerImages.Vendored) == 0 {
		return nil, trace.NotFound("%v metadata doesn't contain image references",
			env.Manifest.Locator())
	}
	return &InspectResponse{
		Manifest: env.Manifest,
		Images:   env.Manifest.Status.DockerImages.Vendored,
	}, nil
}

// getImagesFromRegistry returns Docker images information from the registry
// layers of the application packaged in the specified image.
func getImagesFromRegistry(ctx context.Context, path string) (*InspectResponse, error) {
	env, err := localenv.NewImageEnvironment(localenv.ImageEnvironmentConfig{
		Path: path,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer env.Close()
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer os.RemoveAll(dir)
	err = pack.Unpack(env.Packages, env.Manifest.Locator(), dir, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	images, err := docker.ListImages(ctx, filepath.Join(dir, defaults.RegistryDir))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InspectResponse{
		Manifest: env.Manifest,
		Images:   images,
	}, nil
}
