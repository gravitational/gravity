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

	appservice "github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/ops/suite"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/users/usersservice"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

// TestServices contains a set of services that are used in tests
type TestServices struct {
	appservice.TestServices
	// Agents is the RPC agents service
	Agents *AgentService
	// AgentServer is the RPC agent server
	AgentServer rpcserver.Server
	// Operator is the ops service
	Operator *Operator
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

	testServices := appservice.NewTestServices(dir, c)

	usersService, err := usersservice.New(usersservice.Config{
		Backend: testServices.Backend,
	})
	c.Assert(err, check.IsNil)

	listener, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, check.IsNil)

	proxy := &suite.TestProxy{}
	log := log.WithField("from", "test")
	peerStore := NewAgentPeerStore(testServices.Backend, usersService, proxy, log)
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

	opsService, err := New(Config{
		StateDir:      dir,
		Backend:       testServices.Backend,
		Agents:        agentService,
		Packages:      testServices.Packages,
		TeleportProxy: proxy,
		AuthClient:    &auth.Client{},
		Proxy:         &suite.TestOpsProxy{},
		Users:         usersService,
		Apps:          testServices.Apps,
		ProcessID:     "p1",
	})
	c.Assert(err, check.IsNil)

	return TestServices{
		TestServices: testServices,
		Agents:       agentService,
		AgentServer:  agentServer,
		Operator:     opsService,
		Users:        usersService,
		Dir:          dir,
		Clock:        clockwork.NewFakeClock(),
	}
}
