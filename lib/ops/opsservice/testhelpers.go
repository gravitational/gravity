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

package opsservice

import (
	"net"
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/app"
	appservice "github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/users/usersservice"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/jonboulle/clockwork"
	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

// TestServices contains a set of services that are used in tests
type TestServices struct {
	// Backend is the local backend
	Backend storage.Backend
	// Packages is the local pack service
	Packages pack.PackageService
	// Apps is the local apps service
	Apps app.Applications
	// Agents is the RPC agents service
	Agents *AgentService
	// AgentServer is the RPC agent server
	AgentServer rpcserver.Server
	// Operator is the ops service
	Operator *Operator
	// HelmClient is the mock Helm client
	HelmClient helm.Client
	// Users is the users service
	Users users.Identity
	// Dir is the temporary directory where all data is stored
	Dir string
	// Clock provides time interface
	Clock clockwork.Clock
}

// SetupTestServices initializes backend and package and application services
// that can be used in tests
func SetupTestServices(c *check.C) TestServices {
	dir := c.MkDir()
	backend, err := keyval.NewBolt(keyval.BoltConfig{Path: filepath.Join(dir, "bolt.db")})
	c.Assert(err, check.IsNil)
	objects, err := fs.New(fs.Config{Path: dir})
	c.Assert(err, check.IsNil)
	return SetupTestServicesInDirectory(dir, backend, objects, c)
}

// SetupTestServicesInDirectory initializes backend and package and application services
// in the specified directory using given backend/objects values
func SetupTestServicesInDirectory(dir string, backend storage.Backend, objects blob.Objects, c *check.C) TestServices {
	packService, err := localpack.New(localpack.Config{
		Backend:     backend,
		UnpackedDir: filepath.Join(dir, defaults.UnpackedDir),
		Objects:     objects,
		Clock: &timetools.FreezedTime{
			CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
		},
		DownloadURL: "https://ops.example.com",
	})
	c.Assert(err, check.IsNil)

	appService, err := appservice.New(appservice.Config{
		Backend:  backend,
		StateDir: filepath.Join(dir, defaults.ImportDir),
		Packages: packService,
	})
	c.Assert(err, check.IsNil)

	usersService, err := usersservice.New(usersservice.Config{
		Backend: backend,
	})
	c.Assert(err, check.IsNil)

	listener, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, check.IsNil)

	proxy := &suite.TestProxy{}
	log := log.WithField("from", "test")
	peerStore := NewAgentPeerStore(backend, usersService, proxy, log)
	agentServer, err := rpcserver.New(rpcserver.Config{
		FieldLogger: log,
		Listener:    listener,
		Credentials: rpcserver.TestCredentials(c),
		PeerStore:   peerStore,
	})
	c.Assert(err, check.IsNil)

	agentService := NewAgentService(
		agentServer, peerStore,
		"localhost:0",
		log)

	helmClient, err := helm.NewTestClient(helm.ClientConfig{})
	c.Assert(err, check.IsNil)

	opsService, err := New(Config{
		StateDir:      dir,
		Backend:       backend,
		Agents:        agentService,
		Packages:      packService,
		TeleportProxy: proxy,
		AuthClient:    &auth.Client{},
		Proxy:         &suite.TestOpsProxy{},
		Users:         usersService,
		Apps:          appService,
		ProcessID:     "p1",
		GetHelmClient: func(helm.ClientConfig) (helm.Client, error) {
			return helmClient, nil
		},
	})
	c.Assert(err, check.IsNil)

	return TestServices{
		Backend:     backend,
		Packages:    packService,
		Apps:        appService,
		Agents:      agentService,
		AgentServer: agentServer,
		Operator:    opsService,
		Users:       usersService,
		HelmClient:  helmClient,
		Dir:         dir,
		Clock:       clockwork.NewFakeClock(),
	}
}
