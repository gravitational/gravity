/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// package suite contains a ops service acceptance test suite that is backend
// implementation independent, used both for services and clients
package suite

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/gravitational/gravity/lib/app"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	. "gopkg.in/check.v1"
)

type OpsSuite struct {
	O       ops.Operator
	U       users.Users
	C       *timetools.FreezedTime
	testApp loc.Locator
}

type appTuple struct {
	Name string
	Path string
}

type packageTuple struct {
	Name string
	Path string
}

func SetUpTestPackage(c *C, apps app.Applications, packages pack.PackageService) loc.Locator {
	apptest.CreateRuntimeApplication(apps, c)
	app := apptest.CreateAppWithDeps(apps, packages, c)
	return app.Package
}

func (s *OpsSuite) SetUpTestPackage(apps app.Applications, packages pack.PackageService, c *C) (*loc.Locator, error) {
	s.testApp = SetUpTestPackage(c, apps, packages)
	return &s.testApp, nil
}

func (s *OpsSuite) AccountsCRUD(c *C) {
	accts, err := s.O.GetAccounts()
	c.Assert(err, IsNil)
	c.Assert(len(accts), Equals, 0)

	a, err := s.O.CreateAccount(ops.NewAccountRequest{
		Org: "example.com",
	})
	c.Assert(err, IsNil)
	c.Assert(a.Org, Equals, "example.com")

	accts, err = s.O.GetAccounts()
	c.Assert(err, IsNil)
	c.Assert(accts, DeepEquals, []ops.Account{*a})

	out, err := s.O.GetAccount(a.ID)
	c.Assert(err, IsNil)
	c.Assert(*out, DeepEquals, *a)

	_, err = s.O.GetAccount("some random string")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

func (s *OpsSuite) SitesCRUD(c *C) {
	a, err := s.O.CreateAccount(ops.NewAccountRequest{
		Org: "example.com",
	})
	c.Assert(err, IsNil)

	sites, err := s.O.GetSites(a.ID)
	c.Assert(err, IsNil)
	c.Assert(len(sites), Equals, 0)

	site, err := s.O.CreateSite(ops.NewSiteRequest{
		AppPackage: s.testApp.String(),
		AccountID:  a.ID,
		Provider:   schema.ProviderOnPrem,
		DomainName: "example.com",
	})
	c.Assert(err, IsNil)
	c.Assert(site.State, Equals, ops.SiteStateNotInstalled)

	siteKey := ops.SiteKey{
		SiteDomain: site.Domain,
		AccountID:  a.ID,
	}
	out, err := s.O.GetSite(siteKey)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, site)

	sites, err = s.O.GetSites(a.ID)
	c.Assert(err, IsNil)
	c.Assert(sites, DeepEquals, []ops.Site{*site})

	operations, err := s.O.GetSiteOperations(siteKey, ops.OperationsFilter{})
	c.Assert(err, IsNil)
	c.Assert(len(operations), Equals, 0)

	opKey, err := s.O.CreateSiteInstallOperation(context.TODO(), ops.CreateSiteInstallOperationRequest{
		AccountID:  a.ID,
		SiteDomain: site.Domain,
		Variables:  storage.OperationVariables{},
	})
	c.Assert(err, IsNil)
	c.Assert(opKey, NotNil)

	op, err := s.O.GetSiteOperation(*opKey)
	c.Assert(err, IsNil)
	c.Assert(op.Key(), Equals, *opKey)
	c.Assert(op.Type, Equals, ops.OperationInstall)

	// make sure newly created operation has the progress entry
	// associated with it right away
	progressEntry, err := s.O.GetSiteOperationProgress(*opKey)
	c.Assert(err, IsNil)
	c.Assert(progressEntry.State, Equals, ops.ProgressStateInProgress)

	logStream, err := s.O.GetSiteOperationLogs(*opKey)
	c.Assert(err, IsNil)
	c.Assert(logStream.Close(), IsNil)

	// download crashreport
	reportStream, err := s.O.GetSiteReport(context.TODO(), ops.GetClusterReportRequest{SiteKey: opKey.SiteKey()})
	c.Assert(err, IsNil)
	_, err = io.Copy(ioutil.Discard, reportStream)
	c.Assert(err, IsNil)
	c.Assert(reportStream.Close(), IsNil)

	operations, err = s.O.GetSiteOperations(siteKey, ops.OperationsFilter{})
	c.Assert(err, IsNil)
	c.Assert(operations, DeepEquals, ops.SiteOperations{storage.SiteOperation(*op)})

	// check state
	site, err = s.O.GetSite(siteKey)
	c.Assert(err, IsNil)
	c.Assert(site.State, Equals, ops.SiteStateInstalling)

}

func (s *OpsSuite) InstallInstructions(c *C) {
	a, err := s.O.CreateAccount(ops.NewAccountRequest{
		Org: "example.com",
	})
	c.Assert(err, IsNil)

	_, err = s.U.CreateGatekeeper(users.RemoteAccessUser{
		Email: constants.GatekeeperUser,
		Token: "token",
	})
	c.Assert(err, IsNil)

	s.generateInstallToken(c, "install-token", "example.com")

	site, err := s.O.CreateSite(ops.NewSiteRequest{
		AppPackage:   s.testApp.String(),
		AccountID:    a.ID,
		Provider:     schema.ProviderOnPrem,
		DomainName:   "example.com",
		InstallToken: "install-token",
	})
	c.Assert(err, IsNil)
	c.Assert(site.State, Equals, ops.SiteStateNotInstalled)

	opKey, err := s.O.CreateSiteInstallOperation(context.TODO(), ops.CreateSiteInstallOperationRequest{
		AccountID:  a.ID,
		SiteDomain: site.Domain,
		Variables:  storage.OperationVariables{},
	})
	c.Assert(err, IsNil)
	c.Assert(opKey, NotNil)

	token, err := s.O.GetExpandToken(site.Key())
	c.Assert(err, IsNil)
	c.Assert(token, compare.DeepEquals, &storage.ProvisioningToken{
		Token:       "install-token",
		Type:        storage.ProvisioningTokenTypeExpand,
		AccountID:   site.AccountID,
		SiteDomain:  site.Domain,
		UserEmail:   "agent@example.com",
		OperationID: opKey.OperationID,
	}, Commentf("expected expand token to exist, got %#v", token))

	joinInstructions, err := s.O.GetSiteInstructions(
		token.Token, "master", url.Values{})
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(joinInstructions, "join"), Equals, true)
}

func (s *OpsSuite) generateInstallToken(c *C, token, clusterName string) {
	_, err := s.O.CreateInstallToken(
		ops.NewInstallTokenRequest{
			AccountID: defaults.SystemAccountID,
			UserType:  storage.AdminUser,
			UserEmail: fmt.Sprintf("agent@%v", clusterName),
			Token:     token,
		},
	)
	c.Assert(err, IsNil)
}
