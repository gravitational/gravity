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

package versions

import (
	"strings"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"

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
		// Gravity version 7.1.x adopted Kubernetes 1.19.x, but development
		// has stopped on 7.1.x and is being continued on 8.0.x.
		*semver.New("7.1.0"),
		*semver.New("8.0.0"),
		// Keep the latest version to allow upgrade within the current major
		*semver.New("9.0.0"),
	}

	// UpgradeViaVersions maps older gravity versions to versions that can be
	// used as an intermediate step when upgrading to the current version.
	//
	// Specified versions are treated as described above.
	//
	// TODO(dima): remove this warning once the upgrade-via has been completely ported.
	// This version does not currently support upgrades via intermediate
	// runtimes.
	UpgradeViaVersions = map[semver.Version]Versions{
		*semver.New("7.0.0"): {*semver.New("8.0.0")},
	}
)

// Verify checks that upgrade path between two provided runtimes exists.
//
// An upgrade path between runtimes is considered valid if:
//
//  - Direct upgrade is supported between old and new versions, OR
//  - This is an upgrade with intermediate hops.
func (r RuntimeUpgradePath) Verify(m schema.Manifest) error {
	if err := r.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	// Shortcut to see if runtime version hasn't changed.
	if r.To.Equal(*r.From) {
		r.Info("Runtime version unchanged.")
		return nil
	}
	// Downgrades between runtimes are not allowed.
	if r.To.LessThan(*r.From) {
		r.Warn("Runtime downgrades are not allowed.")
		return trace.BadParameter(downgradeErrorTpl, r.From, r.To)
	}
	// See if there's direct upgrade path from the currently installed version.
	if r.SupportsDirectUpgrade() {
		r.Info("Validated runtime upgrade path: direct.")
		return nil
	}
	// There's no direct upgrade path between the versions, see if there's
	// an upgrade path with intermediate hops.
	intermediateVersions := r.SupportsUpgradeVia()
	if len(intermediateVersions) == 0 {
		r.Warn("Unsupported upgrade path.")
		return trace.BadParameter(unsupportedErrorTpl, r.From, r.To)
	}
	// Make sure that for each required intermediate runtime version, there's
	// a corresponding package in the cluster's package service.
	runtimes, err := r.checkIntermediateRuntimes(m, intermediateVersions)
	if err != nil {
		return trace.Wrap(err)
	}
	r.WithField("via", runtimes).Info("Validated runtime upgrade path: with intermediate.")
	return nil
}

// SupportsDirectUpgrade returns true if a direct upgrade path from the
// source version to the desired version is possible.
func (r RuntimeUpgradePath) SupportsDirectUpgrade() bool {
	upgradeVersions := r.DirectUpgradeVersions
	if len(upgradeVersions) == 0 {
		upgradeVersions = DirectUpgradeVersions
	}
	for _, version := range upgradeVersions {
		if loc.GreaterOrEqualPatch(*r.From, version) {
			return true
		}
	}
	return false
}

// SupportsUpgradeVia returns a list of runtime versions that can be used as
// intermediate hops to upgrade from the source version.
// Returns an empty slice when no upgrade path via intermediate versions is available.
func (r RuntimeUpgradePath) SupportsUpgradeVia() Versions {
	upgradeVersions := r.UpgradeViaVersions
	if len(upgradeVersions) == 0 {
		upgradeVersions = UpgradeViaVersions
	}
	for version, via := range upgradeVersions {
		if loc.GreaterOrEqualPatch(*r.From, version) {
			return via
		}
	}
	return nil
}

// RuntimeUpgradePath describes a possible upgrade path from a specific
// runtime application version to another version
type RuntimeUpgradePath struct {
	// From is the currently installed runtime version.
	From *semver.Version
	// To is the runtime upgrade version.
	To *semver.Version
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// DirectUpgradeVersions defines versions that can upgrade directly.
	DirectUpgradeVersions Versions
	// UpgradeViaVersions defines versions that can upgrade with intermediate hops.
	UpgradeViaVersions map[semver.Version]Versions
}

// Versions represents a list of semvers.
type Versions []semver.Version

// String returns string representation of versions, indicating that these
// versions are treated as minimum patch versions.
func (v Versions) String() string {
	var versions []string
	for _, version := range v {
		versions = append(versions, version.String()+" or greater")
	}
	return strings.Join(versions, ", ")
}

// checkAndSetDefaults validates the request and sets default values.
func (r *RuntimeUpgradePath) checkAndSetDefaults() error {
	if r.From == nil || r.To == nil {
		return trace.BadParameter("runtime versions are not set")
	}
	if len(r.DirectUpgradeVersions) == 0 {
		r.DirectUpgradeVersions = DirectUpgradeVersions
	}
	if len(r.UpgradeViaVersions) == 0 {
		r.UpgradeViaVersions = UpgradeViaVersions
	}
	if r.FieldLogger == nil {
		r.FieldLogger = logrus.WithFields(logrus.Fields{
			trace.Component: "vercheck",
			"from":          r.From.String(),
			"to":            r.To.String(),
		})
	}
	return nil
}

// checkIntermediateRuntimes validates that required intermediate runtimes
// exist in the provided package service and returns their locators.
func (r RuntimeUpgradePath) checkIntermediateRuntimes(m schema.Manifest, intermediateVersions Versions) (runtimes []loc.Locator, err error) {
	for _, intermediateVersion := range intermediateVersions {
		runtimePackage, err := findIntermediateRuntime(m, intermediateVersion)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if trace.IsNotFound(err) {
			r.WithField("version", intermediateVersion).Warn("Required intermediate runtime not found.")
			return nil, trace.BadParameter(needsIntermediateErrorTpl, r.From,
				r.To, intermediateVersions)
		}
		runtimes = append(runtimes, *runtimePackage)
	}
	return runtimes, nil
}

// hasIntermediateRuntime searches for the runtime application package that satisfies the
// specified required intermediate runtime application version.
func findIntermediateRuntime(m schema.Manifest, intermediateVersion semver.Version) (*loc.Locator, error) {
	for _, v := range m.SystemOptions.Dependencies.IntermediateVersions {
		if loc.GreaterOrEqualPatch(v.Version, intermediateVersion) {
			loc := loc.Runtime.WithLiteralVersion(v.Version.String())
			return &loc, nil
		}
	}
	return nil, trace.NotFound("no runtime application for the specified version")
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
Please rebuild it as described in https://goteleport.com/gravity/docs/cluster/#multi-hop-upgrades.`
)
