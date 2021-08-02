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
	"context"

	"github.com/gravitational/gravity/lib/utils"

	dockerapi "github.com/fsouza/go-dockerclient"
)

// Interface defines an interface to docker
type Interface interface {
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
	// Version returns version information about the docker server.
	Version() (*dockerapi.Env, error)
}

// ImageService defines an interface to a private docker registry
type ImageService interface {
	// String provides a text representation of this service
	String() string

	// Sync synchronizes the contents of dir with this private docker registry
	// Returns the list of images synced
	Sync(ctx context.Context, dir string, progress utils.Printer) ([]TagSpec, error)

	// Wrap translates the specified image name to point to the private registry.
	Wrap(image string) string

	// Unwrap translates the specified image name to point to the original repository
	// if it's prefixed with this registry address - functional inverse of Wrap
	Unwrap(image string) string

	// List fetches a list of all images from the registry
	List(context.Context) ([]Image, error)
}

// Image represents a single Docker image.
type Image struct {
	// Repository is the image name.
	Repository string `json:"repository" yaml:"repository"`
	// Tags is the image tags.
	Tags []string `json:"tags" yaml:"tags"`
}

// PullService defines an interface to pull images
type PullService interface {
	// Pull pulls the specified image
	Pull(image string) error
	// IsImagePresent checks if the specified image is available locally
	IsImagePresent(image string) (bool, error)
}
