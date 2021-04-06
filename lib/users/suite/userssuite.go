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

// package credssuite contains a storage acceptance test suite that is
// service implementation independent
package suite

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/teleport"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/tstranex/u2f"
	. "gopkg.in/check.v1"
)

type CredsSuite struct {
	Users users.Identity
}

func (s *CredsSuite) UsersCRUD(c *C) {
	user := "test@gravitational.io"

	// short password
	err := s.Users.UpsertUser(storage.NewUser(user, storage.UserSpecV2{
		Type:     storage.AdminUser,
		Password: "test1",
	}))
	c.Assert(err, NotNil)

	// ok
	err = s.Users.UpsertUser(storage.NewUser(user, storage.UserSpecV2{
		Type:     storage.AdminUser,
		Password: "password1",
	}))
	c.Assert(err, IsNil)

	u, err := s.Users.GetUser(user)
	c.Assert(err, IsNil)
	c.Assert(u.(storage.User).GetAccountID(), Equals, defaults.SystemAccountID)

	pass := []byte("sesame")
	err = s.Users.UpsertPassword(user, pass)
	c.Assert(err, IsNil)
}

func (s *CredsSuite) APIKeysCRUD(c *C) {
	email := "alice@example.com"

	// can't create an API key for non-existent user
	_, err := s.Users.CreateAPIKey(storage.APIKey{UserEmail: email}, false)
	c.Assert(err, NotNil)
	c.Assert(trace.IsNotFound(err), Equals, true)

	// create a user
	err = s.Users.UpsertUser(storage.NewUser(email, storage.UserSpecV2{Type: storage.AgentUser}))
	c.Assert(err, IsNil)

	// now should be able to create an API key
	key, err := s.Users.CreateAPIKey(storage.APIKey{UserEmail: email}, false)
	c.Assert(err, IsNil)

	err = s.Users.DeleteAPIKey(email, key.Token)
	c.Assert(err, IsNil)
}

func (s *CredsSuite) ClusterAuthPreferenceCRUD(c *C) {
	// should not exist
	_, err := s.Users.GetAuthPreference()
	c.Assert(trace.IsNotFound(err), Equals, true)

	// upsert
	cap, err := teleservices.NewAuthPreference(teleservices.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, IsNil)

	err = s.Users.SetAuthPreference(cap)
	c.Assert(err, IsNil)

	// read
	actual, err := s.Users.GetAuthPreference()
	c.Assert(err, IsNil)
	c.Assert(actual.GetType(), Equals, cap.GetType())
	c.Assert(actual.GetSecondFactor(), Equals, cap.GetSecondFactor())
}

func (s *CredsSuite) ClusterConfigStaticTokensCRUD(c *C) {
	// should not exist
	_, err := s.Users.GetStaticTokens()
	c.Assert(trace.IsNotFound(err), Equals, true)

	// create
	staticTokens, err := teleservices.NewStaticTokens(teleservices.StaticTokensSpecV2{
		StaticTokens: []teleservices.ProvisionToken{
			{
				Roles: []teleport.Role{teleport.RoleNode, teleport.RoleProxy, teleport.RoleTrustedCluster},
				Token: "!!token!!",
			},
		},
	})
	c.Assert(err, IsNil)

	err = s.Users.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	// read
	actual, err := s.Users.GetStaticTokens()
	c.Assert(err, IsNil)
	c.Assert(actual, DeepEquals, staticTokens)

}

func (s *CredsSuite) ClusterConfigCRUD(c *C) {
	// should not exist
	_, err := s.Users.GetClusterConfig()
	c.Assert(trace.IsNotFound(err), Equals, true)

	// create
	clusterCfg, err := teleservices.NewClusterConfig(teleservices.ClusterConfigSpecV3{
		SessionRecording: teleservices.RecordAtNode,
	})
	c.Assert(err, IsNil)

	err = s.Users.SetClusterConfig(clusterCfg)
	c.Assert(err, IsNil)

	// read
	actual, err := s.Users.GetClusterConfig()
	c.Assert(err, IsNil)
	c.Assert(actual, DeepEquals, clusterCfg)

}

