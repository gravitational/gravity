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

package utils

import (
	"regexp"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
)

var semverRePattern = `^([a-zA-Z0-9\-\.]*)$`
var semverRe = regexp.MustCompile(semverRePattern)

// SanitizeSemver validates semver pre-release/metadata fields are alphanumeric characters dash and dot as per
// https://semver.org/#semantic-versioning-specification-semver
func SanitizeSemver(ver semver.Version) error {
	if !semverRe.Match([]byte(ver.PreRelease)) {
		return trace.BadParameter("Semver pre-release failed validation.")
	}
	if !semverRe.Match([]byte(ver.Metadata)) {
		return trace.BadParameter("Semver metadata failed validation.")
	}
	return nil
}
