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

package app

import (
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	. "gopkg.in/check.v1"
)

type S struct{}

var _ = Suite(&S{})

func (*S) TestDeduplicatesPackages(c *C) {
	packages := UniqPackages([]pack.PackageEnvelope{
		{Locator: loc.MustParseLocator("example.com/package1:0.0.2")},
		{Locator: loc.MustParseLocator("server.com/package:0.0.1")},
		{Locator: loc.MustParseLocator("example.com/package1:0.0.1")},
		{Locator: loc.MustParseLocator("example.com/package:0.0.1")},
		{Locator: loc.MustParseLocator("example.com/package:0.0.1")},
	})
	c.Assert(packages, DeepEquals, []pack.PackageEnvelope{
		{Locator: loc.MustParseLocator("example.com/package1:0.0.1")},
		{Locator: loc.MustParseLocator("example.com/package1:0.0.2")},
		{Locator: loc.MustParseLocator("example.com/package:0.0.1")},
		{Locator: loc.MustParseLocator("server.com/package:0.0.1")},
	})
}

func (*S) TestDeduplicatesEmpty(c *C) {
	packages := UniqPackages([]pack.PackageEnvelope{})
	c.Assert(packages, DeepEquals, []pack.PackageEnvelope{})
}

func (*S) TestDeduplicatesApps(c *C) {
	apps := UniqApps([]Application{
		{Package: loc.MustParseLocator("server.com/app:0.0.1")},
		{Package: loc.MustParseLocator("example.com/app1:0.0.2")},
		{Package: loc.MustParseLocator("example.com/app1:0.0.1")},
		{Package: loc.MustParseLocator("example.com/app:0.0.1")},
		{Package: loc.MustParseLocator("example.com/app:0.0.1")},
	})
	c.Assert(apps, DeepEquals, []Application{
		{Package: loc.MustParseLocator("example.com/app1:0.0.1")},
		{Package: loc.MustParseLocator("example.com/app1:0.0.2")},
		{Package: loc.MustParseLocator("example.com/app:0.0.1")},
		{Package: loc.MustParseLocator("server.com/app:0.0.1")},
	})
}
