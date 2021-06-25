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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gokyle/hotp"
	"github.com/gravitational/teleport"
	teleauth "github.com/gravitational/teleport/lib/auth"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/tstranex/u2f"
)

// IdentityWithACL returns an instance of the Users interface
// with the specified security context
func IdentityWithACL(backend storage.Backend, identity Identity, user storage.User, checker teleservices.AccessChecker) Identity {
	return &IdentityACL{
		backend:  backend,
		identity: identity,
		user:     user,
		checker:  checker,
		Clock:    clockwork.NewRealClock(),
	}
}

// IdentityACL defines a security aware wrapper around Users
type IdentityACL struct {
	clockwork.Clock
	backend  storage.Backend
	identity Identity
	user     storage.User
	checker  teleservices.AccessChecker
}

func (i *IdentityACL) context() *Context {
	return &Context{Context: teleservices.Context{User: i.user}}
}

func (i *IdentityACL) clusterContext(clusterName string) (*Context, storage.Cluster, error) {
	site, err := i.backend.GetSite(clusterName)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	cluster := storage.NewClusterFromSite(site)
	return &Context{
		Context: teleservices.Context{
			User:     i.user,
			Resource: cluster,
		},
	}, cluster, nil
}

// usersAction checks whether the user has the requested permissions
func (i *IdentityACL) usersAction(action string) error {
	return i.checker.CheckAccessToRule(
		i.context(), defaults.Namespace, teleservices.KindUser, action, false)
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func (i *IdentityACL) currentUserAction(username string) error {
	if username == i.user.GetName() {
		return nil
	}
	return i.checker.CheckAccessToRule(
		i.context(), defaults.Namespace, teleservices.KindUser, teleservices.VerbUpdate, false)
}

func (i *IdentityACL) SetAuth(auth teleauth.ClientI) {
	i.identity.SetAuth(auth)
}

// authConnectorAction is a special checker that grants access to auth
// connectors. It first checks if you have access to the specific connector.
// If not, it checks if the requester has the meta KindAuthConnector access
// (which grants access to all connectors).
func (i *IdentityACL) authConnectorAction(resource string, verb string) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, resource, verb, false); err != nil {
		if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindAuthConnector, verb, false); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// clusterAction checks if user has permissions to perform action
// against cluster with name, action sets `resource` property of the rule
// evaluation access to the cluster object, if multiple verbs are passed,
// they will be checked with `AND` logical operator
func (i *IdentityACL) clusterAction(clusterName string, verbs ...string) error {
	ctx, cluster, err := i.clusterContext(clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, verb := range verbs {
		if err := i.checker.CheckAccessToRule(ctx, cluster.GetMetadata().Namespace, storage.KindCluster, verb, false); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (i *IdentityACL) ActivateCertAuthority(id teleservices.CertAuthID) error {
	return trace.BadParameter("not implemented")
}

func (i *IdentityACL) DeactivateCertAuthority(id teleservices.CertAuthID) error {
	return trace.BadParameter("not implemented")
}

// UpsertTrustedCluster creates or updates a TrustedCluster in the backend.
func (i *IdentityACL) UpsertTrustedCluster(trustedCluster teleservices.TrustedCluster) (teleservices.TrustedCluster, error) {
	if err := i.checker.CheckAccessToRule(i.context(), trustedCluster.GetMetadata().Namespace, teleservices.KindTrustedCluster, teleservices.VerbCreate, false); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), trustedCluster.GetMetadata().Namespace, teleservices.KindTrustedCluster, teleservices.VerbUpdate, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.UpsertTrustedCluster(trustedCluster)
}

// GetTrustedCluster returns a single TrustedCluster by name.
func (i *IdentityACL) GetTrustedCluster(name string) (teleservices.TrustedCluster, error) {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTrustedCluster, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetTrustedCluster(name)
}

// GetTrustedClusters returns all TrustedClusters in the backend.
func (i *IdentityACL) GetTrustedClusters() ([]teleservices.TrustedCluster, error) {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTrustedCluster, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetTrustedClusters()
}

// DeleteTrustedCluster removes a TrustedCluster from the backend by name.
func (i *IdentityACL) DeleteTrustedCluster(name string) error {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTrustedCluster, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteTrustedCluster(name)
}

// CreateRemoteCluster creates a remote cluster
func (i *IdentityACL) CreateRemoteCluster(conn teleservices.RemoteCluster) error {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindRemoteCluster, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateRemoteCluster(conn)
}

// GetRemoteCluster returns a remote cluster by name
func (i *IdentityACL) GetRemoteCluster(clusterName string) (teleservices.RemoteCluster, error) {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindRemoteCluster, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetRemoteCluster(clusterName)
}

// GetRemoteClusters returns a list of remote clusters
func (i *IdentityACL) GetRemoteClusters(opts ...teleservices.MarshalOption) ([]teleservices.RemoteCluster, error) {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindRemoteCluster, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetRemoteClusters()
}

// DeleteRemoteCluster deletes remote cluster by name
func (i *IdentityACL) DeleteRemoteCluster(clusterName string) error {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindRemoteCluster, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteRemoteCluster(clusterName)
}

// DeleteAllRemoteClusters deletes all remote clusters
func (i *IdentityACL) DeleteAllRemoteClusters() error {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindRemoteCluster, teleservices.VerbList, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindRemoteCluster, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllRemoteClusters()
}

// UpsertTunnelConnection upserts tunnel connection
func (i *IdentityACL) UpsertTunnelConnection(conn teleservices.TunnelConnection) error {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertTunnelConnection(conn)
}

// GetTunnelConnections returns tunnel connections for a given cluster
func (i *IdentityACL) GetTunnelConnections(clusterName string, opts ...teleservices.MarshalOption) ([]teleservices.TunnelConnection, error) {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetTunnelConnections(clusterName)
}

// GetAllTunnelConnections returns all tunnel connections
func (i *IdentityACL) GetAllTunnelConnections(opts ...teleservices.MarshalOption) ([]teleservices.TunnelConnection, error) {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetAllTunnelConnections()
}

// DeleteTunnelConnection deletes tunnel connection by name
func (i *IdentityACL) DeleteTunnelConnection(clusterName string, connName string) error {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteTunnelConnection(clusterName, connName)
}

// DeleteTunnelConnections deletes all tunnel connections for cluster
func (i *IdentityACL) DeleteTunnelConnections(clusterName string) error {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbList, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteTunnelConnections(clusterName)
}

// DeleteAllTunnelConnections deletes all tunnel connections for cluster
func (i *IdentityACL) DeleteAllTunnelConnections() error {
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbList, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), defaults.Namespace, teleservices.KindTunnelConnection, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllTunnelConnections()
}

func (i *IdentityACL) CreateAPIKey(key storage.APIKey, upsert bool) (*storage.APIKey, error) {
	if err := i.currentUserAction(key.UserEmail); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateAPIKey(key, upsert)
}

func (i *IdentityACL) GetAPIKeys(username string) (keys []storage.APIKey, err error) {
	if err := i.currentUserAction(username); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetAPIKeys(username)
}

func (i *IdentityACL) GetAPIKeyByToken(token string) (key *storage.APIKey, err error) {
	// token is its own authz, so no extra checks necessary
	return i.identity.GetAPIKeyByToken(token)
}

func (i *IdentityACL) DeleteAPIKey(username, token string) error {
	if err := i.currentUserAction(username); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAPIKey(username, token)
}

func (i *IdentityACL) GetLocalClusterName() (string, error) {
	// anyone can read it, no harm in that
	return i.identity.GetLocalClusterName()
}

func (i *IdentityACL) UpsertLocalClusterName(clusterName string) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindAuthServer, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindAuthServer, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertLocalClusterName(clusterName)
}

