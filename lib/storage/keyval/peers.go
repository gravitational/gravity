package keyval

import (
	"sort"
	"time"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"
)

func (b *backend) GetPeers() ([]storage.Peer, error) {
	ids, err := b.getKeys(b.key(peersP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.Peer
	for _, id := range ids {
		var peer storage.Peer
		err := b.getVal(b.key(peersP, id), &peer)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
		}
		out = append(out, peer)
	}
	return out, nil
}

func (b *backend) UpsertPeer(p storage.Peer) error {
	if err := p.Check(); err != nil {
		return trace.Wrap(err)
	}
	p.LastHeartbeat = b.Now().UTC()
	err := b.upsertVal(b.key(peersP, p.ID), p, forever)
	return trace.Wrap(err)
}

func (b *backend) DeletePeer(id string) error {
	err := b.deleteKey(b.key(peersP, id))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("peer(%v) not found", id)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (b *backend) GetObjects() ([]string, error) {
	keys, err := b.getKeys(b.key(objectsP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Strings(keys)
	return keys, nil
}

func (b *backend) UpsertObjectPeers(hash string, peers []string, ttl time.Duration) error {
	err := b.upsertDir(b.key(objectsP, hash), ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, peerID := range peers {
		err = b.upsertVal(b.key(objectsP, hash, peersP, peerID), peerID, ttl)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (b *backend) GetObjectPeers(hash string) ([]string, error) {
	ids, err := b.getKeys(b.key(objectsP, hash, peersP))
	if err == nil && len(ids) == 0 {
		return nil, trace.NotFound("no object(%v) peers found", hash)
	}
	return ids, trace.Wrap(err)
}

func (b *backend) DeleteObjectPeers(hash string, peers []string) error {
	var errors []error
	for _, peerID := range peers {
		err := b.deleteKey(b.key(objectsP, hash, peersP, peerID))
		if err != nil && !trace.IsNotFound(err) {
			errors = append(errors, trace.Wrap(err, "error deleting %v", peerID))
		}
	}
	return trace.NewAggregate(errors...)
}

func (b *backend) DeleteObject(hash string) error {
	err := b.deleteDir(b.key(objectsP, hash))
	return trace.Wrap(err)
}
