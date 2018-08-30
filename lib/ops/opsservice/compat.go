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

package opsservice

import (
	"strconv"

	"github.com/gravitational/gravity/lib/loc"

	log "github.com/sirupsen/logrus"
	"github.com/gravitational/trace"
)

// isPlanetCompatible determines if the specified planet packages are compatible.
// Compatibility is defined by verifying that the prerelease version component
// of the older package is not below a certain predefined minimum.
//
// The compatibility is important to decide when and if certain operations are
// executed or not.
func isPlanetCompatible(installedPackage, newPackage loc.Locator) (bool, error) {
	verInstalled, err := installedPackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	verNew, err := newPackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	installedNewer := verInstalled.Compare(*verNew) > 0
	minVer := verInstalled
	if installedNewer {
		minVer = verNew
	}

	kubernetesRelease, err := strconv.ParseInt(string(minVer.PreRelease), 10, 64)
	if err != nil {
		log.Warningf("failed to parse kubernetes release: %v", err)
		return false, trace.BadParameter("invalid planet version %q: expected numeric release value but got %q", minVer, minVer.PreRelease)
	}
	return kubernetesRelease >= kubernetesBaseRelease, nil
}

// kubernetesBaseRelease defines the minimum kubernetes release version considered compatible
// (as majorminorpatch)
var kubernetesBaseRelease int64 = 150
