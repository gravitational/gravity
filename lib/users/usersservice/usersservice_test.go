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

package usersservice

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/testutils"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/users/suite"

	"github.com/gravitational/teleport"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"

	"github.com/gokyle/hotp"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestUsers(t *testing.T) { TestingT(t) }

type UsersSuite struct {
	backend storage.Backend
	suite   suite.CredsSuite
	dir     string
	clock   clockwork.FakeClock
}

var _ = Suite(&UsersSuite{
	clock: clockwork.NewFakeClockAt(time.Date(2015, 11, 16, 1, 2, 3, 0, time.UTC)),
})

func (s *UsersSuite) SetUpTest(c *C) {
	log.SetOutput(os.Stderr)
	s.dir = c.MkDir()

	var err error
	s.backend, err = keyval.NewBolt(keyval.BoltConfig{
		Path: filepath.Join(s.dir, "bolt.db"),
	})
	c.Assert(err, IsNil)

	s.suite.Users, err = New(Config{
		Backend: s.backend,
		Clock:   s.clock,
	})
	c.Assert(err, IsNil)

	s.suite.Users.SetAuth(&testutils.AuthClient{})
}

func (s *UsersSuite) TearDownTest(c *C) {
	c.Assert(s.backend.Close(), IsNil)
}

func (s *UsersSuite) TestUsersCRUD(c *C) {
	s.suite.UsersCRUD(c)
}

func (s *UsersSuite) TestCreatesAPIKeyForAgents(c *C) {
	s.suite.CreatesAPIKeyForAgents(c)
}

func (s *UsersSuite) TestCreatesGatekeeper(c *C) {
	s.suite.CreatesGatekeeper(c)
}

func (s *UsersSuite) TestAPIKeysCRUD(c *C) {
	s.suite.APIKeysCRUD(c)
}

func (s *UsersSuite) TestClusterConfigCRUD(c *C) {
	s.suite.ClusterConfigCRUD(c)
}

func (s *UsersSuite) TestClusterNameCRUD(c *C) {
	s.suite.ClusterNameCRUD(c)
}

func (s *UsersSuite) TestU2FCRUD(c *C) {
	s.suite.U2FCRUD(c)
}

func (s *UsersSuite) TestClusterAuthPreferenceCRUD(c *C) {
	s.suite.ClusterAuthPreferenceCRUD(c)
}

func (s *UsersSuite) TestClusterConfigStaticTokensCRUD(c *C) {
	s.suite.ClusterConfigStaticTokensCRUD(c)
}

func (s *UsersSuite) TestBearerAuthentication(c *C) {
	s.suite.BearerAuthentication(c)
}

func (s *UsersSuite) TestLocalKeyStorage(c *C) {
	tempFile := filepath.Join(c.MkDir(), "test")
	k, err := NewLocalKeyStore(tempFile)
	c.Assert(err, IsNil)
	c.Assert(k, NotNil)

	// Create
	entry := users.LoginEntry{
		Email:        "alice@example.com",
		Password:     "pass",
		OpsCenterURL: "https://ops1:3088"}
	e, err := k.UpsertLoginEntry(entry)
	c.Assert(err, IsNil)
	c.Assert(*e, DeepEquals, entry)

	// Read
	o, err := k.GetLoginEntry(entry.OpsCenterURL)
	c.Assert(err, IsNil)
	c.Assert(*o, DeepEquals, entry)

	// Read all
	es, err := k.GetLoginEntries()
	c.Assert(err, IsNil)
	c.Assert(es, DeepEquals, []users.LoginEntry{entry})

	// Delete
	err = k.DeleteLoginEntry(entry.OpsCenterURL)
	c.Assert(err, IsNil)

	// Not found
	_, err = k.GetLoginEntry(entry.OpsCenterURL)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected type: %T", err))
}

