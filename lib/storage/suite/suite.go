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

// package suite contains a storage acceptance test suite that is backend
// implementation independent each storage will use the suite to test itself
package suite

import (
	"fmt"
	"sort"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	helmutils "github.com/gravitational/gravity/lib/utils/helm"

	"github.com/gravitational/teleport/lib/auth/testauthority"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	telefixtures "github.com/gravitational/teleport/lib/fixtures"
	teleservices "github.com/gravitational/teleport/lib/services"
	telesuite "github.com/gravitational/teleport/lib/services/suite"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
)

var now = time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC)

type StorageSuite struct {
	Backend storage.Backend
	Clock   clockwork.FakeClock
}

func (s *StorageSuite) AccountsCRUD(c *C) {
	// Create
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// Read
	o, err := s.Backend.GetAccount(a.ID)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, a)

	// Delete
	err = s.Backend.DeleteAccount(a.ID)
	c.Assert(err, IsNil)

	// Not found
	_, err = s.Backend.GetAccount(a.ID)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%v", err))
}

// UsersEquals compares two users ignoring internal RawObject value
func UsersEquals(c *C, a, b storage.User) {
	a.SetRawObject(nil)
	b.SetRawObject(nil)
	compare.DeepCompare(c, a, b)
}

func (s *StorageSuite) UsersCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// Create application package
	repo, err := s.Backend.CreateRepository(storage.NewRepository("test"))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "test",
		Version:    "0.0.1",
		Manifest:   []byte("1"),
		Type:       string(storage.AppUser),
	})
	c.Assert(err, IsNil)

	// Create site a
	sa, err := s.Backend.CreateSite(storage.Site{
		Created:   now,
		AccountID: a.ID,
		Domain:    "a.example.com",
		App:       *app,
	})
	c.Assert(err, IsNil)
	c.Assert(sa.AccountID, Equals, a.ID)

	// create user
	err = s.Backend.UpsertOIDCConnector(
		storage.NewOIDCConnector(
			"google",
			teleservices.OIDCConnectorSpecV2{
				IssuerURL:    "https://accounts.google.com",
				ClientID:     "id1",
				ClientSecret: "secret",
				RedirectURL:  "https://gravity",
			}))
	c.Assert(err, IsNil)

	u := storage.NewUser("bob@example.com",
		storage.UserSpecV2{
			AccountOwner:   true,
			Type:           "agent",
			AccountID:      a.ID,
			ClusterName:    sa.Domain,
			Password:       "token 1",
			OIDCIdentities: []teleservices.ExternalIdentity{{Username: "bob@example.com", ConnectorID: "google"}},
			Roles:          []string{"admin"},
		})
	o, err := s.Backend.CreateUser(u)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, u)

	o1, err := s.Backend.GetUser(u.GetName())
	c.Assert(err, IsNil)
	UsersEquals(c, o1, u)

	u.SetRoles([]string{"admin", "developer"})

	o, err = s.Backend.UpsertUser(u)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, u)

	o1, err = s.Backend.GetUser(u.GetName())
	c.Assert(err, IsNil)
	UsersEquals(c, o1, u)

	users, err := s.Backend.GetUsers(u.GetAccountID())
	c.Assert(err, IsNil)
	UsersEquals(c, users[0], u)

	o2, err := s.Backend.GetUserByOIDCIdentity(teleservices.ExternalIdentity{Username: "bob@example.com", ConnectorID: "google"})
	c.Assert(err, IsNil)
	UsersEquals(c, o2.(storage.User), u)

	// Update just HOTP
	hotp := []byte("value new")
	u.SetHOTP(hotp)
	err = s.Backend.UpdateUser(u.GetName(), storage.UpdateUserReq{HOTP: &hotp})
	c.Assert(err, IsNil)

	o1, err = s.Backend.GetUser(u.GetName())
	c.Assert(err, IsNil)
	UsersEquals(c, o1, u)

	// Update just Password
	pass := "just token"
	u.SetPassword(pass)
	err = s.Backend.UpdateUser(u.GetName(), storage.UpdateUserReq{Password: &pass})
	c.Assert(err, IsNil)

	o1, err = s.Backend.GetUser(u.GetName())
	c.Assert(err, IsNil)
	UsersEquals(c, o1, u)

	// Update HOTP and Pass hash
	hotp2 := []byte("value new 2")
	u.SetHOTP(hotp2)
	pass2 := "new token"
	u.SetPassword(pass2)
	err = s.Backend.UpdateUser(u.GetName(), storage.UpdateUserReq{HOTP: &hotp2, Password: &pass2})
	c.Assert(err, IsNil)

	o1, err = s.Backend.GetUser(u.GetName())
	c.Assert(err, IsNil)
	UsersEquals(c, o1, u)

	err = s.Backend.DeleteUser(u.GetName())
	c.Assert(err, IsNil)

	_, err = s.Backend.GetUser(u.GetName())
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%v", err))

	// test delete all
	o, err = s.Backend.CreateUser(u)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, u)

	err = s.Backend.DeleteAllUsers()
	c.Assert(err, IsNil)

	err = s.Backend.DeleteAllUsers()
	c.Assert(err, IsNil)

	_, err = s.Backend.GetUser(u.GetName())
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%v", err))
}

func (s *StorageSuite) LocalCluster(c *C) {
	out, err := s.Backend.GetLocalClusterName()
	c.Assert(out, Equals, "")
	c.Assert(trace.IsNotFound(err), DeepEquals, true)
	err = s.Backend.UpsertLocalClusterName("bob")
	c.Assert(err, IsNil)
	out, err = s.Backend.GetLocalClusterName()
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "bob")
}

func (s *StorageSuite) ConnectorsCRUD(c *C) {
	conn := storage.NewOIDCConnector("google", teleservices.OIDCConnectorSpecV2{
		IssuerURL:    "https://accounts.google.com",
		ClientID:     "id1",
		ClientSecret: "secret",
		RedirectURL:  "https://gravity",
	})

	err := s.Backend.UpsertOIDCConnector(conn)
	c.Assert(err, IsNil)

	out, err := s.Backend.GetOIDCConnector(conn.GetName(), true)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, out, conn)

	conns, err := s.Backend.GetOIDCConnectors(true)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, conns, []teleservices.OIDCConnector{conn})

	_, publicKey, err := testauthority.New().GenerateKeyPair("")
	c.Assert(err, IsNil)

	req := teleservices.OIDCAuthRequest{
		ConnectorID:       conn.GetName(),
		Type:              "test",
		CheckUser:         true,
		StateToken:        "tok1",
		RedirectURL:       "https://localhost/redirect",
		PublicKey:         publicKey,
		CertTTL:           time.Hour,
		CreateWebSession:  true,
		ClientRedirectURL: "https://localhost/redirect_client",
	}
	err = s.Backend.CreateOIDCAuthRequest(req)
	c.Assert(err, IsNil)

	oreq, err := s.Backend.GetOIDCAuthRequest(req.StateToken)
	c.Assert(err, IsNil)
	c.Assert(oreq, DeepEquals, &req)
}

