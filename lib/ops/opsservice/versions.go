/*
Copyright 2020 Gravitational, Inc.

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
	"strings"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

var (
	// DirectUpgradeVersions contains gravity versions where there's a direct
	// upgrade path to the current gravity version.
	//
	// Specified versions are treated as a minimum version of the respective
	// release line from which the direct upgrade is possible.
	//
	// For instance, if the version is 5.2.10, the current version can upgrade
	// directly from 5.2.10, 5.2.11 and so on.
	DirectUpgradeVersions = Versions{
		semver.New("5.2.0"),
		semver.New("5.3.0"),
		semver.New("5.4.0"),
		semver.New("5.5.0"),
	}

	// UpgradeViaVersions maps older gravity versions to versions that can be
	// used as an intermediate step when upgrading to the current version.
	//
	// Specified versions are treated as described above.
	UpgradeViaVersions = map[*semver.Version]Versions{
		// Upgrades from 5.0 are possible via 5.2.15 and later 5.2 releases.
		semver.New("5.0.0"): Versions{semver.New("5.2.15")},
	}
)

// checkRuntimeUpgradePathRequest is a request to validate upgrade path between runtimes.
type checkRuntimeUpgradePathRequest struct {
	// fromRuntime is the currently installed runtime version.
	fromRuntime loc.Locator
	// toRuntime is the runtime upgrade version.
	toRuntime loc.Locator
	// directUpgradeVersions defines versions that can upgrade directly.
	directUpgradeVersions Versions
	// upgradeViaVersions defines versions that can upgrade with intermediate hops.
	upgradeViaVersions map[*semver.Version]Versions
	// packages is the cluster package service.
	packages pack.PackageService
}

// checkAndSetDefaults validates the request and sets default values.
func (r *checkRuntimeUpgradePathRequest) checkAndSetDefaults() error {
	if r.fromRuntime.IsEmpty() || r.toRuntime.IsEmpty() {
		return trace.BadParameter("runtime versions are not set")
	}
	if r.packages == nil {
		return trace.BadParameter("package service is not set")
	}
	if len(r.directUpgradeVersions) == 0 {
		r.directUpgradeVersions = DirectUpgradeVersions
	}
	if len(r.upgradeViaVersions) == 0 {
		r.upgradeViaVersions = UpgradeViaVersions
	}
	return nil
}

// supportsDirectUpgrade returns true if a direct upgrade path from the
// provided version to the current version is possible.
func (r *checkRuntimeUpgradePathRequest) supportsDirectUpgrade(from semver.Version) bool {
	for _, version := range r.directUpgradeVersions {
		if loc.GreaterOrEqualPatch(from, *version) {
			return true
		}
	}
	return false
}

// supportsUpgradeVia returns a list of runtime versions that can be used as
// intermediate hops to upgrade from the provided version, or nil if there's
// no upgrade path via intermediate versions.
func (r *checkRuntimeUpgradePathRequest) supportsUpgradeVia(from semver.Version) Versions {
	for version, via := range r.upgradeViaVersions {
		if loc.GreaterOrEqualPatch(from, *version) {
			return via
		}
	}
	return nil
}

// checkRuntimeUpgradePath checks that upgrade path between two provided runtimes exists.
//
// An upgrade path between runtimes is considered valid if:
//
//  - Direct upgrade is supported between old and new versions, OR
//  - This is an upgrade with intermediate hops.
func checkRuntimeUpgradePath(req checkRuntimeUpgradePathRequest) error {
	if err := req.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	versionFrom, err := req.fromRuntime.SemVer()
	if err != nil {
		return trace.Wrap(err)
	}
	// See if there's direct upgrade path from the currently installed version.
	if req.supportsDirectUpgrade(*versionFrom) {
		log.WithFields(log.Fields{
			"from": req.fromRuntime,
			"to":   req.toRuntime,
		}).Info("Validated runtime upgrade path: direct.")
		return nil
	}
	// There's no direct upgrade path between the versions, see if there's
	// an upgrade path with intermediate hops.
	intermediateVersions := req.supportsUpgradeVia(*versionFrom)
	if len(intermediateVersions) == 0 {
		return trace.BadParameter(unsupportedErrorTpl, req.fromRuntime, req.toRuntime)
	}
	// Make sure that for each required intermediate runtime version, there's
	// a corresponding package in the cluster's package service.
	runtimes, err := checkIntermediateRuntimes(req, intermediateVersions)
	if err != nil {
		return trace.Wrap(err)
	}
	log.WithFields(log.Fields{
		"from": req.fromRuntime,
		"to":   req.toRuntime,
		"via":  runtimes,
	}).Info("Validated runtime upgrade path: with intermediate.")
	return nil
}

// checkIntermediateRuntimes validates that required intermediate runtimes
// exist in the provided package service and returns their locators.
func checkIntermediateRuntimes(req checkRuntimeUpgradePathRequest, intermediateVersions Versions) (runtimes []loc.Locator, err error) {
	for _, intermediateVersion := range intermediateVersions {
		runtimePackage, err := findIntermediateRuntime(req.packages, intermediateVersion)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if trace.IsNotFound(err) {
			return nil, trace.BadParameter(needsIntermediateErrorTpl, req.fromRuntime.Version,
				req.toRuntime.Version, intermediateVersions)
		}
		runtimes = append(runtimes, runtimePackage.Locator)
	}
	return runtimes, nil
}

// hasIntermediateRuntime searches for a runtime package that satisfies the
// specified required intermediate runtime version.
func findIntermediateRuntime(packages pack.PackageService, intermediateVersion *semver.Version) (*pack.PackageEnvelope, error) {
	return pack.FindPackage(packages, func(e pack.PackageEnvelope) bool {
		if !loc.IsSameApp(e.Locator, loc.Runtime) {
			return false
		}
		versionLabel := e.RuntimeLabels[pack.PurposeRuntimeUpgrade]
		if versionLabel == "" {
			return false
		}
		version, err := semver.NewVersion(versionLabel)
		if err != nil {
			return false
		}
		return loc.GreaterOrEqualPatch(*version, *intermediateVersion)
	})
}

// Versions represents a list of semvers.
type Versions []*semver.Version

// String returns string representation of versions, indicating that these
// versions are treated as minimum patch versions.
func (v Versions) String() string {
	var versions []string
	for _, version := range v {
		versions = append(versions, version.String()+" or greater")
	}
	return strings.Join(versions, ", ")
}

const (
	// unsupportedErrorTpl is template of an error message that gets returned
	// to a user when the upgrade path is unsupported.
	unsupportedErrorTpl = `Upgrade between Gravity versions %v and %v is unsupported.`
	// needsIntermediateErrorTpl is template of an error message that gets
	// returned to a user when upgrade path requires intermediate runtimes.
	needsIntermediateErrorTpl = `Upgrade between Gravity versions %v and %v is only supported if cluster image includes the following intermediate runtimes:
    %s
This cluster image does not contain required intermediate runtimes.
Please rebuild it as described in https://gravitational.com/gravity/docs/cluster/#direct-upgrades-from-older-lts-versions.`
)
