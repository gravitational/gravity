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