func (s *StorageSuite) SAMLCRUD(c *C) {
	connector := &teleservices.SAMLConnectorV2{
		Kind:    teleservices.KindSAML,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      "saml1",
			Namespace: defaults.Namespace,
		},
		Spec: teleservices.SAMLConnectorSpecV2{
			Issuer:                   "http://example.com",
			SSO:                      "https://example.com/saml/sso",
			AssertionConsumerService: "https://localhost/acs",
			Audience:                 "https://localhost/aud",
			ServiceProviderIssuer:    "https://localhost/iss",
			AttributesToRoles: []teleservices.AttributeMapping{
				{Name: "groups", Value: "admin", Roles: []string{"admin"}},
			},
			Cert: telefixtures.SigningCertPEM,
			SigningKeyPair: &teleservices.SigningKeyPair{
				PrivateKey: telefixtures.SigningKeyPEM,
				Cert:       telefixtures.SigningCertPEM,
			},
		},
	}
	err := connector.CheckAndSetDefaults()
	c.Assert(err, IsNil)
	err = s.Backend.UpsertSAMLConnector(connector)
	c.Assert(err, IsNil)
	out, err := s.Backend.GetSAMLConnector(connector.GetName(), true)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, out, connector)

	connectors, err := s.Backend.GetSAMLConnectors(true)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, []teleservices.SAMLConnector{connector}, connectors)

	out2, err := s.Backend.GetSAMLConnector(connector.GetName(), false)
	c.Assert(err, IsNil)
	connectorNoSecrets := *connector
	connectorNoSecrets.Spec.SigningKeyPair.PrivateKey = ""
	compare.DeepCompare(c, out2, &connectorNoSecrets)

	connectorsNoSecrets, err := s.Backend.GetSAMLConnectors(false)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, []teleservices.SAMLConnector{&connectorNoSecrets}, connectorsNoSecrets)

	err = s.Backend.DeleteSAMLConnector(connector.GetName())
	c.Assert(err, IsNil)

	err = s.Backend.DeleteSAMLConnector(connector.GetName())
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("expected not found, got %T", err))

	_, err = s.Backend.GetSAMLConnector(connector.GetName(), true)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("expected not found, got %T", err))
}

func (s *StorageSuite) WebSessionsCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// Create application package
	repo, err := s.Backend.CreateRepository(storage.NewRepository("test"))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "test",
		Version:    "0.0.1",
		Manifest:   []byte("1"),
		Type:       string(storage.AppUser),
	})
	c.Assert(err, IsNil)

	// Create site a
	sa, err := s.Backend.CreateSite(storage.Site{
		Created:   now,
		AccountID: a.ID,
		Domain:    "a.example.com",
		App:       *app,
	})
	c.Assert(err, IsNil)
	c.Assert(sa.AccountID, Equals, a.ID)

	u := storage.NewUser(
		"bob@example.com",
		storage.UserSpecV2{
			Type:        "agent",
			AccountID:   a.ID,
			ClusterName: sa.Domain,
			Password:    "token 1",
		})
	o, err := s.Backend.CreateUser(u)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, u)

	authority := testauthority.New()
	privateKey, publicKey, err := authority.GenerateKeyPair("")
	c.Assert(err, IsNil)
	cert, err := authority.GenerateUserCert(teleservices.UserCertParams{
		PrivateCASigningKey:   privateKey,
		PublicUserKey:         publicKey,
		Username:              "user",
		AllowedLogins:         []string{"admin", "cenots"},
		TTL:                   time.Hour,
		PermitAgentForwarding: true,
	})
	c.Assert(err, IsNil)

	sess := teleservices.NewWebSession("sid1", teleservices.WebSessionSpecV2{
		User:        u.GetName(),
		Pub:         cert,
		Priv:        privateKey,
		BearerToken: "bearer1",
		Expires:     now.Add(time.Hour),
	})
	err = s.Backend.UpsertWebSession(u.GetName(), sess.GetName(), sess)
	c.Assert(err, IsNil)

	out, err := s.Backend.GetWebSession(u.GetName(), sess.GetName())
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, sess)

	err = s.Backend.DeleteWebSession(u.GetName(), sess.GetName())
	c.Assert(err, IsNil)

	out, err = s.Backend.GetWebSession(u.GetName(), sess.GetName())
	c.Assert(err, NotNil)
	c.Assert(out, IsNil)
}

func (s *StorageSuite) UserTokensCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// create user
	u := storage.NewUser("bob@example.com", storage.UserSpecV2{
		Type:      "agent",
		AccountID: a.ID,
		Password:  "token 1",
	})
	_, err = s.Backend.CreateUser(u)
	c.Assert(err, IsNil)

	// token1 adds user to existing account
	token1 := storage.UserToken{
		Token:   "a",
		User:    u.GetName(),
		Expires: now.Add(time.Hour),
		Type:    storage.UserTokenTypeInvite,
		HOTP:    []byte("hotp1"),
		QRCode:  []byte("qrcode1"),
	}
	storedToken, err := s.Backend.CreateUserToken(token1)
	c.Assert(err, IsNil)
	c.Assert(*storedToken, DeepEquals, token1)

	// token2 adds new account and has expired
	token2 := storage.UserToken{
		Token:   "b",
		User:    u.GetName(),
		Expires: now.Add(-1 * time.Hour),
		Type:    storage.UserTokenTypeReset,
		HOTP:    []byte("hotp2"),
		QRCode:  []byte("qrcode2"),
	}

	storedToken, err = s.Backend.CreateUserToken(token2)
	c.Assert(err, IsNil)
	c.Assert(*storedToken, DeepEquals, token2)

	// token3 is a password recovery token
	token3 := storage.UserToken{
		Token:   "c",
		Expires: now.Add(2 * time.Hour),
		Type:    storage.UserTokenTypeReset,
		User:    u.GetName(),
		HOTP:    []byte("hotp3"),
		QRCode:  []byte("qrcode3"),
	}

	storedToken, err = s.Backend.CreateUserToken(token3)
	c.Assert(err, IsNil)
	c.Assert(*storedToken, DeepEquals, token3)

	// Read
	storedToken, err = s.Backend.GetUserToken(token1.Token)
	c.Assert(err, IsNil)
	c.Assert(*storedToken, DeepEquals, token1)

	storedToken, err = s.Backend.GetUserToken(token3.Token)
	c.Assert(err, IsNil)
	c.Assert(*storedToken, DeepEquals, token3)

	// Delete
	err = s.Backend.DeleteUserTokens(storage.UserTokenTypeReset, u.GetName())
	c.Assert(err, IsNil)

	_, err = s.Backend.GetUserToken(token3.Token)
	c.Assert(trace.IsNotFound(err), Equals, true)

	_, err = s.Backend.GetUserToken(token2.Token)
	c.Assert(trace.IsNotFound(err), Equals, true)

	_, err = s.Backend.GetUserToken(token1.Token)
	c.Assert(err, IsNil)

	err = s.Backend.DeleteUserToken(token1.Token)
	c.Assert(err, IsNil)

	_, err = s.Backend.GetUserToken(token1.Token)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))
}

func (s *StorageSuite) UserInvitesCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// create user invite
	u := storage.UserInvite{
		Name:      "bob@example.com",
		CreatedBy: "alice@example.com",
	}

	err = u.CheckAndSetDefaults()
	c.Assert(err, FitsTypeOf, trace.BadParameter(""), Commentf("Should fail on empty roles"))

	u.Roles = []string{"admin"}

	// create
	out, err := s.Backend.UpsertUserInvite(u)
	c.Assert(err, IsNil)
	c.Assert(*out, DeepEquals, u)

	// read
	invites, err := s.Backend.GetUserInvites()
	c.Assert(err, IsNil)
	c.Assert(invites[0], DeepEquals, u)

	out, err = s.Backend.GetUserInvite(u.Name)
	c.Assert(err, IsNil)
	c.Assert(*out, DeepEquals, u)

	_, err = s.Backend.GetUserInvite("does_not_exist@example.com")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))

	// upsert
	out, err = s.Backend.UpsertUserInvite(u)
	c.Assert(err, IsNil)
	c.Assert(*out, DeepEquals, u)

	// delete
	err = s.Backend.DeleteUserInvite(u.Name)
	c.Assert(err, IsNil)

	_, err = s.Backend.GetUserInvite(u.Name)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))
}

