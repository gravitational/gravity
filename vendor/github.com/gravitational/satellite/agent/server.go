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
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	serf "github.com/hashicorp/serf/client"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// Default RPC port.
const RPCPort = 7575 // FIXME: use serf to discover agents

// RPCServer is the interface that defines the interaction with an agent via RPC.
type RPCServer interface {
	Status(context.Context, *pb.StatusRequest) (*pb.StatusResponse, error)
	LocalStatus(context.Context, *pb.LocalStatusRequest) (*pb.LocalStatusResponse, error)
	Time(context.Context, *pb.TimeRequest) (*pb.TimeResponse, error)
	Stop()
}

// server implements RPCServer for an agent.
type server struct {
	*grpc.Server
	agent *agent
}

// Status reports the health status of a serf cluster by iterating over the list
// of currently active cluster members and collecting their respective health statuses.
func (r *server) Status(ctx context.Context, req *pb.StatusRequest) (resp *pb.StatusResponse, err error) {
	resp = &pb.StatusResponse{}

	resp.Status, err = r.agent.recentStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

// LocalStatus reports the health status of the local serf node.
func (r *server) LocalStatus(ctx context.Context, req *pb.LocalStatusRequest) (resp *pb.LocalStatusResponse, err error) {
	resp = &pb.LocalStatusResponse{}

	resp.Status = r.agent.recentLocalStatus()

	return resp, nil
}

// Time sends back the target node server time
func (r *server) Time(ctx context.Context, req *pb.TimeRequest) (*pb.TimeResponse, error) {
	return &pb.TimeResponse{
		Timestamp: pb.NewTimeToProto(time.Now().UTC()),
	}, nil
}

// newRPCServer creates an agent RPC endpoint for each provided listener.
func newRPCServer(agent *agent, caFile, certFile, keyFile string, rpcAddrs []string) (*server, error) {
	creds, err := credentials.NewServerTLSFromFile(certFile, keyFile)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read certificate/key from %v/%v", certFile, keyFile)
	}

	caCert, err := ioutil.ReadFile(caFile)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		ClientCAs:  caCertPool,
		ClientAuth: tls.RequireAndVerifyClientCert,
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
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
	}
	tlsConfig.BuildNameToCertificate()

	backend := grpc.NewServer(grpc.Creds(creds))
	server := &server{agent: agent, Server: backend}
	pb.RegisterAgentServer(backend, server)

	healthzHandler := newHealthHandler(server)

	// handler is a multiplexer for both gRPC and HTTPS queries.
	// The HTTPS endpoint returns the cluster status as JSON
	handler := grpcHandlerFunc(server, healthzHandler)

	for _, addr := range rpcAddrs {
		go serve(addr, certFile, keyFile, tlsConfig, handler)
	}

	return server, nil
}

func serve(addr, certFile, keyFile string, tlsConfig *tls.Config, handler http.Handler) error {
	server := &http.Server{
		Addr:      addr,
		TLSConfig: tlsConfig,
		Handler:   handler,
	}

	return server.ListenAndServeTLS(certFile, keyFile)
}

// newHealthHandler creates a http.Handler that returns cluster status
// from an HTTPS endpoint listening on the same RPC port as the agent.
func newHealthHandler(s *server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		ctx := context.TODO()
		if r.URL.Path == "/local" || r.URL.Path == "/local/" {
			handleLocalStatus(ctx, s, w, r)
			return
		}

		status, err := s.Status(ctx, nil)
		if err != nil {
			roundtrip.ReplyJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
			return
		}

		httpStatus := http.StatusOK
		if isDegraded(*status.GetStatus()) {
			httpStatus = http.StatusServiceUnavailable
		}

		roundtrip.ReplyJSON(w, httpStatus, status.GetStatus())
	}
}

func handleLocalStatus(ctx context.Context, s *server, w http.ResponseWriter, r *http.Request) {
	status, err := s.LocalStatus(ctx, nil)
	if err != nil {
		roundtrip.ReplyJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	httpStatus := http.StatusOK
	if isNodeDegraded(*status.GetStatus()) {
		httpStatus = http.StatusServiceUnavailable
	}

	roundtrip.ReplyJSON(w, httpStatus, status.GetStatus())
}

// grpcHandlerFunc returns an http.Handler that delegates to
// rpcServer on incoming gRPC connections or other otherwise
func grpcHandlerFunc(rpcServer *server, other http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType := r.Header.Get("Content-Type")
		if r.ProtoMajor == 2 && strings.Contains(contentType, "application/grpc") {
			rpcServer.ServeHTTP(w, r)
		} else {
			other.ServeHTTP(w, r)
		}
	})
}

// DefaultDialRPC is a default RPC client factory function.
// It creates a new client based on address details from the specific serf member.
func DefaultDialRPC(caFile, certFile, keyFile string) DialRPC {
	return func(member *serf.Member) (*client, error) {
		return NewClient(fmt.Sprintf("%s:%d", member.Addr.String(), RPCPort), caFile, certFile, keyFile)
	}
}
