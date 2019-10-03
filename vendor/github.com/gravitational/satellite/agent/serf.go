/*
Copyright 2016 Gravitational, Inc.

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

package agent

import (
	"sync"

	"github.com/gravitational/trace"

	serf "github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/coordinate"
)

// SerfClient is the minimal interface to the serf cluster.
// It enables mocking access to serf network in tests.
type SerfClient interface {
	// Members lists members of the serf cluster.
	Members() ([]serf.Member, error)
	// FindMember finds serf member with the specified name.
	FindMember(name string) (*serf.Member, error)
	// Stop cancels the serf event delivery and removes the subscription.
	Stop(serf.StreamHandle) error
	// Close closes the client.
	Close() error
	// Join attempts to join an existing serf cluster identified by peers.
	// Replay controls if previous user events are replayed once this node has joined the cluster.
	// Returns the number of nodes joined
	Join(peers []string, replay bool) (int, error)
	// UpdateTags will modify the tags on a running serf agent
	UpdateTags(tags map[string]string, delTags []string) error
	// GetCoordinate returns the Serf Coordinate for a specific Node
	GetCoordinate(node string) (*coordinate.Coordinate, error)
}

type NewSerfClientFunc func(serf.Config) (SerfClient, error)

// NewSerfClient returns a new serf client for the specified configuration.
// The client will attempt to reconnect if it detects that the connection to the
// serf agent has been lost
func NewSerfClient(clientConfig serf.Config) (SerfClient, error) {
	client, err := reinit(clientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &retryingClient{
		client: client,
		config: clientConfig,
	}, nil
}

// Members lists members of the serf cluster.
func (r *retryingClient) Members() ([]serf.Member, error) {
	if err := r.reinit(); err != nil {
		return nil, trace.Wrap(err)
	}
	r.RLock()
	defer r.RUnlock()
	return r.client.Members()
}

// FindMember finds serf member with the specified name.
func (r *retryingClient) FindMember(name string) (*serf.Member, error) {
	members, err := r.Members()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, member := range members {
		if member.Name == name {
			return &member, nil
		}
	}
	return nil, trace.NotFound("serf member %q not found", name)
}

// Stop cancels the serf event delivery and removes the subscription.
func (r *retryingClient) Stop(handle serf.StreamHandle) error {
	if err := r.reinit(); err != nil {
		return trace.Wrap(err)
	}
	r.RLock()
	defer r.RUnlock()
	return r.client.Stop(handle)
}

// Join attempts to join an existing serf cluster identified by peers.
// Replay controls if previous user events are replayed once this node has joined the cluster.
// Returns the number of nodes joined
func (r *retryingClient) Join(peers []string, replay bool) (int, error) {
	if err := r.reinit(); err != nil {
		return 0, trace.Wrap(err)
	}
	r.RLock()
	defer r.RUnlock()
	return r.client.Join(peers, replay)
}

// UpdateTags will modify the tags on a running serf agent
func (r *retryingClient) UpdateTags(tags map[string]string, delTags []string) error {
	if err := r.reinit(); err != nil {
		return trace.Wrap(err)
	}
	r.RLock()
	defer r.RUnlock()
	return r.client.UpdateTags(tags, delTags)
}

// Close closes the client
func (r *retryingClient) Close() error {
	r.RLock()
	defer r.RUnlock()
	if r.client.IsClosed() {
		return nil
	}
	return r.client.Close()
}

// GetCoordinate returns the Serf Coordinate for a specific node
func (r *retryingClient) GetCoordinate(node string) (*coordinate.Coordinate, error) {
	if err := r.reinit(); err != nil {
		return nil, trace.Wrap(err)
	}
	r.RLock()
	defer r.RUnlock()
	return r.client.GetCoordinate(node)
}

func (r *retryingClient) reinit() (err error) {
	r.Lock()
	defer r.Unlock()
	client := r.client
	if !client.IsClosed() {
		return nil
	}
	client, err = reinit(r.config)
	if err != nil {
		return trace.Wrap(err)
	}
	r.client = client
	return nil
}

func reinit(clientConfig serf.Config) (*serf.RPCClient, error) {
	client, err := serf.ClientFromConfig(&clientConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

type retryingClient struct {
	sync.RWMutex
	client *serf.RPCClient
	config serf.Config
}
