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

package catalog

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// Catalog defines an interface for an application catalog.
type Catalog interface {
	// Search searches the application catalog.
	Search(pattern string) (apps map[string][]app.Application, err error)
	// Download downloads an application from the catalog.
	Download(name, version string) (path string, err error)
}

// New returns a new application catalog instance.
func New(config Config) (*catalog, error) {
	err := config.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &catalog{
		Config: config,
	}, nil
}

// catalog implements Catalog.
type catalog struct {
	// Config is the catalog configuration.
	Config
}

// Config is the application catalog configuration.
type Config struct {
	// Name is the catalog name.
	Name string
	// Operator is the cluster or Ops Center operator.
	Operator ops.Operator
	// Apps is the cluster or Ops Center application service.
	Apps app.Applications
}

// Check validates the application catalog config.
func (c Config) Check() error {
	if c.Name == "" {
		return trace.BadParameter("missing Name")
	}
	if c.Operator == nil {
		return trace.BadParameter("missing Operator")
	}
	if c.Apps == nil {
		return trace.BadParameter("missing Apps")
	}
	return nil
}

// Search searches for applications in the catalog.
func (c *catalog) Search(pattern string) (map[string][]app.Application, error) {
	apps, err := c.Apps.ListApps(app.ListAppsRequest{
		Repository: defaults.SystemAccountOrg,
		Pattern:    pattern,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return map[string][]app.Application{
		c.Name: apps,
	}, nil
}

// Download downloads the specified application from the catalog.
func (c *catalog) Download(name, version string) (string, error) {
	reader, err := c.Operator.GetAppInstaller(ops.AppInstallerRequest{
		AccountID: defaults.SystemAccountID,
		Application: loc.Locator{
			Repository: defaults.SystemAccountOrg,
			Name:       name,
			Version:    version,
		},
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	defer reader.Close()
	tmpDir, err := ioutil.TempDir("", "app")
	if err != nil {
		return "", trace.Wrap(err)
	}
	path := filepath.Join(tmpDir, filename(name, version))
	err = utils.CopyReader(path, reader)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return path, nil
}

func filename(name, version string) string {
	return fmt.Sprintf("%v-%v.tar", name, version)
}
