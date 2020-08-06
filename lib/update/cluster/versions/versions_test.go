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
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/mailgun/timetools"

	"github.com/coreos/go-semver/semver"
	"gopkg.in/check.v1"
)

func Test(t *testing.T) { check.TestingT(t) }

type Suite struct {
	packages pack.PackageService
}

var _ = check.Suite(&Suite{})

func (s *Suite) SetUpTest(c *check.C) {
	dir := c.MkDir()
	s.packages = newPackageService(dir, c)
	// Create an intermediate runtime package.
	err := s.packages.UpsertRepository(defaults.SystemAccountOrg, time.Time{})
	c.Assert(err, check.IsNil)
	_, err = s.packages.CreatePackage(
		loc.Runtime.WithLiteralVersion("2.0.15"),
		strings.NewReader(""),
		pack.WithLabels(pack.RuntimeUpgradeLabels("2.0.15")))
	c.Assert(err, check.IsNil)
}

func (s *Suite) TestVersions(c *check.C) {
	upgradeRuntime := semver.New("3.0.5")
	tests := []struct {
		from    *semver.Version
		error   string
		comment string
	}{
		{
			from:    semver.New("2.0.5"),
			comment: "Direct upgrade from previous major version",
		},
		{
			from:    semver.New("3.0.1"),
			comment: "Direct upgrade from same major version",
		},
		{
			from:    semver.New("1.0.0"),
			comment: "Upgrade via intermediate runtime",
		},
		{
			from:    semver.New("1.1.0"),
			error:   fmt.Sprintf(needsIntermediateErrorTpl, "1.1.0", upgradeRuntime, Versions{semver.New("2.1.0")}),
			comment: "No required intermediate runtime",
		},
		{
			from:    semver.New("3.0.7"),
			error:   fmt.Sprintf(downgradeErrorTpl, "3.0.7", upgradeRuntime),
			comment: "Downgrade from greater runtime version",
		},
		{
			from:    semver.New("3.0.5"),
			comment: "Unchanged runtime version",
		},
		{
			from:    semver.New("0.0.1"),
			error:   fmt.Sprintf(unsupportedErrorTpl, "0.0.1", upgradeRuntime),
			comment: "Unsupported upgrade path",
		},
	}
	for _, test := range tests {
		err := RuntimeUpgradePath{
			From:                  test.from,
			To:                    upgradeRuntime,
			directUpgradeVersions: directUpgradeVersions,
			upgradeViaVersions:    upgradeViaVersions,
		}.Verify(s.packages)
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
		from    *semver.Version
		direct  bool
		via     Versions
		comment string
	}{
		{
			from:    semver.New("6.1.30"),
			direct:  true,
			via:     Versions(nil),
			comment: "Direct upgrade from a previous major version",
		},
		{
			from:    semver.New("5.0.0"),
			via:     Versions(nil),
			comment: "No upgrade from an older version",
		},
		{
			from: semver.New("5.5.38"),
			via: Versions{
				semver.New("6.1.0"),
			},
			comment: "Upgrade via an intermediate runtime",
		},
	}
	for _, test := range tests {
		path := RuntimeUpgradePath{
			From: test.from,
			To:   upgradeRuntime,
		}
		comment := check.Commentf(test.comment)
		direct := path.SupportsDirectUpgrade()
		via := path.SupportsUpgradeVia()
		c.Assert(direct, check.Equals, test.direct, comment)
		c.Assert(via, check.DeepEquals, test.via, comment)
	}
}

func newPackageService(dir string, c *check.C) *localpack.PackageServer {
	backend, err := keyval.NewBolt(keyval.BoltConfig{Path: filepath.Join(dir, "bolt.db")})
	c.Assert(err, check.IsNil)
	objects, err := fs.New(fs.Config{Path: dir})
	c.Assert(err, check.IsNil)
	pack, err := localpack.New(localpack.Config{
		Backend:     backend,
		UnpackedDir: filepath.Join(dir, defaults.UnpackedDir),
		Objects:     objects,
		Clock: &timetools.FreezedTime{
			CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
		},
		DownloadURL: "https://ops.example.com",
	})
	c.Assert(err, check.IsNil)
	return pack
}

var (
	directUpgradeVersions = Versions{
		semver.New("2.0.0"),
		semver.New("3.0.0"),
	}
	upgradeViaVersions = map[*semver.Version]Versions{
		semver.New("1.0.0"): Versions{semver.New("2.0.10")},
		semver.New("1.1.0"): Versions{semver.New("2.1.0")},
	}
)
