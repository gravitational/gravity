package process

import (
	"context"
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"

	telecfg "github.com/gravitational/teleport/lib/config"
	teleservices "github.com/gravitational/teleport/lib/services"
	"gopkg.in/check.v1"
)

type ProcessSuite struct{}

var _ = check.Suite(&ProcessSuite{})

func (s *ProcessSuite) TestClusterServices(c *check.C) {
	p := Process{
		context: context.TODO(),
	}

	// initially no services are running
	c.Assert(p.clusterServicesRunning(), check.Equals, false)

	service1Launched := make(chan bool)
	service1Done := make(chan bool)
	service1 := func(ctx context.Context) error {
		close(service1Launched)
		defer close(service1Done)
		select {
		case <-ctx.Done():
			return nil
		}
	}

	service2Launched := make(chan bool)
	service2Done := make(chan bool)
	service2 := func(ctx context.Context) error {
		close(service2Launched)
		defer close(service2Done)
		select {
		case <-ctx.Done():
			return nil
		}
	}

	// launch services
	err := p.startClusterServices([]clusterService{
		service1,
		service2,
	})
	c.Assert(err, check.IsNil)
	for i, ch := range []chan bool{service1Launched, service2Launched} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			c.Fatalf("service%v wasn't launched", i+1)
		}
	}
	c.Assert(p.clusterServicesRunning(), check.Equals, true)

	// should not attempt to launch again
	err = p.startClusterServices([]clusterService{
		service1,
		service2,
	})
	c.Assert(err, check.NotNil)
	c.Assert(p.clusterServicesRunning(), check.Equals, true)

	// stop services
	err = p.stopClusterServices()
	c.Assert(err, check.IsNil)
	for i, ch := range []chan bool{service1Done, service2Done} {
		select {
		case <-ch:
		case <-time.After(time.Second):
			c.Fatalf("service%v wasn't stopped", i+1)
		}
	}
	c.Assert(p.clusterServicesRunning(), check.Equals, false)

	// should not attempt to stop again
	err = p.stopClusterServices()
	c.Assert(err, check.NotNil)
	c.Assert(p.clusterServicesRunning(), check.Equals, false)
}

func (s *ProcessSuite) TestReverseTunnelsFromTrustedClusters(c *check.C) {
	var testCases = []struct {
		clusters []teleservices.TrustedCluster
		tunnels  []telecfg.ReverseTunnel
		comment  string
	}{
		{
			clusters: nil,
			tunnels:  nil,
			comment:  "does nothing",
		},
		{
			clusters: []teleservices.TrustedCluster{
				storage.NewTrustedCluster("cluster1", storage.TrustedClusterSpecV2{
					Enabled:              false,
					ReverseTunnelAddress: "cluster1:3024",
					ProxyAddress:         "cluster1:443",
					Token:                "secret",
					Roles:                []string{constants.RoleAdmin},
				}),
			},
			tunnels: nil,
			comment: "ignores disabled clusters",
		},
		{
			clusters: []teleservices.TrustedCluster{
				storage.NewTrustedCluster("cluster1", storage.TrustedClusterSpecV2{
					Enabled:              true,
					ReverseTunnelAddress: "cluster1:3024",
					ProxyAddress:         "cluster1:443",
					Token:                "secret",
					Roles:                []string{constants.RoleAdmin},
				}),
				storage.NewTrustedCluster("cluster2", storage.TrustedClusterSpecV2{
					Enabled:              true,
					ReverseTunnelAddress: "cluster2:3024",
					ProxyAddress:         "cluster2:443",
					Token:                "secret",
					Roles:                []string{constants.RoleAdmin},
					Wizard:               true,
				}),
			},
			tunnels: []telecfg.ReverseTunnel{
				telecfg.ReverseTunnel{
					DomainName: "cluster1",
					Addresses:  []string{"cluster1:3024"},
				},
				telecfg.ReverseTunnel{
					DomainName: "cluster2",
					Addresses:  []string{"cluster2:3024"},
				},
			},
			comment: "considers all remote trusted clusters",
		},
	}
	for _, testCase := range testCases {
		backend, err := keyval.NewBolt(keyval.BoltConfig{
			Path: filepath.Join(c.MkDir(), "test.db"),
		})
		c.Assert(err, check.IsNil)
		for _, cluster := range testCase.clusters {
			err := backend.UpsertTrustedCluster(cluster)
			c.Assert(err, check.IsNil)
		}
		tunnels, err := reverseTunnelsFromTrustedClusters(backend)
		c.Assert(err, check.IsNil)
		c.Assert(tunnels, check.DeepEquals, testCase.tunnels, check.Commentf(testCase.comment))
	}
}