// TestMigrateInstallToken tests migrations of roles scoped to clusters
func (s *UsersSuite) TestMigrateInstallToken(c *C) {
	identity := s.suite.Users
	err := identity.Migrate()
	c.Assert(err, IsNil)

	// test migration of install token rule
	roleName := "install-token-role"
	clusterName := "example.com"
	repoName := "repo.com"
	err = s.backend.UpsertV2Role(newInstallTokenRole(roleName, clusterName, repoName))
	c.Assert(err, IsNil)

	err = identity.Migrate()
	c.Assert(err, IsNil)

	role, err := s.backend.GetRole(roleName)
	c.Assert(err, IsNil)
	c.Assert(role, NotNil)

	rule := findRule(c, storage.KindCluster, role.GetRules(teleservices.Allow))
	compare.DeepCompare(c, rule, &teleservices.Rule{
		Resources: []string{storage.KindCluster},
		Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
		Where: storage.ContainsExpr{
			Left:  storage.StringsExpr([]string{clusterName}),
			Right: storage.ResourceNameExpr,
		}.String(),
	})

	rule = findRule(c, storage.KindRepository, role.GetRules(teleservices.Allow))
	compare.DeepCompare(c, rule, &teleservices.Rule{
		Resources: []string{storage.KindRepository},
		Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
		Where: storage.ContainsExpr{
			Left:  storage.StringsExpr([]string{repoName}),
			Right: storage.ResourceNameExpr,
		}.String(),
	})

	rule = findRule(c, storage.KindApp, role.GetRules(teleservices.Allow))
	compare.DeepCompare(c, rule, &teleservices.Rule{
		Resources: []string{storage.KindApp},
		Verbs:     []string{teleservices.VerbList, teleservices.VerbRead},
	})

}

// TestMigrateClusterAgents tests migrations of roles
// allowed to connect to kubernetes clusters and have scoped apps
func (s *UsersSuite) TestMigrateClusterAgents(c *C) {
	identity := s.suite.Users
	err := identity.Migrate()
	c.Assert(err, IsNil)

	// test migration of install token rule
	roleName := "cluster-agent"
	clusterName := "example.com"
	err = s.backend.UpsertV2Role(newClusterAgentRole(roleName, clusterName))
	c.Assert(err, IsNil)

	err = identity.Migrate()
	c.Assert(err, IsNil)

	role, err := s.backend.GetRole(roleName)
	c.Assert(err, IsNil)
	c.Assert(role, NotNil)
	rule := findRule(c, storage.KindCluster, role.GetRules(teleservices.Allow), storage.VerbConnect)
	compare.DeepCompare(c, rule, &teleservices.Rule{
		Resources: []string{storage.KindCluster},
		Verbs:     []string{storage.VerbConnect},
		Where: storage.ContainsExpr{
			Left:  storage.StringsExpr([]string{clusterName}),
			Right: storage.ResourceNameExpr,
		}.String(),
	})
}

// TestPasswordRecovery verifies that:
//   * User can reset its password using generated recovery token.
//   * Recovery token cannot be reused.
func (s *UsersSuite) TestPasswordRecovery(c *C) {
	const (
		accountID  = "007"
		accountOrg = "mi7.com"
		email      = "james@mi7.com"
	)

	account, err := s.backend.CreateAccount(storage.Account{
		ID:  accountID,
		Org: accountOrg,
	})
	c.Assert(err, IsNil)

	err = s.suite.Users.CreateUser(storage.NewUser(email,
		storage.UserSpecV2{
			AccountID: account.ID,
			Type:      storage.AdminUser,
			Password:  "password",
		}))
	c.Assert(err, IsNil)

	cap, err := teleservices.NewAuthPreference(teleservices.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, IsNil)

	err = s.suite.Users.SetAuthPreference(cap)
	c.Assert(err, IsNil)

	userToken, err := s.suite.Users.CreateResetToken("https://172.28.128.101:3009/portalapi/v1", email, time.Hour)
	c.Assert(err, IsNil)
	c.Assert(userToken.URL, Equals, "https://172.28.128.101:3009/web/reset/"+userToken.Token)

	userToken, err = s.backend.GetUserToken(userToken.Token)
	c.Assert(err, IsNil)

	otp, err := hotp.Unmarshal(userToken.HOTP)
	c.Assert(err, IsNil)

	resetReq := users.UserTokenCompleteRequest{
		Password:          users.Password("password2"),
		TokenID:           userToken.Token,
		SecondFactorToken: otp.OTP(),
	}
	_, err = s.suite.Users.ResetUserWithToken(resetReq)
	c.Assert(err, IsNil)

	_, _, err = s.suite.Users.AuthenticateUser(httplib.AuthCreds{
		Type:     httplib.AuthBasic,
		Username: email,
		Password: "password2",
	})
	c.Assert(err, NotNil, Commentf(
		"Basic auth shouln't work with OTP token set"))

	resetReq.Password = users.Password("password2")
	_, err = s.suite.Users.ResetUserWithToken(resetReq)
	c.Assert(err, NotNil, Commentf("should not be able to reuse password reset token"))
}

