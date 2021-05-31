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

package opsroute

import (
	"crypto/tls"
	"net/http"
	"sync"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/trace"
)

// ClientPool helps to manage connections and clients to remote ops centers
type ClientPool struct {
	ClientPoolConfig
	sync.Mutex
	clients map[string]ops.Operator
}

type ClientPoolConfig struct {
	Backend storage.Backend
	Devmode bool
}

func NewClientPool(config ClientPoolConfig) *ClientPool {
	return &ClientPool{
		clients:          make(map[string]ops.Operator),
		ClientPoolConfig: config,
	}
}

func (p *ClientPool) getClient(url string) ops.Operator {
	p.Lock()
	defer p.Unlock()
	return p.clients[url]
}

func (p *ClientPool) httpClient() *http.Client {
	client := &http.Client{}
	if p.Devmode {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				//nolint:gosec // TODO: fix insecure
				InsecureSkipVerify: true,
			},
		}
	}
	return client
}

func (p *ClientPool) newClient(url, username, password string) (ops.Operator, error) {
	// create remote package service client
	client, err := opsclient.NewAuthenticatedClient(
		url, username, password, opsclient.HTTPClient(p.httpClient()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

func (p *ClientPool) newClientFromLink(link storage.OpsCenterLink) (ops.Operator, error) {
	var err error
	var key *storage.APIKey
	if link.User != nil {
		key = &storage.APIKey{UserEmail: link.User.Email, Token: link.User.Token}
	} else {
		_, key, err = users.GetOpsCenterAgent(link.Hostname, link.SiteDomain, p.Backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	p.Lock()
	defer p.Unlock()
	client, ok := p.clients[link.APIURL]
	if ok {
		return client, nil
	}
	client, err = p.newClient(link.APIURL, key.UserEmail, key.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.clients[link.APIURL] = client
	return client, nil
}

func (p *ClientPool) GetService(link storage.OpsCenterLink) (ops.Operator, error) {
	if client := p.getClient(link.APIURL); client != nil {
		return client, nil
	}
	client, err := p.newClientFromLink(link)
	return client, trace.Wrap(err)
}
