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

package cluster

import (
	"io"
	"sort"
	"time"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// GetPeer returns a new client peer for Object storage
type GetPeer func(peer storage.Peer) (blob.Objects, error)

// Config is a cluster BLOB storage config
type Config struct {
	// Local is a local BLOB storage managed by this peer
	Local blob.Objects
	// Backend is a discovery and metadata backend
	Backend storage.Backend
	// GetPeer returns new peer client based on ID
	GetPeer GetPeer
	// WriteFactor defines how many ack peer writes should be acknowledged
	// before write is considered successfull
	WriteFactor int
	// ID is a peer local ID
	ID string
	// AdvertiseAddr is peer advertise address
	AdvertiseAddr string
	// Clock is clock interface, used in tests
	Clock clockwork.Clock
	// HeartbeatPeriod defines the period between heartbeats
	HeartbeatPeriod time.Duration
	// MissedHeartbeats is how mahy heartbeats the peer
	// should miss before we consider it closed
	MissedHeartbeats int
	// GracePeriod is a period for GC not to delete undetected files
	// to prevent accidental deletion. Defaults to 1 hour
	GracePeriod time.Duration
}

func (c *Config) checkAndSetDefaults() error {
	if c.WriteFactor < 1 {
		c.WriteFactor = defaults.WriteFactor
	}
	if c.Local == nil {
		return trace.BadParameter("missing parameter Local")
	}
	if c.Backend == nil {
		return trace.BadParameter("missing parameter Backend")
	}
	if c.GetPeer == nil && c.WriteFactor != 1 {
		return trace.BadParameter("missing parameter GetPeer")
	}
	if c.ID == "" {
		return trace.BadParameter("missing parameter ID")
	}
	if c.AdvertiseAddr == "" {
		return trace.BadParameter("missing parameter AdvertiseAddr")
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	if c.HeartbeatPeriod == 0 {
		c.HeartbeatPeriod = defaults.HeartbeatPeriod
	}
	if c.MissedHeartbeats == 0 {
		c.MissedHeartbeats = defaults.MissedHeartbeats
	}
	if c.GracePeriod == 0 {
		c.GracePeriod = defaults.GracePeriod
	}
	return nil
}

// New returns cluster BLOB storage that takes care of replication
// of BLOBs across cluster of nodes. It is designed for small clusters O(10)
// and small amount of objects managed O(100)
func New(config Config) (*Cluster, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	close, cancelFn := context.WithCancel(context.TODO())

	entry := log.WithFields(log.Fields{
		trace.Component: constants.ComponentBLOB,
		"id":            config.ID,
		"addr":          config.AdvertiseAddr,
	})

	return &Cluster{
		Config:   config,
		close:    close,
		cancelFn: cancelFn,
		Entry:    entry,
	}, nil
}

// Start starts internal processes
func (c *Cluster) Start() {
	go c.periodically("heartbeat", c.heartbeat)
	go c.periodically("purgeDeleted", c.purgeDeletedObjects)
	go c.periodically("fetchNew", c.fetchNewObjects)
}

type Cluster struct {
	*log.Entry
	Config
	close    context.Context
	cancelFn context.CancelFunc
}

func (c *Cluster) Close() error {
	c.cancelFn()
	return nil
}

// GetBLOBs returns a list of BLOBs in the storage
func (c *Cluster) GetBLOBs() ([]string, error) {
	return c.Backend.GetObjects()
}

func (c *Cluster) GetBLOBEnvelope(hash string) (*blob.Envelope, error) {
	return c.Local.GetBLOBEnvelope(hash)
}

type resultTuple struct {
	envelope *blob.Envelope
	error    error
	peer     storage.Peer
}

func (c *Cluster) periodically(name string, fn func() error) {
	ticker := time.NewTicker(defaults.HeartbeatPeriod)
	defer ticker.Stop()
	if err := fn(); err != nil {
		c.Errorf("Periodic %v failed: %v.", name, err)
	}
	for {
		select {
		case <-c.close.Done():
			c.Info("Returning, cluster is closing.")
			return
		case <-ticker.C:
			if err := fn(); err != nil {
				c.Errorf("Periodic %v failed: %v.", name, err)
			}
		}
	}
}

func (c *Cluster) localPeer() storage.Peer {
	return storage.Peer{
		ID:            c.ID,
		AdvertiseAddr: c.AdvertiseAddr,
		LastHeartbeat: c.Clock.Now().UTC(),
	}
}

func (c *Cluster) heartbeat() error {
	err := c.Backend.UpsertPeer(c.localPeer())
	return trace.Wrap(err)
}

func (c *Cluster) purgeDeletedObjects() error {
	hashes, err := c.Local.GetBLOBs()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, hash := range hashes {
		_, err := c.Backend.GetObjectPeers(hash)
		if err == nil {
			continue
		}
		if !trace.IsNotFound(err) {
			return nil
		}
		envelope, err := c.Local.GetBLOBEnvelope(hash)
		if err != nil {
			return trace.Wrap(err)
		}
		if envelope.Modified.IsZero() {
			log.Warningf("%v has zero modified time!", envelope.Modified)
			continue
		}
		// prevent accidental deletion by using grace period -
		// we keep fresh objects on file storage just in case
		diff := c.Clock.Now().UTC().Sub(envelope.Modified.UTC())
		if diff < c.GracePeriod {
			continue
		} else {
			log.Infof("%v has exceeded grace period(%v) - current diff: %v, going to delete", hash, c.GracePeriod, diff)
		}
		err = c.Local.DeleteBLOB(hash)
		if err != nil {
			c.Errorf("Failed to delete object %v, error: %v.", hash, trace.DebugReport(err))
		}
	}
	return nil
}

func (c *Cluster) fetchNewObjects() error {
	objects, err := c.Backend.GetObjects()
	if err != nil {
		return trace.Wrap(err)
	}
	var missingObjects []string
	for _, hash := range objects {
		f, err := c.Local.OpenBLOB(hash)
		if err == nil {
			f.Close()
		} else {
			missingObjects = append(missingObjects, hash)
		}
	}
	for _, hash := range missingObjects {
		c.Infof("Found missing object %v.", hash)
		err = c.fetchObject(hash)
		if err != nil {
			c.Warningf("Failed to fetch object(%v) %v.", hash, trace.DebugReport(err))
			return trace.Wrap(err)
		}
	}
	return nil
}

func (c *Cluster) fetchObject(hash string) error {
	peerIDs, err := c.Backend.GetObjectPeers(hash)
	if err != nil {
		return trace.Wrap(err)
	}
	peers, err := c.getPeers(peerIDs)
	if err != nil {
		return trace.Wrap(err)
	}
	peers = c.withoutSelf(peers)
	if len(peers) == 0 {
		return trace.NotFound("no active remote peers found for %v", hash)
	}
	var errors []error
	for _, p := range peers {
		objects, err := c.getObjects(p)
		if err != nil {
			c.Errorf("Failure to fetch %v from %v: %v.", hash, p, trace.DebugReport(err))
			errors = append(errors, err)
			continue
		}
		f, err := objects.OpenBLOB(hash)
		if err != nil {
			c.Errorf("Failure to fetch %v from %v: %v.", hash, p, trace.DebugReport(err))
			errors = append(errors, err)
			continue
		}
		defer f.Close()
		envelope, err := c.Local.WriteBLOB(f)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		c.Infof("Successfully fetched %v from %v.", envelope, p)
		err = c.Backend.UpsertObjectPeers(hash, []string{c.ID}, 0)
		if err != nil {
			c.Errorf("Failed to upsert %v object peers: %v.", hash, err)
			return trace.Wrap(err)
		}
		return nil
	}
	return trace.NewAggregate(errors...)
}

// WriteBLOB writes object to the storage, returns object envelope
func (c *Cluster) WriteBLOB(data io.Reader) (*blob.Envelope, error) {
	// get peers
	peers, err := c.getPeers(nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(peers) < c.WriteFactor {
		return nil, trace.ConnectionProblem(
			nil, "not enough peers online %#v, need %v", peers, c.WriteFactor)
	}

	envelope, err := c.Local.WriteBLOB(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	successWrites := []string{c.ID}
	if len(successWrites) >= c.WriteFactor {
		c.Debugf("Got enough success writes for %v %v.", envelope.SHA512, successWrites)
		err := c.Backend.UpsertObjectPeers(envelope.SHA512, successWrites, 0)
		if err != nil {
			return nil, trace.Wrap(err, "failed to write object metadata %v", err)
		}
		return envelope, nil
	}
	f, err := c.Local.OpenBLOB(envelope.SHA512)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer f.Close()

	var errors []error
	for _, p := range peers {
		if p.ID == c.ID {
			continue
		}
		_, err := f.Seek(0, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		peerClient, err := c.GetPeer(p)
		if err != nil {
			c.Infof("%v returned error: %v", p, err)
			errors = append(errors, err)
			continue
		}
		_, err = peerClient.WriteBLOB(f)
		if err != nil {
			c.Infof("%v returned error: %v", p, err)
			errors = append(errors, err)
			continue
		}
		successWrites = append(successWrites, p.ID)
		if len(successWrites) >= c.WriteFactor {
			c.Debugf("Got enough success writes for %v %v.", envelope.SHA512, successWrites)
			err := c.Backend.UpsertObjectPeers(envelope.SHA512, successWrites, 0)
			if err != nil {
				return nil, trace.Wrap(err, "failed to write object metadata %v", err)
			}
			return envelope, nil
		}
	}

	return nil, trace.Wrap(trace.NewAggregate(errors...), "not enough successfull writes")
}

func (c *Cluster) getObjects(p storage.Peer) (blob.Objects, error) {
	if p.ID == c.ID {
		return c.Local, nil
	}
	return c.GetPeer(p)
}

// OpenBLOB opens file identified by hash and returns reader
func (c *Cluster) OpenBLOB(hash string) (blob.ReadSeekCloser, error) {
	ids, err := c.Backend.GetObjectPeers(hash)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	peers, err := c.getPeers(ids)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(peers) == 0 {
		return nil, trace.ConnectionProblem(nil, "no peers found")
	}
	var reader blob.ReadSeekCloser
	for _, p := range peers {
		objects, err := c.getObjects(p)
		if err != nil {
			c.Warnf("%v returned %v", p, err)
			continue
		}
		reader, err = objects.OpenBLOB(hash)
		if err != nil {
			c.Warnf("%v returned %v", p, err)
			continue
		}
		return reader, nil
	}
	return nil, trace.NotFound("failed to find any peer with hash(%v)", hash)
}

// DeleteBLOB deletes BLOB from the storage
func (c *Cluster) DeleteBLOB(hash string) error {
	return trace.Wrap(c.Backend.DeleteObject(hash))
}

// matchPeer finds matching id in the filter
// if the filter is empty, we consider all ids to match the filter
func matchPeer(ids []string, id string) bool {
	if len(ids) == 0 {
		return true
	}
	return utils.StringInSlice(ids, id)
}

func (c *Cluster) withoutSelf(in []storage.Peer) []storage.Peer {
	out := make([]storage.Peer, 0, len(in))
	for i := range in {
		if in[i].ID == c.ID {
			continue
		}
		out = append(out, in[i])
	}
	return out
}

func (c *Cluster) getPeers(ids []string) ([]storage.Peer, error) {
	in, err := c.Backend.GetPeers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(in) == 0 {
		in = []storage.Peer{c.localPeer()}
	}
	missedWindow := time.Duration(c.MissedHeartbeats) * c.HeartbeatPeriod
	out := make([]storage.Peer, 0, len(in))
	for _, p := range in {
		// Skip non-local peer
		if p.ID != c.ID {
			// if it is not part of the specified ID set
			if !matchPeer(ids, p.ID) {
				continue
			}
			// if it's last heartbeat is older than the acceptance time frame
			if c.Clock.Now().UTC().Sub(p.LastHeartbeat) > missedWindow {
				c.Infof("Excluding %v, missed heartbeat window %v, last heartbeat: %v.", p.ID, missedWindow, p.LastHeartbeat)
				continue
			}
		}
		out = append(out, p)
	}
	sort.Sort(&peerSorter{P: out, ID: c.ID})
	return out, nil
}

// peerSorter makes sure local peer always goes first
// and guarantees deterministic peer order
type peerSorter struct {
	P  []storage.Peer
	ID string
}

func (s *peerSorter) Len() int {
	return len(s.P)
}

func (s *peerSorter) Swap(i, j int) {
	s.P[i], s.P[j] = s.P[j], s.P[i]
}

func (s *peerSorter) Less(i, j int) bool {
	// local goes first
	if s.P[i].ID == s.ID {
		return true
	}
	if s.P[j].ID == s.ID {
		return false
	}
	return s.P[i].ID < s.P[j].ID
}
