package periodic

import (
	"os"
	"testing"
	"time"

	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
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
	dir          string

	clock *timetools.FreezedTime

	localServices  opsservice.TestServices
	remoteServices opsservice.TestServices
}

var _ = Suite(&StateCheckerSuite{
	clock: &timetools.FreezedTime{
		CurrentTime: time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC),
	},
})

func (s *StateCheckerSuite) SetUpSuite(c *C) {
	s.localServices = opsservice.SetupTestServices(c)
	apptest.CreateRuntimeApplication(s.localServices.Apps, c)
	s.remoteServices = opsservice.SetupTestServices(c)
	apptest.CreateRuntimeApplication(s.remoteServices.Apps, c)
	s.stateChecker = stateChecker{
		conf: StateCheckerConfig{
			Backend:  s.localServices.Backend,
			Operator: s.remoteServices.Operator,
			Packages: s.localServices.Packages,
			Tunnel:   testutils.FakeReverseTunnel{},
		},
		FieldLogger: logrus.WithField(trace.Component, "state-checker"),
	}
	s.dir = s.localServices.Dir
}

func (s *StateCheckerSuite) TearDownSuite(c *C) {
	os.RemoveAll(s.dir)
}

func (s *StateCheckerSuite) TestStateChecker(c *C) {
	locApp1 := loc.MustParseLocator("local.com/app:1.0.0")
	app1 := apptest.CreateDummyApplication(s.localServices.Apps, locApp1, c)

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

	locApp2 := loc.MustParseLocator("local.com/app:1.0.2")
	app2 := apptest.CreateDummyApplication(s.localServices.Apps, locApp2, c)
	apptest.CreateDummyApplication(s.remoteServices.Apps, locApp2, c)

	// create the same site using remote services but with new app version
	// to emulate update
	_, err = s.remoteServices.Backend.CreateSite(storage.Site{
		AccountID: "account123",
		State:     ops.SiteStateActive,
		Domain:    "local.com",
		Created:   s.clock.UtcNow(),
		App:       app2.PackageEnvelope.ToPackage(),
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
