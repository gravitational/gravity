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
	"time"

	"github.com/gravitational/gravity/lib/rpc/internal/proxy"

	"golang.org/x/net/context"
	. "gopkg.in/check.v1"
)

func (r *S) TestPeerReconnects(c *C) {
	creds := TestCredentials(c)
	store := newPeerStore()
	// Have server go through a proxy so its connection can be manipulated
	upstream := listen(c)
	local := listen(c)
	log := r.Logger.WithField("test", "PeerReconnects")
	proxyAddr := local.Addr().String()
	proxyLink := proxy.New(proxy.NetLink{Local: local, Upstream: upstream.Addr().String()}, log)
	c.Assert(proxyLink.Start(), IsNil)

	srv, err := New(Config{
		FieldLogger:     log.WithField("server", upstream.Addr()),
		Credentials:     creds,
		PeerStore:       store,
		Listener:        upstream,
		commandExecutor: testCommand{"server output"},
	})
	c.Assert(err, IsNil)
	go func() {
		c.Assert(srv.Serve(), IsNil)
	}()
	defer withTestCtx(srv.Stop, c)

	watchCh := make(chan WatchEvent, 2)
	checkTimeout := 100 * time.Millisecond
	config := PeerConfig{
		Config:             Config{Listener: listen(c)},
		WatchCh:            watchCh,
		HealthCheckTimeout: checkTimeout,
	}
	p := r.newPeer(c, config, proxyAddr, log)
	go func() {
		c.Assert(p.Serve(), IsNil)
	}()
	defer withTestCtx(p.Stop, c)

	ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	c.Assert(store.expect(ctx, 1), IsNil)
	cancel()

	select {
	case update := <-watchCh:
		c.Assert(update.Error, IsNil)
		if update.Peer.Addr() != proxyAddr {
			c.Errorf("unknown peer %v", update)
		}
	case <-time.After(5 * time.Second):
		c.Error("timeout waiting for reconnect")
	}

	// Drop connection to server
	proxyLink.Stop()
	local.Close()
	// Give the transport enough time to fail. If the interval between reconnects
	// is negligible, the transport might recover and reconnect
	// to the second instance of the proxy bypassing the failed health check.
	time.Sleep(checkTimeout)

	// Restore connection to server
	local = listenAddr(proxyAddr, c)
	proxyLink = proxy.New(proxy.NetLink{Local: local, Upstream: upstream.Addr().String()}, log)
	c.Assert(proxyLink.Start(), IsNil)
	defer proxyLink.Stop()

	select {
	case update := <-watchCh:
		c.Assert(update.Error, IsNil)
		if update.Peer.Addr() != proxyAddr {
			c.Errorf("unknown peer %v", update)
		}
		// Reconnected
	case <-time.After(5 * time.Second):
		c.Error("timeout waiting for reconnect")
	}
}

// withTestCtx calls the provided method passing it a test context with a timeout
func withTestCtx(fn func(context.Context) error, c *C) {
	ctx, cancel := context.WithTimeout(context.Background(), testContextTimeout)
	defer cancel()
	c.Assert(fn(ctx), IsNil)
}

// testContextTimeout is the default timeout for the context used in tests
const testContextTimeout = 2 * time.Second