func (s *StorageSuite) RepositoriesCRUD(c *C) {
	// Create repositories a and b
	a, err := s.Backend.CreateRepository(storage.NewRepository("a.example.com"))
	c.Assert(err, IsNil)
	c.Assert(a.GetName(), Equals, "a.example.com")

	b, err := s.Backend.CreateRepository(storage.NewRepository("b.example.com"))
	c.Assert(err, IsNil)

	o, err := s.Backend.GetRepository("a.example.com")
	c.Assert(err, IsNil)
	c.Assert(o.GetName(), Equals, a.GetName())
	c.Assert(o.Expiry().Equal(a.Expiry()), Equals, true)

	// create package p1 and add it to repository a
	p1, err := s.Backend.CreatePackage(storage.Package{
		Repository: a.GetName(),
		Name:       "package-1",
		Version:    "0.0.1",
		SHA512:     "hash",
		SizeBytes:  3,
		Created:    now,
	})
	c.Assert(err, IsNil)

	p2 := storage.Package{
		Repository:    b.GetName(),
		Name:          "package-2",
		Version:       "0.0.1",
		SHA512:        "hash2",
		SizeBytes:     3,
		RuntimeLabels: map[string]string{"key": "val", "key1": "val1"},
		Created:       now,
	}

	// upsert should insert package if it's not here
	out, err := s.Backend.UpsertPackage(p2)
	c.Assert(err, IsNil)
	c.Assert(*out, DeepEquals, p2)

	// now get the package p1 from repository a
	op1, err := s.Backend.GetPackage(a.GetName(), p1.Name, p1.Version)
	c.Assert(err, IsNil)
	c.Assert(op1, DeepEquals, p1)

	// now get the package p1 from repository a
	ops1, err := s.Backend.GetPackages(a.GetName())
	c.Assert(err, IsNil)
	c.Assert(ops1, DeepEquals, []storage.Package{*p1})

	// the package p2 is not found in repository a
	_, err = s.Backend.GetPackage(a.GetName(), p2.Name, p2.Version)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))

	// the package p2 is found in repository b
	op2, err := s.Backend.GetPackage(b.GetName(), p2.Name, p2.Version)
	c.Assert(err, IsNil)
	c.Assert(*op2, DeepEquals, p2)

	// Upsert package
	p2.SHA512 = "hash3"
	p2.SizeBytes = 20

	op2, err = s.Backend.UpsertPackage(p2)
	c.Assert(err, IsNil)
	c.Assert(*op2, DeepEquals, p2)

	// make sure package was updated
	op2, err = s.Backend.GetPackage(b.GetName(), p2.Name, p2.Version)
	c.Assert(err, IsNil)
	c.Assert(*op2, DeepEquals, p2)

	// add some labels to p1 and remove some labels to p2
	err = s.Backend.UpdatePackageRuntimeLabels(
		a.GetName(), p1.Name, p1.Version, map[string]string{"a": "b"}, nil)
	c.Assert(err, IsNil)

	op1, err = s.Backend.GetPackage(a.GetName(), p1.Name, p1.Version)
	c.Assert(err, IsNil)
	c.Assert(op1.RuntimeLabels, DeepEquals, map[string]string{"a": "b"})

	err = s.Backend.UpdatePackageRuntimeLabels(
		b.GetName(), p2.Name, p2.Version, map[string]string{"new": "newval"}, []string{"key", "key1"})
	c.Assert(err, IsNil)
	op2, err = s.Backend.GetPackage(b.GetName(), p2.Name, p2.Version)
	c.Assert(err, IsNil)
	c.Assert(op2.RuntimeLabels, DeepEquals, map[string]string{"new": "newval"})

	err = s.Backend.UpdatePackageRuntimeLabels(
		b.GetName(), "not", "here", map[string]string{"new": "newval"}, []string{"key", "key1"})
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))

	// remove package p1 from repository a
	err = s.Backend.DeletePackage(a.GetName(), p1.Name, p1.Version)
	c.Assert(err, IsNil)

	// package p1 is no longer found in repository a
	_, err = s.Backend.GetPackage(a.GetName(), p1.Name, p1.Version)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))

	// delete repository a
	err = s.Backend.DeleteRepository("a.example.com")
	c.Assert(err, IsNil)

	_, err = s.Backend.GetRepository("a.example.com")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))
}

func (s *StorageSuite) PermissionsCRUD(c *C) {
	u := storage.NewUser("bob@example.com",
		storage.UserSpecV2{
			Type:     "agent",
			Password: "token 1",
		})
	_, err := s.Backend.CreateUser(u)
	c.Assert(err, IsNil)

	// Create permission to read from repo
	p, err := s.Backend.CreatePermission(
		storage.Permission{
			UserEmail:    u.GetName(),
			Collection:   "repositories",
			CollectionID: "example.com",
			Action:       "read",
		})
	c.Assert(err, IsNil)
	c.Assert(p.UserEmail, Equals, u.GetName())

	o, err := s.Backend.GetPermission(*p)
	c.Assert(err, IsNil)
	c.Assert(*o, DeepEquals, *p)

	err = s.Backend.DeletePermissionsForUser(u.GetName())
	c.Assert(err, IsNil)

	_, err = s.Backend.GetPermission(*p)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))

	p2, err := s.Backend.CreatePermission(
		storage.Permission{
			UserEmail:  u.GetName(),
			Collection: "repositories",
			Action:     "read",
		})
	c.Assert(err, IsNil)
	c.Assert(p2.UserEmail, Equals, u.GetName())

	o2, err := s.Backend.GetPermission(*p2)
	c.Assert(err, IsNil)
	c.Assert(*o2, DeepEquals, *p2)

	err = s.Backend.DeletePermissionsForUser(u.GetName())
	c.Assert(err, IsNil)

	_, err = s.Backend.GetPermission(*p2)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))
}

func (s *StorageSuite) SitesCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// Create another account
	b, err := s.Backend.CreateAccount(storage.Account{Org: "test2"})
	c.Assert(err, IsNil)
	c.Assert(b.ID, NotNil)

	// Create application package
	repo, err := s.Backend.CreateRepository(storage.NewRepository("example.com"))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "app",
		Version:    "0.0.1",
		Type:       string(storage.AppUser),
		Manifest:   []byte("a"),
		Created:    now,
	})
	c.Assert(err, IsNil)

	// Create site a
	sa, err := s.Backend.CreateSite(storage.Site{
		AccountID:       a.ID,
		Created:         now,
		Provider:        "virsh",
		State:           "created",
		Domain:          "a.example.com",
		App:             *app,
		NextUpdateCheck: now,
	})
	c.Assert(err, IsNil)
	c.Assert(sa.AccountID, Equals, a.ID)

	// Create another application package
	app, err = s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "app",
		Version:    "0.0.2",
		Type:       string(storage.AppUser),
		Manifest:   []byte("b"),
		Created:    now,
	})
	c.Assert(err, IsNil)

	// Create site b
	sb, err := s.Backend.CreateSite(storage.Site{
		AccountID:       a.ID,
		Created:         now,
		Provider:        "aws",
		Domain:          "b.example.com",
		App:             *app,
		NextUpdateCheck: now,
	})
	c.Assert(err, IsNil)
	c.Assert(sb.AccountID, Equals, a.ID)

	// Create site c for another account
	sc, err := s.Backend.CreateSite(storage.Site{
		AccountID:       b.ID,
		Created:         now,
		Provider:        "virsh",
		Domain:          "c.example.com",
		App:             *app,
		NextUpdateCheck: now,
	})
	c.Assert(err, IsNil)
	c.Assert(sc.AccountID, Equals, b.ID)

	// Read site a
	osa, err := s.Backend.GetSite(sa.Domain)
	log.Infof("%v vs %v", now, osa.Created)
	c.Assert(err, IsNil)
	c.Assert(osa, DeepEquals, sa)

	// Read site b
	osb, err := s.Backend.GetSite(sb.Domain)
	c.Assert(err, IsNil)
	c.Assert(osb, DeepEquals, sb)

	sites, err := s.Backend.GetSites(a.ID)
	c.Assert(err, IsNil)
	c.Assert(len(sites), Equals, 2)
	sitesMap := map[string]storage.Site{}
	for _, st := range sites {
		sitesMap[st.Domain] = st
	}
	c.Assert(sitesMap, DeepEquals,
		map[string]storage.Site{sa.Domain: *sa, sb.Domain: *sb})

	// Read all sites for all accounts
	allSites, err := s.Backend.GetAllSites()
	c.Assert(err, IsNil)
	c.Assert(len(allSites), Equals, 3)
	sitesMap = map[string]storage.Site{}
	for _, st := range allSites {
		sitesMap[st.Domain] = st
	}
	c.Assert(sitesMap, DeepEquals,
		map[string]storage.Site{sa.Domain: *sa, sb.Domain: *sb, sc.Domain: *sc})

	// Update site a with variables and servers
	sa.ProvisionerState = []byte("some state")
	sa.State = "new state"
	_, err = s.Backend.UpdateSite(*sa)
	c.Assert(err, IsNil)

	// check the update
	osa, err = s.Backend.GetSite(sa.Domain)
	c.Assert(err, IsNil)
	c.Assert(osa, DeepEquals, sa)

	// check the update
	osa, err = s.Backend.GetSite(sa.Domain)
	c.Assert(err, IsNil)
	c.Assert(osa, DeepEquals, sa)

	err = s.Backend.CompareAndSwapSiteState(sa.Domain, "invalid state", "state 2")
	c.Assert(trace.IsCompareFailed(err), Equals, true, Commentf("%#T", err))

	osa, err = s.Backend.GetSite(sa.Domain)
	c.Assert(err, IsNil)
	c.Assert(osa.State, Equals, sa.State)

	err = s.Backend.CompareAndSwapSiteState(sa.Domain, "new state", "state 2")
	c.Assert(err, IsNil)

	osa, err = s.Backend.GetSite(sa.Domain)
	c.Assert(err, IsNil)
	c.Assert(osa.State, Equals, "state 2")

	// Delete site a
	err = s.Backend.DeleteSite(sa.Domain)
	c.Assert(err, IsNil)

	// Not found
	_, err = s.Backend.GetSite(sa.Domain)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))
}

