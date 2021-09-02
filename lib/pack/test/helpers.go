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

package test

import (
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	"gopkg.in/check.v1"
)

// VerifyPackages ensures that the specified package service contains
// the expected packages.
// No assumptions are made about other packages in the service
func VerifyPackages(packages pack.PackageService, expected []loc.Locator, c *check.C) {
	repositories, err := packages.GetRepositories()
	c.Assert(err, check.IsNil)

	var result []loc.Locator
	for _, repository := range repositories {
		packages, err := packages.GetPackages(repository)
		c.Assert(err, check.IsNil)
		result = append(result, locators(packages)...)
	}

	c.Assert(packagesByName(result), compare.SortedSliceEquals, packagesByName(expected))
}

func locators(envelopes []pack.PackageEnvelope) []loc.Locator {
	out := make([]loc.Locator, 0, len(envelopes))
	for _, env := range envelopes {
		out = append(out, env.Locator)
	}
	return out
}

type packagesByName []loc.Locator

func (r packagesByName) Len() int           { return len(r) }
func (r packagesByName) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r packagesByName) Less(i, j int) bool { return r[i].String() < r[j].String() }
