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

package storage

import (
	"encoding/json"
	"fmt"
	"os/user"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
)

const (
	// AgentUser defines a restricted user type used during OpsCenter operations
	AgentUser = "agent"
	// AdminUser defines a user type with maximum permissions
	AdminUser = "admin"
	// Regular user is standard interactive user
	RegularUser = "regular"
)

var SupportedUserTypes = []string{AgentUser, AdminUser, RegularUser}

// Users collection provides operations on users - both humans and bots
type Users interface {
	// CreateUser creates a user entry
	CreateUser(u User) (User, error)
	// UpsertUser creates or updates a user
	UpsertUser(u User) (User, error)
	// UpdateUser udpates existing users parameters
	UpdateUser(email string, req UpdateUserReq) error
	// DeleteUser deletes a user entry
	DeleteUser(email string) error
	// GetUser returns user by name
	GetUser(email string) (User, error)
	// GetUserRoles returns user roles
	GetUserRoles(email string) ([]teleservices.Role, error)
	// GetUsers returns users registered for account
	GetUsers(accountID string) ([]User, error)
	// DeleteAllUsers deletes all users
	DeleteAllUsers() error
	// GetAllUsers returns all users
	GetAllUsers() ([]User, error)
	// GetSiteUsers returns site users
	GetSiteUsers(siteDomain string) ([]User, error)
	// AddUserLoginAttempt logs user login attempt
	AddUserLoginAttempt(user string, attempt teleservices.LoginAttempt, ttl time.Duration) error
	// GetUserLoginAttempts returns user login attempts
	GetUserLoginAttempts(user string) ([]teleservices.LoginAttempt, error)
	// DeleteUserLoginAttempts removes all login attempts of a user. Should be called after successful login.
	DeleteUserLoginAttempts(user string) error
	// UpsertTOTP upserts TOTP secret key for a user that can be used to generate and validate tokens.
	UpsertTOTP(user string, secretKey string) error
	// GetTOTP returns the secret key used by the TOTP algorithm to validate tokens
	GetTOTP(user string) (string, error)
	// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
	// during the 30 second window it's valid.
	UpsertUsedTOTPToken(user string, otpToken string) error
	// GetUsedTOTPToken returns the last successfully used TOTP token. If no token is found zero is returned.
	GetUsedTOTPToken(user string) (string, error)
	// DeleteUsedTOTPToken removes the used token from the backend. This should only
	// be used during tests.
	DeleteUsedTOTPToken(user string) error
}

// User a human or bot user in the system
type User interface {
	// Resource provides common resource methods
	teleservices.Resource
	// GetFullName returns user full name
	GetFullName() string
	// SetFullName sets user full name
	SetFullName(fullname string)
	// GetOIDCIdentities returns a list of connected OIDCIdentities
	GetOIDCIdentities() []teleservices.ExternalIdentity
	// GetSAMLIdentities returns a list of connected SAMLIdentities
	GetSAMLIdentities() []teleservices.ExternalIdentity
	// GetGithubIdentities returns a list of connected Github identities
	GetGithubIdentities() []teleservices.ExternalIdentity
	// GetRoles returns a list of roles assigned to user
	GetRoles() []string
	// String returns string representation of user
	String() string
	// Equals checks if user equals to another
	Equals(other teleservices.User) bool
	// GetStatus return user login status
	GetStatus() teleservices.LoginStatus
	// SetLocked sets login status to locked
	SetLocked(until time.Time, reason string)
	// SetRoles sets user roles
	SetRoles(roles []string)
	// AddRole adds role to the users' role list
	AddRole(name string)
	// GetExpiry returns ttl of the user
	GetExpiry() time.Time
	// GetCreatedBy returns information about user
	GetCreatedBy() teleservices.CreatedBy
	// SetCreatedBy sets created by information
	SetCreatedBy(teleservices.CreatedBy)
	// Check checks basic user parameters for errors
	Check() error
	// CheckAndSetDefaults checks basic user parameters for errors
	// and sets default values
	CheckAndSetDefaults() error
	// GetRawObject returns raw object data, used for migrations
	GetRawObject() interface{}
	// SetRawObject sets raw object
	SetRawObject(a interface{})
	// WebSessionInfo returns web session information about user
	WebSessionInfo(allowedLogins []string) interface{}
	// GetType returns user type
	GetType() string
	// SetType sets user type
	SetType(string)
	// GetOpsCenter returns a hostname of the Ops Center this usre is authenticated with
	GetOpsCenter() string
	// IsAccountOwner returns account ownership flag
	IsAccountOwner() bool
	// SetHOTP sets HOTP token value
	SetHOTP(h []byte)
	// SetPassword sets password hash
	SetPassword(pass string)
	// GetPassword returns password hash
	GetPassword() string
	// GetHOTP sets HOTP token value
	GetHOTP() []byte
	// GetAccountID returns user account ID
	GetAccountID() string
	// GetClusterName returns cluster name of this user
	GetClusterName() string
	// SetClusterName sets cluster name of this user
	SetClusterName(name string)
	// WithoutSecrets returns user copy but with secrets
	// data removed
	WithoutSecrets() User
	// GetTraits gets the trait map for this user used to populate role variables.
	GetTraits() map[string][]string
	// GetTraits sets the trait map for this user used to populate role variables.
	SetTraits(map[string][]string)
}

