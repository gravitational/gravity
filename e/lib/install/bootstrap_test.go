package install

import (
	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/ops/suite"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

type BootstrapSuite struct{}

var _ = check.Suite(&BootstrapSuite{})

func (s *BootstrapSuite) TestEnsureApp(c *check.C) {
	// setup
	wizardServices := opsservice.SetupTestServices(c)
	opsServices := opsservice.SetupTestServices(c)
	appPackage := suite.SetUpTestPackage(c, opsServices.Apps, opsServices.Packages)
	req := service.AppPullRequest{
		FieldLogger: logrus.WithField(trace.Component, "bootstrap-suite"),
		SrcPack:     opsServices.Packages,
		DstPack:     wizardServices.Packages,
		SrcApp:      opsServices.Apps,
		DstApp:      wizardServices.Apps,
		Package:     appPackage,
	}
	// initially there's no app in the wizard database
	_, err := install.GetApp(wizardServices.Apps)
	c.Assert(err, check.FitsTypeOf, trace.NotFound(""))
	// download it from Ops Center
	_, err = EnsureApp(req)
	c.Assert(err, check.IsNil)
	// verify application package existence
	application, err := install.GetApp(wizardServices.Apps)
	c.Assert(err, check.IsNil)
	c.Assert(application.Package, check.DeepEquals, appPackage)
}
