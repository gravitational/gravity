package docker

import (
	"bytes"
	"os/exec"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
	dockerapi "github.com/fsouza/go-dockerclient"
)

// NewDockerPuller returns an instance of DockerPuller using the specified client
// and credentials
func NewDockerPuller(client DockerInterface) *dockerPuller {
	return &dockerPuller{client: client}
}

// dockerPuller implements a DockerPuller
type dockerPuller struct {
	client DockerInterface
}

// Pull pulls an image using "docker pull" command that lets us take advantage of its cached
// credentials for multiple docker registries
func (r *dockerPuller) Pull(image string) error {
	cmd := exec.Command("docker", "pull", image)
	var out bytes.Buffer
	err := utils.ExecL(cmd, &out, log.WithField(trace.Component, constants.ComponentSystem))
	if err != nil {
		return trace.Wrap(err, out.String())
	}
	return nil
}

// IsImagePresent determines if the specified image is available in docker
func (r *dockerPuller) IsImagePresent(image string) (bool, error) {
	_, err := r.client.InspectImage(image)
	if err == nil {
		return true, nil
	}
	if err == dockerapi.ErrNoSuchImage {
		return false, nil
	}
	return false, trace.Wrap(err)
}
