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
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/rpc/client"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

// NewAgentGroup creates a new agent group from the specified list of peers.
// Call Start on the resulting instance to start the health check loop
func NewAgentGroup(config AgentGroupConfig, from []Peer) (*AgentGroup, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// FIXME: arbitrary channel size
	watchCh := make(chan WatchEvent, 10)
	peersConfig := peersConfig{
		FieldLogger:       config.FieldLogger.WithField(trace.Component, "agent-group"),
		ReconnectStrategy: config.ReconnectStrategy,
		watchCh:           watchCh,
		checkTimeout:      config.HealthCheckTimeout,
	}
	peers, err := newPeers(from, peersConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ctx, cancel := context.WithCancel(context.TODO())
	group := &AgentGroup{
		AgentGroupConfig: config,
		watchCh:          watchCh,
		recvCh:           make(chan WatchEvent),
		peers:            peers,
		ctx:              ctx,
		cancel:           cancel,
	}
	return group, nil
}

// AgentGroupConfig defines agent group configuration
type AgentGroupConfig struct {
	log.FieldLogger
	// ReconnectStrategy configures the strategy for peer reconnects
	ReconnectStrategy
	// HealthCheckTimeout overrides timeout between health check attempts.
	// Defaults to defaults.AgentHealthCheckTimeout
	HealthCheckTimeout time.Duration
	// WatchCh is an optional channel that receives updates
	// when peers reconnect.
	WatchCh chan<- WatchEvent
}

// CheckAndSetDefaults validates this configuration object and sets defaults
func (r *AgentGroupConfig) CheckAndSetDefaults() error {
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "agent.group")
	}

	if r.HealthCheckTimeout == 0 {
		r.HealthCheckTimeout = defaults.AgentReconnectTimeout
	}

	return nil
}

// AgentGroup manages a list of remote agents.
//
// Group is initialized from an initial set of peers. As new peers appear,
// they can be added to the group with group.Add(peer).
// As part of its operation, the group will monitor health of the recorded set of peers
// and reestablish connections to those that failed the check.
type AgentGroup struct {
	AgentGroupConfig
	// watchCh is a channel that receives updates
	// when peers reconnect.
	watchCh chan WatchEvent
	recvCh  chan WatchEvent
	peers   *peers
	ctx     context.Context
	cancel  context.CancelFunc
}

// With returns a client for the peer specified with addr
func (r *AgentGroup) With(addr string) client.Client {
	if clt, exists := r.peers.getClient(addr); exists && clt != nil {
		return clt.Client()
	}
	return errorPeer{trace.NotFound("peer %v not found", addr)}
}

// WithContext returns a client for peer identified with addr.
// This is a blocking method that waits for a new client if
// there's a reconnect operation in progress.
// The specified context can be used to cancel the wait.
func (r *AgentGroup) WithContext(ctx context.Context, addr string) client.Client {
	if clt, exists := r.peers.getClient(addr); exists && clt != nil {
		return clt.Client()
	}

	for {
		select {
		case update := <-r.recvCh:
			if update.Peer.Addr() == addr {
				if update.Error != nil {
					return errorPeer{update.Error}
				}
				return update.Client.Client()
			}
		case <-r.ctx.Done():
			r.Warnf("Context closed: %v.", r.ctx.Err())
			return errorPeer{trace.NotFound("peer %v not found", addr)}
		case <-ctx.Done():
			r.Warnf("Context closed: %v.", ctx.Err())
			return errorPeer{trace.NotFound("peer %v not found", addr)}
		}
	}
}

// Add adds a new peer to the set of peers to control and monitor.
// The connection to the peer will automatically be established in
// background.
func (r *AgentGroup) Add(p Peer) {
	r.peers.add(peer{Peer: p})
}

// Remove removes the specified peer from the group
func (r *AgentGroup) Remove(ctx context.Context, p Peer) error {
	r.peers.delete(peer{Peer: p})
	return nil
}

