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
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// GetPeer returns a new client peer for Object storage
type GetPeer func(peer storage.Peer) (blob.Objects, error)

// Config is a cluster BLOB storage configuration
type Config struct {
	// Local is a local BLOB storage managed by this peer
	Local blob.Objects
	// Backend is a discovery and metadata backend
	Backend storage.Backend
	// GetPeer returns new peer client based on ID
	GetPeer GetPeer
	// WriteFactor defines how many peer writes should be acknowledged
	// before write is considered successful
	WriteFactor int
	// ID is a peer local ID
	ID string
	// AdvertiseAddr is peer advertise address
	AdvertiseAddr string
	// Clock is clock interface, used in tests
	Clock clockwork.Clock
	// HeartbeatPeriod defines the period between heartbeats
	HeartbeatPeriod time.Duration
	// MissedHeartbeats is how many heartbeats the peer
	// should miss before we consider it closed
	MissedHeartbeats int
	// GracePeriod is a period for GC not to delete undetected files
	// to prevent accidental deletion. Defaults to 1 hour
	GracePeriod time.Duration
}

// New returns cluster BLOB storage that takes care of replication
// of BLOBs across cluster of nodes. It is designed for small clusters O(10)
// and small amount of objects managed O(100)
func New(config Config) (*Cluster, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Cluster{
		Config:   config,
		close:    ctx,
		cancelFn: cancel,
		FieldLogger: logrus.WithFields(logrus.Fields{
			trace.Component: constants.ComponentBLOB,
			"id":            config.ID,
			"addr":          config.AdvertiseAddr,
		}),
	}, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.Local == nil {
		return trace.BadParameter("missing parameter Local")
	}
	if r.Backend == nil {
		return trace.BadParameter("missing parameter Backend")
	}
	if r.GetPeer == nil {
		return trace.BadParameter("missing parameter GetPeer")
	}
	if r.ID == "" {
		return trace.BadParameter("missing parameter ID")
	}
	if r.AdvertiseAddr == "" {
		return trace.BadParameter("missing parameter AdvertiseAddr")
	}
	if r.WriteFactor < 1 {
		r.WriteFactor = defaults.WriteFactor
	}
	if r.Clock == nil {
		r.Clock = clockwork.NewRealClock()
	}
	if r.HeartbeatPeriod == 0 {
		r.HeartbeatPeriod = defaults.HeartbeatPeriod
	}
	if r.MissedHeartbeats == 0 {
		r.MissedHeartbeats = defaults.MissedHeartbeats
	}
	if r.GracePeriod == 0 {
		r.GracePeriod = defaults.GracePeriod
	}
	return nil
}

// Cluster is the distributed blob storage
type Cluster struct {
	logrus.FieldLogger
	Config
	close    context.Context
	cancelFn context.CancelFunc
}

// Start starts internal processes
func (c *Cluster) Start() {
	go c.periodically("heartbeat", c.heartbeat)
	go c.periodically("purgeDeleted", c.purgeDeletedObjects)
	go c.periodically("fetchNew", c.fetchNewObjects)
}

// Close stops internal processes
func (c *Cluster) Close() error {
	c.cancelFn()
	return nil
}

// GetBLOBs returns a list of BLOBs in the storage
func (c *Cluster) GetBLOBs() ([]string, error) {
	return c.Backend.GetObjects()
}

// GetBLOBEnvelope returns the blob envelope for the given hash
func (c *Cluster) GetBLOBEnvelope(hash string) (*blob.Envelope, error) {
	return c.Local.GetBLOBEnvelope(hash)
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
			c.Infof("Returning, cluster is closing.")
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
		logger := c.WithField("hash", hash)
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
			logger.WithField("package", envelope.String()).Warn("Blob envelope with zero modification, will ignore.")
			continue
		}
		// prevent accidental deletion by using grace period -
		// we keep fresh objects on file storage just in case
		diff := c.Clock.Now().UTC().Sub(envelope.Modified.UTC())
		if diff < c.GracePeriod {
			continue
		} else {
			logger.WithFields(logrus.Fields{
				"grace-period": c.GracePeriod,
				"diff":         diff,
			}).Info("Object exceeded grace period - will delete.")
		}
		err = c.Local.DeleteBLOB(hash)
		if err != nil {
			logger.WithError(err).Warn("Failed to delete object.")
		}
	}
	return nil
}

