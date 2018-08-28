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

package pack

import (
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"

	"github.com/gravitational/trace"
)

// MakeLocator takes an app package and makes a locator out of it:
//  - if it's in the 'repo/name:ver' format, returns it
//  - if it's in the 'name:ver' format, returns locator with system repo (systemrepo/name:ver)
//  - if it's in the 'name' format, returns locator with system repo and latest meta-version (systemrepo/name:0.0.0+latest)
func MakeLocator(app string) (*loc.Locator, error) {
	locator, err := loc.ParseLocator(app)
	if err == nil {
		return locator, nil
	}
	parts := strings.Split(app, ":")
	if len(parts) == 1 {
		return loc.NewLocator(defaults.SystemAccountOrg, app, loc.LatestVersion)
	}
	if len(parts) == 2 {
		version := parts[1]
		switch version {
		case constants.LatestVersion:
			version = loc.LatestVersion
		case constants.StableVersion:
			version = loc.StableVersion
		}
		return loc.NewLocator(defaults.SystemAccountOrg, parts[0], version)
	}
	return nil, trace.BadParameter(
		"invalid app name format: %v, should be: 'repo/name:ver' or 'name:ver' or 'name'", app)
}
