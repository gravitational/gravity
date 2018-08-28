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

package inprocess

import (
	"net"
	"testing"

	"golang.org/x/net/nettest"
)

func TestConn(t *testing.T) {
	nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
		ln := Listen()

		connCh := make(chan struct {
			net.Conn
			error
		}, 1)
		go func() {
			c2, err := ln.Accept()
			connCh <- struct {
				net.Conn
				error
			}{c2, err}
		}()

		c1, err = ln.Dial()
		if err != nil {
			t.Fatal(err)
		}

		resp := <-connCh
		if resp.error != nil {
			t.Fatal(resp.error)
		}

		stop = func() {
			c1.Close()
			resp.Conn.Close()
			ln.Close()
		}
		return c1, resp.Conn, stop, nil
	})
}
