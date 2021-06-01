/*
Copyright 2018-2019 Gravitational, Inc.

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
	"crypto/subtle"
	"fmt"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gokyle/hotp"
	"github.com/gravitational/teleport"
	teleauth "github.com/gravitational/teleport/lib/auth"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
	"golang.org/x/crypto/bcrypt"
)

// Config holds configuration parameters for users service
type Config struct {
	// Backend is a storage backend
	Backend storage.Backend
	// Clock is an optional clock that helps to fake time in with tests,
	// if omitted, system time is used
	Clock clockwork.Clock
}

type UsersService struct {
	backend storage.Backend
	clock   clockwork.Clock
	auth    teleauth.ClientI
}

// New returns a new instance of UsersService
func New(cfg Config) (users.Identity, error) {
	if cfg.Backend == nil {
		return nil, trace.BadParameter("missing Backend")
	}
	if cfg.Clock == nil {
		cfg.Clock = clockwork.NewRealClock()
	}
	return &UsersService{
		backend: cfg.Backend,
		clock:   cfg.Clock,
	}, nil
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (c *UsersService) ActivateCertAuthority(id teleservices.CertAuthID) error {
	return c.backend.ActivateCertAuthority(id)
}

// DeactivateCertAuthority moves a CertAuthority from the normal list to
// the deactivated list.
func (c *UsersService) DeactivateCertAuthority(id teleservices.CertAuthID) error {
	return c.backend.DeactivateCertAuthority(id)
}

func (c *UsersService) SetAuth(auth teleauth.ClientI) {
	c.auth = auth
}

func (c *UsersService) CreateAPIKey(key storage.APIKey, upsert bool) (*storage.APIKey, error) {
	// make sure the user we're creating an API key for exists
	_, err := c.GetUser(key.UserEmail)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if key.Token == "" {
		key.Token, err = users.CryptoRandomToken(defaults.AgentTokenBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if upsert {
		_, err = c.backend.UpsertAPIKey(key)
	} else {
		_, err = c.backend.CreateAPIKey(key)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &key, nil
}

func (c *UsersService) GetAPIKeys(userEmail string) (keys []storage.APIKey, err error) {
	// verify user existence
	_, err = c.GetUser(userEmail)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keys, err = c.backend.GetAPIKeys(userEmail)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return keys, nil
}

func (c *UsersService) GetAPIKeyByToken(token string) (key *storage.APIKey, err error) {
	key, err = c.backend.GetAPIKey(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (c *UsersService) DeleteAPIKey(userEmail, token string) error {
	return trace.Wrap(c.backend.DeleteAPIKey(userEmail, token))
}

// CreateProvisioningToken creates a new token from the specified template t
func (c *UsersService) CreateProvisioningToken(t storage.ProvisioningToken) (*storage.ProvisioningToken, error) {
	return c.backend.CreateProvisioningToken(t)
}

func (c *UsersService) GetSiteProvisioningTokens(siteDomain string) ([]storage.ProvisioningToken, error) {
	return c.backend.GetSiteProvisioningTokens(siteDomain)
}

// GetProvisioningToken returns token by ID
func (c *UsersService) GetProvisioningToken(token string) (*storage.ProvisioningToken, error) {
	return c.backend.GetProvisioningToken(token)
}

// GetOperationProvisioningToken returns token created for the particular site operation
func (c *UsersService) GetOperationProvisioningToken(clusterName, operationID string) (*storage.ProvisioningToken, error) {
	return c.backend.GetOperationProvisioningToken(clusterName, operationID)
}

// AddUserLoginAttempt logs user login attempt
func (c *UsersService) AddUserLoginAttempt(user string, attempt teleservices.LoginAttempt, ttl time.Duration) error {
	return c.backend.AddUserLoginAttempt(user, attempt, ttl)
}

// GetUserLoginAttempts returns user login attempts
func (c *UsersService) GetUserLoginAttempts(user string) ([]teleservices.LoginAttempt, error) {
	return c.backend.GetUserLoginAttempts(user)
}

// DeleteUserLoginAttempts removes all login attempts of a user. Should be called after successful login.
func (c *UsersService) DeleteUserLoginAttempts(user string) error {
	return c.backend.DeleteUserLoginAttempts(user)
}

// CreateInstallToken creates a new one-time installation token
func (c *UsersService) CreateInstallToken(t storage.InstallToken) (token *storage.InstallToken, err error) {
	// In case token was supplied externally, use the provided value
	data := t.Token
	if data == "" {
		// generate a token for a one-time installation for the specified account
		data, err = users.CryptoRandomToken(defaults.InstallTokenBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		t.Token = data
	}
	email := fmt.Sprintf("install@%v", data)

	user, err := c.backend.GetUser(t.UserEmail)
	if trace.IsNotFound(err) {
		// we create install token with no actual permissions
		user = storage.NewUser(email, storage.UserSpecV2{
			Type:      t.UserType,
			AccountID: t.AccountID,
		})
		var role teleservices.Role
		if t.Application == nil {
			role, err = users.NewOneTimeLinkRole()
		} else {
			role, err = users.NewOneTimeLinkRoleForApp(*t.Application)
		}
		if err != nil {
			return nil, trace.Wrap(err)
		}
		_, err = c.createUserWithRoles(user, []teleservices.Role{role}, nil)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	t.UserEmail = user.GetName()
	token, err = c.backend.CreateInstallToken(t)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

func (c *UsersService) LoginWithInstallToken(tokenID string) (*users.LoginResult, error) {
	token, err := c.GetInstallToken(tokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var result users.LoginResult
	session, err := c.auth.CreateWebSession(token.UserEmail)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	result.Email = token.UserEmail
	result.SessionID = session.GetName()
	return &result, nil
}

// GetInstallToken returns the token by ID
func (c *UsersService) GetInstallToken(tokenID string) (*storage.InstallToken, error) {
	return c.backend.GetInstallToken(tokenID)
}

// GetInstallTokenByUser returns the token by user ID
func (c *UsersService) GetInstallTokenByUser(email string) (*storage.InstallToken, error) {
	return c.backend.GetInstallTokenByUser(email)
}

// GetInstallTokenForCluster returns token by cluster name
func (c *UsersService) GetInstallTokenForCluster(name string) (*storage.InstallToken, error) {
	return c.backend.GetInstallTokenForCluster(name)
}

// UpdateInstallToken updates an existing install token and changes role
// for the user associated with the install token to reduce it's scope
// to the just created cluster
func (c *UsersService) UpdateInstallToken(req users.InstallTokenUpdateRequest) (*storage.InstallToken, teleservices.Role, error) {
	if err := req.Check(); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	token, err := c.GetInstallToken(req.Token)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	token.SiteDomain = req.SiteDomain
	token, err = c.backend.UpdateInstallToken(*token)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	role, err := users.NewInstallTokenRole(token.UserEmail, token.SiteDomain, req.Repository)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	err = c.backend.UpsertRole(role, storage.Forever)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	user, err := c.backend.GetUser(token.UserEmail)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	roles := []string{role.GetName()}
	if err := c.backend.UpdateUser(user.GetName(), storage.UpdateUserReq{
		Roles: &roles,
	}); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return token, role, trace.Wrap(err)
}

// GetTelekubeUser finds user by email
func (c *UsersService) GetTelekubeUser(email string) (storage.User, error) {
	return c.backend.GetUser(email)
}

// GetUser finds user by email
func (c *UsersService) GetUser(email string) (teleservices.User, error) {
	user, err := c.backend.GetUser(email)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

func (c *UsersService) GetUsers() ([]teleservices.User, error) {
	users, err := c.backend.GetAllUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	teleusers := make([]teleservices.User, 0, len(users))
	for i := range users {
		teleusers = append(teleusers, users[i])
	}
	return teleusers, nil
}

// AuthenticateUser authenticates a user by given credentials, it supports
// basic auth only that is used by agents running on sites
func (c *UsersService) AuthenticateUser(creds httplib.AuthCreds) (storage.User, teleservices.AccessChecker, error) {
	var user storage.User
	var err error
	switch creds.Type {
	case httplib.AuthBasic:
		user, err = c.AuthenticateUserBasicAuth(creds.Username, creds.Password)
	case httplib.AuthBearer:
		user, err = c.AuthenticateUserBearerAuth(creds.Password)
	default:
		err = trace.AccessDenied("unsupported auth type: %v", creds.Type)
	}
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	checker, err := c.GetAccessChecker(user)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return user, checker, nil
}

// GetAccessChecker returns access checker for user based on users roles
func (c *UsersService) GetAccessChecker(user storage.User) (teleservices.AccessChecker, error) {
	roles, err := c.backend.GetUserRoles(user.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleservices.NewRoleSet(roles...), nil
}

// AuthenticateUserBasicAuth authenticates user using basic auth, where password's hash
// is checked against stored hash for AdminUser and token is compared as is
// for AgentUser (treated as API key)
func (c *UsersService) AuthenticateUserBasicAuth(username, password string) (storage.User, error) {
	i, err := c.backend.GetUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, ok := i.(storage.User)
	if !ok {
		return nil, trace.BadParameter("unexpected user type %T", i)
	}

	if err = c.checkCanUseBasicAuth(user); err != nil {
		return nil, trace.Wrap(err)
	}

	switch user.GetType() {
	case storage.AgentUser:
		// check the provided password against agent api keys (it may have a few)
		keys, err := c.backend.GetAPIKeys(user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, k := range keys {
			if subtle.ConstantTimeCompare([]byte(k.Token), []byte(password)) == 1 {
				return user, nil
			}
		}

		return nil, trace.AccessDenied("bad agent api key")

	case storage.AdminUser, storage.RegularUser:
		keys, err := c.backend.GetAPIKeys(user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, k := range keys {
			if subtle.ConstantTimeCompare([]byte(k.Token), []byte(password)) == 1 {
				return user, nil
			}
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.GetPassword()), []byte(password)); err == nil {
			return user, nil
		}

		return nil, trace.AccessDenied("bad user or password")
	default:
		return nil, trace.AccessDenied("unsupported user type: %v", user.GetType())
	}
}

func (c *UsersService) checkCanUseBasicAuth(user storage.User) error {
	// don't allow users with TOTP/HOTP tokens set to use Basic Auth
	totp, err := c.GetTOTP(user.GetName())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if len(totp) != 0 {
		return trace.AccessDenied("basic auth not available")
	}
	if len(user.GetHOTP()) != 0 {
		return trace.AccessDenied("basic auth not available")
	}
	return nil
}

// AuthenticateUserBearerAuth is used to authenticate site agent users
// that connect using provisioning tokens or API keys
func (c *UsersService) AuthenticateUserBearerAuth(token string) (storage.User, error) {
	user, err := c.authenticateAPIKey(token)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if user != nil {
		return user, nil
	}
	return c.authenticateProvisioningToken(token)
}

// authenticateAPIKey is a helper to authenticate a user using API key
func (c *UsersService) authenticateAPIKey(token string) (storage.User, error) {
	key, err := c.backend.GetAPIKey(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u, err := c.backend.GetUser(key.UserEmail)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// authenticateProvisioningToken is a helper to authenticate using provisioning token
func (c *UsersService) authenticateProvisioningToken(token string) (storage.User, error) {
	tok, err := c.backend.GetProvisioningToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u, err := c.backend.GetUser(tok.UserEmail)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

// CreateUser creates a new generic user without privileges
func (c *UsersService) CreateUser(user teleservices.User) error {
	u, ok := user.(storage.User)
	if !ok {
		return trace.BadParameter("unexpected user type %T", user)
	}
	_, err := c.createUserWithRoles(u, nil, nil)
	return trace.Wrap(err)
}

// CreateAgent creates a new "robot" agent user used by various automation tools
// (e.g. release automation) with correct privileges
func (c *UsersService) CreateAgent(agent storage.User) (storage.User, error) {
	agent.SetType(storage.AgentUser)
	reader, err := users.NewReaderRole()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateAgent, err := users.NewUpdateAgentRole(agent.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles := []teleservices.Role{reader, updateAgent}
	return c.createUserWithRoles(agent, roles, nil)
}

// CreateGatekeeper creates a new remote access agent user used to connect remote sites
// to Ops Centers.
func (c *UsersService) CreateGatekeeper(gatekeeper users.RemoteAccessUser) (*users.RemoteAccessUser, error) {
	if gatekeeper.Token == "" {
		var err error
		gatekeeper.Token, err = users.CryptoRandomToken(defaults.AgentTokenBytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	user := storage.NewUser(gatekeeper.Email, storage.UserSpecV2{
		Type:      storage.AgentUser,
		OpsCenter: gatekeeper.OpsCenter,
	})

	gatekeeperRole, err := users.NewGatekeeperRole()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles := []teleservices.Role{gatekeeperRole}

	_, err = c.createUserWithRoles(user, roles, &storage.APIKey{UserEmail: gatekeeper.Email, Token: gatekeeper.Token})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &gatekeeper, nil
}

// CreateRemoteAgent creates a new site agent user that replicates the agent of a remote site.
// The user usually has a bound API key which is replicated locally.
func (c *UsersService) CreateRemoteAgent(agent users.RemoteAccessUser) (storage.User, error) {
	return c.createClusterAgent(
		storage.NewUser(agent.Email, storage.UserSpecV2{
			ClusterName: agent.SiteDomain,
			OpsCenter:   agent.OpsCenter,
		}), agent.SiteDomain, false, &storage.APIKey{
			UserEmail: agent.Email,
			Token:     agent.Token,
		})
}

// CreateAgentFromLoginEntry creates a new agent user from the provided	login entry
func (c *UsersService) CreateAgentFromLoginEntry(clusterName string, entry storage.LoginEntry, admin bool) (storage.User, error) {
	opsCenter, err := utils.URLHostname(entry.OpsCenterURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return c.createClusterAgent(storage.NewUser(entry.Email, storage.UserSpecV2{
		ClusterName: clusterName,
		OpsCenter:   opsCenter,
	}), clusterName, admin, &storage.APIKey{
		UserEmail: entry.Email,
		Token:     entry.Password,
	})
}

// CreateClusterAgent creates unprivileged agent user
func (c *UsersService) CreateClusterAgent(clusterName string, agent storage.User) (storage.User, error) {
	return c.createClusterAgent(agent, clusterName, false, nil)
}

// CreateClusterAdminAgent creates privileged agent user
func (c *UsersService) CreateClusterAdminAgent(clusterName string, agent storage.User) (storage.User, error) {
	return c.createClusterAgent(agent, clusterName, true, nil)
}

func (c *UsersService) createClusterAgent(agent storage.User, clusterName string, admin bool, key *storage.APIKey) (storage.User, error) {
	agent.SetClusterName(clusterName)
	agent.SetType(storage.AgentUser)
	var roles []teleservices.Role
	if admin {
		adminRole, err := users.NewAdminRole()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = []teleservices.Role{adminRole}
	} else {
		readerRole, err := users.NewReaderRole()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusterAgentRole, err := users.NewClusterAgentRole(agent.GetName(), clusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = []teleservices.Role{
			readerRole,
			clusterAgentRole,
		}
	}
	return c.createUserWithRoles(agent, roles, key)
}

// CreateAdmin creates a new admin user for the locally running site.
func (c *UsersService) CreateAdmin(email, password string) error {
	err := teleservices.VerifyPassword([]byte(password))
	if err != nil {
		return trace.Wrap(err)
	}

	// find the local site account
	accounts, err := c.backend.GetAccounts()
	if err != nil {
		return trace.Wrap(err)
	}

	if len(accounts) != 1 {
		return trace.BadParameter("expected 1 account, got: %v", accounts)
	}

	sites, err := c.backend.GetSites(accounts[0].ID)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(sites) != 1 {
		return trace.BadParameter("expected 1 site, got: %v", sites)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return trace.Wrap(err)
	}

	role, err := users.NewAdminRole()
	if err != nil {
		return trace.Wrap(err)
	}

	user := storage.NewUser(email, storage.UserSpecV2{
		Type:      storage.AdminUser,
		Roles:     []string{role.GetName()},
		Password:  string(hashedPassword),
		AccountID: accounts[0].ID,
	})
	_, err = c.createUserWithRoles(user, []teleservices.Role{role}, nil)
	return trace.Wrap(err)
}

func (c *UsersService) createUserWithRoles(user storage.User, roles []teleservices.Role, key *storage.APIKey) (storage.User, error) {
	if err := utils.CheckUserName(user.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	if user.GetType() == "" {
		return nil, trace.BadParameter("user type required")
	}

	for i := range roles {
		role := roles[i]
		err := c.backend.CreateRole(role, storage.Forever)
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return nil, trace.Wrap(err)
			}
		}
		user.AddRole(role.GetName())
	}

	_, err := c.backend.CreateUser(user)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err)
		}
	}

	if user.GetType() == storage.AgentUser {
		if key == nil {
			// only generate keys for user if there are no keys yet
			keys, err := c.backend.GetAPIKeys(user.GetName())
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if len(keys) != 0 {
				return user, nil
			}
			key = &storage.APIKey{UserEmail: user.GetName()}
		}
		err = c.upsertAPIKey(*key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return user, nil
}

func (c *UsersService) upsertAPIKey(key storage.APIKey) (err error) {
	if key.Token == "" {
		key.Token, err = users.CryptoRandomToken(defaults.AgentTokenBytes)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	_, err = c.backend.CreateAPIKey(key)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	return nil
}

// getUserTraits returns traits for the provided user.
//
// If the user has traits already assigned (which is the case for SSO users),
// they are returned as-is. Otherwise returns the default set of traits
// extracted from the user roles.
func (c *UsersService) getUserTraits(user storage.User) (map[string][]string, error) {
	if len(user.GetTraits()) != 0 {
		return user.GetTraits(), nil
	}
	roles, err := teleservices.FetchRoles(user.GetRoles(), c, user.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logins, err := roles.CheckLoginDuration(0)
	if err != nil && !trace.IsAccessDenied(err) { // returns 'access denied' if there're no logins which is ok
		return nil, trace.Wrap(err)
	}
	groups, err := roles.CheckKubeGroups(0)
	if err != nil && !trace.IsAccessDenied(err) { // returns 'access denied' if there're no groups which is ok
		return nil, trace.Wrap(err)
	}
	return map[string][]string{
		teleport.TraitLogins:     logins,
		teleport.TraitKubeGroups: groups,
	}, nil
}

// UpsertUser creates a new user or updates existing user
// In case of AgentUser it will generate a random token - API key
// In case of AdminUser or Regular user it requires a password
// to be set and uses bcrypt to store password's hash
func (c *UsersService) UpsertUser(teleuser teleservices.User) error {
	u, ok := teleuser.(storage.User)
	if !ok {
		return trace.BadParameter("unsupported user type: %T", teleuser)
	}
	err := u.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	traits, err := c.getUserTraits(u)
	if err != nil {
		return trace.Wrap(err)
	}
	u.SetTraits(traits)
	var keys []storage.APIKey
	if u.GetType() == storage.AgentUser {
		// generate a unique api key for the agent
		token, err := users.CryptoRandomToken(defaults.AgentTokenBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		keys = []storage.APIKey{{Token: token, UserEmail: u.GetName()}}
	} else {
		err := teleservices.VerifyPassword([]byte(u.GetPassword()))
		if err != nil {
			return trace.Wrap(err)
		}
		// for regular users, don't store passwords in plaintext
		hash, err := bcrypt.GenerateFromPassword(
			[]byte(u.GetPassword()), bcrypt.DefaultCost)
		if err != nil {
			return trace.Wrap(err)
		}
		u.SetPassword(string(hash))
	}
	if _, err := c.backend.UpsertUser(u); err != nil {
		return trace.Wrap(err)
	}
	for _, k := range keys {
		if _, err := c.backend.CreateAPIKey(k); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UpdateUser updates certain user fields
func (c *UsersService) UpdateUser(username string, req storage.UpdateUserReq) error {
	if req.Roles != nil {
		for _, role := range *req.Roles {
			if _, err := c.backend.GetRole(role); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return c.backend.UpdateUser(username, req)
}

// DeleteUser deletes a user by email
func (c *UsersService) DeleteUser(email string) error {
	if email == "" {
		return trace.BadParameter("email")
	}
	err := c.backend.DeleteUser(email)
	return trace.Wrap(err)
}

// DeleteAllUsers deletes all users
func (c *UsersService) DeleteAllUsers() error {
	return c.backend.DeleteAllUsers()
}

func (c *UsersService) GetLocalClusterName() (string, error) {
	return c.backend.GetLocalClusterName()
}

func (c *UsersService) UpsertLocalClusterName(clusterName string) error {
	return c.backend.UpsertLocalClusterName(clusterName)
}

// GetUserByOIDCIdentity returns a user by it's specified OIDC Identity, returns first
// user specified with this identity
func (c *UsersService) GetUserByOIDCIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	return c.backend.GetUserByOIDCIdentity(id)
}

// GetUserBySAMLIdentity returns a user by it's specified SAML Identity, returns first
// user specified with this identity
func (c *UsersService) GetUserBySAMLIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	return c.backend.GetUserBySAMLIdentity(id)
}

// GetUserByGithubIdentity returns a user by it's specified Github Identity, returns first
// user specified with this identity
func (c *UsersService) GetUserByGithubIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	return c.backend.GetUserByGithubIdentity(id)
}

// UpsertPasswordHash upserts user password hash
func (c *UsersService) UpsertPasswordHash(user string, hash []byte) error {
	token := string(hash)
	return trace.Wrap(c.backend.UpdateUser(user, storage.UpdateUserReq{
		Password: &token,
	}))
}

// GetPasswordHash returns the password hash for a given user
func (c *UsersService) GetPasswordHash(username string) ([]byte, error) {
	user, err := c.backend.GetUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []byte(user.GetPassword()), nil
}

// UpsertHOTP upserts HOTP state for user
func (c *UsersService) UpsertHOTP(user string, otp *hotp.HOTP) error {
	bytes, err := hotp.Marshal(otp)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.backend.UpdateUser(user, storage.UpdateUserReq{
		HOTP: &bytes,
	}))
}

// GetHOTP gets HOTP token state for a user
func (c *UsersService) GetHOTP(username string) (*hotp.HOTP, error) {
	user, err := c.backend.GetUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(user.GetHOTP()) == 0 {
		return nil, trace.NotFound("user %v has no 2FA configured", username)
	}
	otp, err := hotp.Unmarshal(user.GetHOTP())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return otp, nil
}

// GetSignupTokens returns a list of signup tokens
func (c *UsersService) GetSignupTokens() ([]teleservices.SignupToken, error) {
	panic("not implemented")
}

// UpsertWebSession updates or inserts a web session for a user and session id
func (c *UsersService) UpsertWebSession(user, sid string, session teleservices.WebSession) error {
	return trace.Wrap(c.backend.UpsertWebSession(user, sid, session))
}

// GetWebSession returns a web session state for a given user and session id
func (c *UsersService) GetWebSession(user, sid string) (teleservices.WebSession, error) {
	return c.backend.GetWebSession(user, sid)
}

// DeleteWebSession deletes web session from the storage
func (c *UsersService) DeleteWebSession(user, sid string) error {
	return trace.Wrap(c.backend.DeleteWebSession(user, sid))
}

// UpsertPassword upserts new password and HOTP token
func (c *UsersService) UpsertPassword(user string, password []byte) error {
	if err := teleservices.VerifyPassword(password); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	if err != nil {
		return trace.Wrap(err)
	}

	err = c.UpsertPasswordHash(user, hash)
	if err != nil {
		return err
	}
	return nil
}

// UpsertTOTP upserts TOTP secret key for a user that can be used to generate and validate tokens.
func (c *UsersService) UpsertTOTP(user string, secretKey string) error {
	return c.backend.UpsertTOTP(user, secretKey)
}

// GetTOTP returns the secret key used by the TOTP algorithm to validate tokens
func (c *UsersService) GetTOTP(user string) (string, error) {
	return c.backend.GetTOTP(user)
}

// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
// during the 30 second window it's valid.
func (c *UsersService) UpsertUsedTOTPToken(user string, otpToken string) error {
	return c.backend.UpsertUsedTOTPToken(user, otpToken)
}

// GetUsedTOTPToken returns the last successfully used TOTP token. If no token is found zero is returned.
func (c *UsersService) GetUsedTOTPToken(user string) (string, error) {
	return c.backend.GetUsedTOTPToken(user)
}

// DeleteUsedTOTPToken removes the used token from the backend. This should only
// be used during tests.
func (c *UsersService) DeleteUsedTOTPToken(user string) error {
	return c.backend.DeleteUsedTOTPToken(user)
}

// UpsertSignupToken upserts signup token - one time token that lets user to create a user account
func (c *UsersService) UpsertSignupToken(token string, tokenData teleservices.SignupToken, ttl time.Duration) error {
	return trace.Errorf("not implemnented")
}

// GetSignupToken returns signup token data
func (c *UsersService) GetSignupToken(token string) (*teleservices.SignupToken, error) {
	userToken, err := c.backend.GetUserToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	invite, err := c.backend.GetUserInvite(userToken.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Teleport calls GetSignupToken during U2F registration, thus we need to
	// convert userToken to Teleport signupToken.
	// TODO: remove it after adding user reset workflow to Teleport.
	userV1 := teleservices.UserV1{
		Name:  invite.Name,
		Roles: invite.Roles,
	}

	return &teleservices.SignupToken{
		Token:   userToken.Token,
		User:    userV1,
		Expires: userToken.Expires,
	}, nil
}

// DeleteSignupToken deletes signup token from the storage
func (c *UsersService) DeleteSignupToken(token string) error {
	return trace.Errorf("not implemnented")
}

// UpsertOIDCConnector upserts OIDC Connector
func (c *UsersService) UpsertOIDCConnector(connector teleservices.OIDCConnector) error {
	return trace.Wrap(c.backend.UpsertOIDCConnector(connector))
}

// DeleteOIDCConnector deletes OIDC Connector
func (c *UsersService) DeleteOIDCConnector(connectorID string) error {
	return trace.Wrap(c.backend.DeleteOIDCConnector(connectorID))
}

// GetOIDCConnector returns OIDC connector data, withSecrets adds or removes client secret from return results
func (c *UsersService) GetOIDCConnector(id string, withSecrets bool) (teleservices.OIDCConnector, error) {
	return c.backend.GetOIDCConnector(id, withSecrets)
}

// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
func (c *UsersService) GetOIDCConnectors(withSecrets bool) ([]teleservices.OIDCConnector, error) {
	return c.backend.GetOIDCConnectors(withSecrets)
}

// CreateOIDCAuthRequest creates new auth request
func (c *UsersService) CreateOIDCAuthRequest(req teleservices.OIDCAuthRequest, ttl time.Duration) error {
	return c.backend.CreateOIDCAuthRequest(req)
}

// CreateSAMLAuthRequest creates new auth request
func (c *UsersService) CreateSAMLAuthRequest(req teleservices.SAMLAuthRequest, ttl time.Duration) error {
	return c.backend.CreateSAMLAuthRequest(req, ttl)
}

// GetSAMLAuthRequest returns SAML auth request if found
func (c *UsersService) GetSAMLAuthRequest(stateToken string) (*teleservices.SAMLAuthRequest, error) {
	return c.backend.GetSAMLAuthRequest(stateToken)
}

// GetOIDCAuthRequest returns OIDC auth request if found
func (c *UsersService) GetOIDCAuthRequest(stateToken string) (*teleservices.OIDCAuthRequest, error) {
	return c.backend.GetOIDCAuthRequest(stateToken)
}

// CreateSAMLConnector upserts SAML Connector
func (c *UsersService) CreateSAMLConnector(connector teleservices.SAMLConnector) error {
	return trace.Wrap(c.backend.CreateSAMLConnector(connector))
}

// UpsertSAMLConnector upserts SAML Connector
func (c *UsersService) UpsertSAMLConnector(connector teleservices.SAMLConnector) error {
	return trace.Wrap(c.backend.UpsertSAMLConnector(connector))
}

// DeleteSAMLConnector deletes SAML Connector
func (c *UsersService) DeleteSAMLConnector(connectorID string) error {
	return trace.Wrap(c.backend.DeleteSAMLConnector(connectorID))
}

// GetSAMLConnector returns SAML connector data, withSecrets adds or removes client secret from return results
func (c *UsersService) GetSAMLConnector(id string, withSecrets bool) (teleservices.SAMLConnector, error) {
	return c.backend.GetSAMLConnector(id, withSecrets)
}

// GetSAMLConnectors returns registered connectors, withSecrets adds or removes client secret from return results
func (c *UsersService) GetSAMLConnectors(withSecrets bool) ([]teleservices.SAMLConnector, error) {
	return c.backend.GetSAMLConnectors(withSecrets)
}

// CreateGithubConnector creates a new Github connector
func (c *UsersService) CreateGithubConnector(connector teleservices.GithubConnector) error {
	return c.backend.CreateGithubConnector(connector)
}

// UpsertGithubConnector creates or updates a new Github connector
func (c *UsersService) UpsertGithubConnector(connector teleservices.GithubConnector) error {
	return c.backend.UpsertGithubConnector(connector)
}

// GetGithubConnectors returns all configured Github connectors
func (c *UsersService) GetGithubConnectors(withSecrets bool) ([]teleservices.GithubConnector, error) {
	return c.backend.GetGithubConnectors(withSecrets)
}

// GetGithubConnector returns a Github connector by its name
func (c *UsersService) GetGithubConnector(name string, withSecrets bool) (teleservices.GithubConnector, error) {
	return c.backend.GetGithubConnector(name, withSecrets)
}

// DeleteGithubConnector deletes a Github connector by its name
func (c *UsersService) DeleteGithubConnector(name string) error {
	return c.backend.DeleteGithubConnector(name)
}

// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
func (c *UsersService) CreateGithubAuthRequest(req teleservices.GithubAuthRequest) error {
	return c.backend.CreateGithubAuthRequest(req)
}

// GetGithubAuthRequest retrieves Github auth request by the token
func (c *UsersService) GetGithubAuthRequest(stateToken string) (*teleservices.GithubAuthRequest, error) {
	return c.backend.GetGithubAuthRequest(stateToken)
}

// UpsertTunnelConnection upserts tunnel connection
func (c *UsersService) UpsertTunnelConnection(conn teleservices.TunnelConnection) error {
	return c.backend.UpsertTunnelConnection(conn)
}

// GetTunnelConnections returns tunnel connections for a given cluster
func (c *UsersService) GetTunnelConnections(clusterName string, opts ...teleservices.MarshalOption) ([]teleservices.TunnelConnection, error) {
	return c.backend.GetTunnelConnections(clusterName)
}

// GetAllTunnelConnections returns all tunnel connections
func (c *UsersService) GetAllTunnelConnections(opts ...teleservices.MarshalOption) ([]teleservices.TunnelConnection, error) {
	return c.backend.GetAllTunnelConnections()
}

// DeleteTunnelConnection deletes tunnel connection by name
func (c *UsersService) DeleteTunnelConnection(clusterName string, connName string) error {
	return c.backend.DeleteTunnelConnection(clusterName, connName)
}

// DeleteTunnelConnections deletes all tunnel connections for cluster
func (c *UsersService) DeleteTunnelConnections(clusterName string) error {
	return c.backend.DeleteTunnelConnections(clusterName)
}

// DeleteAllTunnelConnections deletes all tunnel connections for cluster
func (c *UsersService) DeleteAllTunnelConnections() error {
	return c.backend.DeleteAllTunnelConnections()
}

func (c *UsersService) GetAccount(accountID string) (*users.Account, error) {
	out, err := c.backend.GetAccount(accountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a := users.Account(*out)
	return &a, nil
}

// CreateAccount creates a new user account from the specified attributes
func (c *UsersService) CreateAccount(a users.Account) (*users.Account, error) {
	if err := a.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	out, err := c.backend.CreateAccount(storage.Account(a))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a = users.Account(*out)
	return &a, nil
}

// GetAccounts returns accounts
func (c *UsersService) GetAccounts() ([]users.Account, error) {
	accts, err := c.backend.GetAccounts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]users.Account, len(accts))
	for i, a := range accts {
		out[i] = users.Account(a)
	}
	return out, nil
}

// UpsertU2FRegisterChallenge upserts a U2F challenge for a new user corresponding to the token
func (c *UsersService) UpsertU2FRegisterChallenge(token string, u2fChallenge *u2f.Challenge) error {
	return c.backend.UpsertU2FRegisterChallenge(token, u2fChallenge)
}

// GetU2FRegisterChallenge returns a U2F challenge for a new user corresponding to the token
func (c *UsersService) GetU2FRegisterChallenge(token string) (*u2f.Challenge, error) {
	return c.backend.GetU2FRegisterChallenge(token)
}

// UpsertU2FRegistration upserts a U2F registration from a valid register response
func (c *UsersService) UpsertU2FRegistration(user string, u2fReg *u2f.Registration) error {
	return c.backend.UpsertU2FRegistration(user, u2fReg)
}

// GetU2FRegistration returns a U2F registration from a valid register response
func (c *UsersService) GetU2FRegistration(user string) (*u2f.Registration, error) {
	return c.backend.GetU2FRegistration(user)
}

// UpsertU2FRegistrationCounter upserts a counter associated with a U2F registration
func (c *UsersService) UpsertU2FRegistrationCounter(user string, counter uint32) error {
	return c.backend.UpsertU2FRegistrationCounter(user, counter)
}

// GetU2FRegistrationCounter returns a counter associated with a U2F registration
func (c *UsersService) GetU2FRegistrationCounter(user string) (counter uint32, e error) {
	return c.backend.GetU2FRegistrationCounter(user)
}

// UpsertU2FSignChallenge upserts a U2F sign (auth) challenge
func (c *UsersService) UpsertU2FSignChallenge(user string, u2fChallenge *u2f.Challenge) error {
	return c.backend.UpsertU2FSignChallenge(user, u2fChallenge)
}

// GetU2FSignChallenge returns a U2F sign (auth) challenge
func (c *UsersService) GetU2FSignChallenge(user string) (*u2f.Challenge, error) {
	return c.backend.GetU2FSignChallenge(user)
}

// CreateResetToken resets user password and creates a token to let existing user to change it
func (c *UsersService) CreateResetToken(advertiseURL string, username string, ttl time.Duration) (*storage.UserToken, error) {
	if err := utils.CheckUserName(username); err != nil {
		return nil, trace.Wrap(err)
	}

	if ttl > defaults.MaxUserResetTokenTTL {
		return nil, trace.BadParameter(
			"failed to create a token: maximum token TTL is %v hours",
			int(defaults.MaxUserResetTokenTTL/time.Hour))
	}

	if ttl == 0 {
		ttl = defaults.UserResetTokenTTL
	}

	user, err := c.backend.GetUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if user.GetType() != storage.AdminUser && user.GetType() != storage.RegularUser {
		return nil, trace.BadParameter("this user %v does not support passwords", user.GetName())
	}

	_, err = c.ResetPassword(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userToken, err := c.createUserToken(storage.UserTokenTypeReset, username, ttl)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenURL, err := formatUserTokenURL(advertiseURL, fmt.Sprintf("/web/reset/%v", userToken.Token))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userToken.URL = tokenURL

	// remove any other invite tokens for this user
	err = c.backend.DeleteUserTokens(storage.UserTokenTypeReset, username)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = c.backend.CreateUserToken(*userToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return c.GetUserToken(userToken.Token)
}

// ResetUserWithToken sets user password based on user token and logs in user
// after that in case of successful operation
func (c *UsersService) ResetUserWithToken(req users.UserTokenCompleteRequest) (teleservices.WebSession, error) {
	pass := req.Password
	if err := pass.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword(pass, bcrypt.DefaultCost)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userToken, otpBytes, err := c.ProcessUserTokenCompleteRequest(storage.UserTokenTypeReset, req)
	if err != nil {
		log.Warningf("Failed to get user token: %v.", err)
		return nil, trace.AccessDenied("expired or incorrect token")
	}

	hashString := string(hash)

	err = c.backend.UpdateUser(userToken.User, storage.UpdateUserReq{
		HOTP:     &otpBytes,
		Password: &hashString,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// dispose of the token so it can't be reused
	err = c.backend.DeleteUserToken(req.TokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := c.auth.CreateWebSession(userToken.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// UpdatePassword updates users password based on the old password
func (c *UsersService) UpdatePassword(email string, oldPassword, newPassword users.Password) error {
	if err := oldPassword.Check(); err != nil {
		return trace.Wrap(err)
	}

	if err := newPassword.Check(); err != nil {
		return trace.Wrap(err)
	}

	user, err := c.backend.GetUser(email)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.GetPassword()), oldPassword); err != nil {
		return trace.BadParameter("passwords do not match")
	}

	hash, err := bcrypt.GenerateFromPassword(newPassword, bcrypt.DefaultCost)
	if err != nil {
		return trace.Wrap(err)
	}
	hashString := string(hash)

	err = c.backend.UpdateUser(email, storage.UpdateUserReq{Password: &hashString})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// ResetPassword resets the user password and returns the new one
func (c *UsersService) ResetPassword(email string) (string, error) {
	_, err := c.backend.GetUser(email)
	if err != nil {
		return "", trace.Wrap(err)
	}

	password, err := users.CryptoRandomToken(defaults.ResetPasswordLength)
	if err != nil {
		return "", trace.Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", trace.Wrap(err)
	}

	hashS := string(hash)
	err = c.backend.UpdateUser(email, storage.UpdateUserReq{Password: &hashS})
	if err != nil {
		return "", trace.Wrap(err)
	}

	return password, nil
}

// GetUserToken returns information about this signup token based on its id
func (c *UsersService) GetUserToken(token string) (*storage.UserToken, error) {
	userToken, err := c.backend.GetUserToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// remove data that can not be sent to the client
	userToken.HOTP = nil
	return userToken, nil
}

// CreateInviteToken invites a user
func (c *UsersService) CreateInviteToken(advertiseURL string, userInvite storage.UserInvite) (*storage.UserToken, error) {
	if err := userInvite.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	if userInvite.ExpiresIn > defaults.MaxSignupTokenTTL {
		return nil, trace.BadParameter("failed to create a token: maximum token TTL is %v hours", int(defaults.MaxSignupTokenTTL/time.Hour))
	}

	if userInvite.ExpiresIn == 0 {
		userInvite.ExpiresIn = defaults.SignupTokenTTL
	}

	// Validate that requested roles exist.
	for _, role := range userInvite.Roles {
		if _, err := c.GetRole(role); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	userToken, err := c.createUserToken(storage.UserTokenTypeInvite, userInvite.Name, userInvite.ExpiresIn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tokenURL, err := formatUserTokenURL(advertiseURL, fmt.Sprintf("/web/newuser/%v", userToken.Token))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userToken.URL = tokenURL

	err = c.backend.DeleteUserTokens(storage.UserTokenTypeInvite, userInvite.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = c.backend.UpsertUserInvite(userInvite)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, err = c.backend.CreateUserToken(*userToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return c.GetUserToken(userToken.Token)
}

// GetUserInvites returns user invites
func (c *UsersService) GetUserInvites(accountID string) ([]storage.UserInvite, error) {
	return c.backend.GetUserInvites()
}

// DeleteUserInvite deletes user invite
func (c *UsersService) DeleteUserInvite(accountID, email string) error {
	return c.backend.DeleteUserInvite(email)
}

// ProcessUserTokenCompleteRequest processes user token complete request
func (c *UsersService) ProcessUserTokenCompleteRequest(tokenType string, req users.UserTokenCompleteRequest) (*storage.UserToken, []byte, error) {
	userToken, err := c.backend.GetUserToken(req.TokenID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if userToken.Type != tokenType {
		return nil, nil, trace.BadParameter("unexpected token type: %v", userToken.Type)
	}

	if userToken.Expires.Before(c.clock.Now().UTC()) {
		return nil, nil, trace.BadParameter("expired token")
	}

	cap, err := c.GetAuthPreference()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	switch cap.GetSecondFactor() {
	case teleport.OFF:
		return userToken, nil, nil
	case teleport.OTP, teleport.TOTP, teleport.HOTP:
		hotpValue := req.SecondFactorToken
		otp, err := hotp.Unmarshal(userToken.HOTP)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		ok := otp.Scan(hotpValue, defaults.HOTPFirstTokensRange)
		if !ok {
			return nil, nil, trace.BadParameter("wrong one-time value")
		}

		otpBytes, err := hotp.Marshal(otp)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return userToken, otpBytes, nil
	case teleport.U2F:
		_, err = cap.GetU2F()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		challenge, err := c.GetU2FRegisterChallenge(req.TokenID)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		u2fRes := req.U2FRegisterResponse
		reg, err := u2f.Register(u2fRes, *challenge, &u2f.Config{SkipAttestationVerify: true})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		err = c.UpsertU2FRegistration(userToken.User, reg)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		err = c.UpsertU2FRegistrationCounter(userToken.User, 0)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return userToken, nil, nil
	}

	return nil, nil, trace.BadParameter("unknown second factor type %q", cap.GetSecondFactor())
}

// CreateUserWithToken creates a user with a token
func (c *UsersService) CreateUserWithToken(completeReq users.UserTokenCompleteRequest) (teleservices.WebSession, error) {
	if err := completeReq.Password.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	hash, err := bcrypt.GenerateFromPassword(completeReq.Password, bcrypt.DefaultCost)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userToken, otpBytes, err := c.ProcessUserTokenCompleteRequest(storage.UserTokenTypeInvite, completeReq)
	if err != nil {
		log.Warningf("Failed to get user token: %v.", err)
		return nil, trace.AccessDenied("expired or incorrect token")
	}

	invite, err := c.backend.GetUserInvite(userToken.User)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := c.filterOutDeletedRoles(invite.Roles)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := c.backend.CreateUser(storage.NewUser(invite.Name, storage.UserSpecV2{
		Type:      storage.AdminUser,
		HOTP:      otpBytes,
		Password:  string(hash),
		AccountID: defaults.SystemAccountID,
		Roles:     roles,
		CreatedBy: teleservices.CreatedBy{
			User: teleservices.UserRef{Name: invite.CreatedBy},
			Time: time.Now().UTC(),
		},
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// dispose of the token so it can't be reused
	err = c.backend.DeleteUserToken(completeReq.TokenID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := c.backend.DeleteUserInvite(invite.Name); err != nil {
		return nil, trace.Wrap(err)
	}

	session, err := c.auth.CreateWebSession(user.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return session, nil
}

// GetUsersByAccountID returns user account
func (c *UsersService) GetUsersByAccountID(accountID string) ([]storage.User, error) {
	return c.backend.GetUsers(accountID)
}

// TryAcquireLock grabs a lock that will be released automatically in ttl time
func (c *UsersService) TryAcquireLock(token string, ttl time.Duration) error {
	return c.backend.TryAcquireLock(token, ttl)
}

// AcquireLock grabs a lock that will be released automatically in ttl time
func (c *UsersService) AcquireLock(token string, ttl time.Duration) error {
	return c.backend.AcquireLock(token, ttl)
}

// ReleaseLock releases lock by token name
func (c *UsersService) ReleaseLock(token string) error {
	return c.backend.ReleaseLock(token)
}

// UpsertToken adds provisioning tokens for the auth server
func (*UsersService) UpsertToken(token string, roles teleport.Roles, ttl time.Duration) error {
	return trace.BadParameter("not implemented")
}

// GetToken is called by Teleport to verify the token supplied by a connecting
// trusted cluster, it is expected to be an API key of Gatekeeper user
func (c *UsersService) GetToken(token string) (*teleservices.ProvisionToken, error) {
	key, err := c.GetAPIKeyByToken(token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if key.UserEmail != constants.GatekeeperUser {
		return nil, trace.NotFound("invalid token %v", token)
	}
	return &teleservices.ProvisionToken{
		Roles: []teleport.Role{teleport.RoleTrustedCluster},
		Token: token,
		// set an expiration date in future as Teleport expects a valid TTL
		Expires: time.Now().Add(time.Hour),
	}, nil
}

// DeleteToken deletes provisioning token
func (*UsersService) DeleteToken(token string) error {
	return nil
}

// GetTokens returns all non-expired tokens
func (*UsersService) GetTokens() ([]teleservices.ProvisionToken, error) {
	return nil, nil
}

// GetNodes returns a list of registered servers
func (c *UsersService) GetNodes(namespace string, opts ...teleservices.MarshalOption) ([]teleservices.Server, error) {
	return c.backend.GetNodes(namespace, opts...)
}

// UpsertNode registers node presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (c *UsersService) UpsertNode(server teleservices.Server) error {
	return c.backend.UpsertNode(server)
}

// UpsertNodes upserts multiple nodes
func (c *UsersService) UpsertNodes(namespace string, servers []teleservices.Server) error {
	return c.backend.UpsertNodes(namespace, servers)
}

// GetAuthServers returns a list of registered servers
func (c *UsersService) GetAuthServers() ([]teleservices.Server, error) {
	return c.backend.GetAuthServers()
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (c *UsersService) UpsertAuthServer(server teleservices.Server) error {
	return c.backend.UpsertAuthServer(server)
}

// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (c *UsersService) UpsertProxy(server teleservices.Server) error {
	return c.backend.UpsertProxy(server)
}

// GetProxies returns a list of registered proxies
func (c *UsersService) GetProxies() ([]teleservices.Server, error) {
	return c.backend.GetProxies()
}

// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
func (c *UsersService) UpsertReverseTunnel(tunnel teleservices.ReverseTunnel) error {
	return c.backend.UpsertReverseTunnel(tunnel)
}

// GetReverseTunnels returns a list of registered servers
func (c *UsersService) GetReverseTunnels() ([]teleservices.ReverseTunnel, error) {
	return c.backend.GetReverseTunnels()
}

// GetReverseTunnel returns reverse tunnel by name
func (c *UsersService) GetReverseTunnel(name string) (teleservices.ReverseTunnel, error) {
	return c.backend.GetReverseTunnel(name)
}

// DeleteReverseTunnel deletes reverse tunnel by it's domain name
func (c *UsersService) DeleteReverseTunnel(domainName string) error {
	return c.backend.DeleteReverseTunnel(domainName)
}

// CreateCertAuthority creates a new certificate authority
func (c *UsersService) CreateCertAuthority(ca teleservices.CertAuthority) error {
	return c.backend.CreateCertAuthority(ca)
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (c *UsersService) UpsertCertAuthority(ca teleservices.CertAuthority) error {
	return c.backend.UpsertCertAuthority(ca)
}

// CompareAndSwapCertAuthority updates existing cert authority if the existing
// cert authority value matches the value stored in the backend
func (c *UsersService) CompareAndSwapCertAuthority(new, existing teleservices.CertAuthority) error {
	return c.backend.CompareAndSwapCertAuthority(new, existing)
}

// DeleteCertAuthority deletes particular certificate authority
func (c *UsersService) DeleteCertAuthority(id teleservices.CertAuthID) error {
	return c.backend.DeleteCertAuthority(id)
}

// DeleteAllCertAuthorities deletes all cert authorities
func (c *UsersService) DeleteAllCertAuthorities(caType teleservices.CertAuthType) error {
	return c.backend.DeleteAllCertAuthorities(caType)
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (c *UsersService) GetCertAuthority(id teleservices.CertAuthID, loadSigningKeys bool, opts ...teleservices.MarshalOption) (teleservices.CertAuthority, error) {
	return c.backend.GetCertAuthority(id, loadSigningKeys)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (c *UsersService) GetCertAuthorities(caType teleservices.CertAuthType, loadSigningKeys bool, opts ...teleservices.MarshalOption) ([]teleservices.CertAuthority, error) {
	return c.backend.GetCertAuthorities(caType, loadSigningKeys, opts...)
}

// GetNamespaces returns a list of namespaces
func (c *UsersService) GetNamespaces() ([]teleservices.Namespace, error) {
	return c.backend.GetNamespaces()
}

// UpsertNamespace upserts namespace
func (c *UsersService) UpsertNamespace(n teleservices.Namespace) error {
	return c.backend.UpsertNamespace(n)
}

// GetNamespace returns a namespace by name
func (c *UsersService) GetNamespace(name string) (*teleservices.Namespace, error) {
	return c.backend.GetNamespace(name)
}

// DeleteNamespace deletes a namespace with all the keys from the backend
func (c *UsersService) DeleteNamespace(namespace string) error {
	return c.backend.DeleteNamespace(namespace)
}

// DeleteAllNamespaces deletes all namespaces
func (c *UsersService) DeleteAllNamespaces() error {
	return c.backend.DeleteAllNamespaces()
}

// GetRoles returns a list of roles registered with the local auth server
func (c *UsersService) GetRoles() ([]teleservices.Role, error) {
	return c.backend.GetRoles()
}

// UpsertRole updates parameters about role
func (c *UsersService) UpsertRole(role teleservices.Role, ttl time.Duration) error {
	return c.backend.UpsertRole(role, ttl)
}

// CreateRole creates new role
func (c *UsersService) CreateRole(role teleservices.Role, ttl time.Duration) error {
	return c.backend.CreateRole(role, ttl)
}

// GetRole returns a role by name
func (c *UsersService) GetRole(name string) (teleservices.Role, error) {
	return c.backend.GetRole(name)
}

// DeleteRole deletes a role with all the keys from the backend
func (c *UsersService) DeleteRole(roleName string) error {
	users, err := c.backend.GetAllUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, user := range users {
		for _, role := range user.GetRoles() {
			if role == roleName {
				return trace.BadParameter("%v is in use by %v", roleName, user.GetName())
			}
		}
	}
	return c.backend.DeleteRole(roleName)
}

// DeleteAllRoles deletes all roles
func (c *UsersService) DeleteAllRoles() error {
	return c.backend.DeleteAllRoles()
}

// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
func (c *UsersService) UpsertTrustedCluster(trustedCluster teleservices.TrustedCluster) (teleservices.TrustedCluster, error) {
	return c.auth.UpsertTrustedCluster(trustedCluster)
}

// GetTrustedCluster returns a single TrustedCluster by name.
func (c *UsersService) GetTrustedCluster(name string) (teleservices.TrustedCluster, error) {
	return c.backend.GetTrustedCluster(name)
}

// GetTrustedClusters returns all TrustedClusters in the backend.
func (c *UsersService) GetTrustedClusters() ([]teleservices.TrustedCluster, error) {
	return c.backend.GetTrustedClusters()
}

// DeleteTrustedCluster removes a TrustedCluster from the backend by name.
func (c *UsersService) DeleteTrustedCluster(name string) error {
	return c.auth.DeleteTrustedCluster(name)
}

// CreateRemoteCluster creates a remote cluster
func (c *UsersService) CreateRemoteCluster(conn teleservices.RemoteCluster) error {
	return c.backend.CreateRemoteCluster(conn)
}

// GetRemoteCluster returns a remote cluster by name
func (c *UsersService) GetRemoteCluster(clusterName string) (teleservices.RemoteCluster, error) {
	return c.backend.GetRemoteCluster(clusterName)
}

// GetRemoteClusters returns a list of remote clusters
func (c *UsersService) GetRemoteClusters(opts ...teleservices.MarshalOption) ([]teleservices.RemoteCluster, error) {
	return c.backend.GetRemoteClusters()
}

// DeleteRemoteCluster deletes remote cluster by name
func (c *UsersService) DeleteRemoteCluster(clusterName string) error {
	return c.backend.DeleteRemoteCluster(clusterName)
}

// DeleteAllRemoteClusters deletes all remote clusters
func (c *UsersService) DeleteAllRemoteClusters() error {
	return c.backend.DeleteAllRemoteClusters()
}

// DeleteAllNodes deletes all nodes
func (c *UsersService) DeleteAllNodes(namespace string) error {
	return c.backend.DeleteAllNodes(namespace)
}

// DeleteAllReverseTunnels deletes all reverse tunnels
func (c *UsersService) DeleteAllReverseTunnels() error {
	return c.backend.DeleteAllReverseTunnels()
}

// DeleteAllProxies deletes all proxies
func (c *UsersService) DeleteAllProxies() error {
	return c.backend.DeleteAllProxies()
}

// SetAuthPreference updates cluster auth preference
func (c *UsersService) SetAuthPreference(authP teleservices.AuthPreference) error {
	err := authP.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	return c.backend.UpsertAuthPreference(authP)
}

// GetAuthPreference returns cluster auth preference
func (c *UsersService) GetAuthPreference() (teleservices.AuthPreference, error) {
	return c.backend.GetAuthPreference()
}

// GetClusterName returns cluster name from cluster configuration
func (c *UsersService) GetClusterName() (teleservices.ClusterName, error) {
	return c.backend.GetClusterName()
}

// SetClusterName sets the name of the cluster in the backend. SetClusterName
// can only be called once on a cluster after which it will return trace.AlreadyExists.
func (c *UsersService) SetClusterName(clusterName teleservices.ClusterName) error {
	return c.backend.CreateClusterName(clusterName)
}

// GetStaticTokens returns static tokens from cluster configuration
func (c *UsersService) GetStaticTokens() (teleservices.StaticTokens, error) {
	return c.backend.GetStaticTokens()
}

// SetStaticTokens updates static tokens in cluster configuration
func (c *UsersService) SetStaticTokens(tokens teleservices.StaticTokens) error {
	return c.backend.UpsertStaticTokens(tokens)
}

// GetClusterConfig returns cluster configuration
func (c *UsersService) GetClusterConfig() (teleservices.ClusterConfig, error) {
	return c.backend.GetClusterConfig()
}

// SetClusterConfig returns cluster configuration
func (c *UsersService) SetClusterConfig(config teleservices.ClusterConfig) error {
	return c.backend.UpsertClusterConfig(config)
}

func formatUserTokenURL(advertiseURL string, path string) (string, error) {
	u, err := url.Parse(advertiseURL)
	if err != nil {
		return "", trace.Wrap(err)
	}

	u.RawQuery = ""
	u.Path = path

	return u.String(), nil
}

func (c *UsersService) createUserToken(tokenType string, name string, ttl time.Duration) (*storage.UserToken, error) {
	err := utils.CheckUserName(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := users.CryptoRandomToken(defaults.SignupTokenBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	otp, err := hotp.GenerateHOTP(defaults.HOTPTokenDigits, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	otpQR, err := otp.QR(fmt.Sprintf("Gravity: %v", name))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	otpBytes, err := hotp.Marshal(otp)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &storage.UserToken{
		Token:   token,
		Expires: c.clock.Now().UTC().Add(ttl),
		Type:    tokenType,
		User:    name,
		HOTP:    otpBytes,
		QRCode:  otpQR,
		Created: c.clock.Now().UTC(),
	}, nil
}

func (c *UsersService) filterOutDeletedRoles(roles []string) ([]string, error) {
	var out []string
	for _, role := range roles {
		_, err := c.backend.GetRole(role)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
		} else {
			out = append(out, role)
		}
	}
	return out, nil
}
