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

package server

import (
	"bytes"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/rpc/client"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	. "gopkg.in/check.v1"
)

func (r *S) TestRequiresCert(c *C) {
	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	_, err := client.New(ctx,
		client.Config{
			ServerAddr: "127.0.0.1:25123",
		})
	c.Assert(err, Not(IsNil), Commentf("client certificate should be required"))
}

func (r *S) TestClientExecutesCommandsRemotely(c *C) {
	creds := TestCredentials(c)
	cmd := TestCommand{"server output"}
	log := r.WithField("test", "ClientExecutesCommandsRemotely")
	listener := listen(c)
	srv, err := New(Config{
		FieldLogger:     log.WithField("server", listener.Addr()),
		Listener:        listener,
		Credentials:     creds,
		commandExecutor: cmd,
	})
	c.Assert(err, IsNil)
	go func() {
		c.Assert(srv.Serve(), IsNil)
	}()

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	clt, err := client.New(ctx,
		client.Config{
			ServerAddr:  srv.Addr().String(),
			Credentials: creds.Client,
		})
	c.Assert(err, IsNil)
	r.clientExecutesCommandsWithClient(c, clt, srv, cmd.output)
}

func (r *S) TestAgentsConnectToController(c *C) {
	creds := TestCredentials(c)
	store := newPeerStore()
	log := r.WithField("test", "AgentsConnectToController")
	listener := listen(c)
	srv, err := New(Config{
		FieldLogger: log.WithField("server", listener.Addr()),
		Listener:    listener,
		Credentials: creds,
		PeerStore:   store,
	})
	c.Assert(err, IsNil)

	go func() {
		c.Assert(srv.Serve(), IsNil)
	}()
	defer withTestCtx(srv.Stop, c)

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	peer := r.newPeer(c, PeerConfig{Config: Config{Listener: listen(c)}}, srv.Addr().String(), log)
	go func() {
		c.Assert(peer.Serve(), IsNil)
	}()
	defer withTestCtx(peer.Stop, c)

	err = store.expect(ctx, 1)
	cancel()
	c.Assert(err, IsNil)

	peers := store.getPeers()
	obtained := make([]string, 0, len(peers))
	for _, peer := range peers {
		obtained = append(obtained, peer.Addr())
	}

	c.Assert(obtained, DeepEquals, []string{peer.Addr().String()})
}

func (r *S) TestPeerDisconnect(c *C) {
	creds := TestCredentials(c)
	store := newPeerStore()
	log := r.WithField("test", "PeerDisconnect")
	listener := listen(c)
	srv, err := New(Config{
		FieldLogger: log.WithField("server", listener.Addr()),
		Listener:    listener,
		Credentials: creds,
		PeerStore:   store,
	})
	c.Assert(err, IsNil)
	go func() {
		c.Assert(srv.Serve(), IsNil)
	}()
	defer withTestCtx(srv.Stop, c)

	// launch two peers
	peer1, err := NewPeer(PeerConfig{
		Config: Config{
			FieldLogger: log,
			Listener:    listen(c),
			Credentials: creds,
			systemInfo:  TestSystemInfo{},
		},
	}, srv.Addr().String())
	c.Assert(err, IsNil)
	go func() {
		c.Assert(peer1.Serve(), IsNil)
	}()
	defer withTestCtx(peer1.Stop, c)

	peer2, err := NewPeer(PeerConfig{
		Config: Config{
			FieldLogger: log,
			Listener:    listen(c),
			Credentials: creds,
			systemInfo:  TestSystemInfo{},
		},
	}, srv.Addr().String())
	c.Assert(err, IsNil)
	go func() {
		c.Assert(peer2.Serve(), IsNil)
	}()
	defer withTestCtx(peer2.Stop, c)

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	c.Assert(store.expect(ctx, 2), IsNil)

	// shut one of the peers down
	ctx, cancel = context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	c.Assert(peer1.Stop(ctx), IsNil)

	// make sure only the second peer is left
	c.Assert(store.getPeers(), compare.DeepEquals, []Peer{
		&remotePeer{
			addr:             peer2.Addr().String(),
			creds:            creds.Client,
			reconnectTimeout: peer2.PeerConfig.ReconnectTimeout,
		},
	})
}

func (r *S) TestServerReportsHealth(c *C) {
	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	creds := TestCredentials(c)
	log := r.WithField("test", "ServerReportsHealth")
	listener := listen(c)
	srv, err := New(Config{
		FieldLogger: log.WithField("server", listener.Addr()),
		Listener:    listener,
		Credentials: creds,
	})
	c.Assert(err, IsNil)

	go func() {
		c.Assert(srv.Serve(), IsNil)
	}()
	defer withTestCtx(srv.Stop, c)

	clt, err := newClient(ctx, creds.Client, srv.Addr().String())
	c.Assert(err, IsNil)

	resp, err := clt.Check(ctx, &healthpb.HealthCheckRequest{})
	c.Assert(err, IsNil)
	c.Assert(resp, DeepEquals, &healthpb.HealthCheckResponse{
		Status: healthpb.HealthCheckResponse_SERVING,
	})
}

func (r *S) TestWaitsUntilAgentShutsDown(c *C) {
	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	creds := TestCredentials(c)
	log := r.WithField("test", "WaitsUntilAgentShutsDown")
	listener := listen(c)
	srv, err := New(Config{
		FieldLogger: log.WithField("server", listener.Addr()),
		Listener:    listener,
		Credentials: creds,
	})
	c.Assert(err, IsNil)

	go func() {
		c.Assert(srv.Serve(), IsNil)
	}()

	withTestCtx(srv.Stop, c)
	select {
	case <-srv.Done():
	case <-ctx.Done():
		c.Error("timeout waiting for server to shut down")
	}
}

