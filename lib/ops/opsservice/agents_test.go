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
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/pack"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	licenseapi "github.com/gravitational/license"
	"github.com/gravitational/license/authority"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	. "gopkg.in/check.v1"
)

func TestOpsService(t *testing.T) { TestingT(t) }

type AgentSuite struct {
	backend      storage.Backend
	users        users.Identity
	accessToken  string
	installer    ops.Operator
	agentService *AgentService
	agentServer  rpcserver.Server
	pack         pack.PackageService
	testApp      *loc.Locator
	cluster      *ops.Site
	key          ops.SiteOperationKey
	ca           authority.TLSKeyPair
	ctx          context.Context
}

var _ = Suite(&AgentSuite{})

func (s *AgentSuite) SetUpSuite(c *C) {
	log.SetOutput(os.Stderr)
	log.SetFormatter(&trace.TextFormatter{})

	// init certificate authority, we'll need it in some tests
	ca, err := authority.GenerateSelfSignedCA(csr.CertificateRequest{
		CN: constants.OpsCenterKeyPair,
	})
	c.Assert(err, IsNil)
	s.ca = *ca
	s.ctx = context.Background()
}

func (s *AgentSuite) SetUpTest(c *C) {
	services := SetupTestServices(c)

	s.backend = services.Backend
	s.users = services.Users
	s.agentService = services.Agents
	s.agentServer = services.AgentServer
	s.pack = services.Packages
	s.installer = services.Operator

	var err error
	suite := &suite.OpsSuite{}
	s.testApp, err = suite.SetUpTestPackage(services.Apps, s.pack, c)
	c.Assert(err, IsNil)

	acct, err := s.installer.CreateAccount(ops.NewAccountRequest{
		Org: "testing",
	})
	c.Assert(err, IsNil)

	s.cluster, err = s.installer.CreateSite(ops.NewSiteRequest{
		AccountID:  acct.ID,
		AppPackage: s.testApp.String(),
		Provider:   schema.ProvisionerOnPrem,
		DomainName: "test.localdomain",
	})
	c.Assert(err, IsNil)

	opKey, err := s.installer.CreateSiteInstallOperation(context.TODO(), ops.CreateSiteInstallOperationRequest{
		AccountID:   acct.ID,
		SiteDomain:  s.cluster.Domain,
		Variables:   storage.OperationVariables{},
		Provisioner: schema.ProvisionerOnPrem,
	})
	c.Assert(err, IsNil)
	c.Assert(opKey, NotNil)

	token, err := s.installer.GetExpandToken(s.cluster.Key())
	c.Assert(err, IsNil)
	c.Assert(token, NotNil, Commentf("expected provisioning token to exist, got %+v", token))
	s.key = *opKey
	s.accessToken = token.Token
}

func (s *AgentSuite) TearDownTest(c *C) {
	if s.agentServer != nil {
		ctx, cancel := context.WithTimeout(s.ctx, 2*time.Second)
		defer cancel()
		s.agentServer.Stop(ctx) //nolint:errcheck
	}
}

func (s *AgentSuite) TestInstallAgentCheckLicense(c *C) {
	// generate a license with restrictions
	license, err := licenseapi.NewLicense(licenseapi.NewLicenseInfo{
		MaxNodes:   1,
		ValidFor:   time.Hour,
		TLSKeyPair: s.ca,
	})
	c.Assert(err, IsNil)

	cluster, err := s.backend.GetSite(s.cluster.Domain)
	c.Assert(err, IsNil)

	cluster.License = license
	_, err = s.backend.UpdateSite(*cluster)
	c.Assert(err, IsNil)

	s.testConnectValidatesCondition(c,
		[2]string{"node-1", "node-2"},
		"license allows maximum of 1 nodes, requested: 2",
	)
}

func (s *AgentSuite) TestInstallAgentCheckHostname(c *C) {
	s.testConnectValidatesCondition(c,
		[2]string{"node-1", "node-1"},
		"One of existing peers already has hostname",
	)
}

