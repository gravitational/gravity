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

package ui

import (
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
)

type authType string

const (
	authLocal authType = "local"
	authSSO   authType = "sso"
)

const (
	inviteStatus   = "invited"
	activeStatus   = "active"
	userTypeToHide = "agent"
)

type access struct {
	Register bool `json:"register"`
	Connect  bool `json:"connect"`
	List     bool `json:"list"`
	Read     bool `json:"read"`
	Edit     bool `json:"edit"`
	Create   bool `json:"create"`
	Delete   bool `json:"remove"`
}

// userACL describes user access to resources
type userACL struct {
	// Sessions defines access to recorded sessions
	Sessions access `json:"sessions"`
	// AuthConnectors defines access to auth.connectors
	AuthConnectors access `json:"authConnectors"`
	// Roles defines access to roles
	Roles access `json:"roles"`
	// TrustedClusters defines access to trusted clusters
	TrustedClusters access `json:"trustedClusters"`
	// Clusters defines access to clusters
	Clusters access `json:"clusters"`
	// Licenses defines access to licenses
	Licenses access `json:"licenses"`
	// Repositories defines access to repositories
	Repositories access `json:"repositories"`
	// Users defines access to users
	Users access `json:"users"`
	// LogForwarders defines access to log forwarders
	LogForwarders access `json:"logForwarders"`
	// Apps defines access to applications
	Apps access `json:"apps"`
	// Events defines access to audit events
	Events access `json:"events"`
	// SSHLogins defines access to servers
	SSHLogins []string `json:"sshLogins"`
}

// WebContext is the context of SPA application
type WebContext struct {
	// User describes user fields
	User User `json:"user"`
	// UserACL describes user access control list
	UserACL userACL `json:"userAcl"`
	// ServerVersion represents gravity server version
	ServerVersion version.Info `json:"serverVersion"`
}

// User describes user role consumed by web ui
type User struct {
	// AuthType is auth type of this user
	AuthType authType `json:"authType"`
	// AccountID is a user name
	AccountID string `json:"accountId"`
	// Name is a user name
	Name string `json:"name"`
	// Email is a user email
	Email string `json:"email"`
	// Roles is a list of user roles
	Roles []string `json:"roles"`
	// Created is user creation time
	Created time.Time `json:"created"`
	// Status is a user status
	Status string `json:"status"`
	// Owner is a flag indicating the account owner
	Owner bool `json:"owner"`
}

// IsHiddenUserType tells if user of a given type should be hidden from UI.
func IsHiddenUserType(userType string) bool {
	return userType == userTypeToHide
}

// newUserACL creates new user access control list
func newUserACL(storageUser storage.User, userRoles teleservices.RoleSet, cluster ops.Site) userACL {
	ctx := &teleservices.Context{
		User:     storageUser,
		Resource: ops.NewClusterFromSite(cluster),
	}
	userAccess := newAccess(userRoles, ctx, teleservices.KindUser)
	sessionAccess := newAccess(userRoles, ctx, teleservices.KindSession)
	roleAccess := newAccess(userRoles, ctx, teleservices.KindRole)
	authConnectors := newAccess(userRoles, ctx, teleservices.KindAuthConnector)
	trustedClusterAccess := newAccess(userRoles, ctx, teleservices.KindTrustedCluster)
	clusterAccess := newAccess(userRoles, ctx, storage.KindCluster)
	licenseAccess := newAccess(userRoles, ctx, storage.KindLicense)
	repositoryAccess := newAccess(userRoles, ctx, storage.KindRepository)
	logForwarderAccess := newAccess(userRoles, ctx, storage.KindLogForwarder)
	appAccess := newAccess(userRoles, ctx, storage.KindApp)
	eventAccess := newAccess(userRoles, ctx, teleservices.KindEvent)
	logins := getLogins(userRoles)

	acl := userACL{
		AuthConnectors:  authConnectors,
		TrustedClusters: trustedClusterAccess,
		Sessions:        sessionAccess,
		Roles:           roleAccess,
		Clusters:        clusterAccess,
		Licenses:        licenseAccess,
		Repositories:    repositoryAccess,
		Users:           userAccess,
		LogForwarders:   logForwarderAccess,
		Apps:            appAccess,
		Events:          eventAccess,
		SSHLogins:       logins,
	}

	return acl
}