// TestPasswordRecovery verifies that:
//   * User can reset its password using generated recovery token.
//   * Recovery token cannot be reused.
func (s *UsersSuite) TestCreateUserWithToken(c *C) {
	const (
		accountID  = "007"
		accountOrg = "mi7.com"
		name       = "sister@example.com"
	)

	cap, err := teleservices.NewAuthPreference(teleservices.AuthPreferenceSpecV2{
		Type:         teleport.Local,
		SecondFactor: teleport.OTP,
	})
	c.Assert(err, IsNil)

	err = s.suite.Users.SetAuthPreference(cap)
	c.Assert(err, IsNil)

	role, err := users.NewAdminRole()
	c.Assert(err, IsNil)
	err = s.suite.Users.UpsertRole(role, 0)
	c.Assert(err, IsNil)

	invite := storage.UserInvite{
		Name:      name,
		ExpiresIn: time.Hour,
		CreatedBy: "mother@example.com",
		Roles:     []string{role.GetName()},
	}

	userToken, err := s.suite.Users.CreateInviteToken("https://localhost:434/xxx", invite)
	c.Assert(err, IsNil)
	c.Assert(userToken.URL, Equals, "https://localhost:434/web/newuser/"+userToken.Token)

	tokenID := userToken.Token
	userToken, err = s.backend.GetUserToken(tokenID)
	c.Assert(err, IsNil)

	otp, err := hotp.Unmarshal(userToken.HOTP)
	c.Assert(err, IsNil)

	createReq := users.UserTokenCompleteRequest{
		Password:          users.Password("password2"),
		TokenID:           userToken.Token,
		SecondFactorToken: otp.OTP(),
	}
	_, err = s.suite.Users.CreateUserWithToken(createReq)
	c.Assert(err, IsNil)

	_, err = s.suite.Users.GetUser(name)
	c.Assert(err, IsNil)

	_, err = s.suite.Users.GetUserToken(tokenID)
	c.Assert(trace.IsNotFound(err), Equals, true)
}

