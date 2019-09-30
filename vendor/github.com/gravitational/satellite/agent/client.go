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

package agent

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/gravitational/trace"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Client is an interface to communicate with the serf cluster via agent RPC.
type Client interface {
	// Status reports the health status of a serf cluster.
	Status(context.Context) (*pb.SystemStatus, error)
	// LocalStatus reports the health status of the local serf cluster node.
	LocalStatus(context.Context) (*pb.NodeStatus, error)
	// Time returns the current time on the target node.
	Time(context.Context, *pb.TimeRequest) (*pb.TimeResponse, error)
}

type client struct {
	pb.AgentClient
	conn *grpc.ClientConn
}

// NewClientFunc defines a function that returns RPC agent client.
type NewClientFunc func(addr, caFile, certFile, keyFile string) (*client, error)

// NewClient creates a agent RPC client to the given address
// using the specified client certificate certFile
func NewClient(addr, caFile, certFile, keyFile string) (*client, error) {
	return newClient(addr, "", caFile, certFile, keyFile)
}

// newClient additional allows a serverName override, but is only available
// from unit tests
func newClient(addr, serverName, caFile, certFile, keyFile string) (*client, error) {
	// Load client cert/key
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	// Load the CA of the server
	clientCACert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(clientCACert) {
		return nil, trace.Wrap(err, "failed to append certificates from %v", caFile)
	}

	creds := credentials.NewTLS(&tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ServerName:   serverName,
		// Use TLS Modern capability suites
		// https://wiki.mozilla.org/Security/Server_Side_TLS
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		},
	})

	return NewClientWithCreds(addr, creds)
}

// NewClientWithCreds creates a new agent RPC client to the given address
// using specified credentials creds
func NewClientWithCreds(addr string, creds credentials.TransportCredentials) (*client, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, trace.Wrap(err, "failed to dial")
	}

	c := pb.NewAgentClient(conn)
	return &client{AgentClient: c, conn: conn}, nil
}

// Status reports the health status of the serf cluster.
func (r *client) Status(ctx context.Context) (*pb.SystemStatus, error) {
	opts := []grpc.CallOption{
		grpc.FailFast(false),
	}
	resp, err := r.AgentClient.Status(ctx, &pb.StatusRequest{}, opts...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp.Status, nil
}

// LocalStatus reports the health status of the local serf node.
func (r *client) LocalStatus(ctx context.Context) (*pb.NodeStatus, error) {
	opts := []grpc.CallOption{
		grpc.FailFast(false),
	}
	resp, err := r.AgentClient.LocalStatus(ctx, &pb.LocalStatusRequest{}, opts...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp.Status, nil
}

// Time returns the current time on the target node.
func (r *client) Time(ctx context.Context, req *pb.TimeRequest) (time *pb.TimeResponse, err error) {
	opts := []grpc.CallOption{
		grpc.FailFast(false),
	}
	resp, err := r.AgentClient.Time(ctx, req, opts...)
	if err != nil {
		return nil, ConvertGRPCError(err)
	}
	return resp, nil
}

// Close closes the RPC client connection.
func (r *client) Close() error {
	return r.conn.Close()
}
