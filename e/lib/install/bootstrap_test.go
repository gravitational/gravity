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
