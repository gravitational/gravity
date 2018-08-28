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
