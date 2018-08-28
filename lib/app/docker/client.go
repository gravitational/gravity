package docker

import (
	"time"

	"github.com/gravitational/trace"

	dockerapi "github.com/fsouza/go-dockerclient"
)

// NewClient creates a docker client using endpoint for connection
func NewClient(endpoint string) (*dockerapi.Client, error) {
	return NewClientWithTimeout(endpoint, time.Duration(0))
}

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
