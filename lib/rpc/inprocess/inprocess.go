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

// Package inprocess implements in-process network connections and
// corresponding dialing/listening.
package inprocess

import (
	"errors"
	"net"
	"sync"
)

// Listen creates an in-process listener
func Listen() *listener {
	l := &listener{
		doneCh: make(chan struct{}),
		connCh: make(chan net.Conn, 1),
	}
	return l
}

// Listener is the inprocess listener
type Listener interface {
	net.Listener
	// Dial creates a new inprocess connection
	Dial() (net.Conn, error)
}

// Dial creates a connection to this listener
func (r *listener) Dial() (net.Conn, error) {
	c1, c2 := netPipe()
	select {
	case <-r.doneCh:
		c1.Close()
		c2.Close()
		return nil, errClosed
	case r.connCh <- c1:
		return c2, nil
	}
}

// Accept waits for and returns the next connection to the listener.
func (r *listener) Accept() (net.Conn, error) {
	select {
	case c := <-r.connCh:
		return c, nil
	case <-r.doneCh:
		return nil, errClosed
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (r *listener) Close() error {
	r.Once.Do(func() {
		close(r.doneCh)
	})
	return nil
}

// Addr returns the listener's network address.
func (r *listener) Addr() net.Addr {
	return addr{}
}

type listener struct {
	sync.Once
	doneCh chan struct{}
	connCh chan net.Conn
}

// Network returns the type of network this address has
func (r addr) Network() string { return "inprocess" }

// String returns textual representation of this address
func (r addr) String() string { return "inprocess" }

// addr implements net.Addr
type addr struct{}

var errClosed = errors.New("closed")
