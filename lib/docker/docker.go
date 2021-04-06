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
	"bytes"
	"os/exec"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"

	dockerapi "github.com/fsouza/go-dockerclient"
	log "github.com/sirupsen/logrus"
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

// Login logs into a Docker registry.
func Login(name, username, password string) error {
	cmd := exec.Command("docker", "login", "-u", username, "-p", password, name)
	var out bytes.Buffer
	err := utils.ExecL(cmd, &out, log.WithField(trace.Component, "docker"))
	if err != nil {
		return trace.Wrap(err, out.String())
	}
	return nil
}

// Logout logs out of a Docker registry.
func Logout(name string) error {
	cmd := exec.Command("docker", "logout", name)
	var out bytes.Buffer
	err := utils.ExecL(cmd, &out, log.WithField(trace.Component, "docker"))
	if err != nil {
		return trace.Wrap(err, out.String())
	}
	return nil
}
