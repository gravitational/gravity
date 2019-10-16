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
	"strings"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// newPeers returns a new instance of peers.
func newPeers(from []Peer, config peersConfig) (*peers, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	ps := make(map[string]*peer)
	for _, p := range from {
		ps[p.Addr()] = &peer{Peer: p, doneCh: make(chan struct{})}
	}

	ctx, cancel := context.WithCancel(context.TODO())
	r := &peers{
		FieldLogger:       config.FieldLogger,
		ReconnectStrategy: config.ReconnectStrategy,
		ctx:               ctx,
		cancel:            cancel,
		peers:             ps,
		checkTimeout:      config.checkTimeout,
		watchCh:           config.watchCh,
		updateCh:          make(chan peerUpdate, len(from)),
	}
	return r, nil
}

// String returns a textual representation of this set of peers
func (r *peers) String() string {
	r.RLock()
	defer r.RUnlock()
	peers := make([]string, 0, len(r.peers))
	for _, peer := range r.peers {
		peers = append(peers, peer.String())
	}
	return strings.Join(peers, ",")
}

func (r *peers) start() {
	r.RLock()
	defer r.RUnlock()
	for _, p := range r.peers {
		reconnectCh := make(chan chan clientUpdate)
		go r.monitorPeer(p.Peer, p.Client, reconnectCh, p.doneCh)
		go r.reconnectPeer(p.Peer, reconnectCh, p.doneCh)
	}
	go r.monitorPeers()
}

// validateConnection makes sure connection to all peers can be established
func (r *peers) validateConnection(ctx context.Context) error {
	r.RLock()
	defer r.RUnlock()
	var errors []error
	for _, p := range r.peers {
		errors = append(errors, r.tryPeer(ctx, p))
	}
	return trace.NewAggregate(errors...)
}

// tryPeer tests connection to the provided peer
func (r *peers) tryPeer(ctx context.Context, peer *peer) error {
	client, err := peer.Reconnect(ctx)
	if err != nil {
		r.WithFields(log.Fields{
			log.ErrorKey: err,
			"peer":       peer,
		}).Warn("Failed to connect.")
		return trace.Wrap(err, "RPC agent could not connect to %v", peer.Addr())
	}
	if err := client.Close(); err != nil {
		r.WithFields(log.Fields{
			log.ErrorKey: err,
			"peer":       peer,
		}).Warn("Failed to close client.")
	}
	return nil
}