func (s *StorageSuite) ProgressEntriesCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// Create application package
	repo, err := s.Backend.CreateRepository(storage.NewRepository("example.com"))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "app",
		Version:    "0.0.1",
		Manifest:   []byte("1"),
		Type:       string(storage.AppUser),
	})
	c.Assert(err, IsNil)

	// Create site a
	sa, err := s.Backend.CreateSite(storage.Site{
		Created:   now,
		AccountID: a.ID,
		Domain:    "a.example.com",
		App:       *app,
	})
	c.Assert(err, IsNil)
	c.Assert(sa.AccountID, Equals, a.ID)

	// create operation
	now := time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC)
	op, err := s.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  a.ID,
		SiteDomain: sa.Domain,
		Type:       "test2",
		Created:    now,
		Updated:    now,
		State:      "new",
	})
	c.Assert(err, IsNil)

	pe1 := storage.ProgressEntry{
		SiteDomain:  sa.Domain,
		OperationID: op.ID,
		Created:     now,
		Completion:  10,
		State:       "in_progress",
		Message:     "setting up load balancers",
	}

	ope1, err := s.Backend.CreateProgressEntry(pe1)
	c.Assert(err, IsNil)
	pe1.ID = ope1.ID
	c.Assert(*ope1, DeepEquals, pe1)

	ope1, err = s.Backend.GetLastProgressEntry(sa.Domain, op.ID)
	c.Assert(err, IsNil)
	c.Assert(*ope1, DeepEquals, pe1)

	pe2 := storage.ProgressEntry{
		SiteDomain:  sa.Domain,
		OperationID: op.ID,
		Created:     now.Add(time.Second),
		Completion:  20,
		State:       "in_progress",
		Message:     "setting up virtual network",
	}

	ope2, err := s.Backend.CreateProgressEntry(pe2)
	c.Assert(err, IsNil)
	pe2.ID = ope2.ID
	c.Assert(*ope2, DeepEquals, pe2)

	ope2, err = s.Backend.GetLastProgressEntry(sa.Domain, op.ID)
	c.Assert(err, IsNil)
	c.Assert(*ope2, DeepEquals, pe2)

	// Create for non existent site should fail
	_, err = s.Backend.CreateProgressEntry(storage.ProgressEntry{
		SiteDomain:  "nothere.com",
		OperationID: op.ID,
		Created:     now.Add(time.Second),
		Completion:  20,
		State:       "in_progress",
		Message:     "setting up virtual network",
	})
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *StorageSuite) OperationsCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// Create application package
	repo, err := s.Backend.CreateRepository(storage.NewRepository("example.com"))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "app",
		Version:    "0.0.1",
		Manifest:   []byte("1"),
		Type:       string(storage.AppUser),
	})
	c.Assert(err, IsNil)

	// Create site a
	sa, err := s.Backend.CreateSite(storage.Site{
		AccountID: a.ID,
		Created:   now,
		Domain:    "a.example.com",
		App:       *app,
	})
	c.Assert(err, IsNil)
	c.Assert(sa.Domain, NotNil)
	c.Assert(sa.AccountID, Equals, a.ID)

	now := time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC)

	op1 := storage.SiteOperation{
		AccountID:  a.ID,
		SiteDomain: sa.Domain,
		Type:       "test",
		Created:    now,
		Updated:    now,
		State:      "new",
	}

	out1, err := s.Backend.CreateSiteOperation(op1)
	c.Assert(err, IsNil)
	c.Assert(out1.ID, NotNil)
	op1.ID = out1.ID
	c.Assert(*out1, DeepEquals, op1)

	out1, err = s.Backend.GetSiteOperation(op1.SiteDomain, op1.ID)
	c.Assert(err, IsNil)
	c.Assert(*out1, DeepEquals, op1)

	op1.State = "updated"
	hourLater := now.Add(time.Hour)
	op1.Updated = hourLater
	op1.Servers = []storage.Server{
		{AdvertiseIP: "10.0.0.1", Hostname: "srv1", Role: "master", Created: now},
		{AdvertiseIP: "10.0.0.2", Hostname: "srv2", Role: "node", Created: now},
	}
	out1, err = s.Backend.UpdateSiteOperation(op1)
	c.Assert(err, IsNil)
	c.Assert(*out1, DeepEquals, op1)

	// second update should keep servers intact
	out1, err = s.Backend.UpdateSiteOperation(op1)
	c.Assert(err, IsNil)
	c.Assert(*out1, DeepEquals, op1)

	out2, err := s.Backend.CreateSiteOperation(storage.SiteOperation{
		AccountID:  a.ID,
		SiteDomain: sa.Domain,
		Type:       "test2",
		Created:    hourLater,
		Updated:    hourLater,
		State:      "new",
	})
	c.Assert(err, IsNil)

	ops, err := s.Backend.GetSiteOperations(sa.Domain)
	c.Assert(err, IsNil)
	c.Assert(ops, DeepEquals, []storage.SiteOperation{
		*out2, *out1,
	})
}

func (s *StorageSuite) LoginEntriesCRUD(c *C) {
	// Create
	entry := storage.LoginEntry{
		Email:        "alice@example.com",
		Password:     "pass",
		OpsCenterURL: "https://ops1:3088",
		Created:      s.Clock.Now().UTC(),
	}
	e, err := s.Backend.UpsertLoginEntry(entry)
	c.Assert(err, IsNil)
	c.Assert(*e, DeepEquals, entry)

	// Read
	o, err := s.Backend.GetLoginEntry(entry.OpsCenterURL)
	c.Assert(err, IsNil)
	c.Assert(*o, DeepEquals, entry)

	// Read all
	es, err := s.Backend.GetLoginEntries()
	c.Assert(err, IsNil)
	c.Assert(es, DeepEquals, []storage.LoginEntry{entry})

	// Delete
	err = s.Backend.DeleteLoginEntry(entry.OpsCenterURL)
	c.Assert(err, IsNil)

	// Not found
	_, err = s.Backend.GetLoginEntry(entry.OpsCenterURL)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))
}

func (s *StorageSuite) CreatesApplication(c *C) {
	const repository = "example.com"
	const packageName = "example-app"
	const version = "0.0.1"

	repo, err := s.Backend.CreateRepository(storage.NewRepository(repository))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       packageName,
		Version:    version,
		Manifest:   []byte("1"),
		Type:       string(storage.AppUser),
		Created:    now,
	})
	c.Assert(err, IsNil)
	c.Assert(app, NotNil)

	retrievedApp, err := s.Backend.GetApplication(repository, packageName, version)
	c.Assert(err, IsNil)
	c.Assert(retrievedApp, DeepEquals, app)
}

