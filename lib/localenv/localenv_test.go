package localenv

import (
	"io/ioutil"
	"sort"
	"testing"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func TestEnvironments(t *testing.T) { check.TestingT(t) }

type LocalEnvSuite struct{}

var _ = check.Suite(&LocalEnvSuite{})

func (s *LocalEnvSuite) TestLocalEnv(c *check.C) {
	_, err := NewLocalEnvironment(LocalEnvironmentArgs{
		StateDir:         "",
		LocalKeyStoreDir: "",
		Insecure:         true,
	})
	c.Assert(err, check.NotNil)

}

func (s *LocalEnvSuite) TestParsedUnparsedOpsCenters(c *check.C) {
	opsCenter := "https://fake1.opscenter.gravitational"
	parsedOpsCenter := utils.ParseOpsCenterAddress(opsCenter, defaults.HTTPSPort)
	username := "username@opscenter.gravitational"
	password := "Password1"

	// single state dir
	stateDir := c.MkDir()
	env, err := NewLocalEnvironment(
		LocalEnvironmentArgs{
			LocalKeyStoreDir: c.MkDir(),
			StateDir:         stateDir,
		})
	c.Assert(err, check.IsNil)

	login, err := env.GetLoginEntry(opsCenter)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
	c.Assert(login, check.IsNil)

	login, err = env.GetLoginEntry(parsedOpsCenter)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
	c.Assert(login, check.IsNil)

	err = env.UpsertLoginEntry(opsCenter, username, password)
	c.Assert(err, check.IsNil)

	login, err = env.GetLoginEntry(opsCenter)
	c.Assert(login, check.NotNil)
	c.Assert(err, check.IsNil)

	login, err = env.GetLoginEntry(parsedOpsCenter)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
	c.Assert(login, check.IsNil)

	err = env.UpsertLoginEntry(parsedOpsCenter, username, password)
	c.Assert(err, check.IsNil)

	login, err = env.GetLoginEntry(opsCenter)
	c.Assert(login, check.NotNil)
	c.Assert(err, check.IsNil)

	login, err = env.GetLoginEntry(parsedOpsCenter)
	c.Assert(login, check.NotNil)
	c.Assert(err, check.IsNil)

	files, _ := ioutil.ReadDir(stateDir)
	fileNames := []string{}
	for _, f := range files {
		fileNames = append(fileNames, f.Name())
	}
	sort.Strings(fileNames)
	result := []string{defaults.GravityDBFile, defaults.PackagesDir}
	sort.Strings(result)

	// packages + db end up here, auth stuff ends up in ~
	c.Assert(fileNames, check.DeepEquals, result)

	err = env.Close()
	c.Assert(err, check.IsNil)
}

func (s *LocalEnvSuite) TestLocalEnvSingleStateDir(c *check.C) {
	opsCenter := "https://fake2.opscenter.gravitational"
	username := "username@opscenter.gravitational"
	password := "Password1"

	// single state dir
	stateDir := c.MkDir()
	env, err := NewLocalEnvironment(
		LocalEnvironmentArgs{
			LocalKeyStoreDir: c.MkDir(),
			StateDir:         stateDir,
		})
	c.Assert(err, check.IsNil)

	login, err := env.GetLoginEntry(opsCenter)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
	c.Assert(login, check.IsNil)

	err = env.UpsertLoginEntry(opsCenter, username, password)
	c.Assert(err, check.IsNil)

	login, err = env.GetLoginEntry(opsCenter)
	c.Assert(login, check.NotNil)
	c.Assert(err, check.IsNil)

	files, _ := ioutil.ReadDir(stateDir)
	fileNames := []string{}
	for _, f := range files {
		fileNames = append(fileNames, f.Name())
	}
	sort.Strings(fileNames)
	result := []string{defaults.GravityDBFile, defaults.PackagesDir}
	sort.Strings(result)

	// packages + db end up here, auth stuff ends up in ~
	c.Assert(fileNames, check.DeepEquals, result)

	err = env.Close()
	c.Assert(err, check.IsNil)
}

func (s *LocalEnvSuite) TestLocalEnvSeparateStateAndKeyStoreDir(c *check.C) {
	opsCenter := "https://fake3.opscenter.gravitational"
	username := "username@opscenter.gravitational"
	password := "Password1"

	// separate state + auth
	stateDir := c.MkDir()
	keyDir := c.MkDir()

	env, err := NewLocalEnvironment(
		LocalEnvironmentArgs{
			StateDir:         stateDir,
			LocalKeyStoreDir: keyDir,
		})
	c.Assert(err, check.IsNil)

	login, err := env.GetLoginEntry(opsCenter)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
	c.Assert(login, check.IsNil)

	err = env.UpsertLoginEntry(opsCenter, username, password)
	c.Assert(err, check.IsNil)

	login, err = env.GetLoginEntry(opsCenter)
	c.Assert(login, check.NotNil)
	c.Assert(err, check.IsNil)

	err = env.Close()
	c.Assert(err, check.IsNil)
	files, _ := ioutil.ReadDir(stateDir)
	fileNames := []string{}
	for _, f := range files {
		fileNames = append(fileNames, f.Name())
	}
	sort.Strings(fileNames)
	result := []string{defaults.GravityDBFile, defaults.PackagesDir}
	sort.Strings(result)

	// packages + db end up here
	c.Assert(fileNames, check.DeepEquals, result)

	files, _ = ioutil.ReadDir(keyDir)
	fileNames = []string{}
	for _, f := range files {
		fileNames = append(fileNames, f.Name())
	}
	// auth config exists here
	c.Assert(fileNames, check.DeepEquals, []string{defaults.LocalConfigFile})
}
