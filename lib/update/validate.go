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

package update

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"time"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/rpc"
	rpcclient "github.com/gravitational/gravity/lib/rpc/client"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
	"github.com/xtgo/set"
)

func validate(ctx context.Context, remote fsm.AgentRepository, servers []storage.Server, old, new schema.Manifest) error {
	profiles := make(map[string]string, len(servers))
	nodes := make([]checks.Server, 0, len(servers))
	for _, server := range servers {
		connectCtx, cancel := context.WithTimeout(ctx, defaults.AgentConnectTimeout)
		clt, err := remote.GetClient(connectCtx, server.AdvertiseIP)
		cancel()
		if err != nil {
			return trace.Wrap(err, "failed to connect to agent.\n"+
				"Make sure the node has an agent running by "+
				"issuing `gravity agent deploy` from the upgrade node")
		}

		info, err := checks.GetServerInfo(ctx, clt)
		if err != nil {
			return trace.Wrap(err)
		}
		nodes = append(nodes, checks.Server{server, *info})
		profiles[server.AdvertiseIP] = server.Role
	}

	requirements, err := requirementsFromManifests(old, new, profiles)
	if err != nil {
		return trace.Wrap(err)
	}
	remoteExec := &remoteCommands{remote, profiles}
	c, err := checks.New(remoteExec, nodes, new, requirements)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.Run(ctx))
}

// Exec executes an arbitrary command on the remote node specified with addr.
// The output is written into out
func (r *remoteCommands) Exec(ctx context.Context, addr string, command []string, out io.Writer) error {
	clt, err := r.remote.GetClient(ctx, addr)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(clt.Command(ctx, log.StandardLogger(), out, command...))
}

// CheckPorts validates the cluster port availability
func (r *remoteCommands) CheckPorts(ctx context.Context, req checks.PingPongGame) (checks.PingPongGameResults, error) {
	resp, err := pingPong(ctx, r.remote, req, ports)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckBandwidth validates the cluster network bandwidth
func (r *remoteCommands) CheckBandwidth(ctx context.Context, req checks.PingPongGame) (checks.PingPongGameResults, error) {
	resp, err := pingPong(ctx, r.remote, req, bandwidth)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// Validate validates the node given with addr against the specified manifest.
// Returns the list of failed test results.
func (r *remoteCommands) Validate(ctx context.Context, addr string, manifest schema.Manifest, profileName string) ([]*agentpb.Probe, error) {
	clt, err := r.remote.GetClient(ctx, addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := validationpb.ValidateRequest{
		Manifest: bytes,
		Profile:  profileName,
	}
	failed, err := clt.Validate(ctx, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return failed, nil
}

// remoteCommands allows to execute remote commands and validate remote nodes.
// Implements checks.Remote
type remoteCommands struct {
	remote fsm.AgentRepository
	// profiles maps server address to its profile
	profiles map[string]string
}

func pingPong(ctx context.Context, remote fsm.AgentRepository, game checks.PingPongGame, fn pingpongHandler) (checks.PingPongGameResults, error) {
	resultsCh := make(chan pingpongResult)
	for addr, req := range game {
		clt, err := remote.GetClient(ctx, addr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		go fn(ctx, rpc.AgentAddr(addr), clt, req, resultsCh)
	}

	results := make(checks.PingPongGameResults, len(game))
	for _, req := range game {
		select {
		case result := <-resultsCh:
			if result.err != nil {
				return nil, trace.Wrap(result.err)
			}
			results[result.addr] = *result.resp
		case <-time.After(2 * req.Duration):
			return nil, trace.LimitExceeded("timeout waiting for servers")
		}
	}
	return results, nil
}

func ports(ctx context.Context, addr string, clt rpcclient.Client, req checks.PingPongRequest, resultsCh chan<- pingpongResult) {
	resp, err := clt.CheckPorts(ctx, req.PortsProto())
	if err != nil {
		resultsCh <- pingpongResult{addr: addr, err: err}
		return
	}
	resultsCh <- pingpongResult{addr: addr, resp: checks.ResultFromPortsProto(resp, nil)}
}

func bandwidth(ctx context.Context, addr string, clt rpcclient.Client, req checks.PingPongRequest, resultsCh chan<- pingpongResult) {
	resp, err := clt.CheckBandwidth(ctx, req.BandwidthProto())
	if err != nil {
		resultsCh <- pingpongResult{addr: addr, err: err}
		return
	}
	resultsCh <- pingpongResult{addr: addr, resp: checks.ResultFromBandwidthProto(resp, nil)}
}

type pingpongHandler func(ctx context.Context, addr string, clt rpcclient.Client,
	req checks.PingPongRequest, resultsCh chan<- pingpongResult)

type pingpongResult struct {
	addr string
	resp *checks.PingPongResult
	err  error
}

// requirementsFromManifests generates check requirements as a difference between
// two manifests - old and new.
func requirementsFromManifests(old, new schema.Manifest, profiles map[string]string) (map[string]checks.Requirements, error) {
	result := make(map[string]checks.Requirements)
	for _, profileName := range profiles {
		oldProfile, err := old.NodeProfiles.ByName(profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		newProfile, err := new.NodeProfiles.ByName(profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Compute port requirements for this profile
		tcp, udp, err := diffPorts(old, new, profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req := checks.Requirements{
			OS:      newProfile.Requirements.OS,
			Volumes: diffVolumes(oldProfile.Requirements.Volumes, newProfile.Requirements.Volumes),
			Network: checks.Network{
				Ports: checks.Ports{TCP: tcp, UDP: udp},
			},
		}
		result[profileName] = req
	}

	log.Debugf("Update requirements: %+v.", result)

	return result, nil
}

// diffPorts returns a difference of port requirements between old and new
// for the specified profile
func diffPorts(old, new schema.Manifest, profileName string) (tcp, udp []int, err error) {
	profile, err := old.NodeProfiles.ByName(profileName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	oldTCP, oldUDP, err := checks.PortsForProfile(*profile)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	newProfile, err := new.NodeProfiles.ByName(profileName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	tcp, udp, err = checks.PortsForProfile(*newProfile)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	commonTCP := append(oldTCP, tcp...)
	commonUDP := append(oldUDP, udp...)

	// Do not take ports that are only present in the old
	// manifest into account
	sizeTCP := set.Inter(sort.IntSlice(commonTCP), len(oldTCP))
	sizeUDP := set.Inter(sort.IntSlice(commonUDP), len(oldUDP))
	commonTCP = commonTCP[:sizeTCP]
	commonUDP = commonUDP[:sizeUDP]

	// Compute the difference
	tcp = append(commonTCP, tcp...)
	udp = append(commonUDP, udp...)
	sizeTCP = set.SymDiff(sort.IntSlice(tcp), len(commonTCP))
	sizeUDP = set.SymDiff(sort.IntSlice(udp), len(commonUDP))
	return tcp[:sizeTCP], udp[:sizeUDP], nil
}

func diffVolumes(old, new []schema.Volume) []schema.Volume {
	volumes := append(old, new...)
	size := set.Inter(volumesByPath(volumes), len(old))
	// Compute common volumes
	common := volumes[:size]
	// Compute volumes only present in new
	volumes = append(common, new...)
	size = set.SymDiff(volumesByPath(volumes), len(common))
	return volumes[:size]
}

type volumesByPath []schema.Volume

func (r volumesByPath) Len() int           { return len(r) }
func (r volumesByPath) Less(i, j int) bool { return r[i].Path < r[j].Path }
func (r volumesByPath) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