// CreateProvisioningToken creates a provisioning token from the specified template
func (i *IdentityACL) CreateProvisioningToken(t storage.ProvisioningToken) (*storage.ProvisioningToken, error) {
	if err := i.clusterAction(t.SiteDomain, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateProvisioningToken(t)
}

func (i *IdentityACL) GetSiteProvisioningTokens(siteDomain string) ([]storage.ProvisioningToken, error) {
	if err := i.clusterAction(siteDomain, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetSiteProvisioningTokens(siteDomain)
}

// GetProvisioningToken returns token by ID
func (i *IdentityACL) GetProvisioningToken(token string) (*storage.ProvisioningToken, error) {
	// token is its own authz, so no extra checks are necessary
	return i.identity.GetProvisioningToken(token)
}

// GetOperationProvisioningToken returns token created for the particular site operation
func (i *IdentityACL) GetOperationProvisioningToken(clusterName, operationID string) (*storage.ProvisioningToken, error) {
	if err := i.clusterAction(clusterName, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetOperationProvisioningToken(clusterName, operationID)
}

// AddUserLoginAttempt logs user login attempt
func (i *IdentityACL) AddUserLoginAttempt(username string, attempt teleservices.LoginAttempt, ttl time.Duration) error {
	if err := i.currentUserAction(username); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.AddUserLoginAttempt(username, attempt, ttl)
}

// GetUserLoginAttempts returns user login attempts
func (i *IdentityACL) GetUserLoginAttempts(user string) ([]teleservices.LoginAttempt, error) {
	if err := i.currentUserAction(user); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetUserLoginAttempts(user)
}

// DeleteUserLoginAttempts removes all login attempts of a user. Should be called after successful login.
func (i *IdentityACL) DeleteUserLoginAttempts(user string) error {
	if err := i.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteUserLoginAttempts(user)
}

// CreateInstallToken creates a new one-time installation token
func (i *IdentityACL) CreateInstallToken(t storage.InstallToken) (*storage.InstallToken, error) {
	if err := i.clusterAction(t.SiteDomain, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateInstallToken(t)
}

func (i *IdentityACL) LoginWithInstallToken(token string) (*LoginResult, error) {
	// token is its own authz, no need for extra check
	return i.identity.LoginWithInstallToken(token)
}

// GetInstallToken returns the token by ID
func (i *IdentityACL) GetInstallToken(token string) (*storage.InstallToken, error) {
	// token is its own authz, no need for extra check
	return i.identity.GetInstallToken(token)
}

// GetInstallTokenByUser returns the token by user ID
func (i *IdentityACL) GetInstallTokenByUser(username string) (*storage.InstallToken, error) {
	if err := i.currentUserAction(username); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetInstallTokenByUser(username)
}

// GetInstallTokenForCluster returns the token by cluster name
func (i *IdentityACL) GetInstallTokenForCluster(name string) (*storage.InstallToken, error) {
	if err := i.usersAction(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetInstallTokenForCluster(name)
}

// UpdateInstallToken updates an existing install token and changes role
// for the user associated with the install token to reduce it's scope
// to the just created cluster
func (i *IdentityACL) UpdateInstallToken(req InstallTokenUpdateRequest) (*storage.InstallToken, teleservices.Role, error) {
	if err := i.clusterAction(req.SiteDomain, teleservices.VerbUpdate); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return i.identity.UpdateInstallToken(req)
}

// GetUser finds user by email
func (i *IdentityACL) GetUser(username string) (teleservices.User, error) {
	if err := i.currentUserAction(username); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetUser(username)
}

// GetTelekubeUser finds user by name
func (i *IdentityACL) GetTelekubeUser(username string) (storage.User, error) {
	if err := i.currentUserAction(username); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetTelekubeUser(username)
}

// Migrate launches migrations
func (i *IdentityACL) Migrate() error {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.Migrate()
}

// UpdateUser updates certain user fields
func (i *IdentityACL) UpdateUser(username string, req storage.UpdateUserReq) error {
	if req.Roles != nil { // changing roles requires admin privileges
		if err := i.usersAction(teleservices.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	} else {
		if err := i.currentUserAction(username); err != nil {
			return trace.Wrap(err)
		}
	}
	return i.identity.UpdateUser(username, req)
}

func (i *IdentityACL) GetUsers() ([]teleservices.User, error) {
	if err := i.usersAction(teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := i.usersAction(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetUsers()
}

// AuthenticateUser authenticates a user by given credentials, it supports
// basic auth only that is used by agents running on sites
func (i *IdentityACL) AuthenticateUser(creds httplib.AuthCreds) (storage.User, teleservices.AccessChecker, error) {
	// this is auth method, no need for extra check
	return i.identity.AuthenticateUser(creds)
}

// GetAccessChecker returns access checker for user based on users roles
func (i *IdentityACL) GetAccessChecker(user storage.User) (teleservices.AccessChecker, error) {
	if err := i.currentUserAction(user.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetAccessChecker(user)
}

// CreateUser creates a new generic user without privileges
func (i *IdentityACL) CreateUser(user teleservices.User) error {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateUser(user)
}

// CreateAgent creates a new "robot" agent user used by various automation tools
// (e.g. release automation) with correct privileges
func (i *IdentityACL) CreateAgent(agent storage.User) (storage.User, error) {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateAgent(agent)
}

// CreateGatekeeper creates a new remote access agent user used to connect remote sites
// to Ops Centers.
func (i *IdentityACL) CreateGatekeeper(gatekeeper RemoteAccessUser) (*RemoteAccessUser, error) {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateGatekeeper(gatekeeper)
}

// CreateRemoteAgent creates a new site agent user that replicates the agent of a remote site.
// The user usually has a bound API key which is replicated locally.
func (i *IdentityACL) CreateRemoteAgent(agent RemoteAccessUser) (storage.User, error) {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateRemoteAgent(agent)
}

// CreateAgentFromLoginEntry creates a new agent user from the provided
// login entry
func (i *IdentityACL) CreateAgentFromLoginEntry(clusterName string, entry storage.LoginEntry, admin bool) (storage.User, error) {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateAgentFromLoginEntry(clusterName, entry, admin)
}

// CreateClusterAgent creates a new cluster agent user used during cluster operations
// like install/expand and does not have any administrative privileges
func (i *IdentityACL) CreateClusterAgent(clusterName string, agent storage.User) (storage.User, error) {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateClusterAgent(clusterName, agent)
}

// CreateClusterAdminAgent creates a new privileged cluster agent user used during operations
// like install/expand on master nodes, and has advanced administrative operations
// e.g. create and delete roles, set up OIDC connectors
func (i *IdentityACL) CreateClusterAdminAgent(clusterName string, agent storage.User) (storage.User, error) {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateClusterAdminAgent(clusterName, agent)
}

// CreateAdmin creates a new admin user for the locally running site.
func (i *IdentityACL) CreateAdmin(email, password string) error {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateAdmin(email, password)
}

// UpsertUser creates a new user or updates existing user
// In case of AgentUser it will generate a random token - API key
// In case of AdminUser or Regular user it requires a password
// to be set and uses bcrypt to store password's hash
func (i *IdentityACL) UpsertUser(teleuser teleservices.User) error {
	if err := i.usersAction(teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	createdBy := teleuser.GetCreatedBy()
	if createdBy.IsEmpty() {
		teleuser.SetCreatedBy(teleservices.CreatedBy{
			User: teleservices.UserRef{Name: i.user.GetName()},
			Time: i.Now().UTC(),
		})
	}
	return i.identity.UpsertUser(teleuser)
}

// DeleteUser deletes a user by username
func (i *IdentityACL) DeleteUser(username string) error {
	if err := i.usersAction(teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteUser(username)
}

// DeleteAllUsers deletes all users
func (i *IdentityACL) DeleteAllUsers() error {
	if err := i.usersAction(teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllUsers()
}

// GetUserByOIDCIdentity returns a user by its specified SAML Identity, returns first
// user specified with this identity
func (i *IdentityACL) GetUserByOIDCIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	if err := i.usersAction(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetUserByOIDCIdentity(id)
}

// GetUserBySAMLIdentity returns a user by its specified SAML Identity, returns first
// user specified with this identity
func (i *IdentityACL) GetUserBySAMLIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	if err := i.usersAction(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetUserBySAMLIdentity(id)
}

// GetUserByGithubIdentity returns a user by its specified Github Identity, returns first
// user specified with this identity
func (i *IdentityACL) GetUserByGithubIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	if err := i.usersAction(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetUserByGithubIdentity(id)
}

// UpsertPasswordHash upserts user password hash
func (i *IdentityACL) UpsertPasswordHash(username string, hash []byte) error {
	if err := i.currentUserAction(username); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertPasswordHash(username, hash)
}

// GetPasswordHash returns the password hash for a given user
func (i *IdentityACL) GetPasswordHash(username string) ([]byte, error) {
	if err := i.currentUserAction(username); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetPasswordHash(username)
}

// UpsertHOTP upserts HOTP state for user
func (i *IdentityACL) UpsertHOTP(username string, otp *hotp.HOTP) error {
	if err := i.currentUserAction(username); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertHOTP(username, otp)
}

// GetHOTP gets HOTP token state for a user
func (i *IdentityACL) GetHOTP(username string) (*hotp.HOTP, error) {
	if err := i.currentUserAction(username); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetHOTP(username)
}

// GetSignupTokens returns a list of signup tokens
func (i *IdentityACL) GetSignupTokens() ([]teleservices.SignupToken, error) {
	if err := i.usersAction(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetSignupTokens()
}

// UpsertWebSession updates or inserts a web session for a user and session id
func (i *IdentityACL) UpsertWebSession(username, sid string, session teleservices.WebSession) error {
	if err := i.currentUserAction(username); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertWebSession(username, sid, session)
}

// GetWebSession returns a web session state for a given user and session id
func (i *IdentityACL) GetWebSession(username, sid string) (teleservices.WebSession, error) {
	if err := i.currentUserAction(username); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetWebSession(username, sid)
}

// DeleteWebSession deletes web session from the storage
func (i *IdentityACL) DeleteWebSession(username, sid string) error {
	if err := i.currentUserAction(username); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteWebSession(username, sid)
}

// UpsertPassword upserts new password and HOTP token
func (i *IdentityACL) UpsertPassword(username string, password []byte) error {
	if err := i.currentUserAction(username); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertPassword(username, password)
}

// UpsertTOTP upserts TOTP secret key for a user that can be used to generate and validate tokens.
func (i *IdentityACL) UpsertTOTP(user string, secretKey string) error {
	if err := i.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertTOTP(user, secretKey)
}

// GetTOTP returns the secret key used by the TOTP algorithm to validate tokens
func (i *IdentityACL) GetTOTP(user string) (string, error) {
	if err := i.currentUserAction(user); err != nil {
		return "", trace.Wrap(err)
	}
	return i.identity.GetTOTP(user)
}

// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
// during the 30 second window it's valid.
func (i *IdentityACL) UpsertUsedTOTPToken(user string, otpToken string) error {
	if err := i.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertUsedTOTPToken(user, otpToken)
}

// GetUsedTOTPToken returns the last successfully used TOTP token. If no token is found zero is returned.
func (i *IdentityACL) GetUsedTOTPToken(user string) (string, error) {
	if err := i.currentUserAction(user); err != nil {
		return "", trace.Wrap(err)
	}
	return i.identity.GetUsedTOTPToken(user)
}

// DeleteUsedTOTPToken removes the used token from the backend. This should only
// be used during tests.
func (i *IdentityACL) DeleteUsedTOTPToken(user string) error {
	if err := i.currentUserAction(user); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteUsedTOTPToken(user)
}

// UpsertSignupToken upserts signup token - one time token that lets user to create a user account
func (i *IdentityACL) UpsertSignupToken(token string, tokenData teleservices.SignupToken, ttl time.Duration) error {
	return trace.BadParameter("not implemented")
}

// GetSignupToken returns signup token data
func (i *IdentityACL) GetSignupToken(token string) (*teleservices.SignupToken, error) {
	return nil, trace.BadParameter("not implemented")
}

// DeleteSignupToken deletes signup token from the storage
func (i *IdentityACL) DeleteSignupToken(token string) error {
	return trace.BadParameter("not implemented")
}

// UpsertOIDCConnector upserts OIDC Connector
func (i *IdentityACL) UpsertOIDCConnector(connector teleservices.OIDCConnector) error {
	if err := i.authConnectorAction(teleservices.KindOIDCConnector, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := i.authConnectorAction(teleservices.KindOIDCConnector, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertOIDCConnector(connector)
}

// CreateSAMLConnector creates SAML Connector
func (i *IdentityACL) CreateSAMLConnector(connector teleservices.SAMLConnector) error {
	if err := i.authConnectorAction(teleservices.KindSAMLConnector, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateSAMLConnector(connector)
}

// UpsertSAMLConnector upserts SAML Connector
func (i *IdentityACL) UpsertSAMLConnector(connector teleservices.SAMLConnector) error {
	if err := i.authConnectorAction(teleservices.KindSAMLConnector, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := i.authConnectorAction(teleservices.KindSAMLConnector, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertSAMLConnector(connector)
}

// DeleteOIDCConnector deletes OIDC Connector
func (i *IdentityACL) DeleteOIDCConnector(connectorID string) error {
	if err := i.authConnectorAction(teleservices.KindOIDCConnector, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteOIDCConnector(connectorID)
}

// DeleteSAMLConnector deletes SAML Connector
func (i *IdentityACL) DeleteSAMLConnector(connectorID string) error {
	if err := i.authConnectorAction(teleservices.KindSAMLConnector, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteSAMLConnector(connectorID)
}

// GetOIDCConnector returns OIDC connector data, withSecrets adds or removes client secret from return results
func (i *IdentityACL) GetOIDCConnector(id string, withSecrets bool) (teleservices.OIDCConnector, error) {
	if err := i.authConnectorAction(teleservices.KindOIDCConnector, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetOIDCConnector(id, withSecrets)
}

// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
func (i *IdentityACL) GetOIDCConnectors(withSecrets bool) ([]teleservices.OIDCConnector, error) {
	if err := i.authConnectorAction(teleservices.KindOIDCConnector, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := i.authConnectorAction(teleservices.KindOIDCConnector, teleservices.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return i.identity.GetOIDCConnectors(withSecrets)
}

// GetSAMLConnector returns SAML connector data, withSecrets adds or removes client secret from return results
func (i *IdentityACL) GetSAMLConnector(id string, withSecrets bool) (teleservices.SAMLConnector, error) {
	if err := i.authConnectorAction(teleservices.KindSAMLConnector, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetSAMLConnector(id, withSecrets)
}

// GetSAMLConnectors returns registered connectors, withSecrets adds or removes client secret from return results
func (i *IdentityACL) GetSAMLConnectors(withSecrets bool) ([]teleservices.SAMLConnector, error) {
	if err := i.authConnectorAction(teleservices.KindSAMLConnector, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := i.authConnectorAction(teleservices.KindSAMLConnector, teleservices.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return i.identity.GetSAMLConnectors(withSecrets)
}

// CreateOIDCAuthRequest creates new auth request
func (i *IdentityACL) CreateOIDCAuthRequest(req teleservices.OIDCAuthRequest, ttl time.Duration) error {
	if err := i.authConnectorAction(teleservices.KindOIDCConnector, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateOIDCAuthRequest(req, ttl)
}

// CreateSAMLAuthRequest creates new auth request
func (i *IdentityACL) CreateSAMLAuthRequest(req teleservices.SAMLAuthRequest, ttl time.Duration) error {
	if err := i.authConnectorAction(teleservices.KindSAMLConnector, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateSAMLAuthRequest(req, ttl)
}

// GetOIDCAuthRequest returns OIDC auth request if found
func (i *IdentityACL) GetOIDCAuthRequest(stateToken string) (*teleservices.OIDCAuthRequest, error) {
	if err := i.usersAction(teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetOIDCAuthRequest(stateToken)
}

// GetSAMLAuthRequest returns SAML auth request if found
func (i *IdentityACL) GetSAMLAuthRequest(stateToken string) (*teleservices.SAMLAuthRequest, error) {
	if err := i.usersAction(teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetSAMLAuthRequest(stateToken)
}

// CreateGithubConnector creates a Github connector
func (i *IdentityACL) CreateGithubConnector(connector teleservices.GithubConnector) error {
	if err := i.authConnectorAction(teleservices.KindGithubConnector, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateGithubConnector(connector)
}

// UpsertGithubConnector upserts a Github connector
func (i *IdentityACL) UpsertGithubConnector(connector teleservices.GithubConnector) error {
	if err := i.authConnectorAction(teleservices.KindGithubConnector, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	if err := i.authConnectorAction(teleservices.KindGithubConnector, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertGithubConnector(connector)
}

// GetGithubConnectors returns Github connectors
func (i *IdentityACL) GetGithubConnectors(withSecrets bool) ([]teleservices.GithubConnector, error) {
	if err := i.authConnectorAction(teleservices.KindGithubConnector, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	if withSecrets {
		if err := i.authConnectorAction(teleservices.KindGithubConnector, teleservices.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return i.identity.GetGithubConnectors(withSecrets)
}

// GetGithubConnector returns Github connector
func (i *IdentityACL) GetGithubConnector(id string, withSecrets bool) (teleservices.GithubConnector, error) {
	if err := i.authConnectorAction(teleservices.KindGithubConnector, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetGithubConnector(id, withSecrets)
}

// DeleteGithubConnector deletes Github connector
func (i *IdentityACL) DeleteGithubConnector(connectorID string) error {
	if err := i.authConnectorAction(teleservices.KindGithubConnector, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteGithubConnector(connectorID)
}

// CreateGithubAuthRequest creates a new Github auth request
func (i *IdentityACL) CreateGithubAuthRequest(req teleservices.GithubAuthRequest) error {
	if err := i.authConnectorAction(teleservices.KindGithubConnector, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateGithubAuthRequest(req)
}

// GetGithubAuthRequest returns Github auth request
func (i *IdentityACL) GetGithubAuthRequest(stateToken string) (*teleservices.GithubAuthRequest, error) {
	if err := i.usersAction(teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetGithubAuthRequest(stateToken)
}

// GetAccount returns account
func (i *IdentityACL) GetAccount(accountID string) (*Account, error) {
	if err := i.checker.CheckAccessToRule(i.context(), storage.KindAccount, teledefaults.Namespace, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetAccount(accountID)
}

// UpsertU2FRegisterChallenge upserts a U2F challenge for a new user corresponding to the token
func (i *IdentityACL) UpsertU2FRegisterChallenge(token string, u2fChallenge *u2f.Challenge) error {
	return i.identity.UpsertU2FRegisterChallenge(token, u2fChallenge)
}

// GetU2FRegisterChallenge returns a U2F challenge for a new user corresponding to the token
func (i *IdentityACL) GetU2FRegisterChallenge(token string) (*u2f.Challenge, error) {
	return i.identity.GetU2FRegisterChallenge(token)
}

// UpsertU2FRegistration upserts a U2F registration from a valid register response
func (i *IdentityACL) UpsertU2FRegistration(user string, u2fReg *u2f.Registration) error {
	return i.identity.UpsertU2FRegistration(user, u2fReg)
}

// GetU2FRegistration returns a U2F registration from a valid register response
func (i *IdentityACL) GetU2FRegistration(user string) (*u2f.Registration, error) {
	return i.identity.GetU2FRegistration(user)
}

// UpsertU2FRegistrationCounter upserts a counter associated with a U2F registration
func (i *IdentityACL) UpsertU2FRegistrationCounter(user string, counter uint32) error {
	return i.identity.UpsertU2FRegistrationCounter(user, counter)
}

// GetU2FRegistrationCounter upserts a counter associated with a U2F registration
func (i *IdentityACL) GetU2FRegistrationCounter(user string) (counter uint32, e error) {
	return i.identity.GetU2FRegistrationCounter(user)
}

// UpsertU2FSignChallenge upserts a U2F sign (auth) challenge
func (i *IdentityACL) UpsertU2FSignChallenge(user string, u2fChallenge *u2f.Challenge) error {
	return i.identity.UpsertU2FSignChallenge(user, u2fChallenge)
}

// GetU2FSignChallenge returns a U2F sign (auth) challenge
func (i *IdentityACL) GetU2FSignChallenge(user string) (*u2f.Challenge, error) {
	return i.identity.GetU2FSignChallenge(user)
}

func (i *IdentityACL) CreateAccount(a Account) (*Account, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, storage.KindAccount, teleservices.VerbCreate, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateAccount(a)
}

func (i *IdentityACL) GetAccounts() ([]Account, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, storage.KindAccount, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetAccounts()
}

// CreateResetToken resets user password and generates token that will allow existing user
// to recover a password
func (i *IdentityACL) CreateResetToken(advertiseURL string, email string, ttl time.Duration) (*storage.UserToken, error) {
	if err := i.usersAction(teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}

	return i.identity.CreateResetToken(advertiseURL, email, ttl)
}

// ResetUserWithToken sets user password based on user secret token
// and logs in user after that in case of successful operation
func (i *IdentityACL) ResetUserWithToken(req UserTokenCompleteRequest) (teleservices.WebSession, error) {
	// token is its own auth, so no extra auth is necessary
	return i.identity.ResetUserWithToken(req)
}

// UpdatePassword updates users password based on the old password
func (i *IdentityACL) UpdatePassword(username string, oldPassword, newPassword Password) error {
	if err := i.currentUserAction(username); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpdatePassword(username, oldPassword, newPassword)
}

// ResetPassword resets the user password and returns the new one
func (i *IdentityACL) ResetPassword(username string) (string, error) {
	if err := i.currentUserAction(username); err != nil {
		return "", trace.Wrap(err)
	}
	return i.identity.ResetPassword(username)
}

// GetUserToken returns information about this signup token based on its id
func (i *IdentityACL) GetUserToken(tokenID string) (*storage.UserToken, error) {
	// token is its own auth, no extra auth is necessary
	return i.identity.GetUserToken(tokenID)
}

// CreateInviteToken creates user invite and returns a token
func (i *IdentityACL) CreateInviteToken(advertiseURL string, invite storage.UserInvite) (*storage.UserToken, error) {
	if err := i.usersAction(teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.CreateInviteToken(advertiseURL, invite)
}

// GetUserInvites returns user invites
func (i *IdentityACL) GetUserInvites(accountID string) ([]storage.UserInvite, error) {
	if err := i.usersAction(teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetUserInvites(accountID)
}

// DeleteUserInvite deletes user invite
func (i *IdentityACL) DeleteUserInvite(accountID, email string) error {
	if err := i.usersAction(teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteUserInvite(accountID, email)
}

// CreateUserWithToken creates a user by UserTokenCompleteRequest
func (i *IdentityACL) CreateUserWithToken(req UserTokenCompleteRequest) (teleservices.WebSession, error) {
	// token is it's own auth, no need for extra auth
	return i.identity.CreateUserWithToken(req)
}

// GetUsersByAccountID returns a list of users for given accountID
func (i *IdentityACL) GetUsersByAccountID(accountID string) ([]storage.User, error) {
	if err := i.usersAction(teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetUsersByAccountID(accountID)
}

// TryAcquireLock grabs a lock that will be released automatically in ttl time
func (i *IdentityACL) TryAcquireLock(token string, ttl time.Duration) error {
	return i.identity.TryAcquireLock(token, ttl)
}

// AcquireLock grabs a lock that will be released automatically in ttl time
func (i *IdentityACL) AcquireLock(token string, ttl time.Duration) error {
	return i.identity.AcquireLock(token, ttl)
}

// ReleaseLock releases lock by token name
func (i *IdentityACL) ReleaseLock(token string) error {
	return i.identity.ReleaseLock(token)
}

// UpsertToken adds provisioning tokens for the auth server
func (i *IdentityACL) UpsertToken(token string, roles teleport.Roles, ttl time.Duration) error {
	return trace.BadParameter("not implemented")
}

// GetToken finds and returns token by id
func (i *IdentityACL) GetToken(token string) (*teleservices.ProvisionToken, error) {
	return nil, trace.NotFound("%v not found - this is nop provisioner", token)
}

// DeleteToken deletes provisioning token
func (i *IdentityACL) DeleteToken(token string) error {
	return nil
}

// GetTokens returns all non-expired tokens
func (i *IdentityACL) GetTokens() ([]teleservices.ProvisionToken, error) {
	return nil, nil
}

// GetNodes returns a list of registered servers
func (i *IdentityACL) GetNodes(namespace string, opts ...teleservices.MarshalOption) ([]teleservices.Server, error) {
	if err := i.checker.CheckAccessToRule(i.context(), namespace, teleservices.KindNode, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetNodes(namespace, opts...)
}

// UpsertNode registers node presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (i *IdentityACL) UpsertNode(server teleservices.Server) error {
	if err := i.checker.CheckAccessToRule(i.context(), server.GetNamespace(), teleservices.KindNode, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), server.GetNamespace(), teleservices.KindNode, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertNode(server)
}

// UpsertNodes upserts multiple nodes
func (i *IdentityACL) UpsertNodes(namespace string, servers []teleservices.Server) error {
	if err := i.checker.CheckAccessToRule(i.context(), namespace, teleservices.KindNode, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), namespace, teleservices.KindNode, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertNodes(namespace, servers)
}

// DeleteAllNodes deletes all nodes
func (i *IdentityACL) DeleteAllNodes(namespace string) error {
	if err := i.checker.CheckAccessToRule(i.context(), namespace, teleservices.KindNode, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllNodes(namespace)
}

// GetAuthServers returns a list of registered servers
func (i *IdentityACL) GetAuthServers() ([]teleservices.Server, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindAuthServer, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetAuthServers()
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (i *IdentityACL) UpsertAuthServer(server teleservices.Server) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindAuthServer, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindAuthServer, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertAuthServer(server)
}

// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (i *IdentityACL) UpsertProxy(server teleservices.Server) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindAuthServer, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindAuthServer, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertProxy(server)
}

// DeleteAllProxies deletes all proxies
func (i *IdentityACL) DeleteAllProxies() error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindNode, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllProxies()
}

// GetProxies returns a list of registered proxies
func (i *IdentityACL) GetProxies() ([]teleservices.Server, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindProxy, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetProxies()
}

// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
func (i *IdentityACL) UpsertReverseTunnel(tunnel teleservices.ReverseTunnel) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindReverseTunnel, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindReverseTunnel, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertReverseTunnel(tunnel)
}

// GetReverseTunnels returns a list of registered servers
func (i *IdentityACL) GetReverseTunnels() ([]teleservices.ReverseTunnel, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindReverseTunnel, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetReverseTunnels()
}

// GetReverseTunnel returns reverse tunnel by name
func (i *IdentityACL) GetReverseTunnel(name string) (teleservices.ReverseTunnel, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindReverseTunnel, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetReverseTunnel(name)
}

// DeleteReverseTunnel deletes reverse tunnel by it's domain name
func (i *IdentityACL) DeleteReverseTunnel(domainName string) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindReverseTunnel, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteReverseTunnel(domainName)
}

// DeleteAllReverseTunnels removes all reverse tunnel values
func (i *IdentityACL) DeleteAllReverseTunnels() error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindReverseTunnel, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllReverseTunnels()
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (i *IdentityACL) UpsertCertAuthority(ca teleservices.CertAuthority) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertCertAuthority(ca)
}

// CompareAndSwapCertAuthority updates existing cert authority if the existing
// cert authority value matches the value stored in the backend
func (i *IdentityACL) CompareAndSwapCertAuthority(new, existing teleservices.CertAuthority) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CompareAndSwapCertAuthority(new, existing)
}

// CreateCertAuthority updates or inserts a new certificate authority
func (i *IdentityACL) CreateCertAuthority(ca teleservices.CertAuthority) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateCertAuthority(ca)
}

// DeleteCertAuthority deletes particular certificate authority
func (i *IdentityACL) DeleteCertAuthority(id teleservices.CertAuthID) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteCertAuthority(id)
}

// DeleteAllCertAuthorities deletes all cert authorities
func (i *IdentityACL) DeleteAllCertAuthorities(certAuthType teleservices.CertAuthType) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllCertAuthorities(certAuthType)
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (i *IdentityACL) GetCertAuthority(id teleservices.CertAuthID, loadSigningKeys bool, opts ...teleservices.MarshalOption) (teleservices.CertAuthority, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetCertAuthority(id, loadSigningKeys)
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (i *IdentityACL) GetCertAuthorities(caType teleservices.CertAuthType, loadSigningKeys bool, opts ...teleservices.MarshalOption) ([]teleservices.CertAuthority, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindCertAuthority, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetCertAuthorities(caType, loadSigningKeys, opts...)
}

// GetNamespaces returns a list of namespaces
func (i *IdentityACL) GetNamespaces() ([]teleservices.Namespace, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindNamespace, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetNamespaces()
}

// UpsertNamespace upserts namespace
func (i *IdentityACL) UpsertNamespace(n teleservices.Namespace) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindNamespace, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindNamespace, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertNamespace(n)
}

// GetNamespace returns a namespace by name
func (i *IdentityACL) GetNamespace(name string) (*teleservices.Namespace, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindNamespace, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetNamespace(name)
}

// DeleteNamespace deletes a namespace with all the keys from the backend
func (i *IdentityACL) DeleteNamespace(namespace string) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindNamespace, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteNamespace(namespace)
}

// DeleteAllNamespaces deletes all namespaces
func (i *IdentityACL) DeleteAllNamespaces() error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindNamespace, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllNamespaces()
}

// GetRoles returns a list of roles registered with the local auth server
func (i *IdentityACL) GetRoles() ([]teleservices.Role, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindRole, teleservices.VerbList, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetRoles()
}

// UpsertRole updates parameters about role
func (i *IdentityACL) UpsertRole(role teleservices.Role, ttl time.Duration) error {
	if role.GetMetadata().Labels[constants.SystemLabel] == constants.True {
		return trace.AccessDenied("modifying roles with %v label is prohibited", constants.SystemLabel)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindRole, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindRole, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.UpsertRole(role, ttl)
}

// CreateRole creates role
func (i *IdentityACL) CreateRole(role teleservices.Role, ttl time.Duration) error {
	if role.GetMetadata().Labels[constants.SystemLabel] == constants.True {
		return trace.AccessDenied("creating roles with %v label is prohibited", constants.SystemLabel)
	}
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindRole, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.CreateRole(role, ttl)
}

// GetRole returns a role by name
func (i *IdentityACL) GetRole(name string) (teleservices.Role, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindRole, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetRole(name)
}

// DeleteRole deletes a role with all the keys from the backend
func (i *IdentityACL) DeleteRole(roleName string) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindRole, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	role, err := i.identity.GetRole(roleName)
	if err != nil {
		return trace.Wrap(err)
	}
	if role.GetMetadata().Labels[constants.SystemLabel] == constants.True {
		return trace.AccessDenied("deleting roles with %v label is prohibited", constants.SystemLabel)
	}
	return i.identity.DeleteRole(roleName)
}

// DeleteAllRoles deletes all roles
func (i *IdentityACL) DeleteAllRoles() error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindRole, teleservices.VerbDelete, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.DeleteAllRoles()
}

// SetAuthPreference updates cluster auth preference
func (i *IdentityACL) SetAuthPreference(authP teleservices.AuthPreference) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterAuthPreference, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}

	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterAuthPreference, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}

	return i.identity.SetAuthPreference(authP)
}

// GetAuthPreference returns cluster auth preference
func (i *IdentityACL) GetAuthPreference() (teleservices.AuthPreference, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterAuthPreference, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}

	return i.identity.GetAuthPreference()
}

// GetClusterName returns cluster name
func (i *IdentityACL) GetClusterName() (teleservices.ClusterName, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterName, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}

	return i.identity.GetClusterName()
}

// SetClusterName updates cluster name
func (i *IdentityACL) SetClusterName(clusterName teleservices.ClusterName) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterName, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}

	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterName, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}

	return i.identity.SetClusterName(clusterName)
}

// GetStaticTokens returns static tokens
func (i *IdentityACL) GetStaticTokens() (teleservices.StaticTokens, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindStaticTokens, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetStaticTokens()
}

// SetStaticTokens updates static tokens
func (i *IdentityACL) SetStaticTokens(tokens teleservices.StaticTokens) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindStaticTokens, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}

	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindStaticTokens, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}

	return i.identity.SetStaticTokens(tokens)
}

// GetClusterConfig returns cluster configuration
func (i *IdentityACL) GetClusterConfig() (teleservices.ClusterConfig, error) {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterConfig, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return i.identity.GetClusterConfig()
}

// SetClusterConfig updates cluster configuration
func (i *IdentityACL) SetClusterConfig(config teleservices.ClusterConfig) error {
	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterConfig, teleservices.VerbCreate, false); err != nil {
		return trace.Wrap(err)
	}

	if err := i.checker.CheckAccessToRule(i.context(), teledefaults.Namespace, teleservices.KindClusterConfig, teleservices.VerbUpdate, false); err != nil {
		return trace.Wrap(err)
	}
	return i.identity.SetClusterConfig(config)
}
