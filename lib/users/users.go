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

package users

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/storage"

	teleauth "github.com/gravitational/teleport/lib/auth"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/tstranex/u2f"
)

type RemoteAccessUser storage.RemoteAccessUser

// Identity service manages users and account entries,
// permissions and authentication, signups
type Identity interface {
	Users
	Accounts
	teleservices.Presence
	storage.Locks
	teleservices.ClusterConfiguration
	teleservices.Trust
	teleservices.Access
	teleservices.Identity
	teleservices.Provisioner
}

// Users represents operations on users and permssions,
// it takes care of authentication and authorization
type Users interface {
	// AuthenticateUser authenticates a user by given credentials, it supports
	// Bearer tokens and baisc auth methods
	AuthenticateUser(httplib.AuthCreds) (storage.User, teleservices.AccessChecker, error)

	// GetTelekubeUser returns user by name
	GetTelekubeUser(name string) (storage.User, error)

	// GetAccessChecker returns access checker for user based on users roles
	GetAccessChecker(user storage.User) (teleservices.AccessChecker, error)

	// UpdateUser updates certain user fields
	UpdateUser(name string, req storage.UpdateUserReq) error

	// Migrate is called to migrate legacy data structures to the new format
	Migrate() error

	// SetAuth sets auth handler for users service
	// this is workaround to integrate users service and teleport's
	// auth service until we figure out a better interface/way to do it
	SetAuth(auth teleauth.ClientI)

	// GetSiteProvisioningTokens returns a list of tokens available for the site
	GetSiteProvisioningTokens(siteDomain string) ([]storage.ProvisioningToken, error)

	// GetProvisioningToken returns token by its ID
	GetProvisioningToken(token string) (*storage.ProvisioningToken, error)

	// GetOperationProvisioningToken returns token created for the particular site operation
	GetOperationProvisioningToken(clusterName, operationID string) (*storage.ProvisioningToken, error)

	// CreateProvisioningToken creates a provisioning token from the specified template
	CreateProvisioningToken(storage.ProvisioningToken) (*storage.ProvisioningToken, error)

	// CreateInstallToken creates a new one-time installation token
	CreateInstallToken(storage.InstallToken) (*storage.InstallToken, error)

	// GetInstallToken returns token by its ID
	GetInstallToken(token string) (*storage.InstallToken, error)

	// GetInstallTokenByUser returns token by user ID
	GetInstallTokenByUser(email string) (*storage.InstallToken, error)

	// GetInstallTokenForCluster returns token by cluster name
	GetInstallTokenForCluster(name string) (*storage.InstallToken, error)

	// UpdateInstallToken updates an existing install token and changes role
	// for the user associated with the install token to reduce it's scope
	// to the just created cluster
	UpdateInstallToken(req InstallTokenUpdateRequest) (*storage.InstallToken, teleservices.Role, error)

	// LoginWithInstallToken logs a user using a one-time install token
	LoginWithInstallToken(token string) (*LoginResult, error)

	// CreateAgent creates a new "robot" agent user used by various automation tools (e.g. jenkins)
	// with correct privileges
	CreateAgent(user storage.User) (storage.User, error)

	// CreateRemoteAgent creates a new site agent user that replicates the agent of a remote site.
	// The user usually has a bound API key which is replicated locally
	CreateRemoteAgent(user RemoteAccessUser) (storage.User, error)

	// CreateAgentFromLoginEntry creates a new agent user from the provided
	// login entry
	CreateAgentFromLoginEntry(cluster string, entry storage.LoginEntry, admin bool) (storage.User, error)

	// CreateGatekeeoer creates a new remote access agent user used to connect remote sites
	// to Ops Centers
	CreateGatekeeper(user RemoteAccessUser) (*RemoteAccessUser, error)

	// CreateClusterAgent creates a new cluster agent user used during cluster operations
	// like install/expand and does not have any administrative privileges
	CreateClusterAgent(cluster string, agent storage.User) (storage.User, error)

	// CreateClusterAdminAgent creates a new privileged cluster agent user used during operations
	// like install/expand on master nodes, and has advanced administrative operations
	// e.g. create and delete roles, set up OIDC connectors
	CreateClusterAdminAgent(cluster string, agent storage.User) (storage.User, error)

	// CreateLocalAdmin creates a new admin user for the locally running site
	CreateAdmin(email, password string) error

	// GetAPIKeys returns a list of API keys for the specified user
	GetAPIKeys(userEmail string) ([]storage.APIKey, error)

	// GetAPIKeyByToken returns an API key for the specified token
	GetAPIKeyByToken(token string) (*storage.APIKey, error)

	// CreateAPIKey creates API key for agent user
	CreateAPIKey(key storage.APIKey, upsert bool) (*storage.APIKey, error)

	// DeleteAPIKey creates API Key for agent user
	DeleteAPIKey(userEmail, token string) error
}

