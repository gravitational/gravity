/*
Copyright 2021 Gravitational, Inc.

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
	"fmt"
	"testing"

	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/coreos/go-semver/semver"
	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type Suite struct {
	m schema.Manifest
}

var _ = check.Suite(&Suite{})

func (s *Suite) SetUpTest(c *check.C) {
	clusterApp := apptest.DefaultClusterApplication(loc.MustParseLocator("gravitational.io/app:1.0.0")).Build()
	clusterApp.Manifest.SystemOptions.Dependencies.IntermediateVersions = []schema.IntermediateVersion{{
		Version: newVer("2.0.15"),
		Dependencies: schema.Dependencies{
			Apps: []schema.Dependency{{
				Locator: loc.Runtime.WithLiteralVersion("2.0.15"),
			}},
		},
	}}
	s.m = clusterApp.Manifest
}

func (s *Suite) TestVersions(c *check.C) {
	upgradeRuntime := semver.New("3.0.5")
	tests := []struct {
		from    semver.Version
		error   string
		comment string
	}{
		{
			from:    newVer("2.0.5"),
			comment: "Direct upgrade from previous major version",
		},
		{
			from:    newVer("3.0.1"),
			comment: "Direct upgrade from same major version",
		},
		{
			from:    newVer("1.0.0"),
			comment: "Upgrade via intermediate runtime",
		},
		{
			from:    newVer("1.1.0"),
			error:   fmt.Sprintf(needsIntermediateErrorTpl, "1.1.0", upgradeRuntime, Versions{newVer("2.1.0")}),
			comment: "No required intermediate runtime",
		},
		{
			from:    newVer("3.0.7"),
			error:   fmt.Sprintf(downgradeErrorTpl, "3.0.7", upgradeRuntime),
			comment: "Downgrade from greater runtime version",
		},
		{
			from:    newVer("3.0.5"),
			comment: "Unchanged runtime version",
		},
		{
			from:    newVer("0.0.1"),
			error:   fmt.Sprintf(unsupportedErrorTpl, "0.0.1", upgradeRuntime),
			comment: "Unsupported upgrade path",
		},
	}
	for _, test := range tests {
		err := RuntimeUpgradePath{
			From: &test.from,
			To:   upgradeRuntime,
			DirectUpgradeVersions: Versions{
				newVer("2.0.0"),
				newVer("3.0.0"),
			},
			UpgradeViaVersions: map[semver.Version]Versions{
				newVer("1.0.0"): {newVer("2.0.10")},
				newVer("1.1.0"): {newVer("2.1.0")},
			},
		}.Verify(s.m)
		if test.error != "" {
			c.Assert(err, check.ErrorMatches, test.error, check.Commentf(test.comment))
		} else {
			c.Assert(err, check.IsNil, check.Commentf(test.comment))
		}
	}
}

func (s *Suite) TestVersionQueries(c *check.C) {
	upgradeRuntime := semver.New("7.0.14")
	tests := []struct {
		path    RuntimeUpgradePath
		direct  bool
		via     Versions
		comment string
	}{
		{
			path: RuntimeUpgradePath{
				From: semver.New("6.1.30"),
				To:   upgradeRuntime,
				DirectUpgradeVersions: Versions{
					newVer("6.1.0"),
					newVer("7.0.0"),
				},
			},
			direct:  true,
			comment: "Direct upgrade from a previous major version",
		},
		{
			path: RuntimeUpgradePath{
				From: semver.New("5.0.0"),
				To:   upgradeRuntime,
				DirectUpgradeVersions: Versions{
					newVer("6.1.0"),
					newVer("7.0.0"),
				},
			},
			direct:  false,
			comment: "No upgrade from an older version",
		},
		{
			path: RuntimeUpgradePath{
				From: semver.New("5.5.38"),
				To:   upgradeRuntime,
				DirectUpgradeVersions: Versions{
					newVer("6.1.0"),
					newVer("7.0.0"),
				},
				UpgradeViaVersions: map[semver.Version]Versions{
					newVer("5.5.0"): {newVer("6.1.0")},
				},
			},
			direct: false,
			via: Versions{
				newVer("6.1.0"),
			},
			comment: "Upgrade via an intermediate runtime",
		},
	}
	for _, test := range tests {
		comment := check.Commentf(test.comment)
		direct := test.path.SupportsDirectUpgrade()
		via := test.path.SupportsUpgradeVia()
		c.Assert(direct, check.Equals, test.direct, comment)
		c.Assert(via, check.DeepEquals, test.via, comment)
	}
}

func newVer(v string) semver.Version {
	return *semver.New(v)
}