func (s *CredsSuite) ClusterNameCRUD(c *C) {
	// should not exist
	_, err := s.Users.GetClusterName()
	c.Assert(trace.IsNotFound(err), Equals, true)

	// create
	clusterName, err := teleservices.NewClusterName(teleservices.ClusterNameSpecV2{
		ClusterName: "test",
	})

	c.Assert(err, IsNil)

	err = s.Users.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	// read
	actual, err := s.Users.GetClusterName()
	c.Assert(err, IsNil)
	c.Assert(actual, DeepEquals, clusterName)
}

func (s *CredsSuite) BearerAuthentication(c *C) {
	// no api key, no provisioning token
	user, _, err := s.Users.AuthenticateUser(httplib.AuthCreds{
		Type:     httplib.AuthBearer,
		Password: "tokendoesnotexist",
	})
	c.Assert(err, NotNil)
	c.Assert(user, IsNil)

	// api key
	email1 := "alice@example.com"
	err = s.Users.UpsertUser(storage.NewUser(email1, storage.UserSpecV2{Type: storage.AgentUser}))
	c.Assert(err, IsNil)

	key, err := s.Users.CreateAPIKey(storage.APIKey{UserEmail: email1}, false)
	c.Assert(err, IsNil)

	user, _, err = s.Users.AuthenticateUser(httplib.AuthCreds{
		Type:     httplib.AuthBearer,
		Password: key.Token,
	})
	c.Assert(err, IsNil)
	c.Assert(user.GetName(), Equals, email1)

	// provisioning token
	email2 := "bob@example.com"
	err = s.Users.UpsertUser(storage.NewUser(email2, storage.UserSpecV2{Type: storage.AgentUser}))
	c.Assert(err, IsNil)

	token, err := s.Users.CreateProvisioningToken(storage.ProvisioningToken{
		Token:      "sometoken",
		Type:       storage.ProvisioningTokenTypeInstall,
		AccountID:  "123",
		SiteDomain: "example.com",
		UserEmail:  email2,
	})
	c.Assert(err, IsNil)

	user, _, err = s.Users.AuthenticateUser(httplib.AuthCreds{
		Type:     httplib.AuthBearer,
		Password: token.Token,
	})
	c.Assert(err, IsNil)
	c.Assert(user.GetName(), Equals, email2)
}

func (s *CredsSuite) CreatesAPIKeyForAgents(c *C) {
	// Create agent
	userEmail := "test@gravitational.io"
	_, err := s.Users.CreateRemoteAgent(users.RemoteAccessUser{
		Email: userEmail,
		Token: "abc",
	})
	c.Assert(err, IsNil)

	keys, err := s.Users.GetAPIKeys(userEmail)
	c.Assert(err, IsNil)
	c.Assert(keys, HasLen, 1)

	// create twice, make sure keys are not increasing
	_, err = s.Users.CreateRemoteAgent(users.RemoteAccessUser{
		Email: userEmail,
		Token: "abc",
	})
	c.Assert(err, IsNil)

	keys, err = s.Users.GetAPIKeys(userEmail)
	c.Assert(err, IsNil)
	c.Assert(keys, HasLen, 1)
}

func (s *CredsSuite) CreatesGatekeeper(c *C) {
	// Create user
	userEmail := "gatekeeper@gravitational.io"
	user, err := s.Users.CreateGatekeeper(users.RemoteAccessUser{
		Email: userEmail,
	})
	c.Assert(err, IsNil)

	// API key should be automatically created
	keys, err := s.Users.GetAPIKeys(userEmail)
	c.Assert(err, IsNil)
	c.Assert(keys, HasLen, 1)
	c.Assert(user.Token, Not(Equals), "")
}

