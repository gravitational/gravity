/*
Copyright 2016 Gravitational, Inc.

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

// Package client provides a client interface for interacting with a satellite
// agent.
package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	debugpb "github.com/gravitational/satellite/agent/proto/debug"
	"github.com/gravitational/satellite/lib/rpc"

	"github.com/gravitational/trace"
	serf "github.com/hashicorp/serf/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Config defines configuration required to create a new RPC client.
type Config struct {
	// Address specifies client RPC address.
	Address string
	// CAFile specifies CA file path.
	CAFile string
	// CertFile specifies certificate file path.
	CertFile string
	// KeyFile specifies key file path.
	KeyFile string
}

// CheckAndSetDefaults validates this configuration object.
// Config values that were not specified will be set to their default values if
// available.
func (c *Config) CheckAndSetDefaults() error {
	var errors []error
	if c.Address == "" {
		errors = append(errors, trace.BadParameter("address must be provided"))
	}
	if c.CAFile == "" {
		errors = append(errors, trace.BadParameter("CA file path must be provided"))
	}
	if c.CertFile == "" {
		errors = append(errors, trace.BadParameter("certificate file path must be provided"))
	}
	if c.KeyFile == "" {
		errors = append(errors, trace.BadParameter("key file path must be provided"))
	}
	return trace.NewAggregate(errors...)
}

// Client is an interface to communicate with the serf cluster via agent RPC.
type Client interface {
	// Status reports the health status of a serf cluster.
	Status(context.Context) (*pb.SystemStatus, error)
	// LocalStatus reports the health status of the local serf cluster node.
	LocalStatus(context.Context) (*pb.NodeStatus, error)
	// LastSeen requests the last seen timestamp for a member specified by
	// their serf name.
	LastSeen(context.Context, *pb.LastSeenRequest) (*pb.LastSeenResponse, error)
	// Time returns the current time on the target node.
	Time(context.Context, *pb.TimeRequest) (*pb.TimeResponse, error)
	// Timeline returns the current status timeline.
	Timeline(context.Context, *pb.TimelineRequest) (*pb.TimelineResponse, error)
	// UpdateTimeline requests that the timeline be updated with the specified event.
	UpdateTimeline(context.Context, *pb.UpdateRequest) (*pb.UpdateResponse, error)
	// Profile streams the debug profile specified in req
	Profile(ctx context.Context, req *debugpb.ProfileRequest) (debugpb.Debug_ProfileClient, error)
	// Close closes the RPC client connection.
	Close() error
}

type client struct {
	pb.AgentClient
	debugpb.DebugClient
	conn        *grpc.ClientConn
	callOptions []grpc.CallOption
}

// NewClient creates a agent RPC client to the given address
// using the specified client certificate certFile
func NewClient(ctx context.Context, config Config) (*client, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// Load client cert/key
	cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Load the CA of the server
	clientCACert, err := ioutil.ReadFile(config.CAFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(clientCACert) {
		return nil, trace.Wrap(err, "failed to append certificates from %v", config.CAFile)
	}

	creds := credentials.NewTLS(&tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: rpc.DefaultCipherSuites,
	})
	return NewClientWithCreds(ctx, config.Address, creds)
}

// NewClientWithCreds creates a new agent RPC client to the given address
// using specified credentials creds
func NewClientWithCreds(ctx context.Context, addr string, creds credentials.TransportCredentials) (*client, error) {
	conn, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, trace.Wrap(err, "failed to dial")
	}
	return &client{
		AgentClient: pb.NewAgentClient(conn),
		DebugClient: debugpb.NewDebugClient(conn),
		conn:        conn,
		// TODO: provide option to initialize client with more call options.
		callOptions: []grpc.CallOption{grpc.FailFast(false)},
	}, nil
}

// Status reports the health status of the serf cluster.
func (r *client) Status(ctx context.Context) (*pb.SystemStatus, error) {
	resp, err := r.AgentClient.Status(ctx, &pb.StatusRequest{}, r.callOptions...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp.Status, nil
}

// LocalStatus reports the health status of the local serf node.
func (r *client) LocalStatus(ctx context.Context) (*pb.NodeStatus, error) {
	resp, err := r.AgentClient.LocalStatus(ctx, &pb.LocalStatusRequest{}, r.callOptions...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp.Status, nil
}

// LastSeen requests the last seen timestamp for member specified by their serf
// name.
func (r *client) LastSeen(ctx context.Context, req *pb.LastSeenRequest) (*pb.LastSeenResponse, error) {
	resp, err := r.AgentClient.LastSeen(ctx, req, r.callOptions...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp, nil
}

// Time returns the current time on the target node.
func (r *client) Time(ctx context.Context, req *pb.TimeRequest) (time *pb.TimeResponse, err error) {
	resp, err := r.AgentClient.Time(ctx, req, r.callOptions...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp, nil
}

// Timeline returns the current status timeline.
func (r *client) Timeline(ctx context.Context, req *pb.TimelineRequest) (timeline *pb.TimelineResponse, err error) {
	resp, err := r.AgentClient.Timeline(ctx, req, r.callOptions...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp, nil
}

// UpdateTimeline request the update the timeline with a new event.
func (r *client) UpdateTimeline(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	resp, err := r.AgentClient.UpdateTimeline(ctx, req, r.callOptions...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp, nil
}

// Profile streams the debug profile specified in req
func (r *client) Profile(ctx context.Context, req *debugpb.ProfileRequest) (debugpb.Debug_ProfileClient, error) {
	resp, err := r.DebugClient.Profile(ctx, req, r.callOptions...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp, nil
}

// Close closes the RPC client connection.
func (r *client) Close() error {
	return r.conn.Close()
}

// DialRPC returns RPC client for the provided Serf member.
type DialRPC func(context.Context, *serf.Member) (Client, error)

// DefaultDialRPC is a default RPC client factory function.
// It creates a new client based on address details from the specific serf member.
func DefaultDialRPC(caFile, certFile, keyFile string) DialRPC {
	return func(ctx context.Context, member *serf.Member) (Client, error) {
		config := Config{
			Address:  fmt.Sprintf("%s:%d", member.Addr.String(), rpc.Port),
			CAFile:   caFile,
			CertFile: certFile,
			KeyFile:  keyFile,
		}
		return NewClient(ctx, config)
	}
}
