package ops

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// CheckServers executes a set of preflight tests on a set of servers
// as part of the install operation given with opKey.
// agentService is the access point to the agent cluster for running remote
// commands.
// manifest specifies the application manifest with requirements.
func CheckServers(ctx context.Context, opKey SiteOperationKey,
	infos checks.ServerInfos, servers []storage.Server, agentService AgentService,
	manifest schema.Manifest) error {
	nodes, err := mergeServers(infos, servers)
	if err != nil {
		return trace.Wrap(err)
	}
	remote := &remoteCommands{key: opKey, AgentService: agentService}
	requirements, err := requirementsFromManifest(manifest)
	if err != nil {
		return trace.Wrap(err)
	}
	c, err := checks.New(remote, nodes, manifest, requirements)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	c.TestBandwidth = true
	c.TestDockerDevice = true
	return trace.Wrap(c.Run(ctx))
}

// FormatValidationError formats validation error as a human-readable text
func FormatValidationError(err error) error {
	errors := []error{err}
	if agg, ok := trace.Unwrap(err).(trace.Aggregate); ok {
		errors = agg.Errors()
	}
	var buf bytes.Buffer
	for _, err := range errors {
		fmt.Fprint(&buf, "\n", err.Error())
	}
	return trace.BadParameter(buf.String())
}

// Exec executes an arbitrary command on the remote node specified with addr.
// The output is written into out
func (r *remoteCommands) Exec(ctx context.Context, addr string, args []string, out io.Writer) error {
	return trace.Wrap(r.AgentService.Exec(ctx, r.key, addr, args, out))
}

// CheckPorts validates the cluster port availability
func (r *remoteCommands) CheckPorts(ctx context.Context, req checks.PingPongGame) (checks.PingPongGameResults, error) {
	resp, err := r.AgentService.CheckPorts(ctx, r.key, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckBandwidth validates the cluster network bandwidth
func (r *remoteCommands) CheckBandwidth(ctx context.Context, req checks.PingPongGame) (checks.PingPongGameResults, error) {
	resp, err := r.AgentService.CheckBandwidth(ctx, r.key, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Validate validates the node given with addr against the specified manifest.
// Returns the list of failed test results.
func (r *remoteCommands) Validate(ctx context.Context, addr string,
	manifest schema.Manifest, profileName string) ([]*agentpb.Probe, error) {
	failed, err := r.AgentService.Validate(ctx, r.key, addr, manifest, profileName)
	return failed, trace.Wrap(err)
}

// remoteCommands allows to execute remote commands and validate remote nodes.
// Implements checks.Remote
type remoteCommands struct {
	AgentService
	key SiteOperationKey
}

func requirementsFromManifest(manifest schema.Manifest) (map[string]checks.Requirements, error) {
	result := make(map[string]checks.Requirements)
	for i, profile := range manifest.NodeProfiles {
		tcp, udp, err := checks.PortsForProfile(profile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req := checks.Requirements{
			CPU:     &manifest.NodeProfiles[i].Requirements.CPU,
			RAM:     &manifest.NodeProfiles[i].Requirements.RAM,
			OS:      profile.Requirements.OS,
			Volumes: profile.Requirements.Volumes,
			Network: checks.Network{
				MinTransferRate: profile.Requirements.Network.MinTransferRate,
				Ports:           checks.Ports{TCP: tcp, UDP: udp},
			},
		}
		result[profile.Name] = req
	}
	return result, nil
}

func mergeServers(infos checks.ServerInfos, servers []storage.Server) (result []checks.Server, err error) {
	result = make([]checks.Server, 0, len(servers))
	for _, server := range servers {
		info, err := infos.FindByIP(server.AdvertiseIP)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, checks.Server{server, *info})
	}
	return result, nil
}