func (r *peers) monitorPeers() {
	log := r.WithField("health.checker", r.String())
	log.Info("Monitoring peers.")
	defer log.Info("Health checker loop closing.")
	for {
		select {
		case peerUpdate := <-r.updateCh:
			if peerUpdate.error == nil {
				prevClient := r.update(peerUpdate.peer)
				if prevClient != nil {
					prevClient.Close()
				}
			} else {
				r.delete(peerUpdate.peer)
			}
			event := WatchEvent{
				Peer:   peerUpdate.Peer,
				Client: peerUpdate.Client,
				Error:  peerUpdate.error,
			}
			select {
			case r.watchCh <- event:
				// Notify that peer has reconnected or has been removed
			default:
				log.Infof("Dropped update notification for %v.", event)
			}
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *peers) monitorPeer(p Peer, clt Client, reconnectCh chan<- chan clientUpdate, doneCh chan struct{}) {
	log := r.WithField("monitored", p.Addr())
	log.Info("Monitoring.")
	defer log.Info("Monitoring loop closing.")
	ticker := time.NewTicker(r.checkTimeout)
	defer ticker.Stop()
	respCh := make(chan clientUpdate)
	var err error
	select {
	case <-r.ctx.Done():
		return
	case <-doneCh:
		return
	default:
		clt, err = r.checkPeer(p, clt, reconnectCh, respCh, doneCh)
		if err != nil {
			log.WithError(err).Warn("Failed to reconnect.")
			return
		}
	}
	for {
		select {
		case <-ticker.C:
			clt, err = r.checkPeer(p, clt, reconnectCh, respCh, doneCh)
			if err != nil {
				log.WithError(err).Warn("Failed to reconnect.")
				return
			}
		case <-doneCh:
			return
		case <-r.ctx.Done():
			return
		}
	}
}

// checkPeer verifies that the specified peer p is healthy and reconnects if it detects
// a disconnect.
// Returns the client to the peer or error if reconnecting failed.
func (r *peers) checkPeer(p Peer, clt Client, reconnectCh chan<- chan clientUpdate, respCh chan clientUpdate, doneCh chan struct{}) (Client, error) {
	log := r.WithField("checked", p)
	if clt != nil {
		resp, err := clt.Check(r.ctx, &healthpb.HealthCheckRequest{})
		if err == nil && isPeerHealthy(*resp) {
			return clt, nil
		}
		log.Warnf("Failed health check: %+v (%v).", resp, err)
	}
	select {
	case reconnectCh <- respCh:
		select {
		case resp := <-respCh:
			clt = resp.Client
			// wait for update
			select {
			// make sure to preserve doneCh that can be used to cancel
			// goroutines monitoring this peer
			case r.updateCh <- peerUpdate{peer{Peer: p, Client: clt, doneCh: doneCh}, resp.error}:
				return clt, trace.Wrap(resp.error)
			case <-doneCh:
				return nil, nil
			case <-r.ctx.Done():
				return nil, nil
			}
		case <-doneCh:
			return nil, nil
		case <-r.ctx.Done():
			return nil, nil
		}
	case <-doneCh:
		return nil, nil
	case <-r.ctx.Done():
		return nil, nil
	}
}

func (r *peers) reconnectPeer(p Peer, reqCh <-chan chan clientUpdate, doneCh chan struct{}) {
	log := r.WithField("reconnected", p)
	log.Info("Reconnecting")
	defer log.Info("Reconnect loop closing.")
	for {
		select {
		case respCh := <-reqCh:
			var clt Client
			err := utils.RetryWithInterval(r.ctx, r.Backoff(), func() (err error) {
				clt, err = p.Reconnect(r.ctx)
				if err != nil {
					return r.ShouldReconnect(err)
				}
				return nil
			})
			select {
			case respCh <- clientUpdate{clt, err}:
				if err == nil {
					log.Info("Peer reconnected.")
				}
			case <-r.ctx.Done():
				return
			}
		case <-doneCh:
			return
		case <-r.ctx.Done():
			return
		}
	}
}

// getClient returns a Client for the peer specified with addr
func (r *peers) getClient(addr string) (clt Client, exists bool) {
	var peer *peer
	r.RLock()
	defer r.RUnlock()
	if peer, exists = r.peers[addr]; !exists {
		return nil, false
	}
	return peer.Client, true
}

func (r *peers) add(p peer) {
	r.Lock()
	if _, exists := r.peers[p.Addr()]; exists {
		r.Unlock()
		return
	}
	doneCh := make(chan struct{})
	r.peers[p.Addr()] = &peer{Peer: p.Peer, doneCh: doneCh}
	r.Unlock()

	reconnectCh := make(chan chan clientUpdate)
	go r.monitorPeer(p.Peer, nil, reconnectCh, doneCh)
	go r.reconnectPeer(p.Peer, reconnectCh, doneCh)
}

func (r *peers) delete(p peer) {
	r.Lock()
	peer := r.peers[p.Addr()]
	if peer != nil && peer.doneCh != nil {
		close(peer.doneCh)
	}
	delete(r.peers, p.Addr())
	r.Unlock()
}

func (r *peers) update(p peer) Client {
	r.Lock()
	defer r.Unlock()
	clt := r.peers[p.Addr()].Client
	r.peers[p.Addr()] = &peer{Peer: p.Peer, Client: p.Client, doneCh: p.doneCh}
	return clt
}

// iterate executes the specified handler on each peer with a read lock held.
func (r *peers) iterate(handler func(peer) error) error {
	var errors []error
	r.RLock()
	defer r.RUnlock()
	for _, peer := range r.peers {
		if err := handler(*peer); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (r *peers) close(ctx context.Context) error {
	r.cancel()
	var errors []error
	for _, peer := range r.getPeers() {
		errors = append(errors, peer.Disconnect(ctx))
	}
	return trace.NewAggregate(errors...)
}

func (r *peers) len() int {
	r.RLock()
	defer r.RUnlock()
	return len(r.peers)
}

func (r *peers) getPeers() (peers []Peer) {
	r.RLock()
	defer r.RUnlock()
	for _, p := range r.peers {
		peers = append(peers, p.Peer)
	}
	return peers
}

type peers struct {
	log.FieldLogger
	ReconnectStrategy
	ctx          context.Context
	cancel       context.CancelFunc
	checkTimeout time.Duration
	// updateCh receives peer updates
	updateCh chan peerUpdate
	// watchCh is an optional channel that receives updates
	// when peers reconnect
	watchCh chan<- WatchEvent
	sync.RWMutex
	peers map[string]*peer
}

func (r *peersConfig) checkAndSetDefaults() error {
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithFields(log.Fields{
			trace.Component: "peers",
		})
	}

	if r.Backoff == nil {
		r.Backoff = func() backoff.BackOff { return utils.NewUnlimitedExponentialBackOff() }
	}

	if r.ShouldReconnect == nil {
		// Pass-through by default
		r.ShouldReconnect = func(err error) error {
			switch err := err.(type) {
			case *backoff.PermanentError:
				return err
			default:
				return trace.Retry(err, "failed to reconnect")
			}
		}
	}

	if r.checkTimeout == 0 {
		r.checkTimeout = defaults.AgentHealthCheckTimeout
	}

	return nil
}

type peersConfig struct {
	log.FieldLogger
	watchCh      chan<- WatchEvent
	checkTimeout time.Duration
	ReconnectStrategy
}

func isPeerHealthy(resp healthpb.HealthCheckResponse) bool {
	return resp.Status == healthpb.HealthCheckResponse_SERVING
}

type clientUpdate struct {
	Client
	error
}

type peerUpdate struct {
	peer
	error
}