// InstallTokenUpdateRequest defines a request to update an install token
type InstallTokenUpdateRequest struct {
	// Token identifies the install token
	Token string `json:"token"`
	// SiteDomain defines the domain to associate the install token with
	SiteDomain string `json:"site_domain"`
	// Repository is a repository with app packages
	Repository string `json:"repository"`
}

// UserTokenCompleteRequest defines a request to complete an action assosiated with
// the user token
type UserTokenCompleteRequest struct {
	// SecondFactorToken is 2nd factor token value
	SecondFactorToken string `json:"second_factor_token"`
	// TokenID is this token ID
	TokenID string `json:"token"`
	// Password is user password
	Password Password `json:"password"`
	// U2FRegisterResponse is U2F register response
	U2FRegisterResponse u2f.RegisterResponse `json:"u2f_register_response"`
}

// Check verifies validity of this request object
func (r InstallTokenUpdateRequest) Check() error {
	if r.Token == "" {
		return trace.BadParameter("missing parameter Token")
	}
	if r.SiteDomain == "" {
		return trace.BadParameter("missing parameter SiteDomain")
	}
	return nil
}

// LoginResult defines the result of logging a user in
type LoginResult struct {
	// Email identifies the user to log in
	Email string `json:"email"`
	// SessionID defines the ID of the web session created as a result of
	// logging in
	SessionID string `json:"session_id"`
}

// Account is a collection of sites and represents some company
type Account storage.Account

// Check checks if given account has correct fields
func (a *Account) Check() error {
	return nil
}

// SignupResult represents successfull signup result:
// * Account that was created
// * User that was created
// * WebSession initiated for this user
type SignupResult struct {
	Account    Account                 `json:"account"`
	User       storage.User            `json:"user"`
	WebSession teleservices.WebSession `json:"web_session"`
}

// Accounts represents a collection of accounts in the portal
type Accounts interface {
	// GetAccount returns account by id
	GetAccount(accountID string) (*Account, error)

	// GetAccounts returns a list of accounts registered in the system
	GetAccounts() ([]Account, error)

	// CreateAccount creates a new account from scratch
	CreateAccount(Account) (*Account, error)

	// CreateInviteToken invites a user
	CreateInviteToken(advertiseURL string, invite storage.UserInvite) (*storage.UserToken, error)

	// GetUserInvites returns a list of active user invites for this account
	GetUserInvites(accountID string) ([]storage.UserInvite, error)

	// DeleteUserInvite deletes user invite
	DeleteUserInvite(accountID, id string) error

	// CreateUser adds user to existing account and sets up 2FA authentication for the user
	// after successful operation it generates web session for the newly created user
	CreateUserWithToken(req UserTokenCompleteRequest) (teleservices.WebSession, error)

	// CreateResetToken resets password and generates token that will allow to create
	// a user for existing account using special secret token (once user confirms email address via OIDC protocol)
	CreateResetToken(advertiseURL string, email string, ttl time.Duration) (*storage.UserToken, error)

	// ResetUserWithToken sets user password and hotp value based on password recovery token
	// and logs in user after that in case of successfull operation
	ResetUserWithToken(req UserTokenCompleteRequest) (teleservices.WebSession, error)

	// UpdatePassword sets user password based on old password
	UpdatePassword(email string, oldPassword, newPassword Password) error

	// ResetPassword resets the user password and returns the new one
	ResetPassword(email string) (string, error)

	// GetUserToken returns a token
	GetUserToken(token string) (*storage.UserToken, error)

	// GetUsersByAccountID returns a list of users registered for given account ID
	GetUsersByAccountID(accountID string) ([]storage.User, error)
}

