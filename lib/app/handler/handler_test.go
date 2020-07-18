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
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/client"
	appservice "github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/app/suite"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/users/usersservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/mailgun/timetools"
	. "gopkg.in/check.v1"
	"k8s.io/client-go/kubernetes"
)

func TestHandler(t *testing.T) { TestingT(t) }

type HandlerSuite struct {
	backend storage.Backend
	suite   suite.AppsSuite
	server  *httptest.Server
	user    storage.User
	users   users.Identity

	dir string
}

var _ = Suite(&HandlerSuite{})

func (r *HandlerSuite) SetUpSuite(c *C) {
	r.suite.SetUpSuite(c)
}

func (r *HandlerSuite) SetUpTest(c *C) {
	var err error
	r.dir = c.MkDir()

	r.backend, err = keyval.NewBolt(keyval.BoltConfig{Path: filepath.Join(r.dir, "bolt.db")})
	c.Assert(err, IsNil)

	objects, err := fs.New(fs.Config{Path: r.dir})
	c.Assert(err, IsNil)

	clock := &timetools.FreezedTime{
		CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
	}
	r.suite.Packages, err = localpack.New(localpack.Config{
		Backend:     r.backend,
		UnpackedDir: filepath.Join(r.dir, defaults.UnpackedDir),
		Clock:       clock,
		Objects:     objects,
		DownloadURL: "https://localhost:3009",
	})
	c.Assert(err, IsNil)

	r.users, err = usersservice.New(usersservice.Config{Backend: r.backend})
	c.Assert(err, IsNil)
	role, err := users.NewAdminRole()
	c.Assert(err, IsNil)
	err = r.users.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)
	r.user = storage.NewUser("admin@example.com", storage.UserSpecV2{
		Password: "admin-password",
		Type:     storage.AdminUser,
		Roles:    []string{role.GetName()},
	})
	err = r.users.UpsertUser(r.user)
	c.Assert(err, IsNil)

	r.suite.NewService = func(c *C, dockerClient docker.DockerInterface, imageService docker.ImageService) app.Applications {
		var kubeClient *kubernetes.Clientset
		var err error
		if utils.RunKubernetesTests() {
			kubeClient, _, err = utils.GetLocalKubeClient()
			c.Assert(err, IsNil)
		}
		charts, err := helm.NewRepository(helm.Config{
			Packages: r.suite.Packages,
			Backend:  r.backend,
		})
		c.Assert(err, IsNil)
		applications, err := appservice.New(appservice.Config{
			StateDir:       filepath.Join(r.dir, "import"),
			Backend:        r.backend,
			Packages:       r.suite.Packages,
			DockerClient:   dockerClient,
			ImageService:   imageService,
			Users:          r.users,
			CacheResources: true,
			Client:         kubeClient,
			UnpackedDir:    filepath.Join(r.dir, "apps"),
			Charts:         charts,
		})
		c.Assert(err, IsNil)

		handler, err := NewWebHandler(WebHandlerConfig{
			Users:        r.users,
			Applications: applications,
		})
		c.Assert(err, IsNil)

		// It is important that we launch TLS server as authentication
		// middleware on the handler expects TLS connections.
		r.server = httptest.NewTLSServer(handler)

		apps, err := client.NewAuthenticatedClient(
			r.server.URL, r.user.GetName(), "admin-password",
			client.HTTPClient(r.server.Client()))
		c.Assert(err, IsNil)
		return apps
	}

	r.suite.SetUpTest(c)
}

func (r *HandlerSuite) TearDownTest(c *C) {
	if r.server != nil {
		r.server.Close()
	}
	if r.backend != nil {
		c.Assert(r.backend.Close(), IsNil)
	}
}

func (r *HandlerSuite) TestValidatesManifest(c *C) {
	r.suite.ValidatesManifest(c)
}

func (r *HandlerSuite) TestImportsApplication(c *C) {
	r.suite.ImportsApplication(c)
}

func (r *HandlerSuite) TestRetrievesLogsForFailedImport(c *C) {
	r.suite.RetrievesLogsForFailedImport(c)
}

func (r *HandlerSuite) TestExportsApplication(c *C) {
	r.suite.ExportsApplication(c)
}

func (r *HandlerSuite) TestCreatesApplication(c *C) {
	r.suite.CreatesApplication(c)
}

func (r *HandlerSuite) TestCreatesApplicationWithManifest(c *C) {
	r.suite.CreatesApplicationWithManifest(c)
}

func (r *HandlerSuite) TestCreatesApplicationInstaller(c *C) {
	r.suite.CreatesApplicationInstaller(c)
}

func (r *HandlerSuite) TestDeletesApplication(c *C) {
	r.suite.DeletesApplication(c)
}

func (r *HandlerSuite) TestResolvesManifest(c *C) {
	r.suite.ResolvesManifest(c)
}

func (r *HandlerSuite) TestResources(c *C) {
	r.suite.Resources(c)
}

func (r *HandlerSuite) TestCharts(c *C) {
	r.suite.Charts(c)
}

func (r *HandlerSuite) TestAppHookCycle(c *C) {
	if !utils.RunKubernetesTests() {
		c.Skip("skipping Kubernetes test")
	}
	r.suite.AppHookCycle(c)
}

func (r *HandlerSuite) TestFetchChart(c *C) {
	r.suite.FetchChart(c)
}

func (r *HandlerSuite) TestFetchIndexFile(c *C) {
	r.suite.FetchIndexFile(c)
}

// TestTelekubeInstallScriptChecksSemverSanity ensures that the install script generator is checking the semver for
// malicious input
func TestTelekubeInstallScriptChecksSemverSanity(t *testing.T) {
	h := &WebHandler{}
	assert.Error(t, h.telekubeInstallScript(nil, nil, httprouter.Params{
		httprouter.Param{
			Key:   "version",
			Value: "1.0.1-aaa$(touch grav)",
		},
	}), "validate that telekubeInstallScript throws error on bad input")
}