func (s *CredsSuite) U2FCRUD(c *C) {
	token := "tok1"
	appId := "https://localhost"
	user1 := "user1"

	challenge, err := u2f.NewChallenge(appId, []string{appId})
	c.Assert(err, IsNil)

	err = s.Users.UpsertU2FRegisterChallenge(token, challenge)
	c.Assert(err, IsNil)

	challengeOut, err := s.Users.GetU2FRegisterChallenge(token)
	c.Assert(err, IsNil)
	c.Assert(challenge.Challenge, DeepEquals, challengeOut.Challenge)
	c.Assert(challenge.Timestamp.Unix(), Equals, challengeOut.Timestamp.Unix())
	c.Assert(challenge.AppID, Equals, challengeOut.AppID)
	c.Assert(challenge.TrustedFacets, DeepEquals, challengeOut.TrustedFacets)

	err = s.Users.UpsertU2FSignChallenge(user1, challenge)
	c.Assert(err, IsNil)

	challengeOut, err = s.Users.GetU2FSignChallenge(user1)
	c.Assert(err, IsNil)
	c.Assert(challenge.Challenge, DeepEquals, challengeOut.Challenge)
	c.Assert(challenge.Timestamp.Unix(), Equals, challengeOut.Timestamp.Unix())
	c.Assert(challenge.AppID, Equals, challengeOut.AppID)
	c.Assert(challenge.TrustedFacets, DeepEquals, challengeOut.TrustedFacets)

	derKey, err := base64.StdEncoding.DecodeString("MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEGOi54Eun0r3Xrj8PjyOGYzJObENYI/t/Lr9g9PsHTHnp1qI2ysIhsdMPd7x/vpsL6cr+2EPVik7921OSsVjEMw==")
	c.Assert(err, IsNil)
	pubkeyInterface, err := x509.ParsePKIXPublicKey(derKey)
	c.Assert(err, IsNil)

	pubkey, ok := pubkeyInterface.(*ecdsa.PublicKey)
	c.Assert(ok, Equals, true)

	registration := u2f.Registration{
		Raw:       []byte("BQQY6LngS6fSvdeuPw+PI4ZjMk5sQ1gj+38uv2D0+wdMeenWojbKwiGx0w93vH++mwvpyv7YQ9WKTv3bU5KxWMQzQIJ+PVFsYjEa0Xgnx+siQaxdlku+U+J2W55U5NrN1iGIc0Amh+0HwhbV2W90G79cxIYS2SVIFAdqTTDXvPXJbeAwggE8MIHkoAMCAQICChWIR0AwlYJZQHcwCgYIKoZIzj0EAwIwFzEVMBMGA1UEAxMMRlQgRklETyAwMTAwMB4XDTE0MDgxNDE4MjkzMloXDTI0MDgxNDE4MjkzMlowMTEvMC0GA1UEAxMmUGlsb3RHbnViYnktMC40LjEtMTU4ODQ3NDAzMDk1ODI1OTQwNzcwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQY6LngS6fSvdeuPw+PI4ZjMk5sQ1gj+38uv2D0+wdMeenWojbKwiGx0w93vH++mwvpyv7YQ9WKTv3bU5KxWMQzMAoGCCqGSM49BAMCA0cAMEQCIIbmYKu6I2L4pgZCBms9NIo9yo5EO9f2irp0ahvLlZudAiC8RN/N+WHAFdq8Z+CBBOMsRBFDDJy3l5EDR83B5GAfrjBEAiBl6R6gAmlbudVpW2jSn3gfjmA8EcWq0JsGZX9oFM/RJwIgb9b01avBY5jBeVIqw5KzClLzbRDMY4K+Ds6uprHyA1Y="),
		KeyHandle: []byte("gn49UWxiMRrReCfH6yJBrF2WS75T4nZbnlTk2s3WIYhzQCaH7QfCFtXZb3Qbv1zEhhLZJUgUB2pNMNe89clt4A=="),
		PubKey:    *pubkey,
	}
	err = s.Users.UpsertU2FRegistration(user1, &registration)
	c.Assert(err, IsNil)

	registrationOut, err := s.Users.GetU2FRegistration(user1)
	c.Assert(err, IsNil)
	c.Assert(&registration, DeepEquals, registrationOut)
}