// TestPingPongSuccess tests inter agent network connectivity check
// that is executed by agents. For this test we use it on one box
func (s *AgentSuite) TestPingPongSuccess(c *C) {
	ports, err := teleutils.GetFreeTCPPorts(2)
	c.Assert(err, IsNil)

	addrs := []string{
		fmt.Sprintf("127.0.0.1:%v", ports[0]),
		fmt.Sprintf("127.0.0.1:%v", ports[1]),
	}
	sort.Strings(addrs)
	req := checks.PingPongRequest{
		Duration: time.Second,
		Listen: []validationpb.Addr{
			{Addr: addrs[0], Network: "tcp"},
			{Addr: addrs[1], Network: "tcp"},
		},
		Ping: []validationpb.Addr{
			{Addr: addrs[0], Network: "tcp"},
			{Addr: addrs[1], Network: "tcp"},
		},
		Mode: checks.ModePingPong,
	}

	expectedResults := []validationpb.ServerResult{
		{Server: &validationpb.Addr{Addr: addrs[0], Network: "tcp"}},
		{Server: &validationpb.Addr{Addr: addrs[1], Network: "tcp"}},
	}

	s.testPingPong(c, req, func(agentAddr string, response checks.PingPongGameResults) {
		compare.DeepCompare(c, response, checks.PingPongGameResults{
			agentAddr: checks.PingPongResult{
				ListenResults: expectedResults,
				PingResults:   expectedResults,
			},
		})
	})
}

// TestPingPongFailure simulates ping pong failure -
// one socket will be busy and one target address will be unreachable
func (s *AgentSuite) TestPingPongFailure(c *C) {
	ports, err := teleutils.GetFreeTCPPorts(2)
	c.Assert(err, IsNil)

	addrs := []string{
		fmt.Sprintf("127.0.0.1:%v", ports[0]),
		fmt.Sprintf("127.0.0.1:%v", ports[1]),
	}
	sort.Strings(addrs)
	req := checks.PingPongRequest{
		Duration: time.Second,
		// second listen on the same port will fail
		Listen: []validationpb.Addr{
			{Addr: addrs[0], Network: "tcp"},
			{Addr: addrs[0], Network: "tcp"},
		},
		// addrs[1] is unreachable
		Ping: []validationpb.Addr{
			{Addr: addrs[0], Network: "tcp"},
			{Addr: addrs[1], Network: "tcp"},
		},
		Mode: checks.ModePingPong,
	}
	s.testPingPong(c, req, func(agentAddr string, response checks.PingPongGameResults) {
		// Error message can be for either "listen" or "bind" verb
		listenVerb := "listen"
		// Error message can be for either "getsockopt" or "connect" verb
		pingVerb := "getsockopt"
		for _, results := range response {
			if len(results.ListenResults) == 2 &&
				strings.Contains(results.ListenResults[1].Error, "bind: address already in use") {
				listenVerb = "bind"
			}
			if len(results.PingResults) == 2 &&
				strings.Contains(results.PingResults[1].Error, "connect: connection refused") {
				pingVerb = "connect"
			}
			break
		}
		compare.DeepCompare(c, response, checks.PingPongGameResults{
			agentAddr: checks.PingPongResult{
				ListenResults: []validationpb.ServerResult{
					{Server: &validationpb.Addr{Network: "tcp", Addr: addrs[0]}},
					{
						Code:   1,
						Error:  fmt.Sprintf("listen tcp %v: %v: address already in use", addrs[0], listenVerb),
						Server: &validationpb.Addr{Network: "tcp", Addr: addrs[0]},
					},
				},
				PingResults: []validationpb.ServerResult{
					{Server: &validationpb.Addr{Network: "tcp", Addr: addrs[0]}},
					{
						Code:   1,
						Error:  fmt.Sprintf("dial tcp %v: %v: connection refused", addrs[1], pingVerb),
						Server: &validationpb.Addr{Network: "tcp", Addr: addrs[1]},
					},
				},
			},
		})
	})
}

func (s *AgentSuite) testPingPong(c *C, req checks.PingPongRequest, fn func(agentAddr string, resp checks.PingPongGameResults)) {
	go func() {
		c.Assert(s.agentServer.Serve(), IsNil)
	}()

	listener, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)

	checkTimeout := 100 * time.Millisecond
	watchCh := make(chan rpcserver.WatchEvent, 1)
	config := rpcserver.PeerConfig{
		Config: rpcserver.Config{
			RuntimeConfig: pb.RuntimeConfig{Token: s.accessToken},
			Listener:      listener,
		},
		WatchCh:            watchCh,
		HealthCheckTimeout: checkTimeout,
	}
	agent := rpcserver.NewTestPeer(c, config, s.agentServer.Addr().String(),
		rpcserver.NewTestCommand("test"), newSystemInfo("node-1"))
	go func() {
		c.Assert(agent.Serve(), IsNil)
	}()
	defer func() {
		c.Assert(agent.Stop(context.TODO()), IsNil)
	}()

	select {
	case update := <-watchCh:
		c.Assert(update.Error, IsNil)
	case <-time.After(time.Second):
		c.Error("timeout")
	}

	agentAddr := agent.Addr().String()
	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	response, err := s.agentService.CheckPorts(ctx,
		s.key, map[string]checks.PingPongRequest{agentAddr: req})
	cancel()
	c.Assert(err, IsNil)

	for _, resp := range response {
		sort.Sort(serverResults(resp.ListenResults))
		sort.Sort(serverResults(resp.PingResults))
	}
	fn(agentAddr, response)
}

