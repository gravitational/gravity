/*
Copyright 2019 Gravitational, Inc.

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

package helm

import (
	"sort"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// testClient is the mock Helm client implementation used in tests.
type testClient struct {
	// releases keeps track of the installed releases.
	releases map[string]storage.Release
	// ClientConfig is the test client configuration.
	ClientConfig
}

// NewTestClient creates a new test Helm client.
func NewTestClient(conf ClientConfig) (Client, error) {
	return &testClient{
		releases:     make(map[string]storage.Release),
		ClientConfig: conf,
	}, nil
}

// Install installs a Helm chart and returns release information.
func (c *testClient) Install(p InstallParameters) (storage.Release, error) {
	c.releases[p.Name] = newRelease(p.Name, 0)
	return c.releases[p.Name], nil
}

// List returns list of releases matching provided parameters.
func (c *testClient) List(p ListParameters) ([]storage.Release, error) {
	// Return with keys (release names) sorted alphabetically to have
	// a stable order.
	var keys []string
	for k := range c.releases {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var releases []storage.Release
	for _, k := range keys {
		releases = append(releases, c.releases[k])
	}
	return releases, nil
}

// Get returns a single release with the specified name.
func (c *testClient) Get(name string) (storage.Release, error) {
	release, ok := c.releases[name]
	if !ok {
		return nil, trace.NotFound("release %v not found", name)
	}
	return release, nil
}

// Upgrade upgrades a release.
func (c *testClient) Upgrade(p UpgradeParameters) (storage.Release, error) {
	release, ok := c.releases[p.Release]
	if !ok {
		return nil, trace.NotFound("release %v not found", p.Release)
	}
	c.releases[p.Release] = newRelease(p.Release, release.GetRevision()+1)
	return c.releases[p.Release], nil
}

// Rollback rolls back a release to the specified version.
func (c *testClient) Rollback(p RollbackParameters) (storage.Release, error) {
	_, ok := c.releases[p.Release]
	if !ok {
		return nil, trace.NotFound("release %v not found", p.Release)
	}
	c.releases[p.Release] = newRelease(p.Release, p.Revision)
	return c.releases[p.Release], nil
}

// Revisions returns revision history for a release with the provided name.
func (c *testClient) Revisions(name string) ([]storage.Release, error) {
	return nil, trace.NotImplemented("not implemented")
}

// Uninstall uninstalls a release with the provided name.
func (c *testClient) Uninstall(name string) (storage.Release, error) {
	release, ok := c.releases[name]
	if !ok {
		return nil, trace.NotFound("release %v not found", name)
	}
	delete(c.releases, name)
	return release, nil
}

// Closer allows to cleanup the client.
func (c *testClient) Close() error {
	return nil
}

// Ping pings the Tiller pod and ensures it's up and running.
func (c *testClient) Ping() error {
	return nil
}

func newRelease(name string, revision int) storage.Release {
	return &storage.ReleaseV1{
		Kind:    storage.KindRelease,
		Version: services.V1,
		Metadata: services.Metadata{
			Name: name,
		},
	}
}
