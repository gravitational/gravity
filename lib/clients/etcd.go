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

package clients

import (
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	etcd "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/gravitational/trace"
)

// EtcdConfig is the etcd client configuration
type EtcdConfig struct {
	// Endpoints is etcd endpoints to connect to
	Endpoints []string
	// CAFile is path to etcd certificate authority
	CAFile string
	// CertFile is path to etcd certificate
	CertFile string
	// KeyFile is path to etcd private key
	KeyFile string
	// SecretsDir is the directory with etcd secrets
	SecretsDir string
	// DialTimeout is etcd dial timeout
	DialTimeout time.Duration
}

// CheckAndSetDefaults checks etcd client configuration and sets default values
func (c *EtcdConfig) CheckAndSetDefaults() error {
	if len(c.Endpoints) == 0 {
		c.Endpoints = []string{defaults.EtcdLocalAddr}
	}
	if c.SecretsDir == "" {
		c.SecretsDir = defaults.InGravity(defaults.SecretsDir)
	}
	if c.CAFile == "" {
		c.CAFile = filepath.Join(c.SecretsDir, defaults.RootCertFilename)
	}
	if c.CertFile == "" {
		c.CertFile = filepath.Join(c.SecretsDir, defaults.EtcdCertFilename)
	}
	if c.KeyFile == "" {
		c.KeyFile = filepath.Join(c.SecretsDir, defaults.EtcdKeyFilename)
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = defaults.DialTimeout
	}
	return nil
}

// Etcd returns a new instance of bare bones etcd client
func Etcd(config *EtcdConfig) (etcd.Client, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	transport, err := transport.NewTransport(transport.TLSInfo{
		CAFile:   config.CAFile,
		CertFile: config.CertFile,
		KeyFile:  config.KeyFile,
	}, config.DialTimeout)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return etcd.New(etcd.Config{
		Endpoints:               config.Endpoints,
		Transport:               transport,
		HeaderTimeoutPerRequest: time.Second,
	})
}

// EtcdMembers returns a new instance of etcd members API client
func EtcdMembers(config *EtcdConfig) (etcd.MembersAPI, error) {
	client, err := Etcd(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return etcd.NewMembersAPI(client), nil
}

// DefaultEtcdMembers returns etcd members API client with default configuration
func DefaultEtcdMembers() (etcd.MembersAPI, error) {
	return EtcdMembers(&EtcdConfig{})
}