func (s *AgentSuite) testConnectValidatesCondition(c *C, hostnames [2]string, expectedError string) {
	go func() {
		c.Assert(s.agentServer.Serve(), IsNil)
	}()

	// simulate that there is already a connected agent
	s.agentService.peerStore.groups[s.key] = newTestAgentGroup(c, "192.168.1.1", hostnames[0])

	// new agent connection should fail b/c the license allows only 1
	listener, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, IsNil)

	watchCh := make(chan rpcserver.WatchEvent, 1)
	config := rpcserver.PeerConfig{
		Config: rpcserver.Config{
			RuntimeConfig: pb.RuntimeConfig{Token: s.accessToken},
			Listener:      listener,
		},
		WatchCh: watchCh,
		ReconnectStrategy: rpcserver.ReconnectStrategy{
			ShouldReconnect: utils.ShouldReconnectPeer,
		},
	}
	agent := rpcserver.NewTestPeer(c, config, s.agentServer.Addr().String(),
		rpcserver.NewTestCommand("test"), newSystemInfo(hostnames[1]))
	go func() {
		c.Assert(agent.Serve(), IsNil)
	}()
	defer func() {
		c.Assert(agent.Stop(context.TODO()), IsNil)
	}()

	select {
	case update := <-watchCh:
		c.Assert(update.Error, NotNil)
		c.Assert(strings.Contains(update.Error.Error(), expectedError), Equals, true)
	case <-time.After(10 * time.Second):
		c.Error("timeout")
	}
}

func newTestAgentGroup(c *C, addr, hostname string) *agentGroup {
	group, err := rpcserver.NewAgentGroup(rpcserver.AgentGroupConfig{}, []rpcserver.Peer{testPeer{addr: addr}})
	c.Assert(err, IsNil)

	return &agentGroup{
		AgentGroup: *group,
		hostnames:  map[string]string{addr: hostname},
	}
}

func (r testPeer) String() string {
	return fmt.Sprintf("test peer@%v", r.addr)
}

func (r testPeer) Addr() string {
	return r.addr
}

func (r testPeer) Reconnect(context.Context) (rpcserver.Client, error) {
	return nil, nil
}

func (r testPeer) Disconnect(context.Context) error {
	return nil
}

type testPeer struct {
	addr string
}

func newSystemInfo(hostname string) rpcserver.TestSystemInfo {
	sysinfo := storage.NewSystemInfo(storage.SystemSpecV2{
		Hostname: hostname,
		Filesystems: []storage.Filesystem{
			{
				DirName: "/foo/bar",
				Type:    "tmpfs",
			},
		},
		FilesystemStats: map[string]storage.FilesystemUsage{
			"/foo/bar": {
				TotalKB: 512,
				FreeKB:  0,
			},
		},
		NetworkInterfaces: map[string]storage.NetworkInterface{
			"device0": {
				Name: "device0",
				IPv4: "172.168.0.1",
			},
		},
		Memory: storage.Memory{Total: 1000, Free: 512, ActualFree: 640},
		Swap:   storage.Swap{Total: 1000, Free: 512},
		User: storage.OSUser{
			Name: "admin",
			UID:  "1001",
			GID:  "1001",
		},
		OS: storage.OSInfo{
			ID:      "centos",
			Like:    []string{"rhel"},
			Version: "7.2",
		},
	})
	return rpcserver.TestSystemInfo(*sysinfo)
}

type serverResults []validationpb.ServerResult

func (r serverResults) Len() int      { return len(r) }
func (r serverResults) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r serverResults) Less(i, j int) bool {
	if r[i].Error != "" || r[j].Error != "" {
		return r[i].Error < r[j].Error
	}
	return r[i].Server.Address() < r[j].Server.Address()
}
