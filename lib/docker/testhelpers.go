package docker

import (
	"context"

	"github.com/gravitational/gravity/lib/utils"

	dockerapi "github.com/fsouza/go-dockerclient"
)

// MockDocker is a mock Docker client implementation used in unit tests.
type MockDocker struct{}

// InspectImage retrieves metadata for the specified image
func (d *MockDocker) InspectImage(name string) (*dockerapi.Image, error) {
	return nil, nil
}

// TagImage tags the image specified with name
func (d *MockDocker) TagImage(name string, opt dockerapi.TagImageOptions) error {
	return nil
}

// PushImage pushes the image specified with opts using the specified
// authentication configuration
func (d *MockDocker) PushImage(opts dockerapi.PushImageOptions, auth dockerapi.AuthConfiguration) error {
	return nil
}

// RemoveImage removes the specified image
func (d *MockDocker) RemoveImage(image string) error {
	return nil
}

// CreateContainer creates a container instance based on the given configuration
func (d *MockDocker) CreateContainer(opts dockerapi.CreateContainerOptions) (*dockerapi.Container, error) {
	return nil, nil
}

// RemoveContainer removes the container given with opts
func (d *MockDocker) RemoveContainer(opts dockerapi.RemoveContainerOptions) error {
	return nil
}

// ExportContainer exports the contents of the running container given with opts
// as a tarball
func (d *MockDocker) ExportContainer(opts dockerapi.ExportContainerOptions) error {
	return nil
}

// Version returns version information about the docker server.
func (d *MockDocker) Version() (*dockerapi.Env, error) {
	return nil, nil
}

// MockImageService is a mock image service implementation used in unit tests.
type MockImageService struct{}

// SyncFrom synchronizes the contents of dir with this private docker registry
// Returns the list of images synced
func (s *MockImageService) SyncFrom(ctx context.Context, dir string, progress utils.Printer) ([]TagSpec, error) {
	return nil, nil
}

// SyncTo pulls the specified images from this image service into the specified directory.
func (s *MockImageService) SyncTo(ctx context.Context, dir string, images []Image, progress utils.Printer) error {
	return nil
}

// Wrap translates the specified image name to point to the private registry.
func (s *MockImageService) Wrap(image string) string {
	return image
}

// Unwrap translates the specified image name to point to the original repository
// if it's prefixed with this registry address - functional inverse of Wrap
func (s *MockImageService) Unwrap(image string) string {
	return image
}

// List fetches a list of all images from the registry
func (s *MockImageService) List(context.Context) ([]Image, error) {
	return nil, nil
}
