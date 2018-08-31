package clients

import (
	"time"

	"github.com/gravitational/gravity/lib/app/client"
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
	// DialTimeout is etcd dial timeout
	DialTimeout time.Duration
}

// CheckAndSetDefaults checks etcd client configuration and sets default values
func (c *EtcdConfig) CheckAndSetDefaults() error {
	if len(c.Endpoints) == 0 {
		c.Endpoints = []string{defaults.EtcdLocalAddr}
	}
	if c.CAFile == "" {
		c.CAFile = defaults.Secret(defaults.RootCertFilename)
	}
	if c.CertFile == "" {
		c.CertFile = defautls.Secret(defaults.EtcdCertFilename)
	}
	if c.KeyFile == "" {
		c.KeyFile = defaults.Secret(defaults.EtcdKeyFilename)
	}
	if c.DialTimeout == 0 {
		c.DialTimeout = defaults.DialTimeout
	}
	return nil
}

// Etcd returns a new instance of bare bones etcd client
func Etcd(config *EtcdConfig) (client.Client, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	transport, err := transport.NewTransport(transport.TLSInfo{
		CAFile:   config.CAFile,
		CertFile: config.CertFile,
		KeyFile:  config.KeyFile,
	}, config.DialTimeout)
	if err != nil {
		return trace.Wrap(err)
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
