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
