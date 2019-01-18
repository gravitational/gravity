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
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"

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
		subResult, err := catalog.Search(req.Pattern)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for name, apps := range subResult {
			result.Apps[name] = apps
		}
	}
	return result, nil
}

// DownloadRequest describes a request to download an application tarball.
type DownloadRequest struct {
	// Locator is an application to download.
	Locator loc.Locator
}

// DownloadResult is an application download result.
type DownloadResult struct {
	// Path is the path to the downloaded tarball.
	Path string
}

// Downloads downloads the specified application and returns its path.
func Download(req DownloadRequest) (*DownloadResult, error) {
	log.Debugf("%#v", req)
	localCluster, err := localenv.LocalCluster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var catalog Catalog
	switch req.Locator.Repository {
	case "", localCluster.Domain: // local cluster app is requested
		catalog, err = NewLocal()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	default: // app from remote catalog (Ops Center) is requested
		catalog, err = NewRemoteFor(req.Locator.Repository)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	path, err := catalog.Download(req.Locator.Name, req.Locator.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DownloadResult{
		Path: path,
	}, nil
}
