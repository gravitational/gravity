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
	"net"
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