func (s *StorageSuite) DeletesApplication(c *C) {
	const repository = "example.com"
	const packageName = "example-app"
	const version = "0.0.1"

	repo, err := s.Backend.CreateRepository(storage.NewRepository(repository))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       packageName,
		Version:    version,
		Manifest:   []byte("1"),
		Type:       string(storage.AppUser),
	})
	c.Assert(err, IsNil)
	c.Assert(app, NotNil)

	err = s.Backend.DeletePackage(repository, packageName, version)
	c.Assert(err, IsNil)

	_, err = s.Backend.GetApplication(repository, packageName, version)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))
}

func (s *StorageSuite) RetrievesApplications(c *C) {
	apps := []storage.Package{
		{
			Repository: "example.com",
			Name:       "example-app",
			Manifest:   []byte("1"),
			Version:    "0.0.1",
			Type:       string(storage.AppUser),
			Created:    now,
		},
		{
			Repository: "example.io",
			Name:       "example-app",
			Manifest:   []byte("1"),
			Version:    "0.0.2",
			Type:       string(storage.AppUser),
			Created:    now,
		},
	}

	for _, app := range apps {
		_, err := s.Backend.CreateRepository(storage.NewRepository(app.Repository))
		c.Assert(err, IsNil)

		_, err = s.Backend.CreatePackage(app)
		c.Assert(err, IsNil)
	}

	actualApps, err := s.Backend.GetApplications("", storage.AppType(""))
	c.Assert(err, IsNil)
	sort.Sort(byRepository(actualApps))
	c.Assert(actualApps, DeepEquals, apps)

	app, err := s.Backend.GetApplication("example.io", "example-app", "0.0.2")
	c.Assert(err, IsNil)
	c.Assert((storage.Package)(*app), DeepEquals, apps[1])
}

func (s *StorageSuite) OpsCenterLinksCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// Create application package
	repo, err := s.Backend.CreateRepository(storage.NewRepository("example.com"))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "app",
		Version:    "0.0.1",
		Manifest:   []byte("1"),
		Type:       string(storage.AppUser),
	})
	c.Assert(err, IsNil)

	// Create site a
	sa, err := s.Backend.CreateSite(storage.Site{
		AccountID: a.ID,
		Created:   now,
		Domain:    "a.example.com",
		App:       *app,
	})
	c.Assert(err, IsNil)
	c.Assert(sa.Domain, NotNil)
	c.Assert(sa.AccountID, Equals, a.ID)

	// Create link
	link := storage.OpsCenterLink{
		SiteDomain: sa.Domain,
		Hostname:   "example.com",
		Type:       storage.OpsCenterRemoteAccessLink,
		RemoteAddr: "example.com:32008",
		Enabled:    true,
		APIURL:     "https://example.com",
	}
	e, err := s.Backend.UpsertOpsCenterLink(link, 0)
	c.Assert(err, IsNil)
	c.Assert(*e, DeepEquals, link)

	// Read link
	links, err := s.Backend.GetOpsCenterLinks(sa.Domain)
	c.Assert(err, IsNil)
	c.Assert(links, DeepEquals, []storage.OpsCenterLink{link})

	// Create link of other type
	link2 := storage.OpsCenterLink{
		SiteDomain: sa.Domain,
		Hostname:   "example.com",
		Type:       storage.OpsCenterUpdateLink,
		RemoteAddr: "example.com:443",
		Enabled:    true,
		APIURL:     "https://example.com",
	}
	e2, err := s.Backend.UpsertOpsCenterLink(link2, 0)
	c.Assert(err, IsNil)
	c.Assert(*e2, DeepEquals, link2)

	links, err = s.Backend.GetOpsCenterLinks(sa.Domain)
	c.Assert(err, IsNil)
	c.Assert(links, DeepEquals, []storage.OpsCenterLink{link, link2})
}

type byRepository []storage.Package

func (r byRepository) Len() int      { return len(r) }
func (r byRepository) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byRepository) Less(i, j int) bool {
	return r[i].Repository < r[j].Repository
}

func (s *StorageSuite) CreatesAppImportOperation(c *C) {
	const repository = "example.com"
	const packageName = "example-app"
	const version = "0.0.1"

	op, err := s.Backend.CreateAppOperation(storage.AppOperation{
		Repository:     repository,
		PackageName:    packageName,
		PackageVersion: version,
		Created:        now,
		Updated:        now,
		State:          "started",
		Type:           app.AppOperationImport,
	})
	c.Assert(err, IsNil)
	c.Assert(op, NotNil)
	c.Assert(op.ID, Not(Equals), 0)

	retrievedOp, err := s.Backend.GetAppOperation(op.ID)
	c.Assert(err, IsNil)
	c.Assert(op, DeepEquals, retrievedOp)
}

func (s *StorageSuite) UpdatesAppImportOperation(c *C) {
	const repository = "example.com"
	const packageName = "example-app"
	const version = "0.0.1"

	op, err := s.Backend.CreateAppOperation(storage.AppOperation{
		Repository:     repository,
		PackageName:    packageName,
		PackageVersion: version,
		Created:        now,
		Updated:        now,
		State:          "started",
		Type:           app.AppOperationImport,
	})
	c.Assert(err, IsNil)
	c.Assert(op, NotNil)
	c.Assert(op.ID, Not(Equals), 0)

	hourLater := now.Add(time.Hour)
	updatedOp, err := s.Backend.UpdateAppOperation(storage.AppOperation{
		Repository:     op.Repository,
		PackageName:    op.PackageName,
		PackageVersion: op.PackageVersion,
		ID:             op.ID,
		Updated:        hourLater,
		State:          "finished",
		Type:           op.Type,
	})
	c.Assert(err, IsNil)
	c.Assert(updatedOp.Updated, DeepEquals, hourLater)
	c.Assert(updatedOp.State, Equals, "finished")
}

func (s *StorageSuite) APIKeysCRUD(c *C) {
	u := storage.NewUser("testagent@example.com", storage.UserSpecV2{
		Type: "agent",
	})
	_, err := s.Backend.CreateUser(u)
	c.Assert(err, IsNil)

	key1, err := s.Backend.CreateAPIKey(storage.APIKey{
		Token:     "key1",
		UserEmail: u.GetName(),
	})
	c.Assert(err, IsNil)
	c.Assert(key1.Expires.IsZero(), Equals, true)

	_, err = s.Backend.CreateAPIKey(storage.APIKey{
		Token:     "key2",
		UserEmail: u.GetName(),
	})
	c.Assert(err, IsNil)

	keys, err := s.Backend.GetAPIKeys(u.GetName())
	c.Assert(err, IsNil)
	c.Assert(len(keys), Equals, 2)

	for _, key := range keys {
		foundKey, err := s.Backend.GetAPIKey(key.Token)
		c.Assert(err, IsNil)
		c.Assert(foundKey.Token, Equals, key.Token)
	}

	// try to update the token with expiration time: "create" should fail, "upsert" should succeed
	update := storage.APIKey{Token: "key1", UserEmail: u.GetName(), Expires: time.Now().Add(time.Hour)}
	_, err = s.Backend.CreateAPIKey(update)
	c.Assert(err, NotNil)
	updatedKey, err := s.Backend.UpsertAPIKey(update)
	c.Assert(err, IsNil)
	c.Assert(updatedKey.Expires, DeepEquals, update.Expires)

	err = s.Backend.DeleteAPIKey(u.GetName(), "key1")
	c.Assert(err, IsNil)

	keys, err = s.Backend.GetAPIKeys(u.GetName())
	c.Assert(err, IsNil)
	c.Assert(len(keys), Equals, 1)
}

