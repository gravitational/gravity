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

package proxy

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/rpc/internal/inprocess"

	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestProxy(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) TestProxiesConnections(c *C) {
	link := newLocalLink()
	defer link.stop()
	proxy := New(link, log.WithField("test", "TestProxiesConnections"))
	notifyCh := make(chan struct{}, 1)
	proxy.NotifyCh = notifyCh
	c.Assert(proxy.Start(), IsNil)
	defer proxy.Stop()

	conn, err := link.local.Dial()
	c.Assert(err, IsNil)

	// wait for new connection: this blocks until the proxy has accepted
	// the connection and created handler loop
	<-notifyCh

	s := newServer(1)
	go s.serve(link.upstream)

	payload := []byte("test")
	_, err = conn.Write(payload)
	c.Assert(err, IsNil)
	conn.Close()

	select {
	case resp := <-s.recvCh:
		c.Assert(resp.err, IsNil)
		c.Assert(resp.output.Bytes(), DeepEquals, payload)
	case <-time.After(1 * time.Second):
		c.Error("failed on recv")
	}
}

func (*S) TestCanStopProxyOnDemand(c *C) {
	payload := []byte("test")
	link := newLocalLink()
	logger := log.WithField("test", "TestCanStopProxyOnDemand")
	proxy := New(link, logger)
	notifyCh := make(chan struct{}, 2)
	proxy.NotifyCh = notifyCh
	c.Assert(proxy.Start(), IsNil)
	defer proxy.Stop()

	s := newServer(2)
	go s.serve(link.upstream)
	defer link.stop()

	conn, err := link.local.Dial()
	c.Assert(err, IsNil)

	// wait for new connection: this blocks until the proxy has accepted
	// the connection and created handler loop
	<-notifyCh

	// Stop proxy so the write fails
	proxy.Stop()

	_, err = conn.Write(payload)
	c.Assert(err, ErrorMatches, "io: read/write on closed pipe")
	conn.Close()

	// Restart proxy to be able to write
	link.resetLocal()
	proxy = New(link, logger)
	proxy.NotifyCh = notifyCh
	c.Assert(proxy.Start(), IsNil)
	defer proxy.Stop()

	conn, err = link.local.Dial()
	c.Assert(err, IsNil)

	// wait for new connection: this blocks until the proxy has accepted
	// the connection and created handler loop
	<-notifyCh

	_, err = conn.Write(payload)
	c.Assert(err, IsNil)
	conn.Close()

	select {
	case <-s.recvCh:
		// Skip the first write
	case <-time.After(1 * time.Second):
		c.Error("timeout waiting for write")
	}

	select {
	case resp := <-s.recvCh:
		c.Assert(resp.err, IsNil)
		c.Assert(resp.output.Bytes(), DeepEquals, payload)
	case <-time.After(1 * time.Second):
		c.Error("failed on recv")
	}
}

func newLocalLink() *localLink {
	return &localLink{
		local:    inprocess.Listen(),
		upstream: inprocess.Listen(),
	}
}

func (r *localLink) Listen() (net.Listener, error) {
	return r.local, nil
}

func (r *localLink) Dial() (net.Conn, error) {
	return r.upstream.Dial()
}

func (r *localLink) Close() error {
	return r.local.Close()
}

func (r *localLink) String() string {
	return "localLink"
}

func (r *localLink) resetLocal() {
	r.local = inprocess.Listen()
}

func (r *localLink) stop() {
	r.local.Close()
	r.upstream.Close()
}

type localLink struct {
	local, upstream inprocess.ListenerInterface
}

func newServer(bufferSize int) *server {
	return &server{
		recvCh: make(chan resp, bufferSize),
	}
}

func (r *server) serve(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if !isClosedError(err) {
				r.err = err
			}
			return
		}

		go r.handle(conn)
	}
}

func (r *server) handle(conn net.Conn) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, conn)
	if err == io.EOF {
		err = nil
	}
	conn.Close()
	r.recvCh <- resp{err, buf}
}

type server struct {
	err    error
	recvCh chan resp
}

type resp struct {
	err    error
	output bytes.Buffer
}

func isClosedError(err error) bool {
	return err.Error() == "closed"
}
