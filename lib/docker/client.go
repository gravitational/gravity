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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/gravitational/trace"
)

// NewClientFromEnv creates a docker client using environment
func NewClientFromEnv() (*dockerapi.Client, error) {
	client, err := dockerapi.NewClientFromEnv()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = client.Version()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// NewClientWithTimeout creates a docker client using endpoint for connection
// and a time limit for requests
func NewClientWithTimeout(endpoint string, timeout time.Duration) (*dockerapi.Client, error) {
	client, err := dockerapi.NewClient(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client.HTTPClient.Timeout = timeout
	return client, nil
}

// NewDefaultClient returns a new docker client using defaults
func NewDefaultClient() (DockerInterface, error) {
	endpoint := utils.GetenvWithDefault("DOCKER_HOST", constants.DockerEngineURL)
	client, err := NewClient(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// NewClient creates a docker client using endpoint for connection
func NewClient(endpoint string) (*dockerapi.Client, error) {
	return NewClientWithTimeout(endpoint, time.Duration(0))
}
