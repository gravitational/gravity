package localenv

import (
	"github.com/gravitational/gravity/lib/storage"

	"gopkg.in/check.v1"
)

type RemoteEnvSuite struct{}

var _ = check.Suite(&RemoteEnvSuite{})

func (s *RemoteEnvSuite) TestAutoLogin(c *check.C) {
	dir := c.MkDir()
	// initially there are no login entries so there's nowhere to log into
	env, err := newRemoteEnvironment(dir)
	c.Assert(err, check.IsNil)
	c.Assert(env.Packages, check.IsNil)
	c.Assert(env.Apps, check.IsNil)
	c.Assert(env.Operator, check.IsNil)
	// log into a wizard process
	_, err = env.LoginWizard("https://192.168.1.1:61009")
	c.Assert(err, check.IsNil)
	// make sure services have been initialized
	c.Assert(env.Packages, check.NotNil)
	c.Assert(env.Apps, check.NotNil)
	c.Assert(env.Operator, check.NotNil)
	// make sure a new remote environment is logged in automatically
	env2, err := newRemoteEnvironment(dir)
	c.Assert(err, check.IsNil)
	c.Assert(env2.Packages, check.NotNil)
	c.Assert(env2.Apps, check.NotNil)
	c.Assert(env2.Operator, check.NotNil)
}

func (s *RemoteEnvSuite) TestWizardEntryCleanup(c *check.C) {
	dir := c.MkDir()
	env, err := newRemoteEnvironment(dir)
	c.Assert(err, check.IsNil)
	// log into a wizard
	_, err = env.LoginWizard("https://192.168.1.1:61009")
	c.Assert(err, check.IsNil)
	entry, err := env.wizardEntry()
	c.Assert(err, check.IsNil)
	c.Assert(entry.OpsCenterURL, check.Equals, "https://192.168.1.1:61009")
	// log into another, should override
	_, err = env.LoginWizard("https://192.168.1.2:61009")
	c.Assert(err, check.IsNil)
	entry, err = env.wizardEntry()
	c.Assert(err, check.IsNil)
	c.Assert(entry.OpsCenterURL, check.Equals, "https://192.168.1.2:61009")
	// make sure there's only 1 entry in the database indeed
	err = env.withBackend(func(b storage.Backend) error {
		entries, err := b.GetLoginEntries()
		c.Assert(err, check.IsNil)
		c.Assert(len(entries), check.Equals, 1)
		return nil
	})
	c.Assert(err, check.IsNil)
}
