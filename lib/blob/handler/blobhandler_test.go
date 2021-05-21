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

package handler

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gravitational/gravity/lib/blob/client"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/blob/suite"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/users/usersservice"

	"github.com/gravitational/roundtrip"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestHandler(t *testing.T) { TestingT(t) }

type HandlerSuite struct {
	backend   storage.Backend
	suite     suite.BLOBSuite
	webServer *httptest.Server
	users     users.Identity
	clock     clockwork.FakeClock

	adminUser storage.User

	dir string
}

var _ = Suite(&HandlerSuite{
	clock: clockwork.NewFakeClock(),
})

func (s *HandlerSuite) SetUpSuite(c *C) {
	log.SetOutput(os.Stderr)
}

func (s *HandlerSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	var err error
	s.backend, err = keyval.NewBolt(keyval.BoltConfig{Path: filepath.Join(s.dir, "bolt.db")})
	c.Assert(err, IsNil)

	objects, err := fs.New(s.dir)
	c.Assert(err, IsNil)

	s.users, err = usersservice.New(
		usersservice.Config{Backend: s.backend})
	c.Assert(err, IsNil)

	role, err := users.NewAdminRole()
	c.Assert(err, IsNil)

	err = s.users.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)

	s.adminUser = storage.NewUser("admin@a.example.com", storage.UserSpecV2{
		Password: "admin-password",
		Type:     storage.AdminUser,
		Roles:    []string{role.GetName()},
	})
	err = s.users.UpsertUser(s.adminUser)
	c.Assert(err, IsNil)

	webHandler, err := New(Config{
		Users:   s.users,
		Local:   objects,
		Cluster: objects,
	})
	c.Assert(err, IsNil)
	mux := http.NewServeMux()
	mux.Handle("/objects/", webHandler)
	s.webServer = httptest.NewServer(mux)

	// for regular test, let's be admins, so tests
	// won't be affected by auth issues
	s.suite.Objects, err = client.NewAuthenticatedClient(
		s.webServer.URL, s.adminUser.GetName(), "admin-password",
		roundtrip.HTTPClient(&http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					//nolint:gosec // TODO: fix insecure
					InsecureSkipVerify: true,
				}}}),
	)
	c.Assert(err, IsNil)
}

func (s *HandlerSuite) TestBLOB(c *C) {
	s.suite.BLOB(c)
}

func (s *HandlerSuite) TestBLOBSeek(c *C) {
	s.suite.BLOBSeek(c)
}

func (s *HandlerSuite) TestBLOBWriteTwice(c *C) {
	s.suite.BLOBWriteTwice(c)
}

func (s *HandlerSuite) TestBLOBList(c *C) {
	s.suite.BLOBList(c)
}