func (s *UsersSuite) TestBuiltinRoles(c *C) {
	type check struct {
		hasAccess bool
		verb      string
		namespace string
		rule      string
		context   *users.Context
		//nolint:structcheck
		kubernetesGroups []string
	}
	testCases := []struct {
		name   string
		roles  []teleservices.Role
		checks []check
	}{
		{
			name: "1 - can connect to the cluster by name",
			roles: []teleservices.Role{
				MustCreateClusterAgent("agent", "example.com"),
			},
			checks: []check{
				{
					context: &users.Context{
						Context: teleservices.Context{
							Resource: &storage.ClusterV2{
								Kind:    storage.KindCluster,
								Version: teleservices.V2,
								Metadata: teleservices.Metadata{
									Namespace: teledefaults.Namespace,
									Name:      "example.com",
								},
							},
						},
					},
					kubernetesGroups: users.GetAdminKubernetesGroups(),
					rule:             storage.KindCluster,
					verb:             storage.VerbConnect,
					namespace:        teledefaults.Namespace,
					hasAccess:        true,
				},
				{
					context: &users.Context{
						Context: teleservices.Context{
							Resource: &storage.ClusterV2{
								Kind:    storage.KindCluster,
								Version: teleservices.V2,
								Metadata: teleservices.Metadata{
									Namespace: teledefaults.Namespace,
									Name:      "example2.com",
								},
							},
						},
					},
					rule:      storage.KindCluster,
					verb:      storage.VerbConnect,
					namespace: teledefaults.Namespace,
					hasAccess: false,
				},
				{
					context: &users.Context{
						Context: teleservices.Context{
							Resource: storage.NewRepository("example.com"),
						},
					},
					rule:      storage.KindRepository,
					verb:      teleservices.VerbUpdate,
					namespace: teledefaults.Namespace,
					hasAccess: true,
				},
				{
					context: &users.Context{
						Context: teleservices.Context{
							Resource: storage.NewRepository("example2.com"),
						},
					},
					rule:      storage.KindRepository,
					verb:      teleservices.VerbUpdate,
					namespace: teledefaults.Namespace,
					hasAccess: false,
				},
			},
		},
	}
	for i, tc := range testCases {
		var set teleservices.RoleSet
		for i := range tc.roles {
			set = append(set, tc.roles[i])
		}
		for j, check := range tc.checks {
			comment := Commentf("test case %v '%v', check %v", i, tc.name, j)
			result := set.CheckAccessToRule(check.context, check.namespace, check.rule, check.verb, false)
			if check.hasAccess {
				c.Assert(result, IsNil, comment)
			} else {
				c.Assert(trace.IsAccessDenied(result), Equals, true, comment)
			}
		}
	}
}

func MustCreateClusterAgent(name string, clusterName string) teleservices.Role {
	role, err := users.NewClusterAgentRole(name, clusterName)
	if err != nil {
		panic(err)
	}
	return role
}

func findRule(c *C, resource string, rules []teleservices.Rule, verbs ...string) *teleservices.Rule {
	for _, rule := range rules {
		if teleutils.SliceContainsStr(rule.Resources, resource) {
			if len(verbs) != 0 && !teleutils.SliceContainsStr(rule.Verbs, verbs[0]) {
				continue
			}
			return &rule
		}
	}
	c.Fatalf("rule with %v is not found", resource)
	return nil
}

// newRoleV2 returns instance of the new role
func newRoleV2(name string, spec storage.RoleSpecV2) storage.RoleV2 {
	return storage.RoleV2{
		Kind:    teleservices.KindRole,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: teledefaults.Namespace,
		},
		Spec: spec,
	}
}

// newInstallTokenRole is granted after the cluster has been created
// and it allows modifications to one particular cluster
func newInstallTokenRole(name string, clusterName, repoName string) storage.RoleV2 {
	return newRoleV2(name, storage.RoleSpecV2{
		MaxSessionTTL: teleservices.NewDuration(teledefaults.MaxCertDuration),
		Namespaces:    []string{teleservices.Wildcard},
		// do not allow any valid logins but the login list should not be empty,
		// otherwise teleport will reject the web session
		Logins: []string{"invalid-login"},
		Resources: map[string][]string{
			storage.KindCluster:   teleservices.RW(),
			storage.KindApp:       teleservices.RO(),
			teleservices.KindRole: teleservices.RO(),
		},
		System:       true,
		Clusters:     []string{clusterName},
		Repositories: []string{repoName},
	})
}

// newClusterAgentRole returns new agent role used to  run update
// operations on the cluster
func newClusterAgentRole(name string, clusterName string) storage.RoleV2 {
	return newRoleV2(name, storage.RoleSpecV2{
		Namespaces: []string{teledefaults.Namespace},
		Resources: map[string][]string{
			storage.KindCluster: teleservices.RW(),
			storage.KindApp:     teleservices.RW(),
		},
		System:           true,
		Clusters:         []string{clusterName},
		Repositories:     []string{clusterName},
		KubernetesGroups: users.GetAdminKubernetesGroups(),
	})
}
