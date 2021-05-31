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
	"context"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/network/validation"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// Server defines an RPC server
type Server interface {
	// Serve starts the server loop accepting connections
	Serve() error
	// ServeHTTP implements http.Handler
	ServeHTTP(http.ResponseWriter, *http.Request)
	// Stop requests the server to stop and clean up
	Stop(context.Context) error
	// Addr returns address the server is listening on
	Addr() net.Addr
	// Done returns a channel that's closed when agent shuts down
	Done() <-chan struct{}
}

// New returns a new instance of the unstarted gRPC server
func New(config Config) (*AgentServer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	opts := append([]grpc.ServerOption{},
		grpc.Creds(config.Credentials.Server),
	)

	ctx, cancel := context.WithCancel(context.TODO())
	healthServer := health.NewServer()
	validationServer := validation.NewServer(config.FieldLogger)
	grpcServer := grpc.NewServer(opts...)
	srv := AgentServer{
		grpcServer: grpcServer,
		config:     config,
		ctx:        ctx,
		cancel:     cancel,
	}
	pb.RegisterAgentServer(grpcServer, &srv)
	pb.RegisterDiscoveryServer(grpcServer, &srv)
	healthpb.RegisterHealthServer(grpcServer, healthServer)
	validationpb.RegisterValidationServer(grpcServer, validationServer)

	return &srv, nil
}

// Serve starts the server loop accepting connections
func (srv *AgentServer) Serve() error {
	srv.config.WithField("addr", srv.config.Listener.Addr().String()).Info("Listening.")
	return trace.Wrap(srv.serve(srv.config.Listener))
}

// ServeHTTP implements http.Handler
func (srv *AgentServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.grpcServer.ServeHTTP(w, r)
}

// Stop requests the server to stop and clean up
func (srv *AgentServer) Stop(ctx context.Context) error {
	select {
	case <-srv.ctx.Done():
		return nil
	default:
	}
	for _, c := range srv.config.closers {
		c.Close(ctx)
	}
	srv.grpcServer.GracefulStop()
	srv.cancel()
	return nil
}

// Addr returns address the server is listening on.
func (srv *AgentServer) Addr() net.Addr {
	return srv.config.Listener.Addr()
}

// Done returns a channel that's closed when agent shuts down
func (srv *AgentServer) Done() <-chan struct{} {
	return srv.ctx.Done()
}

func (srv *AgentServer) serve(listener net.Listener) error {
	err := srv.grpcServer.Serve(listener)
	if err != nil && utils.IsClosedConnectionError(err) {
		// Ignore
		err = nil
	}
	srv.config.WithError(err).Info("Server stopped.")

	select {
	case <-srv.ctx.Done():
		return nil
	default:
		return trace.Wrap(err)
	}
}

// Config defines RPC server configuration
type Config struct {
	logrus.FieldLogger
	// Credentials specifies the connect credentials
	Credentials
	// PeerStore specifies the peer store.
	// The store is used to keep track of active peers.
	PeerStore
	// Listener specifies the listener for network connections
	net.Listener
	// RuntimeConfig specifies the runtime agent configuration
	pb.RuntimeConfig
	// ReconnectTimeout specifies the maximum timeout used to reconnect to a peer.
	// Defaults to defaults.RPCAgentBackoffThreshold
	ReconnectTimeout time.Duration
	// AbortHandler specifies an optional handler for aborting the operation.
	// The handler is invoked when serving the Abort API.
	// Note that the handler should avoid invoking blocking gRPC APIs - otherwise the
	// service shut down might block
	AbortHandler func(context.Context) error
	// StopHandler specifies an optional handler for when the agent is stopped.
	// completed indicates whether the agent is stopped after a successfully completed operation
	StopHandler func(ctx context.Context, completed bool) error
	// systemInfo queries system information
	systemInfo
	// commandExecutor is a system command executor.
	// Being an interface provides necessary flexibility for testing.
	commandExecutor
	// closers lists additional resources to close upon receiving a stop command
	closers []closer
}

// CheckAndSetDefaults validates this config and sets defaults
func (r *Config) CheckAndSetDefaults() error {
	if r.PeerStore == nil {
		r.PeerStore = discardPeers
	}

	if r.ReconnectTimeout == 0 {
		r.ReconnectTimeout = defaults.RPCAgentBackoffThreshold
	}

	if r.FieldLogger == nil {
		r.FieldLogger = logrus.WithField(trace.Component, "rpcserver")
	}

	if r.systemInfo == nil {
		r.systemInfo = realSystemInfo{}
	}

	if r.commandExecutor == nil {
		r.commandExecutor = execFunc(osExec)
	}

	return nil
}

// Credentials specifies the connect credentials
type Credentials struct {
	// Client specifies client connect credentials
	Client credentials.TransportCredentials
	// Server specifies server connect credentials
	Server credentials.TransportCredentials
}

// IsEmpty determines if this Credentials is empty
func (r Credentials) IsEmpty() bool {
	return r.Client == nil && r.Server == nil
}

func (realSystemInfo) getSystemInfo() (storage.System, error) {
	return systeminfo.New()
}

type realSystemInfo struct{}

type systemInfo interface {
	getSystemInfo() (storage.System, error)
}

// AgentServer implements a server in the agent cluster
type AgentServer struct {
	config     Config
	grpcServer *grpc.Server
	ctx        context.Context
	cancel     context.CancelFunc
}

type closer interface {
	Close(context.Context) error
}