// fetchNewObjects downloads the objects that have been recorded in the database
// but not available locally.
// It works on best-effort basis - pulling all missing objects before returning
// whatever error(s) it has encountered
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
	var errors []error
	for _, hash := range missingObjects {
		c.Infof("Found missing object %v.", hash)
		err = c.fetchObject(hash)
		if err != nil {
			c.WithFields(logrus.Fields{
				logrus.ErrorKey: err,
				"hash":          hash,
			}).Warn("Failed to fetch object.")
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (c *Cluster) fetchObject(hash string) error {
	peerIDs, err := c.Backend.GetObjectPeers(hash)
	if err != nil {
		return trace.Wrap(err)
	}
	peers, err := c.getPeers(peerIDs...)
	if err != nil {
		return trace.Wrap(err)
	}
	peers = c.withoutSelf(peers)
	if len(peers) == 0 {
		return trace.NotFound("no active remote peers found for %v", hash)
	}
	logger := c.WithField("hash", hash)
	var errors []error
	for _, p := range peers {
		logger := logger.WithField("peer", p.String())
		objects, err := c.getObjects(p)
		if err != nil {
			logger.WithError(err).Error("Failed to fetch object from peer.")
			errors = append(errors, err)
			continue
		}
		f, err := objects.OpenBLOB(hash)
		if err != nil {
			logger.WithError(err).Error("Failed to fetch object from peer.")
			errors = append(errors, err)
			continue
		}
		defer f.Close()
		envelope, err := c.Local.WriteBLOB(f)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		logger.WithField("package", envelope).Debug("Successfully fetched object from peer.")
		err = c.Backend.UpsertObjectPeers(hash, []string{c.ID}, 0)
		if err != nil {
			logger.WithError(err).Error("Failed to upsert peer to the object's list of peers.")
			errors = append(errors, err)
			continue
		}
		return nil
	}
	return trace.NewAggregate(errors...)
}

// WriteBLOB writes object to the storage, returns object envelope
func (c *Cluster) WriteBLOB(data io.Reader) (*blob.Envelope, error) {
	peers, err := c.getPeers()
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
		c.WithFields(logrus.Fields{
			"peers": successWrites,
			"hash":  envelope.SHA512,
		}).Debug("Got enough success writes.")
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
		logger := c.WithField("peer", p.String())
		if p.ID == c.ID {
			continue
		}
		_, err := f.Seek(0, 0)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		peerClient, err := c.GetPeer(p)
		if err != nil {
			logger.WithError(err).Warn("Failed to create client to peer.")
			errors = append(errors, err)
			continue
		}
		_, err = peerClient.WriteBLOB(f)
		if err != nil {
			logger.WithError(err).Warn("Failed to write blob remotely.")
			errors = append(errors, err)
			continue
		}
		successWrites = append(successWrites, p.ID)
		if len(successWrites) >= c.WriteFactor {
			logger.WithFields(logrus.Fields{
				"peers": successWrites,
				"hash":  envelope.SHA512,
			}).Debug("Got enough success writes.")
			err := c.Backend.UpsertObjectPeers(envelope.SHA512, successWrites, 0)
			if err != nil {
				return nil, trace.Wrap(err, "failed to write metadata for object %v: %v",
					envelope.SHA512, err)
			}
			return envelope, nil
		}
	}

	if len(errors) == 0 {
		return nil, trace.NotFound("not enough peers")
	}
	return nil, trace.Wrap(trace.NewAggregate(errors...), "not enough successful writes (want >= %v, got %v)",
		c.WriteFactor, len(successWrites))
}

func (c *Cluster) getObjects(p storage.Peer) (blob.Objects, error) {
	if p.ID == c.ID {
		return c.Local, nil
	}
	return c.GetPeer(p)
}

// OpenBLOB opens file identified by hash and returns reader
func (c *Cluster) OpenBLOB(hash string) (blob.ReadSeekCloser, error) {
	logger := c.WithField("hash", hash)
	ids, err := c.Backend.GetObjectPeers(hash)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	peers, err := c.getPeers(ids...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(peers) == 0 {
		return nil, trace.ConnectionProblem(nil, "no peers found")
	}
	var reader blob.ReadSeekCloser
	for _, p := range peers {
		logger := logger.WithField("peer", p.String())
		objects, err := c.getObjects(p)
		if err != nil {
			logger.WithError(err).Warn("Failed to connect to the blob store.")
			continue
		}
		reader, err = objects.OpenBLOB(hash)
		if err != nil {
			logger.WithError(err).Warn("Failed to access blob.")
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

func (c *Cluster) getPeers(ids ...string) ([]storage.Peer, error) {
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
			// if its last heartbeat is older than the acceptance time frame
			if c.Clock.Now().UTC().Sub(p.LastHeartbeat) > missedWindow {
				c.WithFields(logrus.Fields{
					"peer":           p.String(),
					"time-window":    missedWindow,
					"last-heartbeat": p.LastHeartbeat,
				}).Warn("Exclude stale peer.")
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
