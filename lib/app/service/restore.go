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

package service

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
)

// RestoreRequest describes a request to restore the application integrity
// by downloading missing Docker images from the cluster registry.
type RestoreRequest struct {
	// Packages is a package service where the application resides.
	Packages pack.PackageService
	// Apps is the app service based on the above package service.
	Apps app.Applications
	// ClusterPackages is the cluster's package service.
	ClusterPackages pack.PackageService
	// ClusterApps is the cluster's app service.
	ClusterApps app.Applications
	// Images is the cluster registry interface.
	Images docker.ImageService
	// Locator is the application locator to restore.
	Locator loc.Locator
	// Progress is the progress printer.
	Progress utils.Printer
}

// RestoreApp downloads Docker images that application depends on but does not
// vendor and returns package and app services containing the new application.
//
// It is used to restore integrity of the cluster/application images built as
// an incremental upgrade before uploading them to the cluster storage, since
// they only contain a subset of required images.
func RestoreApp(ctx context.Context, req RestoreRequest) (*LayeredApps, error) {
	application, err := req.Apps.GetApp(req.Locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Unpack the application to a temporary directory.
	dir, err := ioutil.TempDir("", fmt.Sprintf("%v-%v-unpacked", req.Locator.Name, req.Locator.Version))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	defer os.RemoveAll(dir)
	req.Progress.PrintStep("Unpacking application to %v", dir)
	err = pack.Unpack(req.Packages, req.Locator, dir, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Download images that the application doesn't vendor from the cluster
	// registry to the application's registry directory.
	registryDir := filepath.Join(dir, defaults.RegistryDir)
	req.Progress.PrintStep("Resyncing Docker images from the cluster registry")
	err = req.Images.SyncTo(ctx, registryDir, getMissingImages(*application), req.Progress)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Now that the application's "registry" directory contains all images,
	// re-import it again.
	stream, err := archive.Tar(dir, archive.Uncompressed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	layered, err := NewLayeredApps(LayeredAppsConfig{
		Packages: req.ClusterPackages,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.Progress.PrintStep("Importing restored application in %v", layered.Dir)
	_, err = app.ImportApplication(stream, layered.Packages, layered.Apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return layered, nil
}

// getMissingImages returns a list of Docker images that the application
// depends on but does not vendor e.g. in case of an incremental upgrade
// image).
func getMissingImages(application app.Application) (result []docker.Image) {
	images := application.Manifest.Status.DockerImages
	for _, image := range images.All {
		if !images.Vendored.Has(image.Repository, image.Tag) {
			result = append(result, docker.Image{
				Repository: image.Repository,
				Tags:       []string{image.Tag},
			})
		}
	}
	return result
}
