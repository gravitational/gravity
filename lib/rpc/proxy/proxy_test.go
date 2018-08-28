package proxy

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/rpc/inprocess"

	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestProxy(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (_ *S) TestProxiesConnections(c *C) {
	link := newLocalLink()
	proxy := New(link, log.StandardLogger())
	proxy.Start()
	defer proxy.Stop()

	conn, err := link.local.Dial()
	c.Assert(err, IsNil)

	s := newServer(1)
	go s.serve(link.upstream)
	defer link.Close()

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

func (_ *S) TestCanStopProxyOnDemand(c *C) {
	payload := []byte("test")
	link := newLocalLink()
	proxy := New(link, log.StandardLogger())
	c.Assert(proxy.Start(), IsNil)
	defer proxy.Stop()

	s := newServer(2)
	go s.serve(link.upstream)
	defer link.Close()

	conn, err := link.local.Dial()
	c.Assert(err, IsNil)

	// Stop proxy so the write fails
	proxy.Stop()
	link.resetLocal()

	_, err = conn.Write(payload)
	c.Assert(err, ErrorMatches, "io: read/write on closed pipe")
	conn.Close()

	// Restart proxy to be able to write
	proxy = New(link, log.StandardLogger())
	c.Assert(proxy.Start(), IsNil)
	defer proxy.Stop()

	conn, err = link.local.Dial()
	c.Assert(err, IsNil)
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

func (r *localLink) String() string {
	return "localLink"
}

func (r *localLink) resetLocal() {
	r.local = inprocess.Listen()
}

func (r *localLink) Close() error {
	r.local.Close()
	r.upstream.Close()
	return nil
}

type localLink struct {
	local, upstream inprocess.Listener
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
