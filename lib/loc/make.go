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

package loc

import (
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
)

// MakeLocator takes an app package and makes a locator out of it:
//  - if it's in the 'repo/name:ver' format, returns it
//  - if it's in the 'name:ver' format, returns locator with system repo (systemrepo/name:ver)
//  - if it's in the 'name' format, returns locator with system repo and latest meta-version (systemrepo/name:0.0.0+latest)
func MakeLocator(app string) (*Locator, error) {
	return MakeLocatorWithDefault(app, func(name string) string {
		return LatestVersion
	})
}

// MakeLocatorWithDefault is like MakeLocator but uses the provided default
// version if the one isn't specified explicitly, instead of defaulting to
// the latest.
func MakeLocatorWithDefault(app string, defaultVersion defaultVersionFunc) (*Locator, error) {
	locator, err := ParseLocator(app)
	if err == nil {
		return locator, nil
	}
	parts := strings.Split(app, ":")
	if len(parts) == 1 {
		return NewLocator(defaults.SystemAccountOrg, app, defaultVersion(app))
	}
	if len(parts) == 2 {
		version := parts[1]
		switch version {
		case constants.LatestVersion:
			version = LatestVersion
		case constants.StableVersion:
			version = StableVersion
		}
		return NewLocator(defaults.SystemAccountOrg, parts[0], version)
	}
	return nil, trace.BadParameter("invalid package name format %q, expected 'repository/name:version' or 'name:version' or 'name'", app)
}

// defaultVersionFunc defines function that returns default version for
// specified application name.
type defaultVersionFunc func(name string) string
