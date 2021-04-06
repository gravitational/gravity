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

package sni

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/go-vhost"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Dialer is a dialer to remote location
type Dialer func() (net.Conn, error)

// Frontend is a mux Frontend either web handler or remote dialer
type Frontend struct {
	// Host is SNI host,used for routing
	Host string
	// Name is Frontend name, used to identify frontends
	Name string
	// Dial if present, used to dial the remote location
	Dial Dialer
	// Default controls if the default location is set with no SNI routes matched
	Default bool
	// listener is a listener for this frontend
	listener net.Listener
}

// Check checks if this frontend's parameters are valid
func (f *Frontend) Check() error {
	if f.Host == "" {
		return trace.BadParameter("missing parameter Host")
	}
	if f.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if f.Dial == nil {
		return trace.BadParameter("missing parameter Dial")
	}
	return nil
}

// Config is Mux configuration parameters
type Config struct {
	// ListenAddr is the underlying listener address
	ListenAddr string
}

// Check checks if configuration parameters are valid
func (c *Config) Check() error {
	if c.ListenAddr == "" {
		return trace.BadParameter("missing parameter ListenAddr")
	}
	return nil
}

// New returns new instance of Mux
func New(config Config) (*Mux, error) {
	if err := config.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Mux{
		Config:      config,
		FieldLogger: logrus.WithField(trace.Component, "mux"),
		frontends:   make(map[string]*Frontend),
	}, nil
}

// ListenConfig is the SNI mux configuration
type ListenConfig struct {
	// ListenAddr is the SNI listener address
	ListenAddr string
	// Frontends is the list of frontends to initialize SNI mux with
	Frontends []Frontend
}

