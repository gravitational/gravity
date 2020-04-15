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
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// RPCServer is the interface that defines the interaction with an agent via RPC.
type RPCServer interface {
	Status(context.Context, *pb.StatusRequest) (*pb.StatusResponse, error)
	LocalStatus(context.Context, *pb.LocalStatusRequest) (*pb.LocalStatusResponse, error)
	// LastSeen returns the last seen timestamp for a specified member.
	LastSeen(context.Context, *pb.LastSeenRequest) (*pb.LastSeenResponse, error)
	Time(context.Context, *pb.TimeRequest) (*pb.TimeResponse, error)
	Timeline(context.Context, *pb.TimelineRequest) (*pb.TimelineResponse, error)
	UpdateTimeline(context.Context, *pb.UpdateRequest) (*pb.UpdateResponse, error)
	Stop()
}

// server implements RPCServer for an agent.
type server struct {
	*grpc.Server
	agent       Agent
	httpServers []*http.Server
}

// Status reports the health status of a serf cluster by iterating over the list
// of currently active cluster members and collecting their respective health statuses.
func (r *server) Status(ctx context.Context, req *pb.StatusRequest) (resp *pb.StatusResponse, err error) {
	status, err := r.agent.Status()
	if err != nil {
		return nil, GRPCError(err)
	}
	return &pb.StatusResponse{Status: status}, nil
}

// LocalStatus reports the health status of the local serf node.
func (r *server) LocalStatus(ctx context.Context, req *pb.LocalStatusRequest) (resp *pb.LocalStatusResponse, err error) {
	return &pb.LocalStatusResponse{
		Status: r.agent.LocalStatus(),
	}, nil
}

// LastSeen returns the last seen timestamp for a specified member.
func (r *server) LastSeen(ctx context.Context, req *pb.LastSeenRequest) (resp *pb.LastSeenResponse, err error) {
	timestamp, err := r.agent.LastSeen(req.GetName())
	if err != nil {
		return nil, GRPCError(err)
	}
	return &pb.LastSeenResponse{
		Timestamp: pb.NewTimeToProto(timestamp),
	}, nil
}

// Time sends back the target node server time
func (r *server) Time(ctx context.Context, req *pb.TimeRequest) (*pb.TimeResponse, error) {
	return &pb.TimeResponse{
		Timestamp: pb.NewTimeToProto(r.agent.Time().UTC()),
	}, nil
}

// Timeline sends the current status timeline
func (r *server) Timeline(ctx context.Context, req *pb.TimelineRequest) (*pb.TimelineResponse, error) {
	events, err := r.agent.GetTimeline(ctx, req.GetParams())
	if err != nil {
		return nil, GRPCError(err)
	}
	return &pb.TimelineResponse{Events: events}, nil
}

// UpdateTimeline records a new event into the cluster timeline.
// Duplicate requests will have no effect.
func (r *server) UpdateTimeline(ctx context.Context, req *pb.UpdateRequest) (*pb.UpdateResponse, error) {
	if err := r.agent.RecordClusterEvents(ctx, []*pb.TimelineEvent{req.GetEvent()}); err != nil {
		return nil, GRPCError(err)
	}
	if err := r.agent.RecordLastSeen(req.GetName(), req.GetEvent().GetTimestamp().ToTime()); err != nil {
		return nil, GRPCError(err)
	}
	return &pb.UpdateResponse{}, nil
}

// Stop stops the grpc server and any additional http servers.
// TODO: modify Stop to return error
func (r *server) Stop() {
	// TODO: pass context in as a parameter.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := r.stopHTTPServers(ctx); err != nil {
		log.WithError(err).Error("Some HTTP servers failed to shutdown.")
	}

	r.Server.Stop()
}

// stopHTTPServers shuts down all listening http servers.
func (r *server) stopHTTPServers(ctx context.Context) error {
	var errors []error
	for _, srv := range r.httpServers {
		err := srv.Shutdown(ctx)
		if err == http.ErrServerClosed {
			log.WithError(err).Debug("Server has already been shutdown.")
			continue
		}
		if err != nil {
			errors = append(errors, trace.Wrap(err, "failed to shutdown server running on: %s", srv.Addr))
			continue
		}
	}
	return trace.NewAggregate(errors...)
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

	backend := grpc.NewServer(grpc.Creds(creds))
	server := &server{agent: agent, Server: backend}
	pb.RegisterAgentServer(backend, server)

	healthzHandler := newHealthHandler(server)

	// handler is a multiplexer for both gRPC and HTTPS queries.
	// The HTTPS endpoint returns the cluster status as JSON
	// TODO: why does server need to handle both gRPC and HTTPS queries?
	handler := grpcHandlerFunc(server, healthzHandler)

	for _, addr := range rpcAddrs {
		srv := newHTTPServer(addr, newTLSConfig(caCertPool), handler)
		server.httpServers = append(server.httpServers, srv)

		// TODO: separate Start function to start listening.
		go func(srv *http.Server) {
			err := srv.ListenAndServeTLS(certFile, keyFile)
			if err == http.ErrServerClosed {
				log.WithError(err).Debug("Server has been shutdown/closed.")
				return
			}
			if err != nil {
				log.WithError(err).Errorf("Failed to serve on %v.", srv.Addr)
				return
			}
		}(srv)
	}
	return server, nil
}

func newTLSConfig(caCertPool *x509.CertPool) *tls.Config {
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
	return tlsConfig
}

// newHTTPServer constructs a new server using the provided config values.
func newHTTPServer(address string, tlsConfig *tls.Config, handler http.Handler) *http.Server {
	server := &http.Server{
		Addr:      address,
		TLSConfig: tlsConfig,
		Handler:   handler,
	}
	return server
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

		if r.URL.Path == "/history" || r.URL.Path == "/history/" {
			handleHistory(ctx, s, w, r)
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

// handleHistory handles status history API call.
func handleHistory(ctx context.Context, s *server, w http.ResponseWriter, r *http.Request) {
	timeline, err := s.Timeline(ctx, nil)
	if err != nil {
		roundtrip.ReplyJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}

	httpStatus := http.StatusOK
	roundtrip.ReplyJSON(w, httpStatus, timeline)
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

// Agent is the interface to interact with the monitoring agent.
type Agent interface {
	// Start starts agent's background jobs.
	Start() error
	// Close stops background activity and releases resources.
	Close() error
	// Join makes an attempt to join a cluster specified by the list of peers.
	Join(peers []string) error
	// Time reports the current server time.
	Time() time.Time
	// LocalStatus reports the health status of the local agent node.
	LocalStatus() *pb.NodeStatus
	// Status reports the health status of the cluster.
	Status() (*pb.SystemStatus, error)
	// LastSeen returns the last seen timestamp from the specified member.
	LastSeen(name string) (time.Time, error)
	// RecordLastSeen records the last seen timestamp for the specified member.
	RecordLastSeen(name string, timestamp time.Time) error
	// GetTimeline returns the current cluster timeline.
	GetTimeline(ctx context.Context, params map[string]string) ([]*pb.TimelineEvent, error)
	// RecordClusterEvents records the events into the cluster timeline.
	RecordClusterEvents(ctx context.Context, events []*pb.TimelineEvent) error
	// RecordLocalEvents records the events into the local timeline.
	RecordLocalEvents(ctx context.Context, events []*pb.TimelineEvent) error
	// IsMember returns whether this agent is already a member of serf cluster
	IsMember() bool
	// GetConfig returns the agent configuration.
	GetConfig() Config
	// CheckerRepository allows to add checks to the agent.
	health.CheckerRepository
}
