package process

import (
	"os"
	"strconv"
	"testing"

	"github.com/gravitational/gravity/e/lib/ops/service"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func TestProcess(t *testing.T) { check.TestingT(t) }

type MigrateSuite struct{}

var _ = check.Suite(&MigrateSuite{})

func (s *MigrateSuite) SetUpTest(c *check.C) {
	testEnabled := os.Getenv(defaults.TestK8s)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		c.Skip("skipping Kubernetes test")
	}
}

func (s *MigrateSuite) TestMigrateLicense(c *check.C) {
	client, _, err := utils.GetLocalKubeClient()
	c.Assert(err, check.IsNil)

	// create the license config map
	err = service.InstallLicenseConfigMap(client, "license")
	c.Assert(err, check.IsNil)

	// execute migration and make sure that secret is there and config map is gone
	err = migrateLicense(client)
	c.Assert(err, check.IsNil)

	fromSecret, err := service.GetLicenseFromSecret(client)
	c.Assert(err, check.IsNil)
	c.Assert(fromSecret, check.Equals, "license")

	fromConfigMap, err := service.GetLicenseFromConfigMap(client)
	c.Assert(trace.IsNotFound(err), check.Equals, true)
	c.Assert(fromConfigMap, check.Equals, "")
}
