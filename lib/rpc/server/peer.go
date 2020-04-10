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
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/rpc/client"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// NewPeer returns a new instance of the gRPC server as a peer.
// To start the peer, invoke its Serve method.
// Once started, the peer connects to the control server to register its identity.
// The control server establishes reverse connection to execute remote commands.
func NewPeer(config PeerConfig, serverAddr string) (*PeerServer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	server, err := New(config.Config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	addr := config.Listener.Addr().String()
	if config.proxyAddr != "" {
		addr = config.proxyAddr
	}

	serverPeer := &serverPeer{
		systemInfo: config.Config.systemInfo,
		addr:       addr,
		serverAddr: serverAddr,
		config:     config.RuntimeConfig,
		creds:      config.Client,
	}
	peersConfig := peersConfig{
		FieldLogger:       config.WithField(trace.Component, "server-peer"),
		watchCh:           config.WatchCh,
		checkTimeout:      config.HealthCheckTimeout,
		ReconnectStrategy: config.ReconnectStrategy,
	}
	// checker watches the connection to controlling server
	checker, err := newPeers([]Peer{serverPeer}, peersConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	peer := &PeerServer{
		PeerConfig: config,
		server:     server,
		peer:       serverPeer,
		peers:      checker,
	}
	server.closers = append(server.closers, peer)
	return peer, nil
}

// PeerConfig specifies the peer configuration
type PeerConfig struct {
	Config
	// ReconnectStrategy defines the strategy for reconnecting to server
	ReconnectStrategy
	// WatchCh is an optional channel that receives updates
	// when server reconnects.
	WatchCh chan<- WatchEvent
	// HealthCheckTimeout overrides timeout between server health check attempts.
	// Defaults to defaults.AgentHealthCheckTimeout
	HealthCheckTimeout time.Duration
	// proxyAddr specifies alternative address of this peer
	// as reported to the server.
	// Used in network tests
	proxyAddr string
}

// CheckAndSetDefaults validates this configuration object and sets defaults
func (r *PeerConfig) CheckAndSetDefaults() error {
	if r.Listener == nil {
		return trace.BadParameter("Listener is required")
	}

	if r.HealthCheckTimeout == 0 {
		r.HealthCheckTimeout = defaults.AgentReconnectTimeout
	}

	return trace.Wrap(r.Config.CheckAndSetDefaults())
}

// ReconnectStrategy defines a reconnect strategy
type ReconnectStrategy struct {
	// Backoff defines the backoff for reconnects.
	// Defaults to exponential backoff w/o time limit if nil.
	Backoff func() backoff.BackOff `json:"-"`
	// ShouldReconnect makes a decision whether to continue reconnecting
	// or to abort based on the specified error.
	// To signal abort, should return an instance of *backoff.PermanentError.
	// The handler should return a valid error to continue reconnection attempts
	ShouldReconnect func(err error) error `json:"-"`
}

// Client defines the low-level agent client interface
type Client interface {
	pb.AgentClient
	healthpb.HealthClient
	io.Closer
	// Client returns client.Client interface to this client
	Client() client.Client
}

// Peer defines a peer
type Peer interface {
	fmt.Stringer
	// Addr specifies the address of the peer
	Addr() string
	// Reconnect reestablishes a connection to this peer
	Reconnect(context.Context) (Client, error)
	// Disconnect requests a shutdown for this peer
	Disconnect(context.Context) error
}

// PeerStore receives notifications about peers joining the cluster
type PeerStore interface {
	// NewPeer adds a new peer agent
	NewPeer(context.Context, pb.PeerJoinRequest, Peer) error
	// RemovePeer removes the specified peer from the store
	RemovePeer(context.Context, pb.PeerLeaveRequest, Peer) error
}

// String formats this event for logging
func (r WatchEvent) String() string {
	return fmt.Sprintf("watchEvent(%v, client=%v, error=%v)",
		r.Peer.String(), r.Client, r.Error)
}

// WatchEvent describes a peer update
type WatchEvent struct {
	// Peer specifies the peer after reconnect.
	Peer
	// Client specifies the client for peer.
	// Only set if Error == nil
	Client
	// Error specifies the last error encountered during reconnect
	Error error
}

// Serve starts this peer
func (r *PeerServer) Serve() error {
	r.peers.start()
	return r.server.Serve()
}

// ServeWithToken starts this peer using the specified token for authorization
func (r *PeerServer) ServeWithToken(token string) error {
	r.peer.config.Token = token
	r.peers.start()
	return r.server.Serve()
}

// ValidateConnection makes sure that connection to the control server can be established
func (r *PeerServer) ValidateConnection(ctx context.Context) error {
	return r.peers.validateConnection(ctx)
}

// Stop stops this server and its internal goroutines
func (r *PeerServer) Stop(ctx context.Context) error {
	return r.server.Stop(ctx)
}

// Close stops this server and its internal goroutines
func (r *PeerServer) Close(ctx context.Context) error {
	return trace.Wrap(r.peers.close(ctx))
}

// Done returns a channel that's closed when agent shuts down
func (r *PeerServer) Done() <-chan struct{} {
	return r.server.Done()
}

// PeerServer represents a peer connected to a control server
type PeerServer struct {
	// PeerConfig is the peer configuration
	PeerConfig
	server *agentServer
	peer   *serverPeer
	peers  *peers
}

// NewPeer is a no-op
func (_ discardStore) NewPeer(context.Context, pb.PeerJoinRequest, Peer) error { return nil }

// RemovePeer is a no-op
func (_ discardStore) RemovePeer(context.Context, pb.PeerLeaveRequest, Peer) error { return nil }

// discardStore is a no-op implementation of PeerStore
type discardStore struct{}

var discardPeers = discardStore{}

// Addr returns the address of the controlling server.
// Implements Peer
func (r *serverPeer) Addr() string {
	return r.serverAddr
}

// String returns textual representation of this peer
// Implements fmt.Stringer
func (r *serverPeer) String() string {
	return fmt.Sprintf("peer(addr=%v->server=%v)", r.addr, r.serverAddr)
}

// Reconnect establishes a connection to the controlling server and rejoins the cluster.
// Implements Peer
func (r *serverPeer) Reconnect(ctx context.Context) (Client, error) {
	info, err := r.systemInfo.getSystemInfo()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	payload, err := storage.MarshalSystemInfo(info)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := newClient(ctx, r.creds, r.serverAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = clt.PeerJoin(ctx, &pb.PeerJoinRequest{Addr: r.addr, Config: &r.config, SystemInfo: payload})
	if err != nil {
		// Let ReconnectStrategy decide whether the peer should continue reconnecting
		return nil, err
	}

	return clt, nil
}

// Disconnect sends a request to the controlling server to initiate this peer
// shutdown.
// Implements Peer
func (r *serverPeer) Disconnect(ctx context.Context) error {
	info, err := r.systemInfo.getSystemInfo()
	if err != nil {
		return trace.Wrap(err)
	}
	payload, err := storage.MarshalSystemInfo(info)
	if err != nil {
		return trace.Wrap(err)
	}
	clt, err := newClient(ctx, r.creds, r.serverAddr, grpc.FailOnNonTempDialError(true))
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = clt.PeerLeave(ctx, &pb.PeerLeaveRequest{
		Addr:       r.addr,
		Config:     &r.config,
		SystemInfo: payload,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// serverPeer is the peer's end to the controlling server.
// It is used to reestablish the connection in case the server fails
// a health check
type serverPeer struct {
	systemInfo
	addr       string
	serverAddr string
	config     pb.RuntimeConfig
	creds      credentials.TransportCredentials
}

// Addr returns the address of this peer.
// Implements Peer
func (r remotePeer) Addr() string {
	return r.addr
}

// String returns textual representation of this peer
func (r remotePeer) String() string {
	return fmt.Sprintf("peer(addr=%v)", r.addr)
}

// Reconnect establishes a connection to this peer.
// Implements Peer
func (r remotePeer) Reconnect(ctx context.Context) (Client, error) {
	ctx, cancel := context.WithTimeout(ctx, r.reconnectTimeout)
	defer cancel()
	clt, err := newClient(ctx, r.creds, r.addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// Disconnect is no-op for this type of peer
func (r remotePeer) Disconnect(ctx context.Context) error {
	return nil
}

type remotePeer struct {
	addr             string
	creds            credentials.TransportCredentials
	reconnectTimeout time.Duration
}

func newClient(ctx context.Context, creds credentials.TransportCredentials, addr string, dialOpts ...grpc.DialOption) (*agentClient, error) {
	opts := []grpc.DialOption{
		grpc.WithBackoffMaxDelay(defaults.RPCAgentBackoffThreshold),
		grpc.WithBlock(),
		grpc.WithTransportCredentials(creds),
	}
	opts = append(opts, dialOpts...)

	conn, err := grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &agentClient{pb.NewAgentClient(conn), healthpb.NewHealthClient(conn), conn}, nil
}

type agentClient struct {
	pb.AgentClient
	healthpb.HealthClient
	*grpc.ClientConn
}

// Close closes the underlying connection
func (r *agentClient) Close() error {
	return r.ClientConn.Close()
}

// Client returns a new client as client.Client
func (r *agentClient) Client() client.Client {
	return client.NewFromConn(r.ClientConn)
}

// String returns textual representation of this peer
func (r peer) String() string {
	return fmt.Sprintf("peer(addr=%v)", r.Addr())
}

// Command executes the command specified with args on this peer
func (r *peer) Command(ctx context.Context, log log.FieldLogger, stdout, stderr io.Writer, args ...string) error {
	if r.Client == nil {
		return trace.ConnectionProblem(nil, "%v not connected", r.Addr())
	}
	return trace.Wrap(r.Client.Client().Command(ctx, log, stdout, stderr, args...))
}

// GravityCommand executes the gravity command specified with args on this peer
func (r *peer) GravityCommand(ctx context.Context, log log.FieldLogger, stdout, stderr io.Writer, args ...string) error {
	if r.Client == nil {
		return trace.ConnectionProblem(nil, "%v not connected", r.Addr())
	}
	return trace.Wrap(r.Client.Client().GravityCommand(ctx, log, stdout, stderr, args...))
}

// GetSystemInfo queries remote system information
func (r *peer) GetSystemInfo(ctx context.Context) (storage.System, error) {
	if r.Client == nil {
		return nil, trace.ConnectionProblem(nil, "%v not connected", r.Addr())
	}
	system, err := r.Client.Client().GetSystemInfo(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return system, nil
}

// GetRuntimeConfig returns agent's runtime configuration
func (r *peer) GetRuntimeConfig(ctx context.Context) (*pb.RuntimeConfig, error) {
	if r.Client == nil {
		return nil, trace.ConnectionProblem(nil, "%v not connected", r.Addr())
	}
	config, err := r.Client.Client().GetRuntimeConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// GetCurrentTime returns agent's current time as UTC timestamp
func (r *peer) GetCurrentTime(ctx context.Context) (*time.Time, error) {
	if r.Client == nil {
		return nil, trace.ConnectionProblem(nil, "%v not connected", r.Addr())
	}
	ts, err := r.Client.Client().GetCurrentTime(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ts, nil
}

// Shutdown shuts down this peer
func (r *peer) Shutdown(ctx context.Context, req *pb.ShutdownRequest) error {
	if r.Client == nil {
		return nil
	}
	return trace.Wrap(r.Client.Client().Shutdown(ctx, req))
}

// Abort aborts this peer
func (r *peer) Abort(ctx context.Context) error {
	if r.Client == nil {
		return nil
	}
	return trace.Wrap(r.Client.Client().Abort(ctx))
}

// Close closes the underlying client
func (r *peer) Close() error {
	if r.Client == nil {
		return nil
	}
	return trace.Wrap(r.Client.Client().Close())
}

// peer represents a remote agent peer.
// implements client.Client
type peer struct {
	Peer
	Client
	// doneCh is the channel that is closed when this peer shuts down
	doneCh chan struct{}
}
