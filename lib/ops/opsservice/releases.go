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

package opsservice

import (
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// ListReleases returns all currently installed application releases.
func (o *Operator) ListReleases(req ops.ListReleasesRequest) ([]storage.Release, error) {
	releases, err := helm.List(helm.ListParameters{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !req.IncludeIcons {
		return releases, nil
	}
	for i, release := range releases {
		app, err := o.cfg.Apps.GetApp(release.GetLocator())
		if err != nil {
			o.Warnf("Failed to retrieve app for release %v: %v.",
				release, trace.Wrap(err))
			continue
		}
		releases[i].SetChartIcon(app.Manifest.Logo)
	}
	return releases, nil
}
