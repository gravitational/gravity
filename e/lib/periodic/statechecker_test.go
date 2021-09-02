// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package periodic

import (
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/app"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/testutils"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	"github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestPeriodic(t *testing.T) { TestingT(t) }

type StateCheckerSuite struct {
	stateChecker stateChecker

	clock *timetools.FreezedTime

	localServices  opsservice.TestServices
	remoteServices opsservice.TestServices
}

var _ = Suite(&StateCheckerSuite{
	clock: &timetools.FreezedTime{
		CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
	},
})

func (s *StateCheckerSuite) SetUpTest(c *C) {
	s.localServices = opsservice.SetupTestServices(c)
	s.remoteServices = opsservice.SetupTestServices(c)
	s.stateChecker = stateChecker{
		conf: StateCheckerConfig{
			Backend:  s.localServices.Backend,
			Operator: s.remoteServices.Operator,
			Packages: s.localServices.Packages,
			Tunnel:   testutils.FakeReverseTunnel{},
		},
		FieldLogger: logrus.WithField(trace.Component, "state-checker"),
	}
}

func (s *StateCheckerSuite) TestStateChecker(c *C) {
	runtimeApp1 := apptest.RuntimeApplication(loc.MustParseLocator("gravitational.io/runtime:0.0.1"),
		loc.MustParseLocator("gravitational.io/planet:0.0.1")).Build()
	app1 := apptest.CreateApplication(apptest.AppRequest{
		App:      apptest.ClusterApplication(loc.MustParseLocator("gravitational.io/app:1.0.0"), runtimeApp1).Build(),
		Packages: s.localServices.Packages,
		Apps:     s.localServices.Apps,
	}, c)

	site, err := s.localServices.Backend.CreateSite(storage.Site{
		AccountID: "account123",
		State:     ops.SiteStateActive,
		Domain:    "local.com",
		Created:   s.clock.UtcNow(),
		App:       app1.PackageEnvelope.ToPackage(),
	})
	c.Assert(err, IsNil)

	// make sure remote tunnel says the site is offline
	s.stateChecker.conf.Tunnel = testutils.FakeReverseTunnel{
		Sites: []testutils.FakeRemoteSite{
			{
				Name:   "local.com",
				Status: teleport.RemoteClusterStatusOffline,
			},
		},
	}

	err = s.stateChecker.checkSite(*site)
	c.Assert(err, IsNil)

	// reload the local site and check that it has become offline
	site, err = s.localServices.Backend.GetSite("local.com")
	c.Assert(err, IsNil)
	c.Assert(site.State, Equals, ops.SiteStateOffline)

	locApp2 := loc.MustParseLocator("gravitational.io/app:1.0.2")
	runtimeApp2 := apptest.RuntimeApplication(loc.MustParseLocator("gravitational.io/runtime:0.0.2"),
		loc.MustParseLocator("gravitational.io/planet:0.0.2")).Build()
	clusterApp2 := apptest.ClusterApplication(locApp2, runtimeApp2).Build()
	apptest.CreateApplication(apptest.AppRequest{
		App:         clusterApp2,
		PackageSets: []pack.PackageService{s.localServices.Packages, s.remoteServices.Packages},
		AppSets:     []app.Applications{s.localServices.Apps, s.remoteServices.Apps},
	}, c)
	app2, err := s.localServices.Apps.GetApp(locApp2)
	c.Assert(err, IsNil)

	// create the same site using remote services but with new app version
	// to emulate update
	_, err = s.remoteServices.Backend.CreateSite(storage.Site{
		AccountID: "account123",
		State:     ops.SiteStateActive,
		Domain:    "local.com",
		Created:   s.clock.UtcNow(),
		App:       app2.PackageEnvelope.ToPackage(),
		ClusterState: storage.ClusterState{
			Servers: storage.Servers{
				{
					AdvertiseIP: "192.168.1.1",
					Hostname:    "node-1",
					Role:        "worker",
				},
			},
		},
	})
	c.Assert(err, IsNil)

	// add the site to the reverse tunnel now
	s.stateChecker.conf.Tunnel = testutils.FakeReverseTunnel{
		Sites: []testutils.FakeRemoteSite{
			{
				Name:   "local.com",
				Status: teleport.RemoteClusterStatusOnline,
			},
		},
	}

	// run state checker and it should sync the state and app version with remote
	err = s.stateChecker.checkSite(*site)
	c.Assert(err, IsNil)

	// reload the sites and check the local site has been properly updated
	remoteSite, err := s.remoteServices.Backend.GetSite("local.com")
	c.Assert(err, IsNil)
	site, err = s.localServices.Backend.GetSite("local.com")
	c.Assert(err, IsNil)

	c.Assert(site, DeepEquals, remoteSite)
}
