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
	"bytes"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/blob/client"
	"github.com/gravitational/gravity/lib/blob/fs"
	"github.com/gravitational/gravity/lib/blob/handler"
	"github.com/gravitational/gravity/lib/blob/suite"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users/usersservice"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestCluster(t *testing.T) { TestingT(t) }

type ClusterSinglePeer struct {
	suite suite.BLOBSuite
	dir   string
}

var _ = Suite(&ClusterSinglePeer{})
var _ = Suite(&ClusterMultiPeers{})
var _ = Suite(&RPCSuite{})

const (
	heartbeatPeriod  = 100 * time.Millisecond
	missedHeartbeats = 2
	gracePeriod      = time.Minute
)

func (s *ClusterSinglePeer) SetUpTest(c *C) {
	fakeClock := clockwork.NewFakeClockAt(time.Now().UTC())
	log.SetOutput(os.Stderr)
	s.dir = c.MkDir()

	b, err := keyval.NewBolt(keyval.BoltConfig{
		Clock: fakeClock,
		Path:  filepath.Join(s.dir, "bolt.db"),
	})
	c.Assert(err, IsNil)

	local, err := fs.New(s.dir)
	c.Assert(err, IsNil)

	obj, err := New(Config{
		Local:       local,
		WriteFactor: 1,
		Backend:     b,
		GetPeer: func(p storage.Peer) (blob.Objects, error) {
			panic("should not be called with write factor 1")
		},
		HeartbeatPeriod:  heartbeatPeriod,
		MissedHeartbeats: missedHeartbeats,
		Clock:            fakeClock,
		ID:               "peer1",
		AdvertiseAddr:    "https://localhost",
		GracePeriod:      gracePeriod,
	})
	c.Assert(err, IsNil)

	s.suite.Objects = obj
}

func (s *ClusterSinglePeer) TearDownTest(c *C) {
	if s.suite.Objects != nil {
		s.suite.Objects.Close()
	}
}

func (s *ClusterSinglePeer) TestBLOB(c *C) {
	s.suite.BLOB(c)
}

func (s *ClusterSinglePeer) TestBLOBSeek(c *C) {
	s.suite.BLOBSeek(c)
}

func (s *ClusterSinglePeer) TestBLOBWriteTwice(c *C) {
	s.suite.BLOBWriteTwice(c)
}

func (s *ClusterSinglePeer) TestBLOBList(c *C) {
	s.suite.BLOBList(c)
}

const peersCount = 3

type ClusterMultiPeers struct {
	suite        suite.BLOBSuite
	clusterSuite clusterSuite
	dir          string
	objects      []*Cluster
}

func (s *ClusterMultiPeers) SetUpTest(c *C) {
	fakeClock := clockwork.NewFakeClockAt(time.Now().UTC())
	log.SetOutput(os.Stderr)
	log.SetLevel(log.DebugLevel)
	s.dir = c.MkDir()

	b, err := keyval.NewBolt(keyval.BoltConfig{
		Clock: fakeClock,
		Path:  filepath.Join(s.dir, "bolt.db"),
	})
	c.Assert(err, IsNil)

	peers := make([]blob.Objects, peersCount)
	objects := make([]*Cluster, peersCount)
	clients := make([]blob.Objects, peersCount)

	getPeer := func(p storage.Peer) (blob.Objects, error) {
		id, err := strconv.Atoi(p.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return peers[id], nil
	}

	for i := 0; i < peersCount; i++ {
		local, err := fs.New(c.MkDir())
		c.Assert(err, IsNil)
		peers[i] = local
		obj, err := New(Config{
			Local:            peers[i],
			WriteFactor:      peersCount - 1,
			Backend:          b,
			GetPeer:          getPeer,
			HeartbeatPeriod:  heartbeatPeriod,
			MissedHeartbeats: missedHeartbeats,
			Clock:            fakeClock,
			ID:               fmt.Sprintf("%v", i),
			AdvertiseAddr:    "https://localhost",
			GracePeriod:      gracePeriod,
		})
		c.Assert(err, IsNil)
		objects[i] = obj
		objects[i].heartbeat()
		clients[i] = obj
	}

	s.suite.Objects = objects[0]
	s.clusterSuite.objects = objects
	s.clusterSuite.clients = clients
	s.clusterSuite.clock = fakeClock
}

func (s *ClusterMultiPeers) TearDownTest(c *C) {
	for _, obj := range s.objects {
		obj.Close()
	}
}

func (s *ClusterMultiPeers) TestBLOB(c *C) {
	s.suite.BLOB(c)
}

func (s *ClusterMultiPeers) TestBLOBSeek(c *C) {
	s.suite.BLOBSeek(c)
}

func (s *ClusterMultiPeers) TestBLOBWriteTwice(c *C) {
	s.suite.BLOBWriteTwice(c)
}

func (s *ClusterMultiPeers) TestBLOBList(c *C) {
	s.suite.BLOBList(c)
}

func (s *ClusterMultiPeers) TestReplication(c *C) {
	s.clusterSuite.Replication(c)
}

func (s *ClusterMultiPeers) TestCleanup(c *C) {
	s.clusterSuite.Cleanup(c)
}

type RPCSuite struct {
	suite        suite.BLOBSuite
	clusterSuite clusterSuite
}

func (s *RPCSuite) SetUpTest(c *C) {
	log.SetOutput(os.Stderr)
	log.SetLevel(log.DebugLevel)

	dir := c.MkDir()

	var err error
	backend, err := keyval.NewBolt(
		keyval.BoltConfig{Path: filepath.Join(dir, "bolt.db")})
	c.Assert(err, IsNil)

	usersService, err := usersservice.New(
		usersservice.Config{Backend: backend})
	c.Assert(err, IsNil)

	const peerUser = "admin@a.example.com"

	key, err := blob.UpsertUser(usersService, peerUser)
	c.Assert(err, IsNil)

	peers := make([]blob.Objects, peersCount)
	objects := make([]*Cluster, peersCount)
	clients := make([]blob.Objects, peersCount)
	localClients := make([]blob.Objects, peersCount)

	getPeer := func(p storage.Peer) (blob.Objects, error) {
		id, err := strconv.Atoi(p.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return localClients[id], nil
	}

	fakeClock := clockwork.NewFakeClockAt(time.Now().UTC())

	for i := 0; i < 3; i++ {
		local, err := fs.New(c.MkDir())
		c.Assert(err, IsNil)
		peers[i] = local

		obj, err := New(Config{
			Local:            peers[i],
			WriteFactor:      peersCount - 1,
			Backend:          backend,
			GetPeer:          getPeer,
			HeartbeatPeriod:  heartbeatPeriod,
			MissedHeartbeats: missedHeartbeats,
			Clock:            fakeClock,
			ID:               fmt.Sprintf("%v", i),
			AdvertiseAddr:    "https://localhost",
			GracePeriod:      gracePeriod,
		})
		c.Assert(err, IsNil)

		webHandler, err := handler.New(handler.Config{
			Users:   usersService,
			Local:   local,
			Cluster: obj,
		})
		c.Assert(err, IsNil)
		mux := http.NewServeMux()
		mux.Handle("/objects/", webHandler)
		webServer := httptest.NewServer(mux)

		clusterClient, err := client.NewAuthenticatedClient(
			webServer.URL, peerUser, key.Token,
			roundtrip.HTTPClient(&http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					}}}),
		)
		c.Assert(err, IsNil)

		localClient, err := client.NewPeerAuthenticatedClient(
			webServer.URL, peerUser, key.Token,
			roundtrip.HTTPClient(&http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					}}}),
		)

		c.Assert(err, IsNil)
		objects[i] = obj
		objects[i].heartbeat()
		clients[i] = clusterClient
		localClients[i] = localClient
	}

	s.suite.Objects = objects[0]
	s.clusterSuite.objects = objects
	s.clusterSuite.clients = clients
	s.clusterSuite.clock = fakeClock

	c.Assert(err, IsNil)

}