// NewUser returns new user object based on the spec data,
// this is a helpful shortcut
func NewUser(name string, spec UserSpecV2) User {
	return &UserV2{
		Kind:    teleservices.KindUser,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// UserV2 is version 2 resource spec of the user
type UserV2 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is version
	Version string `json:"version"`
	// Metadata is User metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec contains user specification
	Spec UserSpecV2 `json:"spec"`
	// rawObject contains raw object representation
	rawObject interface{}
}

// SetName sets user name
func (u *UserV2) SetName(name string) {
	u.Metadata.Name = name
}

// SetMetadata returns role metadata
func (u *UserV2) SetMetadata() teleservices.Metadata {
	return u.Metadata
}

// GetMetadata returns role metadata
func (u *UserV2) GetMetadata() teleservices.Metadata {
	return u.Metadata
}

// SetExpiry sets expiry time for the object
func (u *UserV2) SetExpiry(expires time.Time) {
	u.Metadata.SetExpiry(expires)
}

// Expires retuns object expiry setting
func (u *UserV2) Expiry() time.Time {
	return u.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (u *UserV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	u.Metadata.SetTTL(clock, ttl)
}

// WithoutSecrets returns user copy but with secrets data removed
func (u *UserV2) WithoutSecrets() User {
	copy := &UserV2{
		Kind:     u.Kind,
		Metadata: u.Metadata,
		Version:  u.Version,
		Spec:     u.Spec,
	}
	copy.Spec.Password = ""
	copy.Spec.HOTP = nil
	return copy
}

// V1 returns V1 version of user resource
func (u *UserV2) V1() *UserV1 {
	return &UserV1{
		Email:        u.Metadata.Name,
		Name:         u.Metadata.Name,
		Type:         u.Spec.Type,
		AccountOwner: u.Spec.AccountOwner,
		AccountID:    u.Spec.AccountID,
		SiteDomain:   u.Spec.ClusterName,
		Password:     u.Spec.Password,
		HOTP:         u.Spec.HOTP,
		Identities:   u.Spec.OIDCIdentities,
	}
}

// GetTraits gets the trait map for this user used to populate role variables.
func (u *UserV2) GetTraits() map[string][]string {
	return u.Spec.Traits
}

// SetTraits sets the trait map for this user used to populate role variables.
func (u *UserV2) SetTraits(traits map[string][]string) {
	u.Spec.Traits = traits
}

func (u *UserV2) V2() *UserV2 {
	return u
}

// Equals checks if user equals to another
func (u *UserV2) Equals(other teleservices.User) bool {
	if u.GetName() != other.GetName() {
		return false
	}
	otherRoles := other.GetRoles()
	if len(u.GetRoles()) != len(otherRoles) {
		return false
	}
	for i := range u.Spec.Roles {
		if u.Spec.Roles[i] != otherRoles[i] {
			return false
		}
	}
	otherIdentities := other.GetOIDCIdentities()
	if len(u.Spec.OIDCIdentities) != len(otherIdentities) {
		return false
	}
	for i := range u.Spec.OIDCIdentities {
		if !u.Spec.OIDCIdentities[i].Equals(&otherIdentities[i]) {
			return false
		}
	}
	return true
}

// WebSessionInfo returns web session information about user
func (u *UserV2) WebSessionInfo(allowedLogins []string) interface{} {
	clone := u.V1()
	clone.Password = ""
	clone.HOTP = nil
	return clone
}

// GetType returns user type
func (u *UserV2) GetType() string {
	return u.Spec.Type
}

// SetType sets user type
func (u *UserV2) SetType(v string) {
	u.Spec.Type = v
}

// GetClusterName returns cluster name of this user
func (u *UserV2) GetClusterName() string {
	return u.Spec.ClusterName
}

// SetClusterName sets cluster name of this user
func (u *UserV2) SetClusterName(name string) {
	u.Spec.ClusterName = name
}

// UserSpecV2Extension is our extension to Teleport's user
const UserSpecV2Extension = `
  "type": {"type": "string"},
  "account_owner": {"type": "boolean"},
  "account_id": {"type": "string"},
  "cluster_name": {"type": "string"},
  "hotp": {"type": "string"},
  "password": {"type": "string"},
  "ops_center": {"type": "string"},
  "full_name": {"type": "string"}
`

// UserSpecV2 is a specification for V2 user
type UserSpecV2 struct {
	// OIDCIdentities lists associated OpenID Connect identities
	// that let user log in using externally verified identity
	OIDCIdentities []teleservices.ExternalIdentity `json:"oidc_identities,omitempty"`

	// SAMLIdentities lists associated SAML identities
	// that let user log in using externally verified identity
	SAMLIdentities []teleservices.ExternalIdentity `json:"saml_identities,omitempty"`

	// GithubIdentities lists associated Github identities
	// that let user log in using externally verified identity
	GithubIdentities []teleservices.ExternalIdentity `json:"github_identities,omitempty"`

	// Roles is a list of roles assigned to user
	Roles []string `json:"roles,omitempty"`

	// Status is a login status of the user
	Status teleservices.LoginStatus `json:"status"`

	// Expires if set sets TTL on the user
	Expires time.Time `json:"expires"`

	// CreatedBy holds information about agent or person created this user
	CreatedBy teleservices.CreatedBy `json:"created_by"`

	// Type is a user type - e.g. human or install agent
	Type string `json:"type"`

	// AccountOwner indicates that this user is owner of the account and
	// can not be deleted without deleting the whole account
	AccountOwner bool `json:"account_owner"`

	// AccountID is an optional account id this user belongs to
	AccountID string `json:"account_id"`

	// ClusterName is the name of the cluster this user belongs to
	ClusterName string `json:"cluster_name"`

	// Password contains bcrypted password for human users
	Password string `json:"password"`

	// HOTP is HOTP secret used to generate 2nd factor auth challenges
	HOTP []byte `json:"hotp,omitempty"`

	// OpsCenter is a hostname of the ops center this user is authenticated with
	// is initialized by OpsCenter when it creates new sites
	OpsCenter string `json:"ops_center"`

	// FullName is full user name
	FullName string `json:"full_name"`

	// Traits are key/value pairs received from an identity provider (through
	// OIDC claims or SAML assertions) or from a system administrator for local
	// accounts. Traits are used to populate role variables.
	Traits map[string][]string `json:"traits,omitempty"`
}

// IsAccountOwner returns account ownership flag
func (u *UserV2) IsAccountOwner() bool {
	return u.Spec.AccountOwner
}

// GetOpsCenter returns a hostname of the Ops Center this usre is authenticated with
func (u *UserV2) GetOpsCenter() string {
	return u.Spec.OpsCenter
}

// GetObject returns raw object data, used for migrations
func (u *UserV2) GetRawObject() interface{} {
	return u.rawObject
}

// SetRawObject sets raw object
func (u *UserV2) SetRawObject(o interface{}) {
	u.rawObject = o
}

// SetCreatedBy sets created by information
func (u *UserV2) SetCreatedBy(b teleservices.CreatedBy) {
	u.Spec.CreatedBy = b
}

// GetCreatedBy returns information about who created user
func (u *UserV2) GetCreatedBy() teleservices.CreatedBy {
	return u.Spec.CreatedBy
}

// GetExpiry returns expiry time for temporary users
func (u *UserV2) GetExpiry() time.Time {
	return u.Spec.Expires
}

// SetRoles sets a list of roles for user
func (u *UserV2) SetRoles(roles []string) {
	u.Spec.Roles = teleutils.Deduplicate(roles)
}

// GetStatus returns login status of the user
func (u *UserV2) GetStatus() teleservices.LoginStatus {
	return u.Spec.Status
}

// GetOIDCIdentities returns a list of connected OIDCIdentities
func (u *UserV2) GetOIDCIdentities() []teleservices.ExternalIdentity {
	return u.Spec.OIDCIdentities
}

// GetSAMLIdentities returns a list of connected SAML identities
func (u *UserV2) GetSAMLIdentities() []teleservices.ExternalIdentity {
	return u.Spec.SAMLIdentities
}

// GetGithubIdentities returns a list of connected Github identities
func (u *UserV2) GetGithubIdentities() []teleservices.ExternalIdentity {
	return u.Spec.GithubIdentities
}

// GetRoles returns a list of roles assigned to user
func (u *UserV2) GetRoles() []string {
	return u.Spec.Roles
}

// AddRole adds a role to user's role list
func (u *UserV2) AddRole(name string) {
	for _, r := range u.Spec.Roles {
		if r == name {
			return
		}
	}
	u.Spec.Roles = append(u.Spec.Roles, name)
}

// GetName returns user name
func (u *UserV2) GetName() string {
	return u.Metadata.Name
}

// GetFullName returns user email
func (u *UserV2) GetFullName() string {
	return u.Spec.FullName
}

// SetFullName sets user full name
func (u *UserV2) SetFullName(fullName string) {
	u.Spec.FullName = fullName
}

// GetAccountID returns user account ID
func (u *UserV2) GetAccountID() string {
	return u.Spec.AccountID
}

// SetHOTP sets HOTP token value
func (u *UserV2) SetHOTP(h []byte) {
	u.Spec.HOTP = h
}

// GetHOTP sets HOTP token value
func (u *UserV2) GetHOTP() []byte {
	return u.Spec.HOTP
}

// SetPassword sets password hash
func (u *UserV2) SetPassword(pass string) {
	u.Spec.Password = pass
}

// GetPassword returns password hash
func (u *UserV2) GetPassword() string {
	return u.Spec.Password
}

func (u *UserV2) String() string {
	return fmt.Sprintf("User(Name=%v, Cluster=%v, Roles=%v, Identities=%v)",
		u.Metadata.Name, u.Spec.ClusterName, u.Spec.Roles, u.Spec.OIDCIdentities)
}

func (u *UserV2) SetLocked(until time.Time, reason string) {
	u.Spec.Status.IsLocked = true
	u.Spec.Status.LockExpires = until
	u.Spec.Status.LockedMessage = reason
}

// Check checks validity of all parameters
func (u *UserV2) Check() error {
	if u.Kind == "" {
		return trace.BadParameter("parameter 'kind' is not set")
	}
	if u.Version == "" {
		return trace.BadParameter("parameter 'version' is not set")
	}
	if err := utils.CheckUserName(u.Metadata.Name); err != nil {
		return trace.Wrap(err)
	}
	if !utils.StringInSlice(SupportedUserTypes, u.GetType()) {
		return trace.BadParameter("unsupported user type %q, supported are: %v",
			u.GetType(), SupportedUserTypes)
	}
	if u.GetType() != AgentUser && u.GetPassword() == "" {
		return trace.BadParameter("parameter 'password' is not set")
	}
	for _, id := range u.Spec.OIDCIdentities {
		if err := id.Check(); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// CheckAndSetDefaults checks that the user is valid and sets some defaults
func (u *UserV2) CheckAndSetDefaults() error {
	if u.Spec.AccountID == "" {
		u.Spec.AccountID = defaults.SystemAccountID
	}
	if err := u.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UserV1 is a struct representing a user in the system, user
// or bot performing operations,
type UserV1 struct {
	// Email is email address used for login, it is globally unique
	Email string `json:"email"`
	// Name aliases the email and is provided for backwards-compatibility
	Name string `json:"name"`
	// Type is a user type - e.g. human or install agent
	Type string `json:"type"`
	// AccountOwner indicates that this user is owner of the account and
	// can not be deleted without deleting the whole account
	AccountOwner bool `json:"account_owner"`
	// AccountID is an optional account id this user belongs to
	AccountID string `json:"account_id"`
	// SiteDomain is an optional site id this user belongs to
	SiteDomain string `json:"site_domain"`
	// Password contains bcrypted password for human users
	Password string `json:"password"`
	// HOTP is HOTP secret used to generate 2nd factor auth challenges
	HOTP []byte `json:"hotp"`
	// AllowedLogins is a list of allowed logins
	AllowedLogins []string `json:"allowed_logins"`
	// Identities is a list of connected OIDCIdentities
	Identities []teleservices.ExternalIdentity `json:"identities"`
}

func (u *UserV1) String() string {
	return fmt.Sprintf("user(email=%v)", u.Email)
}

func (u *UserV1) Check() error {
	if u.AccountID != "" {
		return trace.BadParameter("missing parameter AccountID")
	}
	return nil
}

//V1 returns itself
func (u *UserV1) V1() *UserV1 {
	return u
}

//V2 converts UserV1 to UserV2 format
func (u *UserV1) V2() *UserV2 {
	return &UserV2{
		Kind:    teleservices.KindUser,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      u.Email,
			Namespace: teledefaults.Namespace,
		},
		Spec: UserSpecV2{
			AccountID:      u.AccountID,
			OIDCIdentities: u.Identities,
			ClusterName:    u.SiteDomain,
			Type:           u.Type,
			Password:       u.Password,
			HOTP:           u.HOTP,
			AccountOwner:   u.AccountOwner,
		},
		rawObject: *u,
	}
}

func init() {
	teleservices.SetUserMarshaler(&userMarshaler{})
}

// GetAllowedLogins returns a list of unix logins that are set by default
// for admin users, this feature is going to be deprecated once
// we will be able to set roles via UI
func GetAllowedLogins(currentUser *user.User) []string {
	allowedLogins := []string{"root"}

	// this is for devmode to allow teleport to log into your current userbase
	if currentUser != nil {
		allowedLogins = append(allowedLogins, currentUser.Username)
	}
	return allowedLogins
}

func collectOptions(opts []teleservices.MarshalOption) (*teleservices.MarshalConfig, error) {
	var cfg teleservices.MarshalConfig
	for _, o := range opts {
		if err := o(&cfg); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return &cfg, nil
}

// UnmarshalUser unmarshals user from default representation
func UnmarshalUser(bytes []byte) (User, error) {
	var h teleservices.ResourceHeader
	err := json.Unmarshal(bytes, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var u UserV1
		err := json.Unmarshal(bytes, &u)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return u.V2(), nil
	case teleservices.V2:
		var u UserV2
		if err := teleutils.UnmarshalWithSchema(teleservices.GetUserSchema(UserSpecV2Extension), &u, bytes); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		//nolint:errcheck
		u.Metadata.CheckAndSetDefaults()
		//nolint:errcheck
		u.CheckAndSetDefaults()
		utils.UTC(&u.Spec.CreatedBy.Time)
		utils.UTC(&u.Spec.Expires)
		utils.UTC(&u.Spec.Status.LockExpires)
		utils.UTC(&u.Spec.Status.LockedTime)
		utils.UTC(&u.Spec.Expires)
		u.rawObject = u
		return &u, nil
	}

	return nil, trace.BadParameter("user resource version %v is not supported", h.Version)
}

// MarshalUser marshals user to some representation
func MarshalUser(u teleservices.User, opts ...teleservices.MarshalOption) ([]byte, error) {
	cfg, err := collectOptions(opts)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	type userv1 interface {
		V1() *UserV1
	}

	type userv2 interface {
		V2() *UserV2
	}
	version := cfg.GetVersion()
	switch version {
	case teleservices.V1:
		v, ok := u.(userv1)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v, %T is missing method V1()", teleservices.V1, u)
		}
		userV1 := v.V1()
		userV1.AllowedLogins = GetAllowedLogins((*user.User)(nil))
		return json.Marshal(userV1)
	case teleservices.V2:
		v, ok := u.(userv2)
		if !ok {
			return nil, trace.BadParameter("don't know how to marshal %v, %T is missing method V2()", teleservices.V2, u)
		}
		return json.Marshal(v.V2())
	default:
		return nil, trace.BadParameter("version %v is not supported", version)
	}
}

type userMarshaler struct{}

// UnmarshalUser unmarshals user from JSON
func (*userMarshaler) UnmarshalUser(bytes []byte) (teleservices.User, error) {
	user, err := UnmarshalUser(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// GenerateUser generates new user
func (*userMarshaler) GenerateUser(in teleservices.User) (teleservices.User, error) {
	expires := in.Expiry()
	return &UserV2{
		Kind:    teleservices.KindUser,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      in.GetName(),
			Namespace: teledefaults.Namespace,
			Expires:   &expires,
		},
		Spec: UserSpecV2{
			// always generate password, even though it won't be used
			Traits:           in.GetTraits(),
			CreatedBy:        in.GetCreatedBy(),
			Password:         uuid.New(),
			AccountID:        defaults.SystemAccountID,
			Type:             RegularUser,
			OIDCIdentities:   in.GetOIDCIdentities(),
			SAMLIdentities:   in.GetSAMLIdentities(),
			GithubIdentities: in.GetGithubIdentities(),
			Roles:            in.GetRoles(),
			Expires:          expires,
		},
	}, nil
}

// MarshalUser marshalls user into JSON
func (*userMarshaler) MarshalUser(u teleservices.User, opts ...teleservices.MarshalOption) ([]byte, error) {
	return MarshalUser(u, opts...)
}

// UpdateUserReq instructs update method to update certain fields
// of the user struct, if they are set as not nil
type UpdateUserReq struct {
	// HOTP is a request to update user HOTP token
	HOTP *[]byte
	// Password is a request to update user password
	Password *string
	// Roles sets user roles
	Roles *[]string
	// User full name
	FullName *string
}

// Check will check if all parameters are correct and will return error
func (u *UpdateUserReq) Check() error {
	if u.HOTP == nil && u.Password == nil {
		return trace.BadParameter("need at least one parameter to update")
	}
	return nil
}

// ClusterAgent generates the name of the agent user for the specified cluster
func ClusterAgent(cluster string) string {
	return fmt.Sprintf("agent@%v", cluster)
}

// ClusterAdminAgent generates the name of the admin agent user for the specified cluster
func ClusterAdminAgent(clusterName string) string {
	return fmt.Sprintf("adminagent@%v", clusterName)
}