func (s *StorageSuite) ProvisioningTokensCRUD(c *C) {
	// Create account
	a, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)
	c.Assert(a.ID, NotNil)

	// Create application package
	repo, err := s.Backend.CreateRepository(storage.NewRepository("test"))
	c.Assert(err, IsNil)
	app, err := s.Backend.CreatePackage(storage.Package{
		Repository: repo.GetName(),
		Name:       "test",
		Version:    "0.0.1",
		Type:       string(storage.AppUser),
	})
	c.Assert(err, IsNil)

	// Create site
	sa, err := s.Backend.CreateSite(storage.Site{
		Created:   now,
		AccountID: a.ID,
		Domain:    "a.example.com",
		App:       *app,
	})
	c.Assert(err, IsNil)
	c.Assert(sa.AccountID, Equals, a.ID)

	// Create User
	u := storage.NewUser("bob@example.com", storage.UserSpecV2{
		AccountOwner: true,
		Type:         "agent",
		AccountID:    a.ID,
		ClusterName:  sa.Domain,
		Password:     "token 1",
		Roles:        []string{"admin"},
	})
	o, err := s.Backend.CreateUser(u)
	c.Assert(err, IsNil)
	c.Assert(o, DeepEquals, u)

	// Create site install operation
	op1 := storage.SiteOperation{
		AccountID:  a.ID,
		SiteDomain: sa.Domain,
		Type:       "test",
		Created:    now,
		Updated:    now,
		State:      "new",
	}

	out1, err := s.Backend.CreateSiteOperation(op1)
	c.Assert(err, IsNil)
	c.Assert(out1.ID, NotNil)
	op1.ID = out1.ID
	c.Assert(*out1, DeepEquals, op1)

	// token1 is an install token for site, it expires in now + time.Hour
	// it is connected to install operation
	token1 := storage.ProvisioningToken{
		Token:       "tok1",
		Expires:     now.Add(time.Hour),
		Type:        storage.ProvisioningTokenTypeInstall,
		AccountID:   a.ID,
		SiteDomain:  sa.Domain,
		OperationID: op1.ID,
		UserEmail:   u.GetName(),
	}

	tokout, err := s.Backend.CreateProvisioningToken(token1)
	c.Assert(err, IsNil)
	c.Assert(*tokout, DeepEquals, token1)

	tokout, err = s.Backend.GetProvisioningToken(token1.Token)
	c.Assert(err, IsNil)
	c.Assert(*tokout, DeepEquals, token1)

	tokout, err = s.Backend.GetOperationProvisioningToken(sa.Domain, op1.ID)
	c.Assert(err, IsNil)
	c.Assert(*tokout, DeepEquals, token1)

	tokens, err := s.Backend.GetSiteProvisioningTokens(token1.SiteDomain)
	c.Assert(err, IsNil)
	c.Assert(tokens, DeepEquals, []storage.ProvisioningToken{*tokout})

	// token2 is a long lived provisioning token to add new nodes to the
	// existing cluster
	token2 := storage.ProvisioningToken{
		Token:      "tok2",
		Type:       storage.ProvisioningTokenTypeExpand,
		AccountID:  a.ID,
		SiteDomain: sa.Domain,
		UserEmail:  u.GetName(),
	}

	tokout, err = s.Backend.CreateProvisioningToken(token2)
	c.Assert(err, IsNil)
	c.Assert(*tokout, DeepEquals, token2)

	tokout, err = s.Backend.GetProvisioningToken(token2.Token)
	c.Assert(err, IsNil)
	c.Assert(*tokout, DeepEquals, token2)

	tokens, err = s.Backend.GetSiteProvisioningTokens(token1.SiteDomain)
	c.Assert(err, IsNil)
	c.Assert(tokens, DeepEquals, []storage.ProvisioningToken{token1, token2})

	// explicitly delete long lived token
	err = s.Backend.DeleteProvisioningToken(token2.Token)
	c.Assert(err, IsNil)

	_, err = s.Backend.GetProvisioningToken(token2.Token)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v"))
}

func (s *StorageSuite) SchemaVersionPresent(c *C) {
	version, err := s.Backend.SchemaVersion()
	c.Assert(err, IsNil)
	c.Assert(version, Equals, defaults.DatabaseSchemaVersion)
}

// AuthoritiesCRUD tests certificate authorities implementation
func (s *StorageSuite) AuthoritiesCRUD(c *C) {
	out, err := s.Backend.GetCertAuthorities(teleservices.HostCA, false)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	domain1 := "a.example.com"
	ca1 := telesuite.NewTestCA(teleservices.HostCA, domain1)
	err = s.Backend.UpsertCertAuthority(ca1)
	c.Assert(err, IsNil)

	outca, err := s.Backend.GetCertAuthority(*ca1.ID(), true)
	c.Assert(err, IsNil)
	c.Assert(outca, DeepEquals, ca1)

	out, err = s.Backend.GetCertAuthorities(teleservices.HostCA, true)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca1)

	// another CA
	ca2 := telesuite.NewTestCA(teleservices.UserCA, domain1)
	err = s.Backend.UpsertCertAuthority(ca2)
	c.Assert(err, IsNil)

	outca2, err := s.Backend.GetCertAuthority(*ca2.ID(), true)
	c.Assert(err, IsNil)
	c.Assert(outca2, DeepEquals, ca2)

	// upsert with new values
	priv, pub, _ := testauthority.New().GenerateKeyPair("")
	ca1.Spec.SigningKeys = append(ca1.Spec.SigningKeys, priv)
	ca1.Spec.CheckingKeys = append(ca1.Spec.CheckingKeys, pub)
	ca1.Spec.Roles = append(ca1.Spec.Roles, "admin")

	err = s.Backend.UpsertCertAuthority(ca1)
	c.Assert(err, IsNil)

	outca, err = s.Backend.GetCertAuthority(*ca1.ID(), true)
	c.Assert(err, IsNil)
	c.Assert(outca, DeepEquals, ca1)

	// make sure signing keys don't leak
	ca1.Spec.SigningKeys = nil

	outca, err = s.Backend.GetCertAuthority(*ca1.ID(), false)
	c.Assert(err, IsNil)
	c.Assert(outca, DeepEquals, ca1)

	out, err = s.Backend.GetCertAuthorities(teleservices.HostCA, false)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca1)

	// delete
	err = s.Backend.DeleteCertAuthority(*ca1.ID())
	c.Assert(err, IsNil)

	// make sure it's no longer here
	_, err = s.Backend.GetCertAuthority(*ca1.ID(), true)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))

	// delete all test
	err = s.Backend.UpsertCertAuthority(ca1)
	c.Assert(err, IsNil)

	err = s.Backend.DeleteAllCertAuthorities(teleservices.HostCA)
	c.Assert(err, IsNil)

	err = s.Backend.DeleteAllCertAuthorities(teleservices.HostCA)
	c.Assert(err, IsNil)

	_, err = s.Backend.GetCertAuthority(*ca1.ID(), true)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

// NodesCRUD tests presence service implementation
func (s *StorageSuite) NodesCRUD(c *C) {
	type getFn func() ([]teleservices.Server, error)
	type getNS func(namespace string, opts ...teleservices.MarshalOption) ([]teleservices.Server, error)
	type upsertFn func(server teleservices.Server) error

	type tuple struct {
		get    getFn
		getNS  getNS
		upsert upsertFn
		kind   string
	}
	tcs := []tuple{
		{getNS: s.Backend.GetNodes, upsert: s.Backend.UpsertNode, kind: teleservices.KindNode},
		{get: s.Backend.GetProxies, upsert: s.Backend.UpsertProxy, kind: teleservices.KindProxy},
		{get: s.Backend.GetAuthServers, upsert: s.Backend.UpsertAuthServer, kind: teleservices.KindAuthServer},
	}

	for i, tc := range tcs {
		var out []teleservices.Server
		var err error
		if tc.get != nil {
			out, err = tc.get()
		} else {
			out, err = tc.getNS(defaults.Namespace)
		}
		c.Assert(err, IsNil)
		c.Assert(len(out), Equals, 0)

		nodea := &teleservices.ServerV2{
			Version: teleservices.V2,
			Kind:    tc.kind,
			Metadata: teleservices.Metadata{
				Name:      "a",
				Namespace: defaults.Namespace,
				Labels:    map[string]string{"key": "val"},
			},
			Spec: teleservices.ServerSpecV2{
				Addr:     "localhost:4000",
				Hostname: "nodea",
				CmdLabels: map[string]teleservices.CommandLabelV2{
					"ls": {
						Period:  teleservices.NewDuration(time.Minute),
						Command: []string{"ls", "-l"},
						Result:  "/var /root /tmp",
					},
				},
			},
		}
		err = tc.upsert(nodea)
		c.Assert(err, IsNil)

		if tc.get != nil {
			out, err = tc.get()
		} else {
			out, err = tc.getNS(nodea.Metadata.Namespace)
		}

		c.Assert(err, IsNil)
		c.Assert(out, DeepEquals, []teleservices.Server{nodea}, Commentf("test case %v", i))

		nodea.Spec.Hostname = "nodeb"
		nodea.Metadata.Labels = map[string]string{"key": "val2"}
		nodea.Spec.CmdLabels["ls"] = teleservices.CommandLabelV2{
			Period:  teleservices.NewDuration(time.Hour),
			Command: []string{"ls", "-l"},
			Result:  "/var /root /tmp",
		}

		err = tc.upsert(nodea)
		c.Assert(err, IsNil)

		if tc.get != nil {
			out, err = tc.get()
		} else {
			out, err = tc.getNS(nodea.Metadata.Namespace)
		}
		c.Assert(err, IsNil)
		c.Assert(out, DeepEquals, []teleservices.Server{nodea})
	}
}

