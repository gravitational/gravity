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

package docker

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	dockerarchive "github.com/docker/docker/pkg/archive"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

// TranslateRuntimeImage translates the specified docker image
// into a gravity package specified in req.
func TranslateRuntimeImage(req TranslateImageRequest) error {
	f, err := ioutil.TempFile("", "gravity-runtime")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		f.Close()
		if errRemove := os.Remove(f.Name()); errRemove != nil {
			log.Warnf("Failed to remove tarball %v: %v.",
				f.Name(), errRemove)
		}
	}()

	packageDir, err := ioutil.TempDir("", "runtime")
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer func() {
		if err == nil {
			return
		}
		if errRemove := os.RemoveAll(packageDir); errRemove != nil {
			log.Warnf("Failed to remove intermediate package directory %v: %v.",
				packageDir, errRemove)
		}
	}()

	createOpts := dockerapi.CreateContainerOptions{
		Name: fmt.Sprintf("planet-export-%v", utilrand.String(4)),
		Config: &dockerapi.Config{
			Entrypoint: []string{"/bin/sleep", "infinity"},
			Image:      req.Image,
		},
	}
	container, err := req.CreateContainer(createOpts)
	if err != nil {
		return trace.Wrap(err)
	}

	log := log.WithFields(log.Fields{
		"intermediate tarball":           f.Name(),
		"intermediate package directory": packageDir,
		"container ID":                   container.ID,
		"package":                        req.Package,
	})
	log.Info("Translating docker image to the gravity package.")

	defer func() {
		log.Info("Removing container.")
		if errRemove := req.RemoveContainer(dockerapi.RemoveContainerOptions{
			ID:    container.ID,
			Force: true,
		}); errRemove != nil {
			log.Warnf("Failed to remove container: %v.", errRemove)
		}

	}()

	opts := dockerapi.ExportContainerOptions{
		ID:           container.ID,
		OutputStream: f,
	}
	if err = req.ExportContainer(opts); err != nil {
		return trace.Wrap(err)
	}

	if err := f.Sync(); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to flush file buffers")
	}
	if _, err := f.Seek(0, 0); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to seek file")
	}

	rootfsDir := filepath.Join(packageDir, "rootfs")
	if err := os.MkdirAll(rootfsDir, defaults.SharedDirMask); err != nil {
		return trace.ConvertSystemError(err)
	}
	if err := dockerarchive.Untar(f, rootfsDir,
		&dockerarchive.TarOptions{NoLchown: true}); err != nil {
		return trace.Wrap(err)
	}

	if err := utils.CopyFile(
		filepath.Join(packageDir, pack.ManifestFilename),
		filepath.Join(rootfsDir, "/etc/planet", pack.ManifestFilename)); err != nil {
		return trace.Wrap(err)
	}

	log.Info("Compressing intermediate package directory.")
	reader, err := dockerarchive.Tar(packageDir, dockerarchive.Gzip)
	if err != nil {
		return trace.Wrap(err)
	}

	err = req.UpsertRepository(req.Package.Repository, time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}

	log.Info("Creating resulting package.")
	_, err = req.UpsertPackage(req.Package, reader,
		pack.WithLabels(pack.RuntimePackageLabels))
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	return nil
}

// TranslateImageRequest describes a request to translate runtime docker
// image to telekube package
type TranslateImageRequest struct {
	// Image defines the docker image to translate.
	// The image must have been already pulled and available
	// locally
	Image string
	// Package specifies the resulting telekube package
	Package loc.Locator
	// Client is the docker client
	DockerInterface
	// PackageService is the package service to create package in
	pack.PackageService
}