func (r *S) TestRejectsPeer(c *C) {
	creds := TestCredentials(c)
	store := rejectingStore{}
	log := r.WithField("test", "RejectsPeer")
	listener := listen(c)
	srv, err := New(Config{
		FieldLogger: log.WithField("server", listener.Addr().String()),
		Listener:    listener,
		Credentials: creds,
		PeerStore:   store,
	})
	c.Assert(err, IsNil)

	go func() {
		c.Assert(srv.Serve(), IsNil)
	}()
	defer withTestCtx(srv.Stop, c)

	watchCh := make(chan WatchEvent, 1)
	config := PeerConfig{
		Config: Config{
			FieldLogger: log,
			Listener:    listen(c),
			Credentials: TestCredentials(c),
			systemInfo:  TestSystemInfo{},
		},
		WatchCh: watchCh,
		ReconnectStrategy: ReconnectStrategy{
			ShouldReconnect: utils.ShouldReconnectPeer,
		},
	}
	p, err := NewPeer(config, srv.Addr().String())
	c.Assert(err, IsNil)
	go func() {
		c.Assert(p.Serve(), IsNil)
	}()
	defer withTestCtx(p.Stop, c)

	select {
	case update := <-watchCh:
		c.Assert(trace.Unwrap(update.Error), ErrorMatches, "(?ms).*peer not authorized.*")
	case <-time.After(5 * time.Second):
		c.Error("timeout waiting for connect failure")
	}
}

func (r *S) TestQueriesSystemInfo(c *C) {
	sysinfo := storage.NewSystemInfo(storage.SystemSpecV2{
		Hostname: "foo",
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
	creds := TestCredentials(c)
	log := r.WithField("test", "QueriesSystemInfo")
	listener := listen(c)
	srv, err := New(Config{
		FieldLogger: log.WithField("server", listener.Addr()),
		Listener:    listener,
		Credentials: creds,
		systemInfo:  TestSystemInfo(*sysinfo),
	})
	c.Assert(err, IsNil)

	go func() {
		c.Assert(srv.Serve(), IsNil)
	}()
	defer withTestCtx(srv.Stop, c)

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	clt, err := client.New(ctx,
		client.Config{
			ServerAddr:  srv.Addr().String(),
			Credentials: creds.Client,
		})
	c.Assert(err, IsNil)

	obtained, err := clt.GetSystemInfo(ctx)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, obtained, sysinfo)
}

func (r *S) clientExecutesCommandsWithClient(c *C, clt client.Interface, srv *AgentServer, expectedOutput string) {
	defer withTestCtx(srv.Stop, c)

	clientLog := r.WithField(trace.Component, "client")
	var buf bytes.Buffer
	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	err := clt.Command(ctx, clientLog, &buf, &buf, "test")
	c.Assert(err, IsNil)

	err = clt.Shutdown(ctx, &pb.ShutdownRequest{})
	clt.Close()

	c.Assert(err, IsNil)
	c.Assert(buf.String(), Equals, expectedOutput)
}

func (r *S) newPeer(c *C, config PeerConfig, serverAddr string, log log.FieldLogger) *PeerServer {
	config.FieldLogger = log.WithField("peer", config.Listener.Addr())
	return NewTestPeer(c, config, serverAddr,
		TestCommand{"test output"}, TestSystemInfo{},
	)
}

func (r rejectingStore) NewPeer(ctx context.Context, req pb.PeerJoinRequest, peer Peer) error {
	return status.Error(codes.PermissionDenied, "peer not authorized")
}

func (r rejectingStore) RemovePeer(ctx context.Context, req pb.PeerLeaveRequest, peer Peer) error {
	return status.Error(codes.PermissionDenied, "peer not authorized")
}

type rejectingStore struct{}

// newPeerStore creates a new peer store
func newPeerStore() *peerStore {
	return &peerStore{
		peers:  make(map[string]Peer),
		peerCh: make(chan struct{}, 1),
	}
}

// peerStore receives notifications about peers joining the cluster.
// It implements PeerStore.
type peerStore struct {
	// peerCh gets notified when a new peer joins
	peerCh chan struct{}
	// Mutex protecting the following fields
	sync.Mutex
	peers map[string]Peer
}

// NewPeer receives a new peer
func (r *peerStore) NewPeer(ctx context.Context, req pb.PeerJoinRequest, peer Peer) error {
	r.add(peer)

	// Attempt to notify about the new peer
	select {
	case r.peerCh <- struct{}{}:
	default:
	}

	return nil
}

// RemovePeer removes the specified peer
func (r *peerStore) RemovePeer(ctx context.Context, req pb.PeerLeaveRequest, peer Peer) error {
	r.remove(peer)
	return nil
}

// getPeers returns active peers as a list
func (r *peerStore) getPeers() (peers []Peer) {
	r.Lock()
	defer r.Unlock()
	peers = make([]Peer, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, peer)
	}
	return peers
}

// expect is a blocking method that will return once the specified number of peers
// have joined.
// Returns an error if the context expires sooner than the required number of peers
// are available.
func (r *peerStore) expect(ctx context.Context, peers int) error {
	r.Lock()
	peers = peers - len(r.peers)
	r.Unlock()
	for peers > 0 {
		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-r.peerCh:
			peers = peers - 1
		}
	}
	return nil
}

func (r *peerStore) add(peer Peer) {
	r.Lock()
	defer r.Unlock()
	if _, existing := r.peers[peer.Addr()]; existing {
		return
	}

	r.peers[peer.Addr()] = peer
}

func (r *peerStore) remove(peer Peer) {
	r.Lock()
	defer r.Unlock()
	delete(r.peers, peer.Addr())
}