// ReverseTunnelsCRUD tests presence service - reverse tunnels part
func (s *StorageSuite) ReverseTunnelsCRUD(c *C) {

	out, err := s.Backend.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	tun := teleservices.NewReverseTunnel("example.com", []string{"addr1:100", "addr2:300"})
	err = s.Backend.UpsertReverseTunnel(tun)
	c.Assert(err, IsNil)

	out, err = s.Backend.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []teleservices.ReverseTunnel{tun})

	err = s.Backend.DeleteReverseTunnel("example.com")
	c.Assert(err, IsNil)

	out, err = s.Backend.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)
}

// LocksCRUD tests locking service
func (s *StorageSuite) LocksCRUD(c *C) {
	// test delete lock and acquire
	err := s.Backend.TryAcquireLock("b", time.Hour)
	c.Assert(err, IsNil)

	err = s.Backend.TryAcquireLock("b", time.Hour)
	c.Assert(trace.IsAlreadyExists(err), Equals, true, Commentf("%#v", err))

	err = s.Backend.ReleaseLock("b")
	c.Assert(err, IsNil)

	err = s.Backend.AcquireLock("b", time.Hour)
	c.Assert(err, IsNil)
}

// PeersCRUD tests peers operations
func (s *StorageSuite) PeersCRUD(c *C) {

	out, err := s.Backend.GetPeers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	p1 := storage.Peer{
		ID:            "p1",
		AdvertiseAddr: "https://127.0.0.1:4444",
		LastHeartbeat: s.Clock.Now().UTC(),
	}
	err = s.Backend.UpsertPeer(p1)
	c.Assert(err, IsNil)

	out, err = s.Backend.GetPeers()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []storage.Peer{p1})

	err = s.Backend.DeletePeer(p1.ID)
	c.Assert(err, IsNil)

	out, err = s.Backend.GetPeers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)
}

// ObjectsCRUD tests objects peers operations
func (s *StorageSuite) ObjectsCRUD(c *C) {
	out, err := s.Backend.GetObjects()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	o1 := "object1"
	peers1 := []string{"p1", "p2"}

	err = s.Backend.UpsertObjectPeers(o1, peers1, 0)
	c.Assert(err, IsNil)

	// that's on purpose to test upsert twice
	err = s.Backend.UpsertObjectPeers(o1, peers1, 0)
	c.Assert(err, IsNil)

	out, err = s.Backend.GetObjects()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []string{o1})

	opeers, err := s.Backend.GetObjectPeers(o1)
	c.Assert(err, IsNil)
	c.Assert(opeers, DeepEquals, peers1)

	err = s.Backend.DeleteObjectPeers(o1, []string{"p2"})
	c.Assert(err, IsNil)

	opeers, err = s.Backend.GetObjectPeers(o1)
	c.Assert(err, IsNil)
	c.Assert(opeers, DeepEquals, []string{"p1"})

	err = s.Backend.DeleteObject(o1)
	c.Assert(err, IsNil)

	_, err = s.Backend.GetObjectPeers(o1)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *StorageSuite) ChangesetsCRUD(c *C) {
	// Create
	changeset := storage.PackageChangeset{
		Changes: []storage.PackageUpdate{
			{
				From: loc.MustParseLocator("example.com/a:0.0.1"),
				To:   loc.MustParseLocator("example.com/a:0.0.2"),
			},
		},
	}
	out, err := s.Backend.CreatePackageChangeset(changeset)
	c.Assert(err, IsNil)
	changeset.ID = out.ID
	changeset.Created = out.Created
	c.Assert(*out, DeepEquals, changeset)
	c.Assert(changeset.ID, Not(Equals), "")

	// Read
	out, err = s.Backend.GetPackageChangeset(changeset.ID)
	c.Assert(err, IsNil)
	c.Assert(*out, DeepEquals, changeset)

	// Read all
	changesets, err := s.Backend.GetPackageChangesets()
	c.Assert(err, IsNil)
	c.Assert(changesets, DeepEquals, []storage.PackageChangeset{changeset})

	// Make sure later update comes first
	s.Clock.Advance(time.Hour)
	changeset2, err := s.Backend.CreatePackageChangeset(storage.PackageChangeset{
		Changes: []storage.PackageUpdate{
			{
				From: loc.MustParseLocator("example.com/a:0.0.2"),
				To:   loc.MustParseLocator("example.com/a:0.0.3"),
			},
		},
	})
	c.Assert(err, IsNil)

	// Read all and make sure order is correct
	changesets, err = s.Backend.GetPackageChangesets()
	c.Assert(err, IsNil)
	c.Assert(changesets, DeepEquals, []storage.PackageChangeset{*changeset2, changeset})
}

func (s *StorageSuite) RolesCRUD(c *C) {
	out, err := s.Backend.GetRoles()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	role, err := teleservices.NewRole("role1", teleservices.RoleSpecV3{
		Options: teleservices.RoleOptions{
			MaxSessionTTL: teleservices.NewDuration(teledefaults.MaxCertDuration),
		},
		Allow: teleservices.RoleConditions{
			Namespaces: []string{teledefaults.Namespace},
			Logins:     []string{"root"},
			NodeLabels: teleservices.Labels(map[string]teleutils.Strings{
				teleservices.Wildcard: {teleservices.Wildcard},
			}),
			Rules: []teleservices.Rule{
				{
					Resources: []string{teleservices.Wildcard},
					Verbs:     []string{teleservices.Wildcard},
					Actions: []string{storage.AssignKubernetesGroupsExpr{
						Groups: []string{"admin"},
					}.String()},
				},
			},
		},
	})
	c.Assert(err, IsNil)
	err = s.Backend.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)
	rout, err := s.Backend.GetRole(role.GetMetadata().Name)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, rout, role)

	role.SetLogins(teleservices.Allow, []string{"bob"})
	err = s.Backend.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)
	rout, err = s.Backend.GetRole(role.GetMetadata().Name)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, rout, role)

	err = s.Backend.DeleteRole(role.GetMetadata().Name)
	c.Assert(err, IsNil)

	_, err = s.Backend.GetRole(role.GetMetadata().Name)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))

	// Delete all roles
	err = s.Backend.UpsertRole(role, storage.Forever)
	c.Assert(err, IsNil)

	err = s.Backend.DeleteAllRoles()
	c.Assert(err, IsNil)

	err = s.Backend.DeleteAllRoles()
	c.Assert(err, IsNil)

	_, err = s.Backend.GetRole(role.GetMetadata().Name)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

func (s *StorageSuite) NamespacesCRUD(c *C) {
	out, err := s.Backend.GetNamespaces()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	ns := teleservices.Namespace{
		Kind:    teleservices.KindNamespace,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      defaults.Namespace,
			Namespace: defaults.Namespace,
		},
	}
	err = s.Backend.UpsertNamespace(ns)
	c.Assert(err, IsNil)
	nsout, err := s.Backend.GetNamespace(ns.Metadata.Name)
	c.Assert(err, IsNil)
	compare.DeepCompare(c, nsout, &ns)

	err = s.Backend.DeleteNamespace(ns.Metadata.Name)
	c.Assert(err, IsNil)

	_, err = s.Backend.GetNamespace(ns.Metadata.Name)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

