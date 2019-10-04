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

package monitoring

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/gravitational/trace"
)

// ETCDConfig defines a set of configuration parameters for accessing
// etcd endpoints
type ETCDConfig struct {
	// Endpoints lists etcd server endpoints
	Endpoints []string
	// CAFile is an SSL Certificate Authority file used to secure
	// communication with etcd
	CAFile string
	// CertFile is an SSL certificate file used to secure
	// communication with etcd
	CertFile string
	// KeyFile is an SSL key file used to secure communication with etcd
	KeyFile string
	// InsecureSkipVerify controls whether a client verifies the
	// server's certificate chain and host name.
	InsecureSkipVerify bool
	// Client specifies the optional HTTP client to use for checks.
	// If unspecified, one will be created with default settings
	Client *http.Client
}

// defaultHTTPTimeout defines the default HTTP client timeout for HTTP-based checks
const defaultHTTPTimeout = 10 * time.Second

// defaultTLSHandshakeTimeout specifies the default maximum amount of time
// spent waiting to for a TLS handshake
const defaultTLSHandshakeTimeout = 10 * time.Second

// defaultDialTimeout is the default maximum amount of time a dial will wait for
// a connect to complete.
const defaultDialTimeout = 30 * time.Second

// defaultKeepAlive specifies the default keep-alive period for an active
// network connection.
const defaultKeepAlivePeriod = 30 * time.Second

// etcdChecker is an HTTPResponseChecker that interprets results from
// an etcd HTTP-based healthz end-point.
func etcdChecker(response io.Reader) error {
	payload, err := ioutil.ReadAll(response)
	if err != nil {
		return trace.Wrap(err)
	}

	healthy, err := etcdStatus(payload)
	if err != nil {
		return trace.Wrap(err)
	}

	if !healthy {
		return trace.Errorf("unexpected etcd status: %s", payload)
	}
	return nil
}

// etcdStatus determines if the specified etcd status value
// indicates a healthy service
func etcdStatus(payload []byte) (healthy bool, err error) {
	result := struct{ Health string }{}
	nresult := struct{ Health bool }{}
	err = json.Unmarshal(payload, &result)
	if err != nil {
		err = json.Unmarshal(payload, &nresult)
	}
	if err != nil {
		return false, trace.Wrap(err)
	}

	return (result.Health == "true" || nresult.Health == true), nil
}

// NewClient returns a new HTTP client with default configuration
func (r *ETCDConfig) NewClient() (*http.Client, error) {
	transport, err := r.NewHTTPTransport()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &http.Client{
		Transport: transport,
		Timeout:   defaultHTTPTimeout,
	}, nil
}

// NewHTTPTransport creates a new http.Transport from the specified
// set of attributes.
// The resulting transport can be used to create an http.Client
func (r *ETCDConfig) NewHTTPTransport() (*http.Transport, error) {
	tlsConfig, err := r.clientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   defaultDialTimeout,
			KeepAlive: defaultKeepAlivePeriod,
		}).Dial,
		TLSHandshakeTimeout: defaultTLSHandshakeTimeout,
		TLSClientConfig:     tlsConfig,
	}

	return transport, nil
}

// clientConfig generates a tls.Config object for use by an HTTP client.
func (r *ETCDConfig) clientConfig() (*tls.Config, error) {
	if r.empty() {
		return nil, nil
	}
	cert, err := ioutil.ReadFile(r.CertFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := ioutil.ReadFile(r.KeyFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsCert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		MinVersion:         tls.VersionTLS10,
		InsecureSkipVerify: r.InsecureSkipVerify,
	}
	config.RootCAs, err = newCertPool([]string{r.CAFile})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// Empty determines if the configuration is empty
func (r *ETCDConfig) empty() bool {
	return r.CAFile == "" && r.CertFile == "" && r.KeyFile == ""
}

// newCertPool creates x509 certPool with provided CA files.
func newCertPool(CAFiles []string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()

	for _, CAFile := range CAFiles {
		pemByte, err := ioutil.ReadFile(CAFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for {
			var block *pem.Block
			block, pemByte = pem.Decode(pemByte)
			if block == nil {
				break
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			certPool.AddCert(cert)
		}
	}

	return certPool, nil
}
