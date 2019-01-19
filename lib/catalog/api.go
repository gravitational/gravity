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
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField(trace.Component, "catalog")

// SearchRequest describes an application search request.
type SearchRequest struct {
	// Pattern is an optional application name pattern (a substring).
	Pattern string
	// Local is whether to search local cluster catalog.
	Local bool
	// Remote is whether to search remote Ops Center catalog.
	Remote bool
}

// SearchResult is an application search result.
type SearchResult struct {
	// Apps maps the catalog name (such as 'get.gravitational.io' or local
	// cluster name) to a list of applications found in it.
	Apps map[string][]app.Application
}

// Search searches local and/or remote catalogs for the specified application.
func Search(req SearchRequest) (*SearchResult, error) {
	log.Debugf("%#v", req)
	var catalogs []Catalog
	if req.Local {
		local, err := NewLocal()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		catalogs = append(catalogs, local)
	}
	if req.Remote {
		remote, err := NewRemote()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		catalogs = append(catalogs, remote)
	}
	result := &SearchResult{
		Apps: make(map[string][]app.Application),
	}
	for _, catalog := range catalogs {
		apps, err := catalog.Search(req.Pattern)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result.Apps[catalog.GetName()] = apps
	}
	return result, nil
}

// DownloadRequest describes a request to download an application tarball.
type DownloadRequest struct {
	// Application specifies application to download.
	Application loc.Locator
}

// DownloadResult is an application download result.
type DownloadResult struct {
	// Path is the path to the downloaded tarball.
	Path string
}

// Close removes the downloaded tarball.
//
// Implements io.Closer.
func (r *DownloadResult) Close() error {
	return os.RemoveAll(filepath.Dir(r.Path))
}

// Download downloads the specified application and returns its path.
func Download(req DownloadRequest) (*DownloadResult, error) {
	log.Debugf("%#v", req)
	localCluster, err := localenv.LocalCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var catalog Catalog
	switch req.Application.Repository {
	case "", localCluster.Domain: // local cluster app is requested
		catalog, err = NewLocal()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default: // app from remote catalog (Ops Center) is requested
		catalog, err = NewRemoteFor(req.Application.Repository)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	reader, err := catalog.Download(req.Application.Name, req.Application.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	tmpDir, err := ioutil.TempDir("", "app")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	path := filepath.Join(tmpDir, filename(req.Application.Name, req.Application.Version))
	err = utils.CopyReader(path, reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DownloadResult{
		Path: path,
	}, nil
}

func filename(name, version string) string {
	return fmt.Sprintf("%v-%v.tar", name, version)
}
