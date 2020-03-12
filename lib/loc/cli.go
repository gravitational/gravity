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
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Locators represents a list of package locators.
type Locators []Locator

// IsCumulative indicates that Locators is a cumulative argument.
func (*Locators) IsCumulative() bool {
	return true
}

// Set sets the value of a Locator if the supplied string is a valid locator, returning
// any errors encountered.
func (l *Locators) Set(v string) error {
	out, err := ParseLocator(v)
	if err != nil {
		return trace.Wrap(err, "could not parse %v", v)
	}
	*l = append(*l, *out)
	return nil
}

// String returns a string representation of the Locators type.
func (l *Locators) String() string {
	return fmt.Sprintf("%v", []Locator(*l))
}

// LocatorSlice creates a collection of Locators from a kingpin command line argument.
func LocatorSlice(s kingpin.Settings) *Locators {
	var locs Locators
	s.SetValue(&locs)
	return &locs
}

// DockerImages represent a slice of DockerImage.
type DockerImages []DockerImage

// IsCumulative indicates that DockerImages is a cumulative argument.
func (*DockerImages) IsCumulative() bool {
	return true
}

// Set sets the value of a DockerImage if the supplied string is a valid image locator,
// returning any errors encountered.
func (i *DockerImages) Set(v string) error {
	img, err := ParseDockerImage(v)
	if err != nil {
		return trace.Wrap(err, "could not parse %v", v)
	}
	*i = append(*i, *img)
	return nil
}

// String returns a string representation of the DockerImages type.
func (l *DockerImages) String() string {
	return fmt.Sprintf("%v", []DockerImage(*l))
}

// ImagesSlices creates a collection of DockerImages from a kingpin command line argument.
func ImagesSlice(s kingpin.Settings) *DockerImages {
	var images DockerImages
	s.SetValue(&images)
	return &images
}

// IsUpdate returns true if the specified locator is of newer version than the one in
// the provided list, or if it's missing from the list (meaning it's a new package)
func IsUpdate(update Locator, installed []Locator) (bool, error) {
	for _, locator := range installed {
		if !IsSameApp(update, locator) {
			continue
		}
		installedVer, err := locator.SemVer()
		if err != nil {
			return false, trace.Wrap(err)
		}
		updateVer, err := update.SemVer()
		if err != nil {
			return false, trace.Wrap(err)
		}
		if !installedVer.LessThan(*updateVer) {
			return false, nil
		}
	}
	return true, nil
}

// IsSameApp returns true if the provided locators have the same repository and name
func IsSameApp(app1, app2 Locator) bool {
	return app1.Repository == app2.Repository && app1.Name == app2.Name
}

// GreaterOrEqualPatch returns true if the left version has the same major/minor
// components as the right version but a greater or equal patch component.
func GreaterOrEqualPatch(left, right semver.Version) bool {
	return left.Major == right.Major && left.Minor == right.Minor && left.Patch >= right.Patch
}