func (s *StorageSuite) LoginAttempts(c *C) {
	user := storage.NewUser("user1",
		storage.UserSpecV2{
			Roles: []string{"admin"},
		})

	_, err := s.Backend.CreateUser(user)
	c.Assert(err, IsNil)

	attempts, err := s.Backend.GetUserLoginAttempts(user.GetName())
	c.Assert(err, IsNil)
	c.Assert(len(attempts), Equals, 0)

	clock := clockwork.NewFakeClock()
	attempt1 := teleservices.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.Backend.AddUserLoginAttempt(user.GetName(), attempt1, teledefaults.AttemptTTL)
	c.Assert(err, IsNil)

	attempt2 := teleservices.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.Backend.AddUserLoginAttempt(user.GetName(), attempt2, teledefaults.AttemptTTL)
	c.Assert(err, IsNil)

	attempts, err = s.Backend.GetUserLoginAttempts(user.GetName())
	c.Assert(err, IsNil)
	compare.DeepCompare(c, attempts, []teleservices.LoginAttempt{attempt1, attempt2})
	c.Assert(teleservices.LastFailed(3, attempts), Equals, false)
	c.Assert(teleservices.LastFailed(2, attempts), Equals, true)

	// now try to delete
	err = s.Backend.DeleteUserLoginAttempts(user.GetName())
	c.Assert(err, IsNil)
	attempts, err = s.Backend.GetUserLoginAttempts(user.GetName())
	c.Assert(err, IsNil)
	c.Assert(len(attempts), Equals, 0)
}

func (s *StorageSuite) ClusterAgentCreds(c *C) {
	clusterName := "t1"

	regularAgent := storage.NewUser(fmt.Sprintf("regular@%v", clusterName), storage.UserSpecV2{})
	regularAgent.SetClusterName(clusterName)
	regularAgent.SetType(storage.AgentUser)
	_, err := s.Backend.CreateUser(regularAgent)
	c.Assert(err, IsNil)

	regularKey := storage.APIKey{Token: "regular", UserEmail: regularAgent.GetName()}
	_, err = s.Backend.CreateAPIKey(regularKey)
	c.Assert(err, IsNil)

	adminAgent := storage.NewUser(fmt.Sprintf("admin@%v", clusterName), storage.UserSpecV2{})
	adminAgent.SetClusterName(clusterName)
	adminAgent.SetType(storage.AgentUser)
	adminAgent.SetRoles([]string{constants.RoleAdmin})
	_, err = s.Backend.CreateUser(adminAgent)
	c.Assert(err, IsNil)

	adminKey := storage.APIKey{Token: "admin", UserEmail: adminAgent.GetName()}
	_, err = s.Backend.CreateAPIKey(adminKey)
	c.Assert(err, IsNil)

	login, err := storage.GetClusterAgentCreds(s.Backend, clusterName, false)
	c.Assert(err, IsNil)
	c.Assert(login.Email, Equals, regularAgent.GetName())
	c.Assert(login.Password, Equals, regularKey.Token)

	login, err = storage.GetClusterAgentCreds(s.Backend, clusterName, true)
	c.Assert(err, IsNil)
	c.Assert(login.Email, Equals, adminAgent.GetName())
	c.Assert(login.Password, Equals, adminKey.Token)
}

func (s *StorageSuite) ClusterLogin(c *C) {
	cluster, err := s.Backend.CreateSite(storage.Site{
		AccountID: defaults.SystemAccountID,
		Domain:    "t1",
		Local:     true,
		Created:   now,
	})
	c.Assert(err, IsNil)

	agent := storage.NewUser(fmt.Sprintf("agent@%v", cluster.Domain), storage.UserSpecV2{})
	agent.SetClusterName(cluster.Domain)
	agent.SetType(storage.AgentUser)
	agent.SetRoles([]string{constants.RoleAdmin})
	_, err = s.Backend.CreateUser(agent)
	c.Assert(err, IsNil)

	key := storage.APIKey{Token: "key", UserEmail: agent.GetName()}
	_, err = s.Backend.CreateAPIKey(key)
	c.Assert(err, IsNil)

	login, err := storage.GetClusterLoginEntry(s.Backend)
	c.Assert(err, IsNil)
	c.Assert(login.Email, Equals, agent.GetName())
	c.Assert(login.Password, Equals, key.Token)

	anotherAgentEmail := fmt.Sprintf("anotheragent@%v", cluster.Domain)
	anotherAgentKey := "anotherkey"
	_, err = s.Backend.UpsertLoginEntry(storage.LoginEntry{
		OpsCenterURL: defaults.GravityServiceURL,
		Email:        anotherAgentEmail,
		Password:     anotherAgentKey,
	})
	c.Assert(err, IsNil)

	login, err = storage.GetClusterLoginEntry(s.Backend)
	c.Assert(err, IsNil)
	c.Assert(login.Email, Equals, anotherAgentEmail)
	c.Assert(login.Password, Equals, anotherAgentKey)
}

func (s *StorageSuite) IndexFile(c *C) {
	// Create a test account just so a basic bucket structure gets initialized.
	_, err := s.Backend.CreateAccount(storage.Account{Org: "test"})
	c.Assert(err, IsNil)

	// No index file initially.
	_, err = s.Backend.GetIndexFile()
	c.Assert(err, FitsTypeOf, trace.NotFound(""))

	// Create a new index file.
	indexFile := newIndex()
	indexFile, _ = addToIndex(*indexFile, "alpine", "0.1.0")

	// Save the index file in the database.
	err = s.Backend.CompareAndSwapIndexFile(indexFile, nil)
	c.Assert(err, IsNil)

	// Retrieve it back and compare.
	retrievedFile, err := s.Backend.GetIndexFile()
	c.Assert(err, IsNil)
	compare.DeepCompare(c, retrievedFile, indexFile)

	// Attempt to save it again without providing the previous one should fail.
	err = s.Backend.CompareAndSwapIndexFile(indexFile, nil)
	c.Assert(err, FitsTypeOf, trace.CompareFailed(""))

	// Simulate concurrent update to test compare and swap.
	updatedIndex1, previousIndex1 := addToIndex(*indexFile, "nginx", "0.2.0")
	updatedIndex2, previousIndex2 := addToIndex(*indexFile, "kafka", "0.3.0")

	// Save the first one.
	err = s.Backend.CompareAndSwapIndexFile(updatedIndex1, previousIndex1)
	c.Assert(err, IsNil)

	// Retrieve it back and compare again.
	retrievedFile, err = s.Backend.GetIndexFile()
	c.Assert(err, IsNil)
	compare.DeepCompare(c, retrievedFile, updatedIndex1)

	// Attempt to save the second one via compare-and-swap should fail.
	err = s.Backend.CompareAndSwapIndexFile(updatedIndex2, previousIndex2)
	c.Assert(err, FitsTypeOf, trace.CompareFailed(""))

	// Now just force-insert the second one.
	err = s.Backend.UpsertIndexFile(*updatedIndex2)
	c.Assert(err, IsNil)

	// Verify that it got replaced.
	retrievedFile, err = s.Backend.GetIndexFile()
	c.Assert(err, IsNil)
	compare.DeepCompare(c, retrievedFile, updatedIndex2)
}

func newIndex() *repo.IndexFile {
	return &repo.IndexFile{
		APIVersion: repo.APIVersionV1,
		Generated:  now,
		Entries:    make(map[string]repo.ChartVersions),
	}
}

func addToIndex(indexFile repo.IndexFile, name, version string) (updated, previous *repo.IndexFile) {
	indexCopy := helmutils.CopyIndexFile(indexFile)
	indexCopy.Entries[name] = []*repo.ChartVersion{{
		Metadata: &chart.Metadata{Name: name, Version: version},
		Created:  now,
	}}
	return indexCopy, &indexFile
}
