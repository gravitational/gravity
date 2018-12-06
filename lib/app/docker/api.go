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
	"github.com/docker/distribution/context"
	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/gravitational/gravity/lib/utils"
)

// DockerInterfaces defines an interface to docker
type DockerInterface interface {
	// InspectImage retrieves metadata for the specified image
	InspectImage(name string) (*dockerapi.Image, error)
	// TagImage tags the image specified with name
	TagImage(name string, opt dockerapi.TagImageOptions) error
	// PushImage pushes the image specified with opts using the specified
	// authentication configuration
	PushImage(opts dockerapi.PushImageOptions, auth dockerapi.AuthConfiguration) error
	// RemoveImage removes the specified image
	RemoveImage(image string) error
	// CreateContainer creates a container instance based on the given configuration
	CreateContainer(opts dockerapi.CreateContainerOptions) (*dockerapi.Container, error)
	// RemoveContainer removes the container given with opts
	RemoveContainer(opts dockerapi.RemoveContainerOptions) error
	// ExportContainer exports the contents of the running container given with opts
	// as a tarball
	ExportContainer(opts dockerapi.ExportContainerOptions) error
}

// ImageService defines an interface to a private docker registry
type ImageService interface {
	// Sync synchronizes the contents of dir with this private docker registry
	// Returns the list of images synced
	Sync(ctx context.Context, dir string, progress utils.Emitter) ([]TagSpec, error)

	// Wrap translates the specified image name to point to the private registry.
	Wrap(image string) string

	// Unwrap translates the specified image name to point to the original repository
	// if it's prefixed with this registry address - functional inverse of Wrap
	Unwrap(image string) string

	// Registry returns the registry address for this image service
	Registry() string
}

// DockerPuller defines an interface to pull images
type DockerPuller interface {
	// Pull pulls the specified image
	Pull(image string) error
	// IsImagePresent checks if the specified image is available locally
	IsImagePresent(image string) (bool, error)
}
