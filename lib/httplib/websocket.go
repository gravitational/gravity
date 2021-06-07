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

package httplib

import (
	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

// WebsocketClientForURL creates a new websocket client for the specified url
// using provided headers
func WebsocketClientForURL(URL string, headers http.Header) (*websocket.Conn, error) {
	url, err := url.Parse(URL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if url.Scheme == "http" {
		url.Scheme = "ws"
	} else {
		url.Scheme = "wss"
	}
	origin := headers.Get("Origin")
	if origin == "" {
		origin = "http://localhost"
	}
	delete(headers, "Origin")
	conf, err := websocket.NewConfig(url.String(), origin)
	for key, value := range headers {
		conf.Header[key] = value
	}
	//nolint:gosec // TODO(klizhentas) remove insecure
	conf.TlsConfig = &tls.Config{InsecureSkipVerify: true}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := websocket.DialConfig(conf)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// SetupWebsocketClient sets up roundtrip client to dial web socket method
func SetupWebsocketClient(ctx context.Context, c *roundtrip.Client, endpoint string, localDialer Dialer) (io.ReadCloser, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else {
		u.Scheme = "wss"
	}
	// TODO(klizhentas) fix origin
	wscfg, err := websocket.NewConfig(u.String(), "http://localhost")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	//nolint:gosec // TODO(klizhentas) fix insecure config
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	wscfg.TlsConfig = tlsConfig
	c.SetAuthHeader(wscfg.Header)
	transport, ok := c.HTTPClient().Transport.(*http.Transport)
	if !ok {
		transport = &http.Transport{}
	}
	dial := transport.DialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	conn, err := dial(ctx, "tcp", u.Host)
	if err != nil {
		// try to dial using local resolver in case of error
		log.Warningf("got error, re-dialing with local resolver: %v", trace.DebugReport(err))
		conn, err = localDialer(ctx, "tcp", u.Host)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if u.Scheme == "ws" {
		clt, err := websocket.NewClient(wscfg, conn)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return clt, nil
	}

	tlsConn := tls.Client(conn, wscfg.TlsConfig)
	errC := make(chan error, 2)
	timer := time.AfterFunc(defaults.DialTimeout, func() {
		errC <- trace.ConnectionProblem(nil, "handshake timeout")
	})
	go func() {
		err := tlsConn.Handshake()
		if timer != nil {
			timer.Stop()
		}
		errC <- err
	}()
	if err := <-errC; err != nil {
		conn.Close()
		return nil, trace.Wrap(err)
	}
	clt, err := websocket.NewClient(wscfg, tlsConn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clt, nil
}

// WebSocketReader is a web socket handler,
// ignores all input and streams everything it read from reader
// to the web socket connection
type WebSocketReader struct {
	Reader io.ReadCloser
}

func (w *WebSocketReader) Close() error {
	if w.Reader != nil {
		return w.Reader.Close()
	}
	return nil
}

func (w *WebSocketReader) handleConnection(ws *websocket.Conn) {
	defer ws.Close()
	closeC := make(chan error, 2)
	go func() {
		_, err := io.Copy(ws, w.Reader)
		closeC <- err
	}()

	go func() {
		_, err := io.Copy(ioutil.Discard, ws)
		closeC <- err
	}()

	<-closeC
}

func (w *WebSocketReader) Handler() http.Handler {
	// TODO(klizhentas)
	// we instantiate a server explicitly here instead of using
	// websocket.HandlerFunc to set empty origin checker
	// make sure we check origin when in prod mode
	return &websocket.Server{
		Handler: w.handleConnection,
	}
}
