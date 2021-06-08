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

// Package suite contains a package service acceptance test suite that is backend
// implementation independent each storage will use the suite to test itself
package suite

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"

	"github.com/mailgun/timetools"
	. "gopkg.in/check.v1" //nolint:revive,stylecheck // TODO: tests will be rewritten to use testify
)

// PackageSuite contains acceptance tests for package services
type PackageSuite struct {
	S pack.PackageService
	O blob.Objects
	U users.Users
	C *timetools.FreezedTime
}

// RepositoriesCRUD tests repositories Create, Read, Update operations
func (s *PackageSuite) RepositoriesCRUD(c *C) {
	repos, err := s.S.GetRepositories()
	c.Assert(err, IsNil)
	c.Assert(repos, DeepEquals, []string{})

	a, b := "a.example.com", "b.example.com"

	c.Assert(s.S.UpsertRepository(a, time.Time{}), IsNil)
	c.Assert(s.S.UpsertRepository(b, time.Time{}), IsNil)
	c.Assert(s.S.UpsertRepository(b, time.Time{}), IsNil)

	repos, err = s.S.GetRepositories()
	c.Assert(err, IsNil)
	CompareAsSets(c, []string{a, b}, repos)

	repo, err := s.S.GetRepository(a)
	c.Assert(err, IsNil)
	c.Assert(repo.GetName(), Equals, a)

	c.Assert(s.S.DeleteRepository(a), IsNil)

	_, err = s.S.GetRepository(a)
	c.Assert(trace.IsNotFound(err), Equals, true)

	repos, err = s.S.GetRepositories()
	c.Assert(err, IsNil)
	c.Assert(repos, DeepEquals, []string{b})
}

// PackagesCRUD checks basic packages tests
func (s *PackageSuite) PackagesCRUD(c *C) {
	repoA := "a.example.com"
	c.Assert(s.S.UpsertRepository(repoA, time.Time{}), IsNil)

	pack1Data := []byte("hello, world!")
	loc1 := loc.MustParseLocator("a.example.com/package-1:0.0.1")

	pack1v2Data := []byte("hola world!")
	loc1v2 := loc.MustParseLocator("a.example.com/package-1:0.0.2")
	latestLoc := loc.MustParseLocator(fmt.Sprintf("a.example.com/package-1:0.0.0+%s", pack.LatestLabel))

	pack1, err := s.S.CreatePackage(loc1, bytes.NewBuffer(pack1Data))
	c.Assert(err, IsNil)
	c.Assert(pack1, NotNil)
	c.Assert(pack1.Locator.Name, Equals, loc1.Name)
	c.Assert(pack1.Locator.Version, Equals, loc1.Version)
	c.Assert(pack1.SizeBytes, Equals, int64(len(pack1Data)))
	c.Assert(pack1.SHA512, Equals, hash(pack1Data))

	pack1v2, err := s.S.CreatePackage(loc1v2, bytes.NewBuffer(pack1v2Data))
	c.Assert(err, IsNil)
	c.Assert(pack1v2, NotNil)
	c.Assert(pack1v2.Locator.Name, Equals, loc1v2.Name)
	c.Assert(pack1v2.Locator.Version, Equals, loc1v2.Version)
	c.Assert(pack1v2.SizeBytes, Equals, int64(len(pack1v2Data)))
	c.Assert(pack1v2.SHA512, Equals, hash(pack1v2Data))

	opack1, readclose, err := s.S.ReadPackage(loc1)
	c.Assert(err, IsNil)
	c.Assert(opack1, DeepEquals, pack1)
	out, err := ioutil.ReadAll(readclose)
	c.Assert(err, IsNil)
	c.Assert(string(out), DeepEquals, string(pack1Data))

	opack1Latest, readclose, err := s.S.ReadPackage(latestLoc)
	c.Assert(err, IsNil)
	c.Assert(opack1Latest, DeepEquals, pack1v2)
	out, err = ioutil.ReadAll(readclose)
	c.Assert(err, IsNil)
	c.Assert(string(out), DeepEquals, string(pack1v2Data))

	opack11, err := s.S.ReadPackageEnvelope(loc1)
	c.Assert(err, IsNil)
	c.Assert(opack11, DeepEquals, pack1)

	pack2Data := []byte("hello, world 2!")
	loc2 := loc.MustParseLocator("a.example.com/package-2:0.0.2")

	pack2, err := s.S.CreatePackage(loc2, bytes.NewBuffer(pack2Data), pack.WithLabels(map[string]string{"hello": "there"}))
	c.Assert(err, IsNil)

	packages, err := s.S.GetPackages(repoA)
	c.Assert(err, IsNil)
	packageNames := make([]string, 0, 2)
	for _, p := range packages {
		packageNames = append(packageNames, p.String())
	}
	CompareAsSets(c, []string{pack1.String(), pack1v2.String(), pack2.String()}, packageNames)

	err = s.S.UpdatePackageLabels(
		loc2, map[string]string{"omg": "omg2"}, []string{"hello"})
	c.Assert(err, IsNil)

	pack2, err = s.S.ReadPackageEnvelope(loc2)
	c.Assert(err, IsNil)

	err = s.S.DeletePackage(loc1)
	c.Assert(err, IsNil)

	packages, err = s.S.GetPackages(repoA)
	c.Assert(err, IsNil)
	c.Assert(packages, DeepEquals, []pack.PackageEnvelope{*pack1v2, *pack2})
}

