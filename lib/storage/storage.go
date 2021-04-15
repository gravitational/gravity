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

// Package storage implements storage backends
// for objects in portal - Accounts, Sites and others
// these implementations are supposed to be dumb - no business logic
// just storage logic should be handled to keep the backend implementations
// small.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/tstranex/u2f"
	"helm.sh/helm/v3/pkg/repo"
	v1 "k8s.io/api/core/v1"
	kubeletapis "k8s.io/kubernetes/pkg/kubelet/apis"
)

// Accounts collection modifies and updates account entries,
// where each account is related to some organization
type Accounts interface {
	// CreateAccount creates account entry
	CreateAccount(a Account) (*Account, error)
	// DeleteAccount deletes account entry and all associated data, e.g.
	// sites and all site-specific stuff
	DeleteAccount(id string) error
	// GetAccounts returns list of accounts
	GetAccounts() ([]Account, error)
	// GetAccount returns account entry by it's id
	GetAccount(id string) (*Account, error)
}

// Account represents some organization or company
// that can have multiple sites
type Account struct {
	// ID is a unique organization identifier
	ID string `json:"id"`
	// Org is organisation name
	Org string `json:"org"`
}

// String returns a string representation of an account
func (a Account) String() string {
	return fmt.Sprintf("Account(ID=%s, Org=%s)", a.ID, a.Org)
}

// Sites collection works with sites - a group of servers
type Sites interface {
	// CompareAndSwapSiteState swaps site state to new version only if
	// it's set to the required state
	CompareAndSwapSiteState(domain string, old, new string) error
	// CreateSite creates site entry
	CreateSite(s Site) (*Site, error)
	// UpdateSite updates site properties
	UpdateSite(s Site) (*Site, error)
	// DeleteSite deletes site entry
	DeleteSite(domain string) error
	// GetSites returns a list of sites for account id
	GetSites(accountID string) ([]Site, error)
	// GetAllSites returns a list of all sites for all accounts
	GetAllSites() ([]Site, error)
	// GetSite returns site by account id and site domain
	GetSite(domain string) (*Site, error)
	// GetLocalSite returns local site for a given account ID
	GetLocalSite(accountID string) (*Site, error)
}

// APIKey is a token that agent users use to access the API
type APIKey struct {
	// Token is the api key itself
	Token string `json:"token"`
	// Expires is the key expiration time
	Expires time.Time `json:"expires"`
	// UserEmail is the name of the user the api key belongs to
	UserEmail string `json:"user_email"`
}

// V2 returns V2 from token spec
func (a *APIKey) V2() *TokenV2 {
	expires := a.Expires
	return &TokenV2{
		Kind:    KindToken,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      a.Token,
			Expires:   &expires,
			Namespace: defaults.Namespace,
		},
		Spec: TokenSpecV2{
			User: a.UserEmail,
		},
	}
}

// Check checks api key for parameters
func (a *APIKey) Check() error {
	if a.Token == "" {
		return trace.BadParameter("missing API Key token")
	}
	return nil
}

// APIKeys provides operations with api keys
type APIKeys interface {
	// CreateAPIKey creates a new api key
	CreateAPIKey(APIKey) (*APIKey, error)
	// UpsertAPIKey creates or updates an api key
	UpsertAPIKey(APIKey) (*APIKey, error)
	// GetAPIKeys returns api keys for a user
	GetAPIKeys(username string) ([]APIKey, error)
	// GetAPIKey returns an api key entry by token
	GetAPIKey(token string) (*APIKey, error)
	// DeleteAPIKey deletes an api key
	DeleteAPIKey(username, token string) error
}

// Connectors manages OIDC connectors (OpenID connect configurations)
type Connectors interface {
	// UpsertOIDCConnector upserts OIDC Connector
	UpsertOIDCConnector(teleservices.OIDCConnector) error
	// DeleteOIDCConnector deletes OIDC Connector
	DeleteOIDCConnector(connectorID string) error
	// GetOIDCConnector returns OIDC connector data, withSecrets adds or removes client secret from return results
	GetOIDCConnector(id string, withSecrets bool) (teleservices.OIDCConnector, error)
	// GetOIDCConnectors returns registered connectors, withSecrets adds or removes client secret from return results
	GetOIDCConnectors(withSecrets bool) ([]teleservices.OIDCConnector, error)
	// CreateOIDCAuthRequest creates new auth request
	CreateOIDCAuthRequest(req teleservices.OIDCAuthRequest) error
	// GetOIDCAuthRequest returns OIDC auth request if found
	GetOIDCAuthRequest(stateToken string) (*teleservices.OIDCAuthRequest, error)
	// GetUserByOIDCIdentity returns a user by its specified OIDC Identity, returns first
	// user specified with this identity
	GetUserByOIDCIdentity(id teleservices.ExternalIdentity) (teleservices.User, error)
	// GetUserBySAMLIdentity returns a user by its specified SAML Identity, returns first
	// user specified with this identity
	GetUserBySAMLIdentity(id teleservices.ExternalIdentity) (teleservices.User, error)
	// GetUserByGithubIdentity returns a user by its specified Github Identity, returns first
	// user specified with this identity
	GetUserByGithubIdentity(id teleservices.ExternalIdentity) (teleservices.User, error)
	// CreateSAMLConnector creates SAML Connector
	CreateSAMLConnector(connector teleservices.SAMLConnector) error
	// UpsertSAMLConnector upserts SAML Connector
	UpsertSAMLConnector(connector teleservices.SAMLConnector) error
	// DeleteSAMLConnector deletes SAML Connector
	DeleteSAMLConnector(connectorID string) error
	// GetSAMLConnector returns SAML connector data, withSecrets adds or removes secrets from return results
	GetSAMLConnector(id string, withSecrets bool) (teleservices.SAMLConnector, error)
	// GetSAMLConnectors returns registered connectors, withSecrets adds or removes secret from return results
	GetSAMLConnectors(withSecrets bool) ([]teleservices.SAMLConnector, error)
	// CreateSAMLAuthRequest creates new auth request
	CreateSAMLAuthRequest(req teleservices.SAMLAuthRequest, ttl time.Duration) error
	// GetSAMLAuthRequest returns SAML auth request if found
	GetSAMLAuthRequest(id string) (*teleservices.SAMLAuthRequest, error)
	// CreateGithubConnector creates a new Github connector
	CreateGithubConnector(connector teleservices.GithubConnector) error
	// UpsertGithubConnector creates or updates a new Github connector
	UpsertGithubConnector(connector teleservices.GithubConnector) error
	// GetGithubConnectors returns all configured Github connectors
	GetGithubConnectors(withSecrets bool) ([]teleservices.GithubConnector, error)
	// GetGithubConnector returns a Github connector by its name
	GetGithubConnector(name string, withSecrets bool) (teleservices.GithubConnector, error)
	// DeleteGithubConnector deletes a Github connector by its name
	DeleteGithubConnector(name string) error
	// CreateGithubAuthRequest creates a new auth request for Github OAuth2 flow
	CreateGithubAuthRequest(req teleservices.GithubAuthRequest) error
	// GetGithubAuthRequest retrieves Github auth request by the token
	GetGithubAuthRequest(stateToken string) (*teleservices.GithubAuthRequest, error)
}

