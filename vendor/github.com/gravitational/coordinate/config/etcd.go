/*
Copyright 2016 Gravitational, Inc.

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

package config

import (
	"time"

	"github.com/gravitational/trace"

	etcd "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
)

const (
	// DefaultResponseTimeout specifies the default time limit to wait for response
	// header in a single request made by an etcd client
	DefaultResponseTimeout = 1 * time.Second
	// DefaultDialTimeout is default TCP connect timeout
	DefaultDialTimeout = 30 * time.Second
)

// Config defines the configuration to access etcd
type Config struct {
	// Endpoints lists etcd server endpoints (http://host:port)
	Endpoints []string
	// CAFile defines the SSL Certificate Authority file to used
	// to secure etcd communication
	CAFile string
	// CertFile defines the SSL certificate file to use to secure
	// etcd communication
	CertFile string
	// KeyFile defines the SSL key file to use to secure etcd communication
	KeyFile string
	// HeaderTimeoutPerRequest specifies the time limit to wait for response
	// header in a single request made by a client
	HeaderTimeoutPerRequest time.Duration
	// DialTimeout is dial timeout
	DialTimeout time.Duration
}

func (r *Config) CheckAndSetDefaults() error {
	if len(r.Endpoints) == 0 {
		return trace.BadParameter("need at least one endpoint")
	}
	if r.HeaderTimeoutPerRequest == 0 {
		r.HeaderTimeoutPerRequest = DefaultResponseTimeout
	}
	if r.DialTimeout == 0 {
		r.HeaderTimeoutPerRequest = DefaultResponseTimeout
	}
	return nil
}

// NewClient creates a new instance of an etcd client
func (r *Config) NewClient() (etcd.Client, error) {
	info := transport.TLSInfo{
		CertFile: r.CertFile,
		KeyFile:  r.KeyFile,
		CAFile:   r.CAFile,
	}
	transport, err := transport.NewTransport(info, 30*time.Second)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := etcd.New(etcd.Config{
		Endpoints:               r.Endpoints,
		Transport:               transport,
		HeaderTimeoutPerRequest: r.HeaderTimeoutPerRequest,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}