// UpsertPackages tests upsert flow
func (s *PackageSuite) UpsertPackages(c *C) {
	pack1Data := []byte("hello, world!")
	loc1 := loc.MustParseLocator("a.example.com/package-1:0.0.1")

	pack1, err := s.S.UpsertPackage(
		loc1, bytes.NewBuffer(pack1Data))
	c.Assert(err, IsNil)
	c.Assert(pack1, NotNil)
	c.Assert(pack1.Locator.Name, Equals, loc1.Name)
	c.Assert(pack1.Locator.Version, Equals, loc1.Version)
	c.Assert(pack1.SizeBytes, Equals, int64(len(pack1Data)))
	c.Assert(pack1.SHA512, Equals, hash(pack1Data))

	o1, r1, err := s.S.ReadPackage(loc1)
	c.Assert(err, IsNil)
	c.Assert(o1, DeepEquals, pack1)
	out, err := ioutil.ReadAll(r1)
	c.Assert(err, IsNil)
	c.Assert(string(out), DeepEquals, string(pack1Data))
	c.Assert(r1.Close(), IsNil)

	labels := map[string]string{"hello": "there"}
	pack1NewData := []byte("hello, world!")
	pack1, err = s.S.UpsertPackage(loc1, bytes.NewBuffer(pack1NewData), pack.WithLabels(labels))
	c.Assert(err, IsNil)

	o2, r2, err := s.S.ReadPackage(loc1)
	c.Assert(err, IsNil)
	c.Assert(o2, DeepEquals, pack1)
	out, err = ioutil.ReadAll(r2)
	c.Assert(err, IsNil)
	c.Assert(string(out), DeepEquals, string(pack1NewData))
	c.Assert(pack1.SizeBytes, Equals, int64(len(pack1NewData)))
	c.Assert(pack1.SHA512, Equals, hash(pack1NewData))
	c.Assert(r2.Close(), IsNil)
}

// DeleteRepository makes sure that when repository is deleted, all its package blobs are also deleted
func (s *PackageSuite) DeleteRepository(c *C) {
	err := s.S.UpsertRepository("example.com", time.Time{})
	c.Assert(err, IsNil)

	data := []byte("hello, world!")
	loc := loc.MustParseLocator("example.com/package:0.0.1")

	pack, err := s.S.CreatePackage(loc, bytes.NewBuffer(data))
	c.Assert(err, IsNil)

	sha512 := pack.SHA512

	err = s.S.DeleteRepository("example.com")
	c.Assert(err, IsNil)

	blob, err := s.O.GetBLOBEnvelope(sha512)
	c.Assert(blob, IsNil)
	c.Assert(trace.IsNotFound(err), Equals, true)
}

func hash(v []byte) string {
	h, err := utils.SHA512Half(v)
	if err != nil {
		panic(err)
	}
	return h
}

func CompareAsSets(c *C, expected, actual []string) {
	expectedMap := make(map[string]bool)
	for _, k := range expected {
		expectedMap[k] = true
	}
	actualMap := make(map[string]bool)
	for _, k := range actual {
		actualMap[k] = true
	}
	c.Assert(expectedMap, DeepEquals, actualMap)
}
