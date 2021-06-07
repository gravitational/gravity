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

package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
)

// Pool manages pool of authenticated HTTP peer clients,
// used by BLOB cluster storage to replicate data
type Pool struct {
	sync.Mutex
	clients  map[string]*Client
	username string
	password string
}

// GetPeer returns new instance of the Objects client
func (p *Pool) GetPeer(peer storage.Peer) (blob.Objects, error) {
	p.Lock()
	defer p.Unlock()
	if p.clients[peer.ID] != nil {
		return p.clients[peer.ID], nil
	}
	// TODO(klizhentas) I will turn on proper CA checking in subsequent PRs
	insecureClient := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: defaults.DialTimeout}).DialContext,
			TLSClientConfig: &tls.Config{
				//nolint:gosec // TODO: fix insecure
				InsecureSkipVerify: true,
			},
			TLSHandshakeTimeout:   defaults.DialTimeout,           // 30s
			IdleConnTimeout:       defaults.ConnectionIdleTimeout, // 2m
			ResponseHeaderTimeout: defaults.ReadHeadersTimeout,    // 30s
		},
	}
	client, err := NewPeerAuthenticatedClient(peer.AdvertiseAddr, p.username,
		p.password, roundtrip.HTTPClient(insecureClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.clients[peer.ID] = client
	return client, nil
}

// NewPool returns new instance of the pool managing connections to the peers
func NewPool(username, password string) (*Pool, error) {
	if username == "" || password == "" {
		return nil, trace.BadParameter("missing username or password")
	}
	return &Pool{username: username, password: password, clients: map[string]*Client{}}, nil
}

const CurrentVersion = "objects/v1"

// Client is HTTP client to BLOB storage
type Client struct {
	roundtrip.Client
}

// NewPeerAuthenticatedClient returns client authenticated as a user with given password
func NewPeerAuthenticatedClient(addr, username, password string, params ...roundtrip.ClientParam) (*Client, error) {
	params = append(params, roundtrip.BasicAuth(username, password))
	return NewPeerClient(addr, params...)
}

// NewPeerClient returns new client that communicates with peer-local interface
func NewPeerClient(addr string, params ...roundtrip.ClientParam) (*Client, error) {
	c, err := roundtrip.NewClient(addr, CurrentVersion+"/local", params...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Client{*c}, nil
}

// NewAuthenticatedClient returns client authenticated as a user with given password
func NewAuthenticatedClient(addr, username, password string, params ...roundtrip.ClientParam) (*Client, error) {
	params = append(params, roundtrip.BasicAuth(username, password))
	return NewClient(addr, params...)
}

// NewClient returns HTTP client communicating to cluster BLOB service
func NewClient(addr string, params ...roundtrip.ClientParam) (*Client, error) {
	c, err := roundtrip.NewClient(addr, CurrentVersion+"/cluster", params...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Client{*c}, nil
}

func (c *Client) Close() error {
	return nil
}

func (c *Client) DeleteBLOB(hash string) error {
	_, err := c.Delete(c.Endpoint("blobs", hash))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *Client) GetBLOBs() ([]string, error) {
	out, err := c.Get(c.Endpoint("blobs"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var hashes []string
	if err := json.Unmarshal(out.Bytes(), &hashes); err != nil {
		return nil, trace.Wrap(err)
	}
	return hashes, nil
}

func (c *Client) GetBLOBEnvelope(hash string) (*blob.Envelope, error) {
	out, err := c.Get(c.Endpoint("blobs", hash, "envelope"), url.Values{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var envelope blob.Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		return nil, trace.Wrap(err)
	}
	return &envelope, nil
}

func (c *Client) WriteBLOB(data io.Reader) (*blob.Envelope, error) {
	file := roundtrip.File{
		Name:     "file",
		Filename: "file",
		Reader:   data,
	}
	out, err := c.PostForm(c.Endpoint("blobs"), url.Values{}, file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var envelope *blob.Envelope
	if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
		return nil, trace.Wrap(err)
	}
	return envelope, nil
}

func (c *Client) OpenBLOB(hash string) (blob.ReadSeekCloser, error) {
	endpoint := c.Endpoint("blobs", hash)

	_, err := telehttplib.ConvertResponse(c.RoundTrip(func() (*http.Response, error) {
		//nolint:noctx	// TODO: use context
		req, err := http.NewRequest("HEAD", endpoint, nil)
		if err != nil {
			return nil, err
		}
		c.SetAuthHeader(req.Header)
		return c.HTTPClient().Do(req)
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.OpenFile(context.TODO(), endpoint, url.Values{})
}

// PostForm is a generic method that issues http POST request to the server
func (c *Client) PostForm(
	endpoint string,
	vals url.Values,
	files ...roundtrip.File) (*roundtrip.Response, error) {

	return telehttplib.ConvertResponse(
		c.Client.PostForm(context.TODO(), endpoint, vals, files...))
}

// Get issues http GET request to the server
func (c *Client) Get(u string, params url.Values) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.Get(context.TODO(), u, params))
}

// Delete issues http DELETE Request to the server
func (c *Client) Delete(u string) (*roundtrip.Response, error) {
	return telehttplib.ConvertResponse(c.Client.Delete(context.TODO(), u))
}