// NewWebContext creates a context for web client
func NewWebContext(storageUser storage.User, identity users.Identity, cluster ops.Site) (*WebContext, error) {
	userRoles, err := teleservices.FetchRoles(storageUser.GetRoles(), identity, storageUser.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webCtx := WebContext{
		ServerVersion: version.Get(),
		User:          NewUserByStorageUser(storageUser),
		UserACL:       newUserACL(storageUser, userRoles, cluster),
	}

	return &webCtx, nil
}

// NewUserByInvite creates an instance of UIUser using invite
func NewUserByInvite(invite storage.UserInvite) User {
	return User{
		AuthType: authLocal,
		Email:    invite.Name,
		Status:   inviteStatus,
		Created:  invite.Created,
		Roles:    invite.Roles,
		Owner:    false,
	}
}

// NewUserByStorageUser creates an instance of UI User using Storage User
func NewUserByStorageUser(storageUser storage.User) User {
	// local user
	authType := authLocal

	// check for any SSO identities
	isSSO := len(storageUser.GetOIDCIdentities()) > 0 ||
		len(storageUser.GetGithubIdentities()) > 0 ||
		len(storageUser.GetSAMLIdentities()) > 0

	if isSSO {
		// SSO user
		authType = authSSO
	}

	return User{
		AuthType:  authType,
		AccountID: storageUser.GetAccountID(),
		Name:      storageUser.GetFullName(),
		Email:     storageUser.GetName(),
		Status:    activeStatus,
		Created:   storageUser.GetCreatedBy().Time,
		Roles:     storageUser.GetRoles(),
		Owner:     storageUser.IsAccountOwner(),
	}
}

func newAccess(roleSet teleservices.RoleSet, ctx *teleservices.Context, kind string) access {
	listAccess := hasAccess(roleSet, ctx, kind, teleservices.VerbList)
	if kind == storage.KindCluster {
		// ACL operator will filter out clusters on its own
		listAccess = true
	}
	return access{
		Register: hasAccess(roleSet, ctx, kind, storage.VerbRegister),
		Connect:  hasAccess(roleSet, ctx, kind, storage.VerbConnect),
		List:     listAccess,
		Read:     hasAccess(roleSet, ctx, kind, teleservices.VerbRead),
		Edit:     hasAccess(roleSet, ctx, kind, teleservices.VerbUpdate),
		Create:   hasAccess(roleSet, ctx, kind, teleservices.VerbCreate),
		Delete:   hasAccess(roleSet, ctx, kind, teleservices.VerbDelete),
	}
}

func hasAccess(roleSet teleservices.RoleSet, ctx *teleservices.Context, kind string, verbs ...string) bool {
	for _, verb := range verbs {
		err := roleSet.CheckAccessToRule(ctx, defaults.Namespace, kind, verb, false)
		if err != nil {
			return false
		}
	}

	return true
}

func getLogins(roleSet teleservices.RoleSet) []string {
	allowed := []string{}
	denied := []string{}
	for _, role := range roleSet {
		denied = append(denied, role.GetLogins(teleservices.Deny)...)
		allowed = append(allowed, role.GetLogins(teleservices.Allow)...)
	}

	allowed = utils.Deduplicate(allowed)
	denied = utils.Deduplicate(denied)
	userLogins := []string{}
	for _, login := range allowed {
		match, _ := teleservices.MatchLogin(denied, login)
		if !match {
			userLogins = append(userLogins, login)
		}
	}

	return userLogins
}