// LoginEntry represents local login entry for local
// agents running on hosts
// TODO: We don't want users to refer to storage package,
//idea, may be make it internal go package?
type LoginEntry storage.LoginEntry

func (l LoginEntry) String() string {
	return fmt.Sprintf(
		"%v %v", l.OpsCenterURL, l.Email)
}

// KeyStore stores logins for remote portals on computers
type KeyStore struct {
	backend storage.LoginEntries
}

// CredsConfig stores configuration for credentials config
type CredsConfig struct {
	// Backend is a storage backend
	Backend storage.LoginEntries
}

func NewCredsService(cfg CredsConfig) (*KeyStore, error) {
	if cfg.Backend == nil {
		return nil, trace.BadParameter("missing Backend parameter")
	}
	return &KeyStore{
		backend: cfg.Backend,
	}, nil
}

func (c *KeyStore) GetCurrentOpsCenter() string {
	return c.backend.GetCurrentOpsCenter()
}

func (c *KeyStore) SetCurrentOpsCenter(o string) error {
	return c.backend.SetCurrentOpsCenter(o)
}

// UpsertLoginEntry creates or updates login entry for remote OpsCenter
func (c *KeyStore) UpsertLoginEntry(l LoginEntry) (*LoginEntry, error) {
	if _, err := url.Parse(l.OpsCenterURL); err != nil {
		return nil, trace.Wrap(badURL(l.OpsCenterURL))
	}
	if l.Password == "" {
		return nil, trace.BadParameter("missing parameter Password")
	}
	_, err := c.backend.UpsertLoginEntry(storage.LoginEntry(l))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &l, nil
}