// Shutdown requests agents to shut down
func (r *AgentGroup) Shutdown(ctx context.Context, req *pb.ShutdownRequest) error {
	err := r.peers.iterate(func(p peer) error {
		return trace.Wrap(p.Shutdown(ctx, req))
	})
	return trace.Wrap(err)
}

// Abort requests agents to abort the operation and uninstall
func (r *AgentGroup) Abort(ctx context.Context) error {
	return r.peers.iterate(func(p peer) error {
		r.WithField("peer", p).Info("Abort peer.")
		return p.Abort(ctx)
	})
}

// Start starts this group's internal goroutines
func (r *AgentGroup) Start() {
	go r.updateLoop()
	r.peers.start()
}

// Close closes all remote agent clients
func (r *AgentGroup) Close(ctx context.Context) error {
	r.cancel()
	r.peers.close(ctx)
	err := r.peers.iterate(func(p peer) error {
		return trace.Wrap(p.Close())
	})
	return trace.Wrap(err)
}

// String returns textual representation of this group
func (r AgentGroup) String() string {
	return fmt.Sprintf("group(%v)", r.peers.String())
}

// NumPeers returns the number of peers in this group
func (r *AgentGroup) NumPeers() int {
	return r.peers.len()
}

// WatchChan returns the channel that receives peer updates
func (r *AgentGroup) WatchChan() chan<- WatchEvent {
	return r.watchCh
}

// GetPeers returns the list of monitored peers
func (r *AgentGroup) GetPeers() []Peer {
	return r.peers.getPeers()
}

func (r *AgentGroup) updateLoop() {
	for {
		select {
		case update := <-r.watchCh:
			// Distribute the value between receivers.
			// Do not block if there's no receiver
			select {
			case r.WatchCh <- update:
			default:
				log.Infof("Dropped update notification for %v.", update)
			}
			select {
			case r.recvCh <- update:
			default:
			}
		case <-r.ctx.Done():
			return
		}
	}
}

func (r errorPeer) Command(ctx context.Context, log log.FieldLogger, stdout, stderr io.Writer, args ...string) error {
	return trace.Wrap(r.error)
}

func (r errorPeer) GravityCommand(ctx context.Context, log log.FieldLogger, stdout, stderr io.Writer, args ...string) error {
	return trace.Wrap(r.error)
}

func (r errorPeer) Validate(context.Context, *validationpb.ValidateRequest) ([]*agentpb.Probe, error) {
	return nil, trace.Wrap(r.error)
}

func (r errorPeer) GetSystemInfo(context.Context) (storage.System, error) {
	return nil, trace.Wrap(r.error)
}

func (r errorPeer) GetRuntimeConfig(context.Context) (*pb.RuntimeConfig, error) {
	return nil, trace.Wrap(r.error)
}

func (r errorPeer) GetCurrentTime(context.Context) (*time.Time, error) {
	return nil, trace.Wrap(r.error)
}

func (r errorPeer) CheckPorts(context.Context, *validationpb.CheckPortsRequest) (*validationpb.CheckPortsResponse, error) {
	return nil, trace.Wrap(r.error)
}

func (r errorPeer) CheckBandwidth(context.Context, *validationpb.CheckBandwidthRequest) (*validationpb.CheckBandwidthResponse, error) {
	return nil, trace.Wrap(r.error)
}

func (r errorPeer) CheckDisks(context.Context, *validationpb.CheckDisksRequest) (*validationpb.CheckDisksResponse, error) {
	return nil, trace.Wrap(r.error)
}

func (r errorPeer) Shutdown(context.Context, *pb.ShutdownRequest) error {
	return trace.Wrap(r.error)
}

func (r errorPeer) Abort(context.Context) error {
	return trace.Wrap(r.error)
}

func (r errorPeer) Close() error {
	return trace.Wrap(r.error)
}

// errorPeer represents a peer lookup failure.
// Implements client.Client
type errorPeer struct {
	error
}