// Listen creates a new SNI mux that listens on the specified address, starts
// it and adds all provided frontends to it. Returns the created mux
func Listen(config ListenConfig) (*Mux, error) {
	mux, err := New(Config{ListenAddr: config.ListenAddr})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = mux.Run()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = mux.AddFrontends(config.Frontends...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return mux, nil
}

// Mux is SNI multiplexer
type Mux struct {
	sync.RWMutex
	Config
	mux *vhost.TLSMuxer
	logrus.FieldLogger
	frontends       map[string]*Frontend
	defaultFrontend *Frontend
}

// notFoundHandler is used when no SNI routes matched
func (m *Mux) notFoundHandler() {
	for {
		conn, err := m.mux.NextError()
		if conn == nil {
			m.Debugf("Failed to mux next connection: %v.", err)
			if _, ok := err.(vhost.Closed); ok || utils.IsClosedConnectionError(err) {
				return
			}
		} else {
			defaultFrontend := m.DefaultFrontend()
			if _, ok := err.(vhost.NotFound); ok && defaultFrontend != nil {
				go m.proxyConnection(conn, defaultFrontend)
			} else {
				m.Debugf("failed to mux connection from %v, error: %v", conn.RemoteAddr(), err)
				conn.Close()
			}
		}
	}
}

// DefaultFrontend returns default frontend, nil if not set
func (m *Mux) DefaultFrontend() *Frontend {
	m.RLock()
	defer m.RUnlock()
	return m.defaultFrontend
}

func (m *Mux) serveFrontend(f *Frontend) {
	m.Debugf("serveFrontend(%v)", f.Host)
	defer f.listener.Close()
	for {
		// accept next connection to this frontend
		conn, err := f.listener.Accept()
		if conn == nil {
			m.Warningf("%v listener closed", f.Host)
			return
		}
		if err != nil {
			m.Warningf("failed to accept new connection for '%v': %v", conn.RemoteAddr(), err)
			if e, ok := err.(net.Error); ok {
				if e.Temporary() {
					continue
				}
			}
			return
		}
		m.Debugf("accepted new connection for %v from %v", f.Host, conn.RemoteAddr())

		go m.proxyConnection(conn, f)
	}
}

// ExistingFrontends returns map of existing frontends, read only
func (m *Mux) ExistingFrontends() map[string]*Frontend {
	out := make(map[string]*Frontend)
	m.RLock()
	defer m.RUnlock()
	for name := range m.frontends {
		out[name] = m.frontends[name]
	}
	return out
}

// HasFrontend returns true if there's a frontend matching given SNI host
func (m *Mux) HasFrontend(host string) bool {
	m.RLock()
	defer m.RUnlock()
	_, ok := m.frontends[host]
	return ok
}

// AddFrontend adds frontend, returns error if SNI Host route already exists
func (m *Mux) AddFrontend(f Frontend) error {
	if err := f.Check(); err != nil {
		return trace.Wrap(err)
	}
	m.Infof("AddFrontend(%v).", f.Host)
	m.Lock()
	defer m.Unlock()
	if _, ok := m.frontends[f.Host]; ok {
		return trace.AlreadyExists("frontend %v already exists", f.Host)
	}
	listener, err := m.mux.Listen(f.Host)
	if err != nil {
		return trace.Wrap(err)
	}
	f.listener = newCloseOnceListener(listener)
	go m.serveFrontend(&f)
	m.frontends[f.Host] = &f
	if f.Default {
		m.Debugf("setting default frontend to %v", f.Host)
		m.defaultFrontend = &f
	}
	return nil
}

// AddFrontends adds multiple SNI frontends
func (m *Mux) AddFrontends(frontends ...Frontend) error {
	for _, f := range frontends {
		err := m.AddFrontend(f)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// RemoveFrontend removes frontend by SNI Host name
func (m *Mux) RemoveFrontend(host string) error {
	if host == "" {
		return trace.BadParameter("missing parameter host")
	}
	m.Lock()
	defer m.Unlock()
	m.Debugf("remove frontend %v", host)
	f, ok := m.frontends[host]
	if !ok {
		return trace.NotFound("frontend %v not found", f.Host)
	}
	m.Debugf("closing %v frontend", host)
	err := f.listener.Close()
	if err != nil {
		return trace.Wrap(err)
	}
	if m.defaultFrontend != nil && m.defaultFrontend.Host == host {
		m.defaultFrontend = nil
	}
	delete(m.frontends, host)
	return nil
}

// Run binds to listening socket and starts routing requests
func (m *Mux) Run() error {
	l, err := net.Listen("tcp", m.ListenAddr)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	m.mux, err = vhost.NewTLSMuxer(l, defaults.ConnectionDeadlineTimeout)
	if err != nil {
		l.Close()
		return trace.Wrap(err)
	}

	go m.notFoundHandler()
	return nil
}

func (m *Mux) proxyConnection(c net.Conn, f *Frontend) {
	err := m.proxyConn(c, f)
	if err != nil && err != io.EOF {
		m.Warningf("failed to proxy: %v", trace.DebugReport(err))
	}
}

func (m *Mux) proxyConn(c net.Conn, f *Frontend) (err error) {
	start := time.Now()
	m.Debugf("[LATENCY] proxyConn to %v %v", f.Host, start)
	defer func() {
		m.Debugf("[LATENCY] proxyConn to %v in %v", f.Host, time.Since(start))
	}()

	conn, err := f.Dial()
	if err != nil {
		c.Close()
		m.Debugf("failed to dial backend connection for %v: %v", f.Host, err)
		return trace.ConvertSystemError(err)
	}
	m.Debugf("initiated new connection to backend: %v %v", conn.LocalAddr(), conn.RemoteAddr())
	m.joinConnections(c, conn)
	return nil
}

func (m *Mux) joinConnections(c1 net.Conn, c2 net.Conn) {
	defer c1.Close()
	defer c2.Close()
	errc := make(chan error, 2)
	halfJoin := func(dst net.Conn, src net.Conn) {
		var err error
		var copied int64
		defer func() { errc <- err }()
		copied, err = io.Copy(dst, src)
		if err != nil && !utils.IsClosedConnectionError(err) {
			m.Debugf("copy from %v to %v failed after %d bytes with error %v", src.RemoteAddr(), dst.RemoteAddr(), copied, err)
		}
	}
	m.Debugf("joining connections: %v %v", c1.RemoteAddr(), c2.RemoteAddr())
	go halfJoin(c1, c2)
	go halfJoin(c2, c1)
	err := <-errc
	if err != nil {
		m.Warningf("Connection closed with %v.", err)
	}
}

func newCloseOnceListener(l net.Listener) net.Listener {
	return &closeOnceListener{Listener: l}
}

type closeOnceListener struct {
	net.Listener
	sync.Once
}

func (c *closeOnceListener) Close() error {
	err := io.EOF
	c.Do(func() {
		err = c.Listener.Close()
	})
	return err
}