// DeleteLoginEntry deletes the login entry for the specified opsCenterURL from the storage
func (c *KeyStore) DeleteLoginEntry(opsCenterURL string) error {
	if _, err := url.Parse(opsCenterURL); err != nil {
		return trace.Wrap(badURL(opsCenterURL))
	}
	_, err := c.backend.GetLoginEntry(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := c.backend.DeleteLoginEntry(opsCenterURL); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetLoginEntry returns the login entry for the specified opsCenterURL from the storage
func (c *KeyStore) GetLoginEntry(opsCenterURL string) (*LoginEntry, error) {
	if _, err := url.Parse(opsCenterURL); err != nil {
		return nil, trace.Wrap(badURL(opsCenterURL))
	}
	le, err := c.backend.GetLoginEntry(opsCenterURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	loginEntry := LoginEntry(*le)
	return &loginEntry, nil
}

// GetLoginEntries lists all login entries
func (c *KeyStore) GetLoginEntries() ([]LoginEntry, error) {
	entries, err := c.backend.GetLoginEntries()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]LoginEntry, len(entries))
	for i, e := range entries {
		out[i] = LoginEntry(e)
	}
	return out, nil
}

func badURL(opsCenterURL string) error {
	return trace.BadParameter("expected a valid OpsCenter URL, got %v", opsCenterURL)
}

// CryptoRandomToken generates crypto-strong pseudo random token
func CryptoRandomToken(length int) (string, error) {
	randomBytes := make([]byte, length)
	if _, err := rand.Reader.Read(randomBytes); err != nil {
		return "", trace.Wrap(err)
	}
	return hex.EncodeToString(randomBytes), nil
}

// Password is a helper type that enforces some sanity
// constraints on the password entered by user
type Password []byte

// Check returns nil, if password matches relaxed requirements
func (p *Password) Check() error {
	if len(*p) < defaults.MinPasswordLength {
		return trace.BadParameter("password is shorter than the minimum of %v characters", defaults.MinPasswordLength)
	}
	if len(*p) > defaults.MaxPasswordLength {
		return trace.BadParameter("password is longer than the maximum of %v characters", defaults.MaxPasswordLength)
	}
	return nil
}

// GetSiteAgent returns API key for a registered site agent user
func GetSiteAgent(siteName string, backend storage.Backend) (*storage.APIKey, error) {
	users, err := backend.GetSiteUsers(siteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var user storage.User
	for i := range users {
		if users[i].GetType() == storage.AgentUser {
			user = users[i]
			break
		}
	}
	if user == nil {
		return nil, trace.NotFound("could not find agent user for site %v", siteName)
	}
	keys, err := backend.GetAPIKeys(user.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(keys) == 0 {
		return nil, trace.NotFound("%v agent user has no API keys", user.GetName())
	}
	return &keys[0], nil
}

// GetOpsCenterAgent returns agent user authenticated to the OpsCenter
func GetOpsCenterAgent(opsCenter, clusterName string, backend storage.Backend) (storage.User, *storage.APIKey, error) {
	users, err := backend.GetSiteUsers(clusterName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var user storage.User
	for i := range users {
		if users[i].GetType() == storage.AgentUser && users[i].GetOpsCenter() == opsCenter {
			user = users[i]
			break
		}
	}
	if user == nil {
		return nil, nil, trace.NotFound(
			"could not find agent for cluster %v and Gravity Hub %v", clusterName, opsCenter)
	}
	keys, err := backend.GetAPIKeys(user.GetName())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(keys) == 0 {
		return nil, nil, trace.NotFound(
			"could not find tokens for agent %v", user.GetName())
	}
	return user, &keys[0], nil
}

// CreateOpsCenterAgent creates a new agent user/API key pair. The user will be
// used to represent the cluster specified with clusterName on the Ops Center
// opsCenter once it has connected to it
func CreateOpsCenterAgent(opsCenter, clusterName string, users Users) (storage.User, *storage.APIKey, error) {
	token, err := CryptoRandomToken(defaults.ProvisioningTokenBytes)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	user, err := users.CreateRemoteAgent(RemoteAccessUser{
		Email:      fmt.Sprintf("agent.%v@%v", opsCenter, clusterName),
		Token:      token,
		SiteDomain: clusterName,
		OpsCenter:  opsCenter,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return user, &storage.APIKey{
		Token:     token,
		UserEmail: user.GetName(),
	}, nil
}

// FindConnector searches for a connector of any supported kind with the provided name
func FindConnector(identity Identity, name string) (teleservices.Resource, error) {
	oidc, err := identity.GetOIDCConnector(name, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		return oidc, nil
	}
	github, err := identity.GetGithubConnector(name, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		return github, nil
	}
	saml, err := identity.GetSAMLConnector(name, false)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		return saml, nil
	}
	return nil, trace.NotFound("connector %q not found", name)
}

// FindAllConnectors returns all existing auth connectors
func FindAllConnectors(identity Identity) (resources []teleservices.Resource, err error) {
	oidc, err := identity.GetOIDCConnectors(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, connector := range oidc {
		resources = append(resources, connector)
	}
	github, err := identity.GetGithubConnectors(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, connector := range github {
		resources = append(resources, connector)
	}
	saml, err := identity.GetSAMLConnectors(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, connector := range saml {
		resources = append(resources, connector)
	}
	return resources, nil
}

// FindPreferredConnector returns a preferred auth connector to use
//
// If cluster authentication preference specifies one, it is returned.
// If only 1 connector is registered, it is returned.
// Otherwise, an error is returned.
func FindPreferredConnector(identity Identity) (teleservices.Resource, error) {
	// first see if cluster authentication preference specifies a connector
	cap, err := identity.GetAuthPreference()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cap.GetConnectorName() != "" {
		return FindConnector(identity, cap.GetConnectorName())
	}
	// otherwise find all connectors and see if there's only one
	connectors, err := FindAllConnectors(identity)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(connectors) == 0 {
		return nil, trace.NotFound("there are no registered auth connectors")
	}
	if len(connectors) > 1 {
		return nil, trace.NotFound("there are %v registered auth connectors",
			len(connectors))
	}
	return connectors[0], nil
}

const (
	ActionRead   = "read"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
)