// NewOIDCConnector returns a new OIDC connector with specified name and spec
func NewOIDCConnector(name string, spec teleservices.OIDCConnectorSpecV2) *teleservices.OIDCConnectorV2 {
	return &teleservices.OIDCConnectorV2{
		Kind:    teleservices.KindOIDCConnector,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// NewGithubConnector returns a new Github connector with specified name and spec
func NewGithubConnector(name string, spec teleservices.GithubConnectorSpecV3) *teleservices.GithubConnectorV3 {
	return &teleservices.GithubConnectorV3{
		Kind:    teleservices.KindGithubConnector,
		Version: teleservices.V3,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// NewSAMLConnector returns a new SAML connector with specified name and spec
func NewSAMLConnector(name string, spec teleservices.SAMLConnectorSpecV2) *teleservices.SAMLConnectorV2 {
	return &teleservices.SAMLConnectorV2{
		Kind:    teleservices.KindSAMLConnector,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// WebSessions take care of the user web sessions and help implement
// teleport's Identity backend
type WebSessions interface {
	UpsertWebSession(username, sid string, session teleservices.WebSession) error
	// GetWebSession returns a web session state for a given user and session id
	GetWebSession(username, sid string) (teleservices.WebSession, error)
	// DeleteWebSession deletes web ession for user and session ide
	DeleteWebSession(username, sid string) error
}

// UserToken is a temporary token used to create and reset a user
type UserToken struct {
	// Token is a unique randomly generated token
	Token string `json:"token"`
	// User is user name associated with this token
	User string `json:"user"`
	// Expires sets the token expiry time
	Expires time.Time `json:"expires"`
	// Type is token type
	Type string `json:"type"`
	// HOTP is a secret value of one time password secret generator
	HOTP []byte `json:"hotp"`
	// QRCode is a QR code value
	QRCode []byte `json:"qr_code"`
	// Created holds information about when the token was created
	Created time.Time `json:"created"`
	// URL is this token URL
	URL string `json:"url"`
}

// CheckUserToken returns nil if the value is correct, error otherwise
func CheckUserToken(s string) error {
	switch s {
	case UserTokenTypeInvite, UserTokenTypeReset:
		return nil
	}
	return trace.BadParameter("unsupported token type: %v", s)
}

const (
	// UserTokenTypeInvite adds new user to existing account
	UserTokenTypeInvite = "invite"
	// UserTokenTypeReset resets user credentials
	UserTokenTypeReset = "reset"
)

// UserInvite represents a promise to add user to account
type UserInvite struct {
	// Name is the user of this user
	Name string `json:"name"`
	// CreatedBy is a user who sends the invite
	CreatedBy string `json:"created_by"`
	// Created is a time this user invite has been created
	Created time.Time `json:"created"`
	// Roles are the roles that will be assigned to invited user
	Roles []string `json:"roles"`
	// ExpiresIn sets the token expiry time
	ExpiresIn time.Duration `json:"expires_in"`
}

// CheckAndSetDefaults checks and sets defaults for user invite
func (u *UserInvite) CheckAndSetDefaults() error {
	if err := utils.CheckUserName(u.Name); err != nil {
		return trace.Wrap(err)
	}
	if u.CreatedBy == "" {
		return trace.BadParameter("missing CreatedBy")
	}
	if u.Created.IsZero() {
		u.Created = time.Now().UTC()
	}
	if len(u.Roles) == 0 {
		return trace.BadParameter("roles can't be empty")
	}
	return nil
}

// UserInvites manages user invites
type UserInvites interface {
	// UpsertUserInvite upserts a new user invite
	UpsertUserInvite(u UserInvite) (*UserInvite, error)
	// GetUserInvites returns a list of user invites
	GetUserInvites() ([]UserInvite, error)
	// DeleteUserInvite deletes user invite
	DeleteUserInvite(token string) error
	// GetUserInvite returns user invite by user name
	GetUserInvite(username string) (*UserInvite, error)
}

// UserTokens collection operates on one-time tokens used for
// creating new accounts and adding users to existing accounts,
// as well as recovering passwords
type UserTokens interface {
	// CreateUserToken creates a temporary authentication token
	CreateUserToken(t UserToken) (*UserToken, error)
	// DeleteUserToken deletes token by its id
	DeleteUserToken(token string) error
	// GetUserToken returns a token if it has not expired yet
	GetUserToken(token string) (*UserToken, error)
	// DeleteUserTokens deletes user tokens
	DeleteUserTokens(tokenType string, user string) error
}

// U2F collection operates on U2F signups, logins, and password resets
type U2F interface {
	// UpsertU2FRegisterChallenge upserts a U2F challenge for a new user corresponding to the token
	UpsertU2FRegisterChallenge(token string, u2fChallenge *u2f.Challenge) error
	// GetU2FRegisterChallenge returns a U2F challenge for a new user corresponding to the token
	GetU2FRegisterChallenge(token string) (*u2f.Challenge, error)
	// UpsertU2FRegistration upserts a U2F registration from a valid register response
	UpsertU2FRegistration(user string, u2fReg *u2f.Registration) error
	// GetU2FRegistration returns a U2F registration from a valid register response
	GetU2FRegistration(user string) (*u2f.Registration, error)
	// UpsertU2FRegistrationCounter upserts a counter associated with a U2F registration
	UpsertU2FRegistrationCounter(user string, counter uint32) error
	// UpsertU2FRegistrationCounter upserts a counter associated with a U2F registration
	GetU2FRegistrationCounter(user string) (counter uint32, e error)
	// GetU2FSignChallenge returns a U2F sign (auth) challenge
	UpsertU2FSignChallenge(user string, u2fChallenge *u2f.Challenge) error
	// GetU2FSignChallenge returns a U2F sign (auth) challenge
	GetU2FSignChallenge(user string) (*u2f.Challenge, error)
}

// Mount describes a mount on a server
type Mount struct {
	// Name identifies the mount
	Name string `json:"name"`
	// Source is the directory to mount
	Source string `json:"source"`
	// Destination is the mount destination directory
	Destination string `json:"destination"`
	// CreateIfMissing is whether to create the source directory if it doesn't exist
	CreateIfMissing bool `json:"create_if_missing"`
	// SkipIfMissing is whether to avoid mounting a directory if the source does not exist
	// on host
	SkipIfMissing bool `json:"skip_if_missing"`
	// UID sets UID for a volume path on the host
	UID *int `json:"uid,omitempty"`
	// GID sets GID for a volume path on the host
	GID *int `json:"gid,omitempty"`
	// Mode sets file mode for a volume path on the host
	// accepts octal format
	Mode string `json:"mode,omitempty"`
	// Recursive means that all mount points inside this mount should also be mounted
	Recursive bool `json:"recursive,omitempty"`
}

// Docker defines the configuration specific to docker
type Docker struct {
	// Device defines the block device (disk or partition) to use
	// for a devicemapper configuration
	Device Device `json:"device"`
	// LVMSystemDirectory specifies the location of lvm system directory
	// if the storage driver is `devicemapper`
	LVMSystemDirectory string `json:"system_directory"`
}

// SiteOperation represents any modification of the site,
// e.g. adding or deleting a server or a group of servers
type SiteOperation struct {
	// ID is a unique operation ID
	ID string `json:"id"`
	// AccountID - id of the account this site belongs to
	AccountID string `json:"account_id"`
	// SiteDomain - ID of the site this operation refers to
	SiteDomain string `json:"site_domain"`
	// Type is operation type - e.g. add or delete servers
	Type string `json:"type"`
	// Created is a time when this operation was created
	Created time.Time `json:"created"`
	// CreatedBy specifies the user who created the operation
	CreatedBy string `json:"created_by,omitempty"`
	// Updated is a time when this operation was last updated
	Updated time.Time `json:"updated"`
	// State represents current operation state
	State string `json:"state"`
	// Provisioner defines the provisioner used for this operation
	Provisioner string `json:"provisioner"`
	// Servers stores servers affected by the operation, e.g.
	// in case of 'install' or 'provision_servers' it will store the
	// servers that will be added and configured, for 'deprovision_servers'
	// it will store the servers that will be deleted
	Servers Servers `json:"servers"`
	// Shrink is set when the operation type is shrink (removing nodes from the cluster)
	Shrink *ShrinkOperationState `json:"shrink,omitempty"`
	// InstallExpand is set when the operation is install or expand
	InstallExpand *InstallExpandOperationState `json:"install_expand,omitempty"`
	// Uninstall is for uninstalling gravity and it's data
	Uninstall *UninstallOperationState `json:"uninstall,omitempty"`
	// Update is for updating application on the gravity site
	Update *UpdateOperationState `json:"update,omitempty"`
	// UpdateEnviron defines the runtime environment update state
	UpdateEnviron *UpdateEnvarsOperationState `json:"update_environ,omitempty"`
	// UpdateConfig defines the state of the cluster configuration update operation
	UpdateConfig *UpdateConfigOperationState `json:"update_config,omitempty"`
	// Reconfigure contains reconfiguration operation state
	Reconfigure *ReconfigureOperationState `json:"reconfigure,omitempty"`
}

func (s *SiteOperation) Check() error {
	if s.Type == "" {
		return trace.BadParameter("missing operation type")
	}
	if s.SiteDomain == "" {
		return trace.BadParameter("missing operation site domain")
	}
	return nil
}

// Vars returns operation specific variables
func (s *SiteOperation) Vars() OperationVariables {
	if s.InstallExpand != nil {
		return s.InstallExpand.Vars
	}
	if s.Shrink != nil {
		return s.Shrink.Vars
	}
	if s.Uninstall != nil {
		return s.Uninstall.Vars
	}
	if s.Update != nil {
		return s.Update.Vars
	}
	return OperationVariables{}
}

// IsEqualTo returns true if the operation is equal to the provided operation.
func (s *SiteOperation) IsEqualTo(other SiteOperation) bool {
	// Compare a few essential fields only.
	return s.ID == other.ID && s.AccountID == other.AccountID &&
		s.SiteDomain == other.SiteDomain && s.Type == other.Type &&
		s.State == other.State
}

// SiteOperations colection represents a list of operations performed
// on the site, e.g. provisioning servers, or upgrading applications
type SiteOperations interface {
	// CreateSiteOperation creates a new site operation
	CreateSiteOperation(SiteOperation) (*SiteOperation, error)
	// GetSiteOperation returns the operation identified by the operation id
	// and site id
	GetSiteOperation(siteDomain, operationID string) (*SiteOperation, error)
	// GetSiteOperations returns a list of operations performed on this
	// site sorted by time (latest operations come first)
	GetSiteOperations(siteDomain string) ([]SiteOperation, error)
	// UpdateSiteOperation updates site operation state
	UpdateSiteOperation(SiteOperation) (*SiteOperation, error)
	// DeleteSiteOperation removes an unstarted site operation
	DeleteSiteOperation(siteDomain, operationID string) error
	// CreateOperationPlan saves a new operation plan
	CreateOperationPlan(OperationPlan) (*OperationPlan, error)
	// GetOperationPlan returns plan for the specified operation
	GetOperationPlan(clusterName, operationID string) (*OperationPlan, error)
	// CreateOperationPlanChange creates a new state transition entry for a plan
	CreateOperationPlanChange(PlanChange) (*PlanChange, error)
	// GetOperationPlanChangelog returns all state transition entries for a plan
	GetOperationPlanChangelog(clusterName, operationID string) (PlanChangelog, error)
}

// Reason details the reason a site is in a particular state
type Reason string

const (
	// ReasonLicenseInvalid means that the license installed on the site is not valid
	ReasonLicenseInvalid Reason = "license_invalid"
	// ReasonStatusCheckFailed means that the site's status check failed
	ReasonStatusCheckFailed Reason = "status_check_failed"
	// ReasonClusterDegraded means one or more of cluster nodes are degraded
	ReasonClusterDegraded Reason = "cluster_degraded"
)

// Description returns human-readable description of the reason
func (r *Reason) Description() string {
	switch *r {
	case ReasonLicenseInvalid:
		return "the license is not valid"
	case ReasonStatusCheckFailed:
		return "application status check failed"
	case ReasonClusterDegraded:
		return "one or more of cluster nodes are not healthy"
	case "":
		return ""
	default:
		return "unknown reason"
	}
}

func (r *Reason) Check() error {
	switch *r {
	case "", ReasonLicenseInvalid, ReasonStatusCheckFailed, ReasonClusterDegraded:
		return nil
	}
	return trace.BadParameter("unsupported reason: %s", *r)
}

// Site is a group of servers that belongs to some account and
// having some application installed
type Site struct {
	// Domain is a site specific unique domain name (e.g. site.example.com)
	Domain string `json:"domain"`
	// Created records the time when site was created
	Created time.Time `json:"created"`
	// CreatedBy is the email of a user who created the site
	CreatedBy string `json:"created_by"`
	// AccountID is the id of the account this site belongs to
	AccountID string `json:"account_id"`
	// State represents the state of this site, e.g. 'created', 'configured'
	State string `json:"state"`
	// Reason is the code describing the state the site is currently in
	Reason Reason `json:"reason"`
	// Provider is a provider selected for this site
	Provider string `json:"provider"`
	// License is the license currently installed on this site
	License string `json:"license"`
	// TODO: this should probably move to SiteOperation as well
	// ProvisionerState is a provisioner-specific state
	// that used to track some resources allocated for the cloud
	// e.g. disks, VMs
	ProvisionerState []byte `json:"provisioner_state"`
	// App is application installed on this site, e.g.
	// "gravitational.io/mattermost:1.2.1"
	App Package `json:"app"`
	// Local specifies whether this site is local to the running
	// process (opscenter or site)
	Local bool `json:"local"`
	// Labels is a custom key/value metadata attached to the site (think AWS tags)
	Labels map[string]string `json:"labels"`
	// FinalInstallStepComplete indicates whether the site has completed the final installation step
	FinalInstallStepComplete bool `json:"final_install_step_complete"`
	// Resources is optional byte-string with K8s resources injected at site creation
	Resources []byte `json:"resources"`
	// Location is a location where the site is deployed, for example AWS region name
	Location string `json:"location"`
	// Flavor is the initial cluster flavor.
	Flavor string `json:"flavor"`
	// DisabledWebUI specifies whether OpsCenter and WebInstallWizard are disabled
	DisabledWebUI bool `json:"disabled_web_ui"`
	// UpdateInterval is how often the site checks for and downloads newer versions of the
	// installed application
	UpdateInterval time.Duration `json:"update_interval"`
	// NextUpdateCheck is the timestamp of the upcoming updates check for the site
	NextUpdateCheck time.Time `json:"next_update_check"`
	// ClusterState holds the current cluster state, e.g. nodes in the cluster and information
	// about them
	ClusterState ClusterState `json:"cluster_state"`
	// ServiceUser specifies the service user for planet
	ServiceUser OSUser `json:"service_user"`
	// CloudConfig provides additional cloud configuration
	CloudConfig CloudConfig `json:"cloud_config"`
	// DNSOverrides contains DNS overrides for this cluster
	// TODO(dmitri): move to DNSConfig
	DNSOverrides DNSOverrides `json:"dns_overrides"`
	// DNSConfig defines cluster local DNS configuration
	DNSConfig DNSConfig `json:"dns_config"`
	// InstallToken specifies the original token the cluster was installed with
	InstallToken string `json:"install_token"`
}

// Check validates the cluster object's fields.
func (s *Site) Check() error {
	if s.AccountID == "" {
		return trace.BadParameter("missing parameter AccountID")
	}
	if s.Domain == "" {
		return trace.BadParameter("missing parameter DomainName")
	}
	if s.Created.IsZero() {
		return trace.BadParameter("missing parameter Created")
	}
	if err := s.Reason.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Servers returns the cluster's servers.
func (s *Site) Servers() Servers {
	return s.ClusterState.Servers
}

// ClusterState defines the state of the cluster
type ClusterState struct {
	// Servers is a list of servers in the cluster
	Servers Servers `json:"servers"`
	// Docker specifies current cluster Docker configuration
	Docker DockerConfig `json:"docker"`
}

type nodeKey struct {
	profile      string
	instanceType string
}

// ClusterNodeSpec converts Servers list to node spec
func (s *ClusterState) ClusterNodeSpec() []ClusterNodeSpecV2 {
	mapping := map[nodeKey]ClusterNodeSpecV2{}
	for _, server := range s.Servers {
		key := nodeKey{profile: server.Role, instanceType: server.InstanceType}
		spec, ok := mapping[key]
		if !ok {
			mapping[key] = ClusterNodeSpecV2{
				Profile:      server.Role,
				Count:        1,
				InstanceType: server.InstanceType,
			}
		} else {
			spec.Count += 1
			mapping[key] = spec
		}
	}
	var out []ClusterNodeSpecV2
	for _, spec := range mapping {
		out = append(out, spec)
	}
	sort.Sort(sortedNodeSpec(out))
	return out
}

// sortedNodeSpec sorts node spec by name
type sortedNodeSpec []ClusterNodeSpecV2

// Len returns length of a role list
func (s sortedNodeSpec) Len() int {
	return len(s)
}

// Less stacks latest attempts to the end of the list
func (s sortedNodeSpec) Less(i, j int) bool {
	if s[i].Profile == s[j].Profile {
		return s[i].Count < s[j].Count
	}
	return s[i].Profile <= s[j].Profile
}

// Swap swaps two attempts
func (s sortedNodeSpec) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// ProfileMap returns servers mapped by server profile
func (s *ClusterState) ProfileMap() map[string][]Server {
	out := make(map[string][]Server)
	for i := range s.Servers {
		server := s.Servers[i]
		servers, ok := out[server.Role]
		if !ok {
			out[server.Role] = []Server{server}
		} else {
			servers = append(servers, server)
			out[server.Role] = servers
		}
	}
	return out
}

// FindServer returns a server by hostname
func (s *ClusterState) FindServer(hostname string) (*Server, error) {
	for _, server := range s.Servers {
		if server.Hostname == hostname {
			return &server, nil
		}
	}
	return nil, trace.NotFound("couldn't find server %q in cluster state: %v", hostname, s)
}

// FindServerByIP returns a server by advertise IP
func (s *ClusterState) FindServerByIP(ip string) (*Server, error) {
	for _, server := range s.Servers {
		if server.AdvertiseIP == ip {
			return &server, nil
		}
	}
	return nil, trace.NotFound("couldn't find server %q in cluster state: %v", ip, s)
}

// HasServer returns true if cluster state contains server with specified hostname
func (s ClusterState) HasServer(hostname string) bool {
	_, err := s.FindServer(hostname)
	return err == nil
}

// Applications defines operations on the site applications
type Applications interface {
	// GetApplication queries an existing application
	GetApplication(repository, packageName, packageVersion string) (*Package, error)
	// GetApplications lists all applications for the specified repository
	GetApplications(repository string, appType AppType) ([]Package, error)
}

// AppOperations defines the interface to handle operations on applications
type AppOperations interface {
	// CreateAppOperation creates a new application operation
	CreateAppOperation(op AppOperation) (*AppOperation, error)
	// GetAppOperation queries an operation in progress
	GetAppOperation(id string) (*AppOperation, error)
	// UpdateAppImportOperation updates an operation in progress
	UpdateAppOperation(op AppOperation) (*AppOperation, error)
}

// AppOperation represents operations on applications
// e.g. updating or removing
type AppOperation struct {
	// Repository defines the repository of the application package
	Repository string `json:"repository"`
	// PackageName defines the name of the application package
	PackageName string `json:"package_name"`
	// PackageVersion defines the version of the application package
	PackageVersion string `json:"package_version"`
	// ID identifies the operation
	ID string `json:"operation_id"`
	// Type defines application operation type
	Type string `json:"type"`
	// Created specifies the time when the operation was created
	Created time.Time `json:"created"`
	// Updated specifies the time when the operation was last updated
	Updated time.Time `json:"updated"`
	// State represents current operation state
	State string `json:"state"`
}

func (a *AppOperation) Check() error {
	if a.Repository == "" {
		return trace.BadParameter("missing parameter Repository")
	}
	if a.PackageName == "" {
		return trace.BadParameter("missing parameter PackageName")
	}
	if a.PackageVersion == "" {
		return trace.BadParameter("missing parameter PackageVersion")
	}
	if a.Type == "" {
		return trace.BadParameter("missing parameter Type")
	}
	return nil
}

// AppProgressEntry is a structured entry indicating operation progress
type AppProgressEntry struct {
	// ID is auto generated ID
	ID string `json:"id"`
	// Repository defines the repository of the application package
	Repository string `json:"repository"`
	// PackageName defines the name of the application package
	PackageName string `json:"package_name"`
	// PackageVersion defines the version of the application package
	PackageVersion string `json:"package_version"`
	// OperationID identifies the application operation
	OperationID string `json:"operation_id"`
	// Created is a time when this entry was created
	Created time.Time `json:"created"`
	// Completion is a number from 0 (just started) to 100 (completed)
	Completion int `json:"completion"`
	// State is a string that indicates current operation state
	State string `json:"state"`
	// Message defines a text message describing the operation
	Message string `json:"message"`
}

func (a *AppProgressEntry) Check() error {
	if a.Repository == "" {
		return trace.BadParameter("missing parameter Repository")
	}
	if a.PackageName == "" {
		return trace.BadParameter("missing parameter PackageName")
	}
	if a.PackageVersion == "" {
		return trace.BadParameter("missing parameter PackageVersion")
	}
	if a.OperationID == "" {
		return trace.BadParameter("missing parameter OperationID")
	}
	return nil
}

// AppProgressEntries collection stores progress entries for the appication operations
type AppProgressEntries interface {
	// CreateAppProgressEntry adds a progress entry for the specified application
	CreateAppProgressEntry(p AppProgressEntry) (*AppProgressEntry, error)
	// GetLastAppProgressEntry queries the last progress entry for the specified application
	GetLastAppProgressEntry(operationID string) (*AppProgressEntry, error)
}

// ProgressEntry is a structured entry indicating operation progress
type ProgressEntry struct {
	// ID is auto generated ID
	ID string `json:"id"`
	// SiteDomain is a reference to existing site domain
	SiteDomain string `json:"site_domain"`
	// OperationID is id of the operation this progress entry refers to
	OperationID string `json:"operation_id"`
	// Created is a time when this entry was created
	Created time.Time `json:"created"`
	// Completion is a number from 0 (just started) to 100 (completed)
	Completion int `json:"completion"`
	// Step defines the current operation step as a value from a step matrix
	// Step matrix is a finite set of steps that comprise an operation
	Step int `json:"step"`
	// State is a string that indicates current operation state
	State string `json:"state"`
	// Message is a text message describing the operation
	Message string `json:"message"`
}

func (p *ProgressEntry) Check() error {
	if p.SiteDomain == "" {
		return trace.BadParameter("missing site domain")
	}
	if p.OperationID == "" {
		return trace.BadParameter("missing operation id")
	}
	if p.Created.IsZero() {
		return trace.BadParameter("missing created field")
	}
	return nil
}

// IsCompleted returns true if the progress entry is completed
func (p ProgressEntry) IsCompleted() bool {
	return p.Completion == constants.Completed
}

// IsEqual returns true if the progress entry is equal to the other entry
func (p ProgressEntry) IsEqual(other ProgressEntry) bool {
	return p.Completion == other.Completion && p.Message == other.Message
}

// ProgressEntries collection stores progress entries for the operations
type ProgressEntries interface {
	// CreateProgressEntry adds a progress entry for this site
	CreateProgressEntry(p ProgressEntry) (*ProgressEntry, error)
	// GetLastProgressEntry gets a progress entry for this site
	GetLastProgressEntry(siteDomain, operationID string) (*ProgressEntry, error)
}

// Package is any named and versioned blob with an optional manifest
type Package struct {
	// Repository is a package repository
	Repository string `json:"repository"`
	// Name is a full package name
	Name string `json:"name"`
	// Version is a package version in SemVer format
	Version string `json:"version"`
	// SHA512 is a sha512 hash of the data in storage
	SHA512 string `json:"checksum"`
	// SizePytes is a package size in bytes
	SizeBytes int `json:"size_bytes"`
	// Created is the time the package was created at
	Created time.Time `json:"created"`
	// CreatedBy is the email of a user who created the package
	CreatedBy string `json:"created_by"`
	// RuntimeLabels are optional key=value pairs metadata that
	// can be assigned to a package, they are not a part of
	// the package, and assigned at a run time,
	// they are useful for denoting packages currently installed
	// in the system
	RuntimeLabels map[string]string `json:"runtime_labels"`
	// Type defines the type of the package
	Type string `json:"type"`
	// Hidden defines the package visibility
	Hidden bool `json:"hidden"`
	// Encrypted indicates whether the package data is encrypted
	Encrypted bool `json:"encrypted"`
	// Manifest defines the application manifest for an application package
	Manifest []byte `json:"manifest"`
	// Base refers to the package this application is based on
	Base *Package `json:"base,omitempty"`
}

// Locator returns new locator from the package repository, name and version
func (p *Package) Locator() loc.Locator {
	return loc.Locator{
		Repository: p.Repository,
		Name:       p.Name,
		Version:    p.Version,
	}
}

// SetRuntimeLabel sets runtime label name and value for the package
func (p *Package) SetRuntimeLabel(name, val string) {
	if p.RuntimeLabels == nil {
		p.RuntimeLabels = map[string]string{name: val}
	} else {
		p.RuntimeLabels[name] = val
	}
}

func (p Package) String() string {
	return fmt.Sprintf("package(%v/%v:%v)", p.Repository, p.Name, p.Version)
}

func (p *Package) Check() error {
	if p.Repository == "" {
		return trace.BadParameter("%v missing repository name", p)
	}
	if p.Name == "" {
		return trace.BadParameter("%v missing package name", p)
	}
	if p.Version == "" {
		return trace.BadParameter("%v missing package version", p)
	}
	return nil
}

// Repositories interface provides operations on repositories and
// packages. Repository is a collection of packages - arbitrary blobs
// with metadata, name and version.
type Repositories interface {
	// Creates a repository - a collection of packages
	CreateRepository(r Repository) (Repository, error)

	// GetRepository returns a repository by a given name,
	// or NotFoundError if repository is not found
	GetRepository(name string) (Repository, error)

	// DeleteRepository deletes a repository and associated packages
	DeleteRepository(name string) error

	// GetRepositories returns list of repositories
	GetRepositories() ([]Repository, error)

	// CreatePackage creates a package in a repository, will return
	// error if a given package already exists
	CreatePackage(p Package) (*Package, error)

	// UpsertPackage creates or updates a package in a repository
	UpsertPackage(p Package) (*Package, error)

	// DeletePackage deletes a package from repository
	DeletePackage(repository string, packageName, packageVersion string) error

	// GetPackage returns a package by it's name and version a repository
	GetPackage(repository string, packageName, packageVersion string) (*Package, error)

	// GetPackages returns s list of packages in a repository, in case if
	// if prevName and prevVersion are not empty, returns packages greater
	// than given names and version in lexicographical order
	GetPackages(repository string) ([]Package, error)

	// UpdatePackageRuntimeLabels is an atomic operation that sets runtime labels
	// for a set of package, adding and removing labels in one atomic operation
	UpdatePackageRuntimeLabels(repository, packageName, packageVersion string, addLabels map[string]string, removeLabels []string) error
}

// Permission represent action that user can perform on objects
// in certain collections
// e.g. user can read packages from gravitational repository:
//
// <UserID: install-agent> has permission to <Action: read> packages to <Collection: repository> <CollectionID: gravitational>
//
// e.g. user can add new repositories
//
// <UserID: admin> has permission to <Action: create> repositories in <Collection: portal_repositories>
type Permission struct {
	// UserEmail this the user this rule refers to
	UserEmail string `json:"user_email"`

	// Action on object, one of create, read, delete
	Action string `json:"action"`

	// Collection is a collection this rule refers to e.g. "repository"
	Collection string `json:"collection"`

	// Collection ID, e.g. repository name, can be empty in case
	// if there is only one object
	CollectionID string `json:"collection_id"`
}

func (p *Permission) Check() error {
	if p.UserEmail == "" {
		return trace.BadParameter("missing email")
	}
	if p.Action == "" {
		return trace.BadParameter("missing action")
	}
	if p.Collection == "" {
		return trace.BadParameter("missing collection")
	}
	return nil
}

// RepositoriyAccessRules collection manages repository access rules -
// read, create, delete
type Permissions interface {
	CreatePermission(p Permission) (*Permission, error)
	GetPermission(p Permission) (*Permission, error)
	GetUserPermissions(email string) ([]Permission, error)
	DeletePermissionsForUser(email string) error
}

func (r Permission) String() string {
	return fmt.Sprintf(
		"permission(email=%v, action=%v, collection=%v:%v)",
		r.UserEmail, r.Action, r.Collection, r.CollectionID)
}

// LoginEntry represents local agent login with remote portal,
// used to pull and push packages
type LoginEntry struct {
	// Email is user email
	Email string `yaml:"email"`
	// Password is a password or token
	Password string `yaml:"token"`
	// OpsCenterURL is URL of the OpsCenter
	OpsCenterURL string `yaml:"opscenter"`
	// Expires is optional setting when this token/password expires
	Expires time.Time `yaml:"expires"`
	// AccountID is account id this user belongs to
	AccountID string `yaml:"account_id"`
	// Created is when the entry was created
	Created time.Time `yaml:"created"`
}

func (l *LoginEntry) Check() error {
	if l.Password == "" {
		return trace.BadParameter("missing parameter Password")
	}
	if l.OpsCenterURL == "" {
		return trace.BadParameter("missing parameter Gravity Hub URL")
	}
	return nil
}

// String returns the login entry string representation
func (l LoginEntry) String() string {
	return fmt.Sprintf("LoginEntry(Email=%v, GravityHub=%v, Created=%v)",
		l.Email, l.OpsCenterURL, l.Created.Format(constants.HumanDateFormat))
}

// LoginEntries store local agent logins with remote portals
type LoginEntries interface {
	UpsertLoginEntry(l LoginEntry) (*LoginEntry, error)
	GetLoginEntries() ([]LoginEntry, error)
	GetLoginEntry(opsCenterURL string) (*LoginEntry, error)
	DeleteLoginEntry(opsCenterURL string) error
	GetCurrentOpsCenter() string
	SetCurrentOpsCenter(string) error
}

// SystemMetadata stores system-relevant data on the host
type SystemMetadata interface {
	// GetDNSConfig returns current DNS configuration
	GetDNSConfig() (*DNSConfig, error)
	// SetDNSConfig sets current DNS configuration
	SetDNSConfig(DNSConfig) error
	// GetSELinux returns whether SELinux support is on
	GetSELinux() (enabled bool, err error)
	// SetSELinux sets SELinux support
	SetSELinux(enabled bool) error
}

// DefaultDNSConfig defines the default cluster local DNS configuration
var DefaultDNSConfig = DNSConfig{
	Port:  defaults.DNSPort,
	Addrs: []string{defaults.DNSListenAddr},
}

// LegacyDNSConfig defines the local DNS configuration on older clusters
var LegacyDNSConfig = DNSConfig{
	Port:  defaults.DNSPort,
	Addrs: []string{"127.0.0.1"},
}

// String returns textual representation of this DNS configuration
func (r DNSConfig) String() string {
	var addrs []string
	for _, addr := range r.Addrs {
		addrs = append(addrs, fmt.Sprintf("%v:%v", addr, r.Port))
	}
	return strings.Join(addrs, ",")
}

// Addr returns the DNS server address as ip:port.
// Requires that !r.IsEmpty.
func (r DNSConfig) Addr() string {
	return fmt.Sprintf("%v:%v", r.Addrs[0], r.Port)
}

// IsEmpty returns whether this configuration is empty
func (r DNSConfig) IsEmpty() bool {
	return len(r.Addrs) == 0
}

// DNSConfig describes a DNS server
type DNSConfig struct {
	// Addrs lists local cluster DNS server IP addresses
	Addrs []string `json:"addrs"`
	// Port specifies the DNS port to use for dns
	Port int `json:"port"`
}

// PackageChangeset is a set of package updates from one version to another
type PackageChangeset struct {
	ID string `json:"id"`
	// Changes is a list of package updates
	Changes []PackageUpdate `json:"changes"`
	// Created is the time when this update was created
	Created time.Time `json:"created"`
}

// String returns user-friendly representation of this update
func (u PackageChangeset) String() string {
	changes := make([]string, len(u.Changes))
	for i, c := range u.Changes {
		changes[i] = c.String()
	}
	return fmt.Sprintf("changeset(id=%v, created=%v, changes=%v)", u.ID, u.Created, strings.Join(changes, ", "))
}

// ReversedChangeset returns changeset with all changes inversed
func (u *PackageChangeset) ReversedChanges() []PackageUpdate {
	changes := make([]PackageUpdate, len(u.Changes))
	for i, c := range u.Changes {
		update := PackageUpdate{
			From:   c.To,
			To:     c.From,
			Labels: c.Labels,
		}
		if c.ConfigPackage != nil {
			update.ConfigPackage = &PackageUpdate{
				From:   c.ConfigPackage.To,
				To:     c.ConfigPackage.From,
				Labels: c.ConfigPackage.Labels,
			}
		}
		changes[i] = update
	}
	return changes
}

// Check checks the validity of this object
func (u *PackageChangeset) Check() error {
	if len(u.Changes) == 0 {
		return trace.BadParameter("missing parameter Changes")
	}
	return nil
}

// PackageUpdate represents package change from one version to another
type PackageUpdate struct {
	// From is currently installed version
	From loc.Locator `json:"from"`
	// To is the target version
	To loc.Locator `json:"to"`
	// Labels defines optional identifying set of labels
	Labels map[string]string `json:"labels,omitempty"`
	// ConfigPackage specifies optional configuration package dependency
	ConfigPackage *PackageUpdate `json:"config_package,omitempty"`
}

// String formats this update as human-readable text
func (u *PackageUpdate) String() string {
	format := func(u *PackageUpdate) string {
		return fmt.Sprintf("%v -> %v", u.From, u.To)
	}
	if u.ConfigPackage == nil {
		return fmt.Sprintf("update(%v)", format(u))
	}
	return fmt.Sprintf("update(%v, config:%v)",
		format(u), format(u.ConfigPackage))
}

// PackageChangesets tracks server local package changes - updates and downgrades
type PackageChangesets interface {
	// CreatePackageChangeset creates new changeset
	CreatePackageChangeset(u PackageChangeset) (*PackageChangeset, error)
	// GetPackageChangesets lists package changesets
	GetPackageChangesets() ([]PackageChangeset, error)
	// GetPackageChangeset returns update by id
	GetPackageChangeset(id string) (*PackageChangeset, error)
}

// ProvisioningToken is used to add new servers to the cluster
type ProvisioningToken struct {
	// Token is a unique randomly generated token
	Token string `json:"token"`
	// Expires sets the token expiry time, zero time if never expires
	Expires time.Time `json:"expires"`
	// Type is token type - 'install' or 'expand'
	Type ProvisioningTokenType `json:"type"`
	// AccountID is the account this signup token
	// is associated with in case if that's user signup token
	AccountID string `json:"account_id"`
	// SiteDomain is the site this token is associated with
	SiteDomain string `json:"site_domain"`
	// OperationID is the id of the operation (install or expand)
	OperationID string `json:"operation_id"`
	// UserEmail links this token to the user with permissions,
	// usually it's a site agent user
	UserEmail string `json:"user_email"`
}

func (p *ProvisioningToken) Check() error {
	if p.Token == "" {
		return trace.BadParameter("missing Token")
	}
	if err := p.Type.Check(); err != nil {
		return trace.Wrap(err)
	}
	if p.AccountID == "" {
		return trace.BadParameter("missing AccountID")
	}
	if p.SiteDomain == "" {
		return trace.BadParameter("missing SiteDomain")
	}
	return nil
}

// IsExpand returns true if this is an expand token.
func (p *ProvisioningToken) IsExpand() bool {
	return p.Type == ProvisioningTokenTypeExpand
}

// IsTeleport returns true if this is a teleport token.
func (p *ProvisioningToken) IsTeleport() bool {
	return p.Type == ProvisioningTokenTypeTeleport
}

// IsPersistent returns true if this token does not expire.
func (p *ProvisioningToken) IsPersistent() bool {
	return p.Expires.IsZero()
}

// ProvisioningTokenType specifies token type
type ProvisioningTokenType string

const (
	// ProvisioningTokenTypeInstall is cluster agent token
	ProvisioningTokenTypeInstall = "install"
	// ProvisioningTokenTypeExpand is used to validate joining nodes
	ProvisioningTokenTypeExpand = "expand"
	// ProvisioningTokenTypeTeleport is used by Teleport nodes to authenticate with auth server
	ProvisioningTokenTypeTeleport = "teleport"
)

// Check returns nil if the value is correct, error otherwise
func (s *ProvisioningTokenType) Check() error {
	switch *s {
	case ProvisioningTokenTypeInstall, ProvisioningTokenTypeExpand, ProvisioningTokenTypeTeleport:
		return nil
	}
	return trace.BadParameter("unsupported token type: %v", *s)
}

// Tokens interface defines a token management layer.
// Token types include those for adding new servers to the cluster during install or expand operations
// or running one-time installations.
type Tokens interface {
	// CreateProvisioningToken creates a temporary authentication token
	CreateProvisioningToken(t ProvisioningToken) (*ProvisioningToken, error)
	// DeleteProvisioningToken deletes a token specified by token
	DeleteProvisioningToken(token string) error
	// GetProvisioningToken returns a token if it has not expired yet
	GetProvisioningToken(token string) (*ProvisioningToken, error)
	// GetOperationProvisioningToken returns an existing token for the particular operation if
	// it has not expired yet
	GetOperationProvisioningToken(clusterName, operationID string) (*ProvisioningToken, error)
	// GetSiteProvisioningTokens returns a list of tokens for the site specified with siteDomain
	// that have not expired yet
	GetSiteProvisioningTokens(siteDomain string) ([]ProvisioningToken, error)
	// CreateInstallToken creates a token for a one-time install operation
	CreateInstallToken(InstallToken) (*InstallToken, error)
	// GetInstallToken returns an active install token with the specified ID
	GetInstallToken(token string) (*InstallToken, error)
	// GetInstallTokenByUser returns an active install token with the specified user ID
	GetInstallTokenByUser(email string) (*InstallToken, error)
	// GetInstallTokenForCluster returns an active install token for the specified cluster
	GetInstallTokenForCluster(name string) (*InstallToken, error)
	// UpdateInstallToken updates the specified install token
	UpdateInstallToken(InstallToken) (*InstallToken, error)
}

// InstallToken defines a one-time installation token
type InstallToken struct {
	// Token is a unique randomly generated character sequence
	Token string `json:"token"`
	// Expires sets the token expiry time, zero time if never expires
	Expires time.Time `json:"expires"`
	// AccountID is the account this signup token
	// is associated with in case if that's user signup token
	AccountID string `json:"account_id"`
	// SiteDomain defines a site this token will be associated with
	// once the installation has started
	SiteDomain string `json:"site_domain"`
	// Application defines the application package this token is bound to.
	// Only set for one-time installations
	Application *loc.Locator `json:"application,omitempty"`
	// UserEmail links this token to a user with permissions to execute a one-time
	// installation of a specific site
	UserEmail string `json:"user_email"`
	// UserType defines the type of user to create and associate with this token
	UserType string `json:"type"`
}

func (p *InstallToken) Check() error {
	if p.Token == "" {
		return trace.BadParameter("missing token")
	}
	if p.AccountID == "" {
		return trace.BadParameter("missing account id")
	}
	if p.UserType == AgentUser && p.Application == nil {
		return trace.BadParameter("missing application package")
	}
	return nil
}

// Peer is a peer node of the package management service
type Peer struct {
	ID            string    `json:"id"`
	AdvertiseAddr string    `json:"advertise_addr"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

func (p Peer) String() string {
	return fmt.Sprintf("peer(%v, %v, %v)", p.ID, p.AdvertiseAddr, p.LastHeartbeat)
}

func (p *Peer) Check() error {
	if p.ID == "" {
		return trace.BadParameter("missing parameter ID")
	}
	if p.AdvertiseAddr == "" {
		return trace.BadParameter("missing parameter AdvertiseAddr")
	}
	return nil
}

type Peers interface {
	GetPeers() ([]Peer, error)
	UpsertPeer(p Peer) error
	DeletePeer(id string) error
}

// Objects stores binary objects metadata
type Objects interface {
	GetObjects() ([]string, error)
	UpsertObjectPeers(hash string, peers []string, expires time.Duration) error
	GetObjectPeers(hash string) ([]string, error)
	DeleteObjectPeers(hash string, peers []string) error
	DeleteObject(hash string) error
}

const (
	// NodeTypeNode is a type of teleport node - SSH Node
	NodeTypeNode = "node"
	// NodeTypeNode is a type of teleport node - SSH Proxy server
	NodeTypeProxy = "proxy"
	// NodeTypeAuth is a type of teleport node - SSH Auth server
	NodeTypeAuth = "auth"
)

const (
	// OpsCenterRemoteAccessLink is a link used to provide remote access via Teleport
	OpsCenterRemoteAccessLink = "remote_access"
	// OpsCenterUpdateLink is a link to fetch periodic updates
	OpsCenterUpdateLink = "update"
)

// OpsCenterLink is a link between remote OpsCenter and a local site
type OpsCenterLink struct {
	// SiteDomain is the domain name of the site
	SiteDomain string `json:"site_domain"`
	// Hostname is OpsCenter hostname we are connected to
	Hostname string `json:"hostname"`
	// Type is a link type (e.g. updates, remote_access)
	Type string `json:"type"`
	// RemoteAddr is a remote address used for updates or remote access
	RemoteAddr string `json:"remote_address"`
	// APIURL is a URL of remote ops center
	APIURL string `json:"api_url"`
	// Enabled is whether this link is enabled
	Enabled bool `json:"enabled"`
	// User defines an optional user context to use for remote access
	User *RemoteAccessUser `json:"user"`
	// Wizard indicates whether this is a link to a wizard
	Wizard bool `json:"wizard"`
}

// Check checks if OpsCenter link parameters are correct
func (l *OpsCenterLink) Check() error {
	if l.SiteDomain == "" {
		return trace.BadParameter("missing parameter SiteDomain")
	}
	if l.Hostname == "" {
		return trace.BadParameter("missing parameter Hostname")
	}
	if l.Type == "" {
		return trace.BadParameter("missing parameter Type")
	}
	if l.Type != OpsCenterRemoteAccessLink && l.Type != OpsCenterUpdateLink {
		return trace.BadParameter("link type should be either %v or %v, got: %v",
			OpsCenterRemoteAccessLink, OpsCenterRemoteAccessLink, l.Type)
	}
	if l.RemoteAddr == "" {
		return trace.BadParameter("missing parameter RemoteAddr")
	}
	if l.APIURL == "" {
		return trace.BadParameter("missing parameter APIURL")
	}
	if _, err := teleutils.ParseAddr(l.RemoteAddr); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Links is a legacy an interface for remote Ops Center links which have been
// superseded by trusted clusters, kept only for migration purposes
type Links interface {
	// UpsertOpsCenterLink updates or creates new OpsCenter link
	UpsertOpsCenterLink(l OpsCenterLink, ttl time.Duration) (*OpsCenterLink, error)
	// GetOpsCenterLinks returns a list of OpsCenter links
	GetOpsCenterLinks(siteDomain string) ([]OpsCenterLink, error)
}

// Check validates this object
func (r *RemoteAccessUser) Check() error {
	if r.SiteDomain == "" {
		return trace.BadParameter("Cluster name is required")
	}
	return nil
}

// RemoteAccessUser groups the attributes to identify or create a user to use
// to connect a cluster to a remote OpsCenter
type RemoteAccessUser struct {
	// Email identifies the user
	Email string `json:"email"`
	// Token identifies the API key for this user
	Token string `json:"token"`
	// SiteDomain identifies the cluster this user represents
	SiteDomain string `json:"site_domain"`
	// OpsCenter defines the OpsCenter on the other side
	OpsCenter string `json:"ops_center"`
}

// Locks is the locking service
type Locks interface {
	// AcquireLock grabs a lock that will be released automatically in ttl time
	// blocks until lock is available
	AcquireLock(token string, ttl time.Duration) error

	// TryAcquireLock grabs a lock that will be released automatically in ttl time
	// tries once and either succeeds right away or fails
	TryAcquireLock(token string, ttl time.Duration) error

	// ReleaseLock releases lock by token name
	ReleaseLock(token string) error
}

// LegacyRoles is used in testing
type LegacyRoles interface {
	// UpsertV1Role creates or updates V2 role
	// used for migration purposes
	UpsertV2Role(role RoleV2) error
}

// Backend is a combination of all collections
// and a couple of common methods like Closer
type Backend interface {
	io.Closer
	clockwork.Clock
	teleservices.Trust
	teleservices.Presence
	teleservices.Access
	ClusterConfiguration
	U2F
	Locks
	WebSessions
	UserTokens
	Tokens
	UserInvites
	Applications
	AppOperations
	AppProgressEntries
	Users
	APIKeys
	Connectors
	Accounts
	Sites
	SiteOperations
	ProgressEntries
	Repositories
	Permissions
	LoginEntries
	Migrations
	Peers
	Objects
	PackageChangesets
	Links
	ClusterImport
	LegacyRoles
	SystemMetadata
	Charts
}

const (
	// MaxLimit sets maximum pagination limit
	MaxLimit = 1000
	// Forever indicates to store value forever
	Forever = 0
)

// AppType defines an application type
type AppType string

// Check makes sure app type is valid
func (t AppType) Check() error {
	if string(t) == "" {
		return nil
	}
	switch t {
	case AppUser, AppService, AppRuntime:
		return nil
	default:
		return trace.BadParameter("invalid app type %q", t)
	}
}

const (
	// AppUser defines a type for user apps
	//
	// User apps are the ones that a user builds, publishes into
	// OpsCenters and installs (e.g. mattermost). These are the
	// only apps that are visible in OpsCenter by default.
	AppUser AppType = "user"

	// AppService defines a type for service apps
	//
	// Service apps are "building blocks" that cannot be installed
	// separately from a user app but provide essential services to
	// user apps that take dependency on them (e.g. dns, logging).
	AppService AppType = "service"

	// AppRuntime defines a type for runtime apps
	//
	// Runtime apps serve as a backbone for user apps, they are the
	// lowest-level base for any application (e.g. kubernetes of a
	// certain version).
	AppRuntime AppType = "runtime"
)

// Server is used during site install process
// and is configured by users during manual install or
// by automatic provisioner when creating environment from scratch
type Server struct {
	// AdvertiseIP is the IP that will be used for inter host communication
	AdvertiseIP string `json:"advertise_ip"`
	// Hostname is the server hostname
	Hostname string `json:"hostname"`
	// Nodename as assigned by the cloud provider (if any).
	// In case of Amazon private DNS zone, this will be the `PrivateDnsName`
	Nodename string `json:"nodename"`
	// Role is application specific role, e.g. "database"
	Role string `json:"role"`
	// InstanceType is provisioned instance type
	InstanceType string `json:"instance_type"`
	// InstanceID is cloud specific instance ID
	InstanceID string `json:"instance_id"`
	// ClusterRole is the node's system role, "master" or "node"
	ClusterRole string `json:"cluster_role"`
	// Provisioner is the provisioner the server was provisioned with
	Provisioner string `json:"provisioner"`
	// OSInfo identifies the host operating system
	OSInfo OSInfo `json:"os"`
	// Mounts lists mount configurations for a server profile instance
	Mounts []Mount `json:"mounts"`
	// SystemState defines the system configuration for gravity - location
	// of state directory, etc.
	SystemState SystemState `json:"system_state"`
	// Docker defines docker-specific configuration parameters
	// For example, it specifies which disk/partition to use for devicemapper
	// direct-lvm configuration
	Docker Docker `json:"docker"`
	// User is current OS user information
	User OSUser `json:"user"`
	// Created is the timestamp when the server was created
	Created time.Time `json:"created"`
	// SELinux specifies whether the node has SELinux support on
	SELinux bool `json:"selinux,omitempty"`
}

// IsEqualTo returns true if this and the provided server are the same server.
func (s *Server) IsEqualTo(other Server) bool {
	// Compare only a few "main" fields that should give enough confidence
	// in deciding whether it's the same node or not.
	return s.AdvertiseIP == other.AdvertiseIP &&
		s.Hostname == other.Hostname &&
		s.Role == other.Role
}

// StateDir returns directory where all gravity data is stored on this server
func (s *Server) StateDir() string {
	if s.SystemState.StateDir != "" {
		return s.SystemState.StateDir
	}
	return defaults.GravityDir
}

// KubeNodeID returns the identity of the node within the kubernetes cluster (kubectl get node)
// when running on a cloud environment such as AWS, kubelet tends to pick up it's hostname from the cloud provider API.
// So when running on these environments, we should ensure our hostnames match what kubernetes will be doing.
// When not running on a cloud environment with this behaviour, we will identify nodes by their Advertise IP address
// More Information:
// https://github.com/kubernetes/kubernetes/pull/58114#pullrequestreview-88022039
// https://github.com/kubernetes/kubernetes/issues/54482
// https://github.com/kubernetes/kubernetes/issues/58084
func (s *Server) KubeNodeID() string {
	if s.Nodename != "" {
		return s.Nodename
	}
	return s.AdvertiseIP
}

// ObjectPeerID returns the peer ID of this server
func (s *Server) ObjectPeerID() string {
	return s.AdvertiseIP
}

// EtcdPeerURL returns etcd peer advertise URL with the server's IP.
func (s *Server) EtcdPeerURL() string {
	return fmt.Sprintf("https://%v:%v", s.AdvertiseIP, defaults.EtcdPeerPort)
}

// IsMaster returns true if the server has a master role
func (s *Server) IsMaster() bool {
	return s.ClusterRole == string(schema.ServiceRoleMaster)
}

// GetNodeLabels returns a consistent set of labels that should be applied to the node
func (s *Server) GetNodeLabels(profileLabels map[string]string) map[string]string {
	labels := map[string]string{
		defaults.KubernetesAdvertiseIPLabel:            s.AdvertiseIP,
		defaults.KubernetesRoleLabel:                   s.ClusterRole,
		v1.LabelHostname:                               s.KubeNodeID(),
		v1.LabelArchStable:                             "amd64", // Only amd64 is currently supported
		v1.LabelOSStable:                               "linux", // Only linux is currently supported
		defaults.FormatKubernetesNodeRoleLabel(s.Role): s.Role,
	}
	for k, v := range profileLabels {
		// Several of the labels applied by default are used internally within gravity or gravity components.
		// allowing a user to override these labels via the profile creates some risk, that they may overwrite a node
		// label gravity uses itself. As such, for now, only apply the profile labels if they do not conflict with a
		// default label.
		if _, ok := labels[k]; !ok {
			labels[k] = v
		}
	}
	return labels
}

// GetKubeletLabels returns the node's labels that can be set by kubelet.
func (s *Server) GetKubeletLabels(profileLabels map[string]string) map[string]string {
	allLabels := s.GetNodeLabels(profileLabels)
	result := make(map[string]string)
	for key, val := range allLabels {
		if utils.IsKubernetesLabel(key) {
			if kubeletapis.IsKubeletLabel(key) {
				result[key] = val
			}
		} else {
			result[key] = val
		}
	}
	return result
}

// Fields returns log fields describing the server.
func (s *Server) Fields() logrus.Fields {
	return logrus.Fields{"hostname": s.Hostname, "ip": s.AdvertiseIP}
}

// Strings formats this server as readable text
func (s Server) String() string {
	return fmt.Sprintf("Server(AdvertiseIP=%v, Hostname=%v, Role=%v, ClusterRole=%v)",
		s.AdvertiseIP,
		s.Hostname,
		s.Role,
		s.ClusterRole)
}

// Hostnames returns a list of hostnames for the provided servers
func Hostnames(servers []Server) (hostnames []string) {
	for _, server := range servers {
		hostnames = append(hostnames, server.Hostname)
	}
	return hostnames
}

// DNSOverrides defines a cluster's DNS host/zone overrides
type DNSOverrides struct {
	// Hosts maps a hostname to an IP address it will resolve to
	Hosts map[string]string `json:"hosts"`
	// Zones maps a DNS zone to nameservers it will be served by
	Zones map[string][]string `json:"zones"`
}

// FormatHosts formats host overrides to a string
func (d DNSOverrides) FormatHosts() string {
	var overrides []string
	for hostname, ip := range d.Hosts {
		overrides = append(overrides, fmt.Sprintf("%v/%v", hostname, ip))
	}
	return strings.Join(overrides, ",")
}

// FormatZones formats zone overrides to a string
func (d DNSOverrides) FormatZones() string {
	var overrides []string
	for zone, nameservers := range d.Zones {
		for _, ns := range nameservers {
			overrides = append(overrides, fmt.Sprintf("%v/%v", zone, ns))
		}
	}
	return strings.Join(overrides, ",")
}

// SystemState defines the system configuration for gravity - location
// of state directory, etc.
type SystemState struct {
	// Disk defines the block device (disk or partition) to use
	// for gravity system state directory
	Device Device `json:"device"`
	// StateDir is where all gravity data is stored on the server
	StateDir string `json:"state_dir"`
}

// Devices defines a list of devices
type Devices []Device

// GetByName looks up a device by name
func (r Devices) GetByName(name DeviceName) Device {
	for _, device := range r {
		if device.Path() == name.Path() {
			return device
		}
	}
	return Device{}
}

// Device defines a device on a host: block device or a partition
type Device struct {
	// Name identifies the device
	Name DeviceName `json:"name"`
	// Type defines the type of device: disk or partition
	Type DeviceType `json:"type"`
	// SizeMB of the device in MB
	SizeMB uint64 `json:"size_mb"`
}

// UnmarshalJSON interpets input as either a Device or a device name (backwards-compatibility)
func (r *Device) UnmarshalJSON(p []byte) error {
	type serializableDevice Device
	var device serializableDevice
	if err := json.Unmarshal(p, &device); err != nil {
		// Backwards compatibility - parse as a device name
		return json.Unmarshal(p, &r.Name)
	}
	*r = Device(device)
	return nil
}

// MarshalJSON serializes this device as text
func (r Device) MarshalJSON() ([]byte, error) {
	type serializableDevice Device
	device := serializableDevice(r)
	return json.Marshal(&device)
}

// Path returns the absolute path to the device node in /dev
func (r Device) Path() string { return r.Name.Path() }

// DeviceType defines a device type
type DeviceType string

const (
	// DeviceDisk defines a block device
	DeviceDisk DeviceType = "disk"
	// DevicePartition defines a partition on a device
	DevicePartition DeviceType = "part"
)

// DeviceName identifies a device by name
type DeviceName string

// Path builds the device node path (in /dev)
func (r DeviceName) Path() string {
	if len(r) > 0 && !strings.HasPrefix(string(r), "/dev") {
		return filepath.Join("/dev", string(r))
	}
	return string(r)
}

// MarshalText formats device as text with full path
func (r DeviceName) MarshalText() ([]byte, error) {
	return []byte(r.Path()), nil
}

// UnmarshalText reads device name from text
func (r *DeviceName) UnmarshalText(p []byte) error {
	*r = DeviceName(p)
	return nil
}

// Migrations defines an interface to schema migration management
type Migrations interface {
	// SchemaVersion returns the version of the schema
	SchemaVersion() (int, error)
}

// Leader describes a leader election campaign
type Leader interface {
	// AddWatch starts watching the key for changes and sending them
	// to the valuesC channel.
	AddWatch(key string, retry time.Duration, valuesC chan string)

	// AddVoter adds a new voter.
	// The voter will participate in the election until paused with StepDown
	// The voter can be cancelled via the specified context.
	AddVoter(ctx context.Context, key, value string, term time.Duration) error

	// StepDown instructs the voter to pause election and give up its leadership
	StepDown()
}

// InstallExpandOperationState defines the state of an install or expand operation
type InstallExpandOperationState struct {
	// Profiles contains certain details about servers provisioned during
	// the operation, e.g. roles, counts, instance types
	Profiles map[string]ServerProfile `json:"profiles"`
	// Servers defines (user-affected) configuration of each active server
	// instance
	Servers Servers `json:"servers"`
	// Agents defines the list of agent attributes (like download instructions,
	// etc.) to use on the client
	Agents map[string]AgentProfile `json:"agents"`
	// Subnets describes selected overlay/service network subnets for this
	// operation
	Subnets Subnets `json:"subnets"`
	// Vars is a set of variables specific to this operation, e.g. AWS
	// credentials or region
	Vars OperationVariables `json:"vars"`
	// Package is the application being installed
	Package loc.Locator `json:"package"`
}

// OperationVariables is operation-specific set of variables
type OperationVariables struct {
	// System is a set of variables common for each provider
	System SystemVariables `json:"system"`
	// OnPrem is a set of onprem-specific variables
	OnPrem OnPremVariables `json:"onprem"`
	// AWS is a set of AWS-specific variables
	AWS AWSVariables `json:"aws"`
	// Values are helm values in a marshaled yaml format
	Values []byte `json:"values,omitempty"`
}

// ToMap converts operation variables into a JSON object for easier use in templates
func (v OperationVariables) ToMap() (map[string]interface{}, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var object map[string]interface{}
	err = json.Unmarshal(bytes, &object)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return object, nil
}

// SystemVariables represents a set of operation variables common for each provider
type SystemVariables struct {
	// ClusterName is the name of the cluster the operation is for
	ClusterName string `json:"cluster_name"`
	// OpsURL is remote Ops Center URL
	OpsURL string `json:"ops_url"`
	// Devmode is whether the operation is running in dev mode
	Devmode bool `json:"devmode"`
	// Token is the agent token
	Token string `json:"token"`
	// TeleportProxyAddress is the address of teleport proxy
	TeleportProxyAddress string `json:"teleport_proxy_address"`
	// Docker overrides configuration from the manifest
	Docker DockerConfig `json:"docker"`
}

// IsEmpty returns whether this configuration is empty
func (r DockerConfig) IsEmpty() bool {
	return r.StorageDriver == "" && len(r.Args) == 0
}

// DockerConfig overrides Docker configuration for the cluster
type DockerConfig struct {
	// StorageDriver specifies a storage driver to use
	StorageDriver string `json:"storage_driver,omitempty"`
	// Args specifies additional options to the docker daemon
	Args []string `json:"args,omitempty"`
}

// Check makes sure the docker config is correct
func (d DockerConfig) Check() error {
	if d.StorageDriver != "" && !utils.StringInSlice(constants.DockerSupportedDrivers, d.StorageDriver) {
		return trace.BadParameter("unrecognized docker storage driver %q, supported are: %v",
			d.StorageDriver, constants.DockerSupportedDrivers)
	}
	return nil
}

// OnPremVariables is a set of operation variables specific to onprem provider
type OnPremVariables struct {
	// PodCIDR specifies the network range for pods
	PodCIDR string `json:"pod_cidr"`
	// ServiceCIDR specifies the network range for services
	ServiceCIDR string `json:"service_cidr"`
	// VxlanPort is the overlay network port
	VxlanPort int `json:"vxlan_port"`
}

// AWSVariables is a set of operation variables specific to AWS provider
type AWSVariables struct {
	// AMI is the Amazon Machine Image name
	AMI string `json:"ami"`
	// Region is the AWS region
	Region string `json:"region"`
	// AccessKey is the AWS API access key
	AccessKey string `json:"access_key"`
	// SecretKey is the AWS API secret key
	SecretKey string `json:"secret_key"`
	// SessionToken is the AWS API session token
	SessionToken string `json:"session_token"`
	// VPCID is the AWS VPC ID
	VPCID string `json:"vpc_id"`
	// VPCCIDR is the AWS VPC CIDR
	VPCCIDR string `json:"vpc_cidr"`
	// SubnetID is the AWS subnet ID
	SubnetID string `json:"subnet_id"`
	// SubnetCIDR is the AWS subnet CIDR
	SubnetCIDR string `json:"subnet_cidr"`
	// InternetGatewayID is the AWS internet gateway ID
	InternetGatewayID string `json:"igw_id"`
	// KeyPair is the AWS key pair name
	KeyPair string `json:"key_pair"`
}

// SetDefaults fills in some unset fiels with their default values if they have them
func (v *AWSVariables) SetDefaults() {
	if v.Region == "" {
		v.Region = defaults.AWSRegion
	}
	if v.VPCCIDR == "" {
		v.VPCCIDR = defaults.AWSVPCCIDR
	}
	if v.SubnetCIDR == "" {
		v.SubnetCIDR = defaults.AWSSubnetCIDR
	}
}

// ServerProfile describes server that was provisioned during install/expand
type ServerProfile struct {
	// Description is the server description
	Description string `json:"description"`
	// Labels is the server labels
	Labels map[string]string `json:"labels"`
	// ServiceRole is the server role (e.g. "master" or "node")
	ServiceRole string `json:"service_role"`
	// Request contains instance type and count that were provisioned
	Request ServerProfileRequest `json:"request"`
}

// ServerProfileRequest contains information about how many nodes of a certain type were
// requested for install/expand
type ServerProfileRequest struct {
	// InstanceType is the instance type to provision
	InstanceType string `json:"instance_type"`
	// Count is the number of servers to provision
	Count int `json:"count"`
}

// Subnets describes selected overlay/service network subnets for an operation
type Subnets struct {
	// Overlay is the Kubernetes overlay network (flannel) subnet
	Overlay string `json:"overlay"`
	// Service is the subnet for Kubernetes services
	Service string `json:"service"`
}

// IsEmpty determines if this subnet descriptor is empty
func (r Subnets) IsEmpty() bool {
	return r.Overlay == "" && r.Service == ""
}

// DefaultSubnets defines a default Subnets descriptor to use for onprem installations
var DefaultSubnets = Subnets{
	Overlay: defaults.PodSubnet,
	Service: defaults.ServiceSubnet,
}

// Servers is a list of servers
type Servers []Server

// Profiles returns a map of node profiles for these servers.
func (r Servers) Profiles() map[string]string {
	result := make(map[string]string, len(r))
	for _, server := range r {
		result[server.AdvertiseIP] = server.Role
	}
	return result
}

// IsEqualTo returns true if the provided list contains all the same servers
// as this list.
func (r Servers) IsEqualTo(other Servers) bool {
	if len(r) != len(other) {
		return false
	}
	for _, server := range r {
		otherServer := other.FindByIP(server.AdvertiseIP)
		if otherServer == nil {
			return false
		}
		if !otherServer.IsEqualTo(server) {
			return false
		}
	}
	return true
}

// FindByIP returns a server with the specified IP
func (r Servers) FindByIP(ip string) *Server {
	for _, server := range r {
		if server.AdvertiseIP == ip {
			return &server
		}
	}
	return nil
}

// Masters returns a list of master nodes
func (r Servers) Masters() (masters []Server) {
	for _, server := range r {
		if server.IsMaster() {
			masters = append(masters, server)
		}
	}
	return
}

// MasterIPs returns a list of advertise IPs of master nodes.
func (r Servers) MasterIPs() (ips []string) {
	for _, master := range r.Masters() {
		ips = append(ips, master.AdvertiseIP)
	}
	return ips
}

// String formats this list of servers as text
func (r Servers) String() string {
	var formats []string
	for _, server := range r {
		formats = append(formats, server.String())
	}
	return strings.Join(formats, ", ")
}

type AgentProfile struct {
	// Instructions defines the set of shell commands to download and start an agent
	// on a host
	Instructions string `json:"instructions"`
	// AgentURL is connection string for install agent
	AgentURL string `json:"agent_url"`
	// Token is the token used to connect to the agent server
	Token string `json:"token"`
}

// ShrinkOperationState contains information about shrink operation
type ShrinkOperationState struct {
	// Vars is a set of variables for this operation
	Vars OperationVariables `json:"vars"`
	// LegacyHostnames is used during migrations,
	// find a way to get rid of it
	LegacyHostnames []string `json:"servers"`
	// Servers is a list of servers to remove
	Servers []Server `json:"server_specs"`
	// Force controls whether the operation ignores intermediate errors
	Force bool `json:"force"`
	// NodeRemoved indicates whether the node has already been removed from the cluster
	// Used in cases where we recieve an event where the node is being terminated, but may
	// not have disconnected from the cluster yet.
	NodeRemoved bool `json:"node_removed"`
}

// UpdateOperationState describes the state of the update operation.
type UpdateOperationState struct {
	// UpdatePackage references the application package to update to
	UpdatePackage string `json:"update_package"`
	// ChangesetID is id of the package changeset used by this operation
	ChangesetID string `json:"changeset_id,omitempty"`
	// UpdateServiceName is a name of systemd service performing update
	UpdateServiceName string `json:"update_service_name,omitempty"`
	// RollbackServiceName is a name of systemd service performing rollback
	RollbackServiceName string `json:"rollback_service_name,omitempty"`
	// ServerUpdates contains servers and their update state
	ServerUpdates []ServerUpdate `json:"server_updates,omitempty"`
	// Manual specifies whether this update operation was created in manual mode
	Manual bool `json:"manual"`
	// Vars are variables specific to this operation
	Vars OperationVariables `json:"vars"`
}

// UpdateEnvarsOperationState describes the state of the operation to update cluster environment variables.
type UpdateEnvarsOperationState struct {
	// PrevEnv specifies the previous environment state
	PrevEnv map[string]string `json:"prev_env,omitempty"`
	// Env defines new cluster environment variables
	Env map[string]string `json:"env,omitempty"`
}

// Package returns the update package locator
func (s UpdateOperationState) Package() (*loc.Locator, error) {
	locator, err := loc.ParseLocator(s.UpdatePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return locator, nil
}

// UpdateConfigOperationState describes the state of the operation to update cluster configuration
type UpdateConfigOperationState struct {
	// PrevConfig specifies the previous configuration state
	PrevConfig []byte `json:"prev_config,omitempty"`
	// Config specifies the raw configuration resource
	Config []byte `json:"config,omitempty"`
}

// ReconfigureOperationState defines the reconfiguration operation state.
type ReconfigureOperationState struct {
	// AdvertiseAddr is the advertise address the node's being changed to.
	AdvertiseAddr string `json:"advertise_addr"`
}

// ServerUpdate represents server that is being updated
type ServerUpdate struct {
	// Server is a server being updated
	Server teleservices.ServerV1 `json:"server"`
	// State defines the state of server update operation
	// (e.g. started, in-progress or completed/failed)
	State string `json:"state"`
}

// String returns debug-friendly representation of the server udpate
func (s *ServerUpdate) String() string {
	return fmt.Sprintf("serverUpdate(%v, %v)", s.Server.ID, s.State)
}

const (
	ServerUpdateStart              = ""
	ServerUpdateSuccess            = "update_success"
	ServerUpdateInProgress         = "update_in_progress"
	ServerUpdateRollbackInProgress = "rollback_in_progress"
	ServerUpdateRollbackSuccess    = "rollback_success"
	ServerUpdateFailed             = "failed"
)

type UninstallOperationState struct {
	// Force enforces uninstall even if application uninstall failed
	Force bool `json:"force"`
	// Vars is standard operation variables set
	Vars OperationVariables `json:"vars"`
}

// ClusterImport defines the interface to manage status of cluster state import
type ClusterImport interface {
	// GetClusterImportStatus returns the state of cluster state import - e.g. whether it has
	// already been done
	GetClusterImportStatus() (bool, error)
	// SetClusterImported marks cluster import as complete.
	// After cluster import has completed, no other site instance will attempt
	// to import the state
	SetClusterImported() error
}

// ClusterConfiguration stores the cluster configuration in the DB.
type ClusterConfiguration interface {
	// SetClusterName gets services.ClusterName
	GetClusterName() (teleservices.ClusterName, error)
	// CreateClusterName creates teleservices.ClusterName
	CreateClusterName(teleservices.ClusterName) error
	// GetStaticTokens gets teleservices.StaticTokens
	GetStaticTokens() (teleservices.StaticTokens, error)
	// UpsertStaticTokens upserts teleservices.StaticToken
	UpsertStaticTokens(teleservices.StaticTokens) error
	// GetAuthPreference gets services.AuthPreference
	GetAuthPreference() (teleservices.AuthPreference, error)
	// UpsertAuthPreference upserts teleservices.AuthPreference
	UpsertAuthPreference(teleservices.AuthPreference) error
	// GetClusterConfig gets services.ClusterConfig
	GetClusterConfig() (teleservices.ClusterConfig, error)
	// UpsertClusterConfig upserts teeleservices.ClusterConfig
	UpsertClusterConfig(teleservices.ClusterConfig) error
}

// CloudConfig represents additional cloud provider-specific configuration
type CloudConfig struct {
	// GCENodeTags lists additional node tags on GCE
	GCENodeTags []string `json:"gce_node_tags,omitempty"`
}

// Charts defines methods related to Helm chart repository functionality.
type Charts interface {
	// GetIndexFile returns the chart repository index file.
	GetIndexFile() (*repo.IndexFile, error)
	// CompareAndSwapIndexFile updates the chart repository index file.
	CompareAndSwapIndexFile(new, existing *repo.IndexFile) error
	// UpsertIndexFile creates or replaces chart repository index file.
	UpsertIndexFile(repo.IndexFile) error
}
