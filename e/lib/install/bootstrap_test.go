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
