package server

import (
	"net"
	"os"
	"testing"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

func TestServer(t *testing.T) { check.TestingT(t) }

type S struct {
	*log.Logger
}

var _ = check.Suite(&S{})

func init() {
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
		log.SetOutput(os.Stderr)
	}
}

func (r *S) SetUpSuite(c *check.C) {
	r.Logger = log.StandardLogger()
	if testing.Verbose() {
		trace.SetDebug(true)
		r.Logger.Level = log.DebugLevel
	}
}

func listen(c *check.C) net.Listener {
	return listenAddr("127.0.0.1:0", c)
}

func listenAddr(addr string, c *check.C) net.Listener {
	listener, err := net.Listen("tcp4", addr)
	c.Assert(err, check.IsNil)
	return listener
}