func (s *RPCSuite) TestBLOB(c *C) {
	s.suite.BLOB(c)
}

func (s *RPCSuite) TestBLOBSeek(c *C) {
	s.suite.BLOBSeek(c)
}

func (s *RPCSuite) TestBLOBWriteTwice(c *C) {
	s.suite.BLOBWriteTwice(c)
}

func (s *RPCSuite) TestBLOBList(c *C) {
	s.suite.BLOBList(c)
}

func (s *RPCSuite) TestReplication(c *C) {
	s.clusterSuite.Replication(c)
}

func (s *RPCSuite) TestCleanup(c *C) {
	s.clusterSuite.Cleanup(c)
}

type clusterSuite struct {
	objects []*Cluster
	clients []blob.Objects
	clock   clockwork.FakeClock
}

func (s *clusterSuite) Replication(c *C) {
	peer1, peer2 := s.objects[1], s.objects[2]

	data := []byte("hello, there, cluster!")

	envelope, err := s.clients[0].WriteBLOB(bytes.NewBuffer(data))
	c.Assert(err, IsNil)
	time.Sleep(2 * heartbeatPeriod)

	c.Assert(peer1.fetchNewObjects(), IsNil)
	c.Assert(peer2.fetchNewObjects(), IsNil)

	bf, err := peer1.Local.OpenBLOB(envelope.SHA512)
	c.Assert(err, IsNil)
	bout, err := ioutil.ReadAll(bf)
	c.Assert(err, IsNil)

	df, err := peer2.Local.OpenBLOB(envelope.SHA512)
	c.Assert(err, IsNil)
	dout, err := ioutil.ReadAll(df)
	c.Assert(err, IsNil)

	c.Assert(string(bout), Equals, string(data))
	c.Assert(string(dout), Equals, string(data))
}

func (s *clusterSuite) Cleanup(c *C) {
	peer1, peer2, peer3 := s.objects[0], s.objects[1], s.objects[2]

	data := []byte("hello, there, cluster!")

	envelope, err := s.clients[0].WriteBLOB(bytes.NewBuffer(data))
	c.Assert(err, IsNil)
	time.Sleep(2 * heartbeatPeriod)

	c.Assert(peer2.fetchNewObjects(), IsNil)
	c.Assert(peer3.fetchNewObjects(), IsNil)

	c.Assert(peer1.DeleteBLOB(envelope.SHA512), IsNil)

	for _, o := range s.objects {
		c.Assert(o.purgeDeletedObjects(), IsNil)
		f, err := o.Local.OpenBLOB(envelope.SHA512)
		// grace period has not expired
		c.Assert(err, IsNil)
		c.Assert(f.Close(), IsNil)
	}

	s.clock.Advance(gracePeriod + time.Minute)

	for _, o := range s.objects {
		c.Assert(o.purgeDeletedObjects(), IsNil)
		_, err = o.Local.OpenBLOB(envelope.SHA512)
		// grace period has expired, objects are deleted
		c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
	}
}
