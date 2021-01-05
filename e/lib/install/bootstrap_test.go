package install

import (
	"context"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/ops/suite"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

type BootstrapSuite struct {
	installer *Installer
}

var _ = check.Suite(&BootstrapSuite{})

func (s *BootstrapSuite) SetUpSuite(c *check.C) {
	wizardServices := opsservice.SetupTestServices(c)
	opsServices := opsservice.SetupTestServices(c)
	application := suite.SetUpTestPackage(c, opsServices.Apps, opsServices.Packages)
	s.installer = &Installer{
		Installer: &install.Installer{
			FieldLogger: logrus.WithField(trace.Component, "bootstrap-suite"),
			AppPackage:  application,
			Apps:        wizardServices.Apps,
			Packages:    wizardServices.Packages,
		},
		Remote: &environment.Remote{
			RemoteEnvironment: &localenv.RemoteEnvironment{
				Apps:     opsServices.Apps,
				Packages: opsServices.Packages,
			},
		},
	}
}

func (s *BootstrapSuite) TestEnsureApp(c *check.C) {
	// initially there's no app in the wizard database
	_, err := install.GetApp(s.installer.Apps)
	c.Assert(err, check.FitsTypeOf, trace.NotFound(""))
	// download it from Ops Center
	err = s.installer.ensureApp(context.TODO())
	c.Assert(err, check.IsNil)
	// now it should be there
	application, err := install.GetApp(s.installer.Apps)
	c.Assert(err, check.IsNil)
	c.Assert(application.Package, check.DeepEquals, s.installer.AppPackage)
}
