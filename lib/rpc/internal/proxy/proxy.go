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

// Package proxy implements a simple network proxy for tests
package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns a new proxy for the given link
func New(link Link, log log.FieldLogger) *Proxy {
	ctx, cancel := context.WithCancel(context.Background())
	return &Proxy{
		FieldLogger: log.WithField("proxy", link.String()),
		link:        link,
		doneCh:      ctx.Done(),
		cancel:      cancel,
	}
}

// Proxy defines a link between two endpoints
type Proxy struct {
	log.FieldLogger
	// NotifyCh signals new connections
	NotifyCh chan<- struct{}
	// StartedCh signals when proxy starts servicing
	StartedCh chan<- struct{}

	link Link
	// doneCh signals that the connections should be dropped
	// and proxy loop stopped
	doneCh <-chan struct{}
	cancel context.CancelFunc
	// wg allows to track lifespan of internal processes
	wg sync.WaitGroup
}

// Start starts the proxy
func (r *Proxy) Start() error {
	listener, err := r.link.Listen()
	if err != nil {
		return trace.Wrap(err)
	}
	r.wg.Add(1)
	go r.serve(listener)
	r.Info("Proxy started.")
	return nil
}

// Stop stops the proxy and drops all active connections
func (r *Proxy) Stop() {
	r.cancel()
	r.link.Close()
	r.wg.Wait()
	r.Info("Proxy stopped.")
}

// Link allows to build a proxying link between two endpoints.
type Link interface {
	fmt.Stringer
	// Listen returns a listener to the local side of the link
	Listen() (net.Listener, error)
	// Dials dials to the remote side of the link
	Dial() (net.Conn, error)
	// Close closes the local link
	Close() error
}

// NetLink links two network endpoints.
// Implements Link
type NetLink struct {
	// Local specifies the local side of the connection
	Local net.Listener
	// Upstream specifies the remote side of the connection
	Upstream string
}

// Listen returns a new listener for the local side of the connection
// Implements Link
func (r NetLink) Listen() (net.Listener, error) {
	return r.Local, nil
}

// Close closes the local link
func (r NetLink) Close() error {
	return r.Local.Close()
}

// Dials dials the remote endpoint.
// Implements Link
func (r NetLink) Dial() (net.Conn, error) {
	conn, err := net.Dial("tcp", r.Upstream)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// Strings provides a textual representation of this link
func (r NetLink) String() string {
	return fmt.Sprintf("netlink(local=%v, upstream=%v)", r.Local.Addr(), r.Upstream)
}

func (r *Proxy) serve(listener net.Listener) {
	defer r.wg.Done()
	r.notifyStartedServing()
	for {
		c1, err := listener.Accept()
		if err != nil {
			select {
			case <-r.doneCh:
				return
			default:
			}
			r.WithError(err).Warn("Failed to accept.")
			return
		}
		r.Infof("Accept connection from %v.", c1.RemoteAddr())

		c2, err := r.link.Dial()
		if err != nil {
			r.WithFields(log.Fields{
				log.ErrorKey: err,
				"addr":       r.link,
			}).Warn("Failed to dial local link.")
			c1.Close()
			return
		}
		r.WithField("addr", c2.RemoteAddr()).Info("Upstream connection.")

		r.wg.Add(3)
		errCh := make(chan error, 2)
		go r.proxyConn(c1, c2, errCh)
		go r.proxyConn(c2, c1, errCh)
		go r.watchConns(errCh, c1, c2, listener)

		r.notifyNewConnection()
	}
}

func (r *Proxy) watchConns(errCh <-chan error, closers ...io.Closer) {
	defer r.wg.Done()
	select {
	case err := <-errCh:
		if err != nil {
			r.WithError(err).Warn("Failed in proxyConn.")
		}
	case <-r.doneCh:
	}
	for _, c := range closers {
		c.Close()
	}
}

func (r *Proxy) notifyStartedServing() {
	select {
	case r.StartedCh <- struct{}{}:
	default:
	}
}

func (r *Proxy) notifyNewConnection() {
	select {
	case r.NotifyCh <- struct{}{}:
	default:
	}
}

func (r *Proxy) proxyConn(c1 io.Writer, c2 io.Reader, errCh chan<- error) {
	defer r.wg.Done()
	_, err := io.Copy(c1, c2)
	errCh <- err
}
