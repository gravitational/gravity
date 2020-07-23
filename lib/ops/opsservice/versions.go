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
	"github.com/sirupsen/logrus"
)

// The purpose of the code in this module is to prevent users from triggering
// unsupported upgrade paths between Gravity versions during cluster upgrades.
//
// An upgrade path is generally considered unsupported if the "upgrade from"
// and "upgrade to" versions are too many major/minor versions apart such as
// the direct upgrade between their respective embedded Kubernetes versions
// is not supported according to Kubernetes version skew policy (normally 2
// releases):
//
// https://kubernetes.io/docs/setup/release/version-skew-policy/
//
// Kubernetes versions embedded into all Gravity releases are available on
// our releases page:
//
// https://gravitational.com/gravity/docs/changelog/
//
// Some Gravity versions support automatic upgrades between far-apart versions
// if the "upgrade to" cluster image includes one or more runtimes that can
// be used as intermediate hops:
//
// https://gravitational.com/gravity/docs/cluster/#direct-upgrades-from-older-lts-versions

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
		semver.New("5.4.0"),
		semver.New("5.5.0"),
		semver.New("5.6.0"),
		semver.New("6.0.0"),
		semver.New("6.1.0"),
	}

	// UpgradeViaVersions maps older gravity versions to versions that can be
	// used as an intermediate step when upgrading to the current version.
	//
	// Specified versions are treated as described above.
	//
	// This version does not currently support upgrades via intermediate
	// runtimes.
	UpgradeViaVersions = map[*semver.Version]Versions{}
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
	// FieldLogger is used for logging.
	logrus.FieldLogger
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
	if r.FieldLogger == nil {
		r.FieldLogger = logrus.WithFields(logrus.Fields{
			trace.Component: "vercheck",
			"from":          r.fromRuntime,
			"to":            r.toRuntime,
		})
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
	versionTo, err := req.toRuntime.SemVer()
	if err != nil {
		return trace.Wrap(err)
	}
	// Shortcut to see if runtime version hasn't changed.
	if versionTo.Equal(*versionFrom) {
		req.Info("Runtime version unchanged.")
		return nil
	}
	// Downgrades between runtimes are not allowed.
	if versionTo.LessThan(*versionFrom) {
		req.Warn("Runtime downgrades are not allowed.")
		return trace.BadParameter(downgradeErrorTpl, req.fromRuntime.Version,
			req.toRuntime.Version)
	}
	// See if there's direct upgrade path from the currently installed version.
	if req.supportsDirectUpgrade(*versionFrom) {
		req.Info("Validated runtime upgrade path: direct.")
		return nil
	}
	// There's no direct upgrade path between the versions, see if there's
	// an upgrade path with intermediate hops.
	intermediateVersions := req.supportsUpgradeVia(*versionFrom)
	if len(intermediateVersions) == 0 {
		req.Warn("Unsupported upgrade path.")
		return trace.BadParameter(unsupportedErrorTpl, req.fromRuntime.Version,
			req.toRuntime.Version)
	}
	// Make sure that for each required intermediate runtime version, there's
	// a corresponding package in the cluster's package service.
	runtimes, err := checkIntermediateRuntimes(req, intermediateVersions)
	if err != nil {
		return trace.Wrap(err)
	}
	req.WithField("via", runtimes).Info("Validated runtime upgrade path: with intermediate.")
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
			req.Warnf("Required intermediate runtime for %v not found.", intermediateVersion)
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
	// downgradeErrorTpl is template of an error message that gets returned
	// to a user when new runtime version is less than the installed one.
	downgradeErrorTpl = `Downgrade between Gravity versions %v and %v is not allowed.`
	// needsIntermediateErrorTpl is template of an error message that gets
	// returned to a user when upgrade path requires intermediate runtimes.
	needsIntermediateErrorTpl = `Upgrade between Gravity versions %v and %v is only supported if cluster image includes the following intermediate runtimes:
    %s
This cluster image does not contain required intermediate runtimes.
Please rebuild it as described in https://gravitational.com/gravity/docs/cluster/#direct-upgrades-from-older-lts-versions.`
)

var (
	// TeleportBrokenJoinTokenVersion is version of the release affected by
	// the issue with Teleport using incorrect auth token on joined nodes.
	//
	// Github issue: https://github.com/gravitational/gravity/issues/1445.
	// KB: https://community.gravitational.com/t/recover-teleport-nodes-failing-to-join-due-to-bad-token/649.
	TeleportBrokenJoinTokenVersion = semver.New("5.5.40")
)
