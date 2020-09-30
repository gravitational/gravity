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

package validation

import (
	"encoding/json"
	"time"

	"github.com/gravitational/gravity/lib/checks"
	pb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/satellite/agent/health"
	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/monitoring"
	"github.com/gravitational/trace"

	"github.com/davecgh/go-spew/spew"
	"github.com/gogo/protobuf/types"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// NewServer creates a new validation server
func NewServer(log log.FieldLogger) *Server {
	return &Server{log}
}

// Server is a validation server.
// Implements pb.ValidationServer
type Server struct {
	// FieldLogger specifies the logger to use for the server
	log.FieldLogger
}

// CheckPorts executes a network ports test
func (r *Server) CheckPorts(ctx context.Context, req *pb.CheckPortsRequest) (*pb.CheckPortsResponse, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	duration, err := types.DurationFromProto(req.Duration)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	listenCh := make(chan pb.ServerResult, len(req.Listen))
	for _, listenServer := range req.Listen {
		go func(server *pb.Addr) {
			err := listen(ctx, *server, duration)
			if err != nil {
				listenCh <- pb.ServerResult{Code: 1, Error: err.Error(), Server: server}
			} else {
				listenCh <- pb.ServerResult{Server: server}
			}
		}(listenServer)
	}

	pingCh := make(chan pb.ServerResult, len(req.Ping))
	for _, pingServer := range req.Ping {
		go func(server *pb.Addr) {
			// retry a few times because other agents' servers may still be starting up
			const attempts = 4
			err := utils.Retry(duration/attempts, attempts, func() error {
				return trace.Wrap(ping(*server, duration/attempts))
			})
			if err != nil {
				pingCh <- pb.ServerResult{Code: 1, Error: err.Error(), Server: server}
			} else {
				pingCh <- pb.ServerResult{Server: server}
			}
		}(pingServer)
	}

	// allow some extra time for collecting results
	timeout := time.After(2 * duration)

	dumpConfig := spew.ConfigState{DisableCapacities: true, DisablePointerAddresses: true, Indent: " "}
	response := &pb.CheckPortsResponse{}
	for range req.Listen {
		select {
		case listen := <-listenCh:
			response.Listen = append(response.Listen, &listen)
		case <-timeout:
			diff := computeDiff(req.Listen, response.Listen)
			r.Errorf("missing listen results from: %v", dumpConfig.Sdump(diff))
			return nil, trace.LimitExceeded(
				"timeout spinning up listen servers: %v", dumpConfig.Sdump(diff))
		}
	}

	for range req.Ping {
		select {
		case ping := <-pingCh:
			response.Ping = append(response.Ping, &ping)
		case <-timeout:
			diff := computeDiff(req.Ping, response.Ping)
			log.Errorf("missing ping results from: %v", dumpConfig.Sdump(diff))
			return nil, trace.LimitExceeded(
				"timeout pinging servers: %v", dumpConfig.Sdump(diff))
		}
	}

	return response, nil
}

// Validate validatest this node against the requirements
// from a manifest.
func (_ *Server) Validate(ctx context.Context, req *pb.ValidateRequest) (resp *pb.ValidateResponse, err error) {
	var manifest schema.Manifest
	if err := json.Unmarshal(req.Manifest, &manifest); err != nil {
		return nil, trace.Wrap(err)
	}

	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err, "failed to determine state directory")
	}

	profile, err := manifest.NodeProfiles.ByName(req.Profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	v := checks.ManifestValidator{
		Manifest: manifest,
		Profile:  *profile,
		StateDir: stateDir,
	}
	var failedProbes []*agentpb.Probe
	if req.FullRequirements {
		v.Docker = &storage.DockerConfig{
			StorageDriver: req.Docker.StorageDriver,
		}
		failedProbes, err = v.Validate(ctx)
		failedProbes = append(failedProbes, checks.RunBasicChecks(ctx, req.Options)...)
	} else {
		failedProbes, err = validateManifest(ctx, v)
		failedProbes = append(failedProbes, runLocalChecks(ctx)...)
	}

	return &pb.ValidateResponse{Failed: failedProbes}, trace.Wrap(err)
}

func listen(ctx context.Context, server pb.Addr, duration time.Duration) error {
	if server.Network == "tcp" {
		return trace.Wrap(listenTCP(ctx, server.Addr, duration))
	}
	return trace.Wrap(listenUDP(ctx, server.Addr, duration))
}

func ping(server pb.Addr, duration time.Duration) error {
	if server.Network == "tcp" {
		return trace.Wrap(pingTCP(server.Addr, duration))
	}
	return trace.Wrap(pingUDP(server.Addr, duration))
}

func computeDiff(expected []*pb.Addr, actual []*pb.ServerResult) (diff []*pb.Addr) {
	total := map[string]*pb.Addr{}
	for _, server := range expected {
		total[server.Address()] = server
	}

	for _, result := range actual {
		repr := result.Server.Address()
		if _, exists := total[repr]; exists {
			delete(total, repr)
		}
	}

	for _, server := range total {
		diff = append(diff, server)
	}
	return diff
}

// validateManifest validates the node against the specified profile.
// The profile requirements are skipped as these are only meaningful during
// installation.
func validateManifest(ctx context.Context, v checks.ManifestValidator) (failedProbes []*agentpb.Probe, err error) {
	var errors []error
	failed, err := schema.ValidateDocker(ctx, v.Manifest.Docker(v.Profile), v.StateDir)
	if err != nil {
		errors = append(errors, trace.Wrap(err,
			"error validating docker requirements, see syslog for details"))
	}
	failedProbes = append(failedProbes, failed...)

	failedProbes = append(failedProbes, schema.ValidateKubelet(ctx, v.Profile, v.Manifest)...)
	return failedProbes, trace.NewAggregate(errors...)
}

func runLocalChecks(ctx context.Context) (failed []*agentpb.Probe) {
	checks := monitoring.NewCompositeChecker("local",
		[]health.Checker{
			monitoring.NewIPForwardChecker(),
			monitoring.NewBridgeNetfilterChecker(),
			monitoring.NewMayDetachMountsChecker(),
			monitoring.DefaultBootConfigParams(),
		},
	)

	var reporter health.Probes
	checks.Check(ctx, &reporter)

	for _, probe := range reporter {
		if probe.Status == agentpb.Probe_Failed {
			failed = append(failed, probe)
		}
	}

	return failed
}
