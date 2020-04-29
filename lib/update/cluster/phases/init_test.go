/*
Copyright 2020 Gravitational, Inc.

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

package phases

import (
	"testing"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestPhases(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

// TestPeerCleaner tests peer clean up scenarios.
// TODO(dmitri): this is ideally implemented as an integration test on a real etcd backend
func (*S) TestPeerCleaner(c *C) {
	var testCases = []struct {
		comment  string
		c        peerCleaner
		expected peerCleaner
	}{
		{
			comment: "no changes to peer state",
			c: peerCleaner{
				log: log.WithField("test", "TestPeerCleaner"),
				p: &testPeerManager{
					objects: []blobObject{
						{
							hash:  "hash1",
							peers: []string{"peer1", "peer2"},
						},
					},
					peers: []storage.Peer{
						{ID: "peer1"},
						{ID: "peer2"},
					},
				},
				existingPeers: []string{"peer2", "peer1"},
			},
			expected: peerCleaner{
				p: &testPeerManager{
					objects: []blobObject{
						{
							hash:  "hash1",
							peers: []string{"peer1", "peer2"},
						},
					},
					peers: []storage.Peer{
						{ID: "peer1"},
						{ID: "peer2"},
					},
				},
			},
		},
		{
			comment: "removes stale peer and its references",
			c: peerCleaner{
				log: log.WithField("test", "TestPeerCleaner"),
				p: &testPeerManager{
					objects: []blobObject{
						{
							hash:  "hash1",
							peers: []string{"stale_peer", "peer2"},
						},
						{
							hash:  "hash2",
							peers: []string{"peer2"},
						},
					},
					peers: []storage.Peer{
						{ID: "stale_peer"},
						{ID: "peer2"},
					},
				},
				existingPeers: []string{"peer2"},
			},
			expected: peerCleaner{
				p: &testPeerManager{
					objects: []blobObject{
						{
							hash:  "hash1",
							peers: []string{"peer2"},
						},
						{
							hash:  "hash2",
							peers: []string{"peer2"},
						},
					},
					peers: []storage.Peer{
						{ID: "peer2"},
					},
				},
			},
		},
		{
			comment: "removes stale peer and its references; removes lost object",
			c: peerCleaner{
				log: log.WithField("test", "TestPeerCleaner"),
				p: &testPeerManager{
					objects: []blobObject{
						{
							hash:  "hash1",
							peers: []string{"stale_peer", "peer2"},
						},
						{
							hash:  "lost_object",
							peers: []string{"stale_peer"},
						},
					},
					peers: []storage.Peer{
						{ID: "peer2"},
						{ID: "stale_peer"},
					},
				},
				existingPeers: []string{"peer2"},
			},
			expected: peerCleaner{
				p: &testPeerManager{
					objects: []blobObject{
						{
							hash:  "hash1",
							peers: []string{"peer2"},
						},
					},
					peers: []storage.Peer{
						{ID: "peer2"},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		comment := Commentf(tc.comment)
		c.Assert(tc.c.cleanPeers(), IsNil, comment)
		c.Assert(tc.c.p, DeepEquals, tc.expected.p, comment)
	}
}

func (r *testPeerManager) GetObjects() (hashes []string, err error) {
	hashes = make([]string, 0, len(r.objects))
	for _, o := range r.objects {
		hashes = append(hashes, o.hash)
	}
	return hashes, nil
}

func (r *testPeerManager) GetObjectPeers(hash string) (peers []string, err error) {
	for _, o := range r.objects {
		if o.hash == hash {
			return o.peers, nil
		}
	}
	return nil, trace.NotFound("no peers for %v", hash)
}

func (r *testPeerManager) GetPeers() (peers []storage.Peer, err error) {
	return r.peers, nil
}

func (r *testPeerManager) DeleteObject(hash string) error {
	for i, o := range r.objects {
		if o.hash == hash {
			r.objects = append(r.objects[:i], r.objects[i+1:]...)
			return nil
		}
	}
	return trace.NotFound("no object with hash %v", hash)
}

func (r *testPeerManager) DeletePeer(peerID string) error {
	for i, p := range r.peers {
		if p.ID == peerID {
			r.peers = append(r.peers[:i], r.peers[i+1:]...)
			return nil
		}
	}
	return trace.NotFound("no peer with ID %v", peerID)
}

func (r *testPeerManager) DeleteObjectPeers(hash string, removePeers []string) error {
	for i, o := range r.objects {
		if o.hash == hash {
			for j, peer := range o.peers {
				for _, removePeer := range removePeers {
					if peer == removePeer {
						r.objects[i].peers = append(r.objects[i].peers[:j], r.objects[i].peers[j+1:]...)
					}
				}
				// Etcd backend does not return an error if a peer to be removed
				// is not in the object's peer list
			}
			return nil
		}
	}
	return trace.NotFound("no object with hash %v", hash)
}

func deleteItems(items []string, remove ...string) []string {
	for i, item := range items {
		for _, r := range remove {
			if item == r {
				items = append(items[:i], items[i+1:]...)
			}
		}
	}
	return items
}

type testPeerManager struct {
	objects []blobObject
	peers   []storage.Peer
}

type blobObject struct {
	hash  string
	peers []string
}
