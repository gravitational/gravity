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
	"github.com/gravitational/trace"
	serf "github.com/hashicorp/serf/client"
	"github.com/hashicorp/serf/coordinate"
)

// MockSerfClient is a mock/fake Serf Client used in testing
type MockSerfClient struct {
	members []serf.Member
	coords  map[string]*coordinate.Coordinate
}

// NewMockSerfClient is a helper function used to create a mock/fake Serf Client
// used in testing
func NewMockSerfClient(members []serf.Member, coords map[string]*coordinate.Coordinate) *MockSerfClient {
	return &MockSerfClient{
		members: members,
		coords:  coords,
	}
}

// Members is a function that returns the Serf member nodes
func (c *MockSerfClient) Members() ([]serf.Member, error) {
	return c.members, nil
}

// FindMember returns serf member by name
func (c *MockSerfClient) FindMember(name string) (*serf.Member, error) {
	for _, member := range c.members {
		if member.Name == name {
			return &member, nil
		}
	}
	return nil, trace.NotFound("member %v not found", name)
}

// Stop is a NOOP function used to implement the Mock Serf Client
func (c *MockSerfClient) Stop(serf.StreamHandle) error {
	return nil
}

// Close is a NOOP function used to implement the Mock Serf Client
func (c *MockSerfClient) Close() error {
	return nil
}

// Join is a NOOP function used to implement the Mock Serf Client
func (c *MockSerfClient) Join(peers []string, replay bool) (int, error) {
	return 0, nil
}

// UpdateTags is a NOOP function used to implement the Mock Serf Client
func (c *MockSerfClient) UpdateTags(tags map[string]string, delTags []string) error {
	return nil
}

// GetCoordinate get&returns the (fake) Serf Coordinate for a specific (fake) node
// and it's mostly used during testing
func (c *MockSerfClient) GetCoordinate(node string) (*coordinate.Coordinate, error) {
	return c.coords[node], nil
}
