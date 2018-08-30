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

package transfer

import (
	"io"
	"io/ioutil"
	"os"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"

	"github.com/gravitational/trace"
)

// ExportSite transfers site state from specified site into a temporary file
// and returns a reader to it.
// tempDir defines the temporary working directory and should not be deleted
// by caller until returned ReadCloser is closed
func ExportSite(site *storage.Site, src ExportBackend, tempDir string, clusters []storage.TrustedCluster) (io.ReadCloser, error) {
	if tempDir == "" {
		return nil, trace.BadParameter("missing parameter tempDir")
	}
	f, err := ioutil.TempFile(tempDir, "gravity-export")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := f.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	dst, err := keyval.NewBolt(keyval.BoltConfig{Path: f.Name()})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := copySite(site, dst, src, clusters); err != nil {
		dst.Close()
		return nil, trace.Wrap(err)
	}
	if err := dst.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	f, err = os.Open(f.Name())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return f, nil
}

// ImportSite imports site state from the specified path into the provided backend.
func ImportSite(path string, dst storage.Backend) error {
	src, err := keyval.NewBolt(keyval.BoltConfig{Path: path})
	if err != nil {
		return trace.Wrap(err)
	}
	defer src.Close()
	accounts, err := src.GetAccounts()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(accounts) != 1 {
		return trace.BadParameter("expected 1 account, got %v", len(accounts))
	}
	sites, err := src.GetSites(accounts[0].ID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(sites) != 1 {
		return trace.BadParameter("expected 1 site, got %v", len(sites))
	}
	site := sites[0]
	err = copySite(&site, dst, src, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
